package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPreuploadedMailAttachmentsAndLimits(t *testing.T) {
	f := newFileFixture(t)
	content := "uploaded PDF bytes"
	f.call(0, "PUT", "/api/v1/files/content?path=mail-out/test.pdf", content, 200)
	var ref outgoingAttachment
	json.Unmarshal(f.call(0, "POST", "/api/v1/mail/attachments", `{"path":"mail-out/test.pdf","name":"Билет.pdf"}`, 200).Body.Bytes(), &ref)
	if ref.ETag == "" {
		t.Fatal("missing committed version")
	}
	f.call(1, "POST", "/api/v1/mail/attachments", `{"path":"mail-out/test.pdf","name":"Билет.pdf"}`, 404)
	files, err := f.s.resolveOutgoingFiles(f.users[0], []outgoingAttachment{ref})
	if err != nil {
		t.Fatal(err)
	}
	raw := buildOutgoingMIME("owner@example.com", "Owner", []string{"owner@example.com"}, nil, nil, "test", "body", "", "<test@example.com>")
	raw, err = attachOutgoingFiles(raw, files)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseMIMEMessage(raw)
	if len(parsed.Attachments) != 1 || string(parsed.Attachments[0].Data) != content || parsed.Attachments[0].Filename != "Билет.pdf" {
		t.Fatal("preuploaded MIME mismatch")
	}
	if _, err := f.s.resolveOutgoingFiles(f.users[1], []outgoingAttachment{ref}); err == nil {
		t.Fatal("cross-account reference accepted")
	}
	bad := ref
	bad.Path = "../" + f.users[0] + "/mail-out/test.pdf"
	if _, err := f.s.resolveOutgoingFiles(f.users[1], []outgoingAttachment{bad}); err == nil {
		t.Fatal("path traversal accepted")
	}
	// A changed file cannot silently replace the attachment selected by the sender.
	f.call(0, "PUT", "/api/v1/files/content?path=mail-out/test.pdf", "different bytes", 200)
	if _, err := f.s.resolveOutgoingFiles(f.users[0], []outgoingAttachment{ref}); err == nil {
		t.Fatal("stale attachment accepted")
	}
	json.Unmarshal(f.call(0, "POST", "/api/v1/mail/attachments", `{"path":"mail-out/test.pdf","name":"Билет.pdf"}`, 200).Body.Bytes(), &ref)
	body, _ := json.Marshal(map[string]any{"to": []string{"owner@example.com"}, "text": "test", "attachments": []outgoingAttachment{ref}})
	f.s.cfg.SMTP.MaxSize = 100
	f.call(0, "POST", "/api/v1/mail/send", string(body), 413)
	boxes, _ := f.s.store.ListMailboxes(context.Background(), f.users[0])
	if len(boxes) != 0 {
		t.Fatal("oversize message was persisted before rejection")
	}
	var limits map[string]int64
	json.Unmarshal(f.call(0, "GET", "/api/v1/mail/limits", "", 200).Body.Bytes(), &limits)
	if limits["message_bytes"] != 100 {
		t.Fatal(limits)
	}
	f.s.cfg.SMTP.MaxSize = 25 << 20
	f.call(0, "POST", "/api/v1/mail/send", string(body), 200)
	boxes, _ = f.s.store.ListMailboxes(context.Background(), f.users[0])
	if len(boxes) != 2 {
		t.Fatal("local mail not saved and delivered")
	}
}

func TestOutgoingAttachmentRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("pdf-test"), 200000)
	raw := buildOutgoingMIME("sender@example.com", "Sender", []string{"user@example.com"}, nil, nil, "test", "body", "<p>body</p>", "<test@example.com>")
	got, err := attachOutgoingFiles(raw, []outgoingAttachment{{Name: "Билет.pdf", Data: base64.StdEncoding.EncodeToString(data)}})
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseMIMEMessage(got)
	if len(parsed.Attachments) != 1 || !bytes.Equal(parsed.Attachments[0].Data, data) || parsed.Attachments[0].Filename != "Билет.pdf" {
		t.Fatal("attachment round trip failed")
	}
}
func TestStreamUploadLimitPreservesDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	os.WriteFile(path, []byte("old"), 0600)
	r := httptest.NewRequest("PUT", "/", bytes.NewReader([]byte("new")))
	r.ContentLength = maxWebUploadBytes + 1
	if _, err := streamUpload(httptest.NewRecorder(), r, path); err == nil {
		t.Fatal("accepted oversize")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Fatal("overwritten")
	}
	r = httptest.NewRequest("PUT", "/", bytes.NewReader([]byte("new")))
	if _, err := streamUpload(httptest.NewRecorder(), r, path); err != nil {
		t.Fatal(err)
	}
}

func TestOutgoingUnicodeHeadersAreEncoded(t *testing.T) {
	raw := buildOutgoingMIME("sender@example.com", "Роман Кислов", []string{"user@example.com"}, nil, []string{"hidden@example.com"}, "Тема письма", "Текст", "", "<test@example.com>")
	head := bytes.SplitN(raw, []byte("\r\n\r\n"), 2)[0]
	for _, b := range head {
		if b > 127 {
			t.Fatalf("raw UTF-8 header: %s", head)
		}
	}
	if bytes.Contains(head, []byte("Bcc:")) || bytes.Contains(head, []byte("hidden@example.com")) {
		t.Fatal("Bcc leaked")
	}
}
