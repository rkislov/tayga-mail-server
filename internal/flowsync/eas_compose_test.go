// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func composeSource(t *testing.T, ctx context.Context, h *easHandler, u *storage.User, raw []byte) *storage.Message {
	t.Helper()
	path, err := h.ms.EnsureFolder(u.Email, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := h.store.EnsureMailbox(ctx, u.ID, "INBOX", path)
	if err != nil {
		t.Fatal(err)
	}
	rel, size, err := h.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := h.store.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size, InternalDate: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func composeRequest(t *testing.T, h *easHandler, u *storage.User, command, id, options string, mime []byte) (*httptest.ResponseRecorder, *requestTrace) {
	t.Helper()
	xml := fmt.Sprintf(`<%s xmlns="ComposeMail:"><ClientId>%s</ClientId>%s<Mime>%s</Mime></%s>`,
		command, id, options, base64.StdEncoding.EncodeToString(mime), command)
	wire, err := encodeWire(xml)
	if err != nil {
		t.Fatal(err)
	}
	trace := &requestTrace{}
	r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd="+command+"&DeviceId=iphone&DeviceType=iPhone", bytes.NewReader(wire))
	r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
	r.Header.Set("MS-ASProtocolVersion", "16.1")
	r = r.WithContext(context.WithValue(withUser(r.Context(), u), traceKey{}, trace))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w, trace
}

func composeErrorStatus(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	decoded, err := decodeWire(w.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return extractTag(string(decoded), "Status")
}

func TestComposeNativeReplaceMIMEAndReplay(t *testing.T) {
	for _, command := range []string{"SendMail", "SmartReply", "SmartForward"} {
		t.Run(command, func(t *testing.T) {
			ctx, st, u, h, _, _ := syncFixture(t)
			source := composeSource(t, ctx, h, u, []byte("From: bob@example.com\r\nTo: alice@example.com\r\n\r\nOriginal must not be appended"))
			var delivered [][]byte
			h.sender = &mailSubmission{store: st, ms: h.ms, submit: func(_ context.Context, _ *storage.User, recipients []string, raw []byte) error {
				if strings.Join(recipients, ",") != "bob@example.com,hidden@example.com" {
					t.Fatalf("recipients: %v", recipients)
				}
				delivered = append(delivered, append([]byte(nil), raw...))
				return nil
			}}
			options := `<SaveInSentItems/>`
			if command != "SendMail" {
				options += `<ReplaceMime/><Source><FolderId>` + source.MailboxID + `</FolderId><ItemId>` + source.ID + `</ItemId></Source>`
			}
			raw := []byte("From: alice@example.com\r\nTo: bob@example.com\r\nBcc: hidden@example.com\r\nSubject: Re: Test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>Ответ с iPhone</p>\r\n--x\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=ticket.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQ=\r\n--x--\r\n")
			for i := 0; i < 2; i++ {
				w, trace := composeRequest(t, h, u, command, "same-client-id", options, raw)
				if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "0" || trace.Result != "compose_accepted" {
					t.Fatalf("native success: status=%d body=%q trace=%s", w.Code, w.Body.String(), trace.Result)
				}
				if !strings.Contains(w.Header().Get("MS-ASProtocolCommands"), command) {
					t.Fatal("command not advertised")
				}
			}
			if len(delivered) != 1 || bytes.Contains(delivered[0], []byte("Bcc:")) || bytes.Contains(delivered[0], []byte("must not be appended")) {
				t.Fatal("duplicate submission, Bcc leak, or appended original")
			}
			content, err := parseMessageContent(delivered[0])
			if err != nil || !strings.Contains(content.HTML, "Ответ с iPhone") || len(content.Attachments) != 1 || string(content.Attachments[0].Data) != "%PDF-1.4" {
				t.Fatalf("MIME changed: %#v, %v", content, err)
			}
			sent, err := st.GetMailbox(ctx, u.ID, "Sent")
			if err != nil {
				t.Fatal(err)
			}
			copies, err := st.ListMessages(ctx, sent.ID)
			if err != nil || len(copies) != 1 {
				t.Fatalf("Sent copies: %d, %v", len(copies), err)
			}
			w, _ := composeRequest(t, h, u, command, "same-client-id", options, append(raw, '!'))
			if composeErrorStatus(t, w) != "110" || len(delivered) != 1 {
				t.Fatal("reused ClientId accepted with different MIME")
			}
		})
	}
}

func TestComposeSourceOwnershipAndFailures(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	other, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "other", Email: "other@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	foreign := composeSource(t, ctx, h, other, []byte("From: other@example.com\r\n\r\nPrivate original"))
	own := composeSource(t, ctx, h, u, []byte("From: bob@example.com\r\n\r\nOriginal"))
	raw := []byte("From: alice@example.com\r\nTo: bob@example.com\r\n\r\nReply")
	submitted := 0
	h.sender = &mailSubmission{store: st, ms: h.ms, submit: func(context.Context, *storage.User, []string, []byte) error {
		submitted++
		return fmt.Errorf("queue unavailable")
	}}
	sourceXML := func(m *storage.Message) string {
		return `<ReplaceMime/><Source><FolderId>` + m.MailboxID + `</FolderId><ItemId>` + m.ID + `</ItemId></Source>`
	}
	for _, command := range []string{"SmartReply", "SmartForward"} {
		for _, options := range []string{"", sourceXML(foreign), strings.Replace(sourceXML(own), own.MailboxID, foreign.MailboxID, 1)} {
			w, trace := composeRequest(t, h, u, command, "missing-source", options, raw)
			if composeErrorStatus(t, w) != "150" || trace.Result != "compose_source_not_found" {
				t.Fatal("missing/foreign source accepted")
			}
		}
	}
	if submitted != 0 {
		t.Fatal("source checks ran after submission")
	}
	w, trace := composeRequest(t, h, u, "SmartReply", "retryable", sourceXML(own), raw)
	if composeErrorStatus(t, w) != "110" || trace.Result != "compose_submission_failed" {
		t.Fatal("queue failure acknowledged as success")
	}
	h.sender.submit = func(context.Context, *storage.User, []string, []byte) error { submitted++; return nil }
	w, _ = composeRequest(t, h, u, "SmartReply", "retryable", sourceXML(own), raw)
	if w.Code != 200 || w.Body.Len() != 0 || submitted != 2 {
		t.Fatal("failed submission cannot be retried")
	}
	for _, invalid := range []struct {
		id     string
		raw    []byte
		status string
	}{
		{"", raw, "101"}, {"empty", nil, "101"},
		{"forged", bytes.Replace(raw, []byte("alice@example.com"), []byte("forged@example.com"), 1), "110"},
	} {
		w, _ = composeRequest(t, h, u, "SmartReply", invalid.id, sourceXML(own), invalid.raw)
		if composeErrorStatus(t, w) != invalid.status || submitted != 2 {
			t.Fatal("invalid request submitted")
		}
	}
}

func TestSmartComposeWithoutReplaceMIME(t *testing.T) {
	for _, command := range []string{"SmartReply", "SmartForward"} {
		t.Run(command, func(t *testing.T) {
			ctx, st, u, h, _, _ := syncFixture(t)
			original := []byte("From: bob@example.com\r\nTo: alice@example.com\r\nBcc: original-hidden@example.com\r\nSubject: Original\r\nMessage-ID: <original@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<html><head><style>p{color:red}</style></head><body><p>Исходное письмо</p><img src=\"cid:image\"></body></html>\r\n--x\r\nContent-Type: image/png\r\nContent-ID: <image>\r\nContent-Disposition: inline; filename=photo.png\r\nContent-Transfer-Encoding: base64\r\n\r\naW1hZ2U=\r\n--x\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=ticket.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQ=\r\n--x--\r\n")
			msg := composeSource(t, ctx, h, u, original)
			draft := []byte("From: alice@example.com\r\nTo: bob@example.com\r\nSubject: Reply\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nНовый текст")
			options := `<Source><FolderId>` + msg.MailboxID + `</FolderId><ItemId>` + msg.ID + `</ItemId></Source>`
			var submitted [][]byte
			h.sender = &mailSubmission{store: st, ms: h.ms, submit: func(_ context.Context, _ *storage.User, _ []string, raw []byte) error {
				submitted = append(submitted, raw)
				return nil
			}}
			for i := 0; i < 2; i++ {
				w, _ := composeRequest(t, h, u, command, "merge-replay", options, draft)
				if w.Code != 200 || w.Body.Len() != 0 {
					t.Fatalf("merge failed: %d %q", w.Code, w.Body.String())
				}
			}
			if len(submitted) != 1 {
				t.Fatal("non-deterministic merged MIME bypassed replay protection")
			}
			content, err := parseMessageContent(submitted[0])
			if err != nil || !strings.Contains(content.Text, "Новый текст") || !strings.Contains(content.Text, "Исходное письмо") ||
				!strings.Contains(content.HTML, "p{color:red}") || !strings.Contains(content.HTML, `cid:image`) ||
				bytes.Contains(submitted[0], []byte("original-hidden@example.com")) {
				t.Fatalf("merge lost content or leaked Bcc: text=%q HTML=%q err=%v", content.Text, content.HTML, err)
			}
			wantAttachments := 1
			if command == "SmartForward" {
				wantAttachments = 2
			}
			if len(content.Attachments) != wantAttachments || string(content.Attachments[0].Data) != "image" || content.Attachments[0].Name != "photo.png" || !content.Attachments[0].Inline {
				t.Fatalf("merged attachments: %#v", content.Attachments)
			}
			if command == "SmartForward" && string(content.Attachments[1].Data) != "%PDF-1.4" {
				t.Fatal("forward lost attachment data")
			}
		})
	}
}
