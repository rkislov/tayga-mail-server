// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSubmissionReplaySenderAndBlindRecipients(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	calls := 0
	sender := &mailSubmission{store: st, ms: h.ms, submit: func(_ context.Context, _ *storage.User, recipients []string, raw []byte) error {
		calls++
		if len(recipients) != 2 || bytes.Contains(raw, []byte("Bcc:")) {
			t.Fatalf("blind recipient handling: %v", recipients)
		}
		return nil
	}}
	raw := []byte("From: alice@example.com\r\nTo: bob@example.com\r\nBcc: hidden@example.com\r\nSubject: Test\r\n\r\nHello")
	for i := 0; i < 2; i++ {
		if err := sender.send(ctx, u, "same", raw, true); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("submitted %d times", calls)
	}
	if err := sender.send(ctx, u, "same", append(raw, '!'), true); err == nil {
		t.Fatal("accepted reused client ID")
	}
	if err := sender.send(ctx, u, "other", bytes.Replace(raw, []byte("alice@example.com"), []byte("forged@example.com"), 1), false); err == nil {
		t.Fatal("accepted forged sender")
	}
	mb, err := st.GetMailbox(ctx, u.ID, "Sent")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := st.ListMessages(ctx, mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("sent copies: %d", len(messages))
	}
}

func TestMIMEBodyAttachmentsAndOpaqueWire(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\nSubject: =?UTF-8?B?0KLQtdGB0YI=?=\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\nSGVsbG8=\r\n--x\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=ticket.pdf\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0xLjQ=\r\n--x--\r\n")
	content, err := parseMessageContent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if content.Subject != "Тест" || content.Text != "Hello" || len(content.Attachments) != 1 || string(content.Attachments[0].Data) != "%PDF-1.4" {
		t.Fatalf("bad MIME result: %#v", content)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	wire, err := encodeWire(`<SendMail xmlns="ComposeMail:"><ClientId>abc</ClientId><Mime>` + encoded + `</Mime></SendMail>`)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWire(wire)
	if err != nil {
		t.Fatal(err)
	}
	if extractTag(string(decoded), "Mime") != encoded {
		t.Fatal("MIME opaque data corrupted")
	}
}

func TestProtocolXMLRejectsAmbiguousAndDeepDocuments(t *testing.T) {
	for _, input := range []string{"", `<a/><b/>`, `oops<a/>`, strings.Repeat("<a>", 70) + strings.Repeat("</a>", 70)} {
		if _, err := parseProtocolXML([]byte(input)); err == nil {
			t.Fatal("accepted invalid XML")
		}
	}
}

func TestPingDetectsPendingChangesAndValidatesHeartbeat(t *testing.T) {
	ctx, st, u, h, dev, ab := syncFixture(t)
	_, err := st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: ab, UID: "ping", HrefName: "ping.vcf", Data: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Ping\r\nEND:VCARD\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	body := `<Ping><HeartbeatInterval>60</HeartbeatInterval><Folders><Folder><Id>` + ab + `</Id><Class>Contacts</Class></Folder></Folders></Ping>`
	out, err := h.ping(ctx, u, dev, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if extractTag(out, "Status") != "2" {
		t.Fatal(out)
	}
	out, err = h.ping(ctx, u, dev, []byte(`<Ping><HeartbeatInterval>1</HeartbeatInterval></Ping>`))
	if err != nil || extractTag(out, "Status") != "5" {
		t.Fatal(out, err)
	}
}

func TestEWSDraftSendAndHierarchy(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	calls := 0
	ews := &ewsHandler{store: st, ms: h.ms, sender: &mailSubmission{store: st, ms: h.ms, submit: func(context.Context, *storage.User, []string, []byte) error { calls++; return nil }}}
	request := `<CreateItem MessageDisposition="SaveOnly"><Items><Message><Subject>Draft</Subject><Body BodyType="Text">Hello</Body><ToRecipients><Mailbox><EmailAddress>bob@example.com</EmailAddress></Mailbox></ToRecipients></Message></Items></CreateItem>`
	out, err := ews.createItem(ctx, u, request)
	if err != nil {
		t.Fatal(err)
	}
	id := extractAttr(out, "ItemId", "Id")
	if id == "" {
		t.Fatal(out)
	}
	out, err = ews.sendItem(ctx, u, `<SendItem SaveItemToFolder="true"><ItemIds><ItemId Id="`+id+`"/></ItemIds></SendItem>`)
	if err != nil || extractTag(out, "ResponseCode") != "NoError" || calls != 1 {
		t.Fatal(out, err, calls)
	}
	out, err = ews.syncEWSHierarchy(ctx, u, `<SyncFolderHierarchy/>`)
	if err != nil {
		t.Fatal(err)
	}
	key := extractTag(out, "SyncState")
	if key == "" || !strings.Contains(out, "Create") {
		t.Fatal(out)
	}
	out, err = ews.syncEWSHierarchy(ctx, u, `<SyncFolderHierarchy><SyncState>`+key+`</SyncState></SyncFolderHierarchy>`)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if changes := doc.find("Changes"); changes == nil || len(changes.Children) != 0 {
		t.Fatal(out)
	}
}

func TestSyncMailPreferencesWindowAndGetChanges(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	add := func(subject string, date time.Time) *storage.Message {
		t.Helper()
		raw := []byte("From: bob@example.com\r\nTo: alice@example.com\r\nSubject: " + subject + "\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>Привет, мир!</p>")
		rel, size, err := h.ms.Deliver(u.Email, "INBOX", raw)
		if err != nil {
			t.Fatal(err)
		}
		m, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size, InternalDate: date, Subject: subject, FromAddr: "bob@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	add("Old", time.Now().AddDate(0, 0, -30))
	recent := add("Recent", time.Now())
	request := func(key, extra string) string {
		return `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + mb.ID + `</CollectionId>` + extra + `</Collection></Collections></Sync>`
	}
	call := func(body string) string {
		t.Helper()
		out, err := h.syncCollections(ctx, u, dev, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := call(request("0", `<Options><FilterType>3</FilterType><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>10</TruncationSize></BodyPreference></Options>`))
	key := extractTag(out, "SyncKey")
	out = call(request(key, ""))
	doc, err := parseProtocolXML([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	commands := doc.find("Commands")
	if commands == nil || len(commands.Children) != 1 || commands.Children[0].value("ServerId") != recent.ID {
		t.Fatal(out)
	}
	body := commands.find("Body")
	if body == nil || body.value("Type") != "1" || body.value("Truncated") != "1" || !utf8.ValidString(body.value("Data")) || len([]byte(body.value("Data"))) > 10 {
		t.Fatal(out)
	}
	key = extractTag(out, "SyncKey")
	add("New", time.Now())
	out = call(request(key, `<GetChanges>0</GetChanges>`))
	doc, _ = parseProtocolXML([]byte(out))
	if doc.find("Commands") != nil {
		t.Fatal(out)
	}
	out = call(request(extractTag(out, "SyncKey"), ""))
	doc, _ = parseProtocolXML([]byte(out))
	if commands := doc.find("Commands"); commands == nil || len(commands.Children) != 1 {
		t.Fatal(out)
	}
	out = call(request(extractTag(out, "SyncKey"), `<Options><FilterType>8</FilterType></Options>`))
	if extractTag(out, "Status") != "4" {
		t.Fatal(out)
	}
}
