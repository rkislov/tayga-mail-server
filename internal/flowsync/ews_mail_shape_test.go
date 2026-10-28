package flowsync

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestEWSNativeMimeAndMetadataShapes(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	inbox, err := st.EnsureMailbox(ctx, user.ID, "INBOX", "")
	if err != nil {
		t.Fatal(err)
	}
	raw := "From: sender@example.com\r\nSubject: Big attachment\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: text/html\r\n\r\n<b>Keep HTML in the original</b>\r\n--b\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=big.bin\r\n\r\n" + strings.Repeat("x", 33<<20) + "\r\n--b--\r\n"
	path, size, err := h.ms.Deliver(user.Email, "INBOX", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	request := func(shape string) string {
		return `<GetItem><ItemShape><BaseShape>IdOnly</BaseShape>` + shape + `</ItemShape><ItemIds><ItemId Id="` + msg.ID + `"/></ItemIds></GetItem>`
	}
	// Opening MIME must bypass the 32 MiB preview-part limit and preserve bytes.
	got, err := h.getMailItem(ctx, user, msg, request(`<IncludeMimeContent>true</IncludeMimeContent>`))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML([]byte(got))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(doc.find("MimeContent").Text)
	if err != nil || string(decoded) != raw {
		t.Fatalf("raw message changed: %v", err)
	}
	if doc.find("Body") != nil || len(doc.find("Message").Children) != 2 {
		t.Fatal("MIME-only request includes preview properties")
	}
	// The background metadata request must stay small, even for large attachments.
	got, err = h.getMailItem(ctx, user, msg, request(`<IncludeMimeContent>false</IncludeMimeContent><AdditionalProperties><FieldURI FieldURI="item:Subject"/><FieldURI FieldURI="item:HasAttachments"/><FieldURI FieldURI="item:Size"/></AdditionalProperties>`))
	if err != nil {
		t.Fatal(err)
	}
	doc, err = parseProtocolXML([]byte(got))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 4096 || doc.find("Body") != nil || doc.find("MimeContent") != nil || doc.find("Attachments") != nil || doc.find("HasAttachments").Text != "true" {
		t.Fatalf("metadata response wrong: %s", got)
	}
	if len(doc.find("Message").Children) != 4 {
		t.Fatalf("unrequested properties returned: %s", got)
	}
	if got, err = h.getMailItem(ctx, &storage.User{ID: "other-user"}, msg, request(`<IncludeMimeContent>true</IncludeMimeContent>`)); err != nil || !strings.Contains(got, "ErrorAccessDenied") || strings.Contains(got, "MimeContent") {
		t.Fatal("MIME request bypassed ownership")
	}
}

func TestEWSNativeMalformedMimeStillDownloads(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	inbox, _ := st.EnsureMailbox(ctx, user.ID, "INBOX", "")
	raw := "Subject: Broken MIME\r\nContent-Type: multipart/mixed; boundary=missing\r\n\r\nno MIME boundary"
	path, size, err := h.ms.Deliver(user.Email, "INBOX", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.batchItems(ctx, user, "GetItem", `<GetItem><ItemShape><BaseShape>IdOnly</BaseShape><IncludeMimeContent>1</IncludeMimeContent></ItemShape><ItemIds><ItemId Id="`+msg.ID+`"/></ItemIds></GetItem>`, h.getItem)
	if err != nil || !strings.Contains(got, base64.StdEncoding.EncodeToString([]byte(raw))) {
		t.Fatalf("original cannot be downloaded: %v %s", err, got)
	}
}
