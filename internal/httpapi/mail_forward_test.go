package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestForwardAttachmentCopiesOwnedBytesAndUsesImmutableReference(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	mb, err := f.s.store.EnsureMailbox(ctx, u.ID, "INBOX", f.s.ms.UserRoot(u.Email))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := attachOutgoingFiles([]byte("From: sender@example.com\r\nSubject: Original\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nOriginal body"), []outgoingAttachment{{Name: "план.pdf", Data: base64.StdEncoding.EncodeToString([]byte("%PDF-original"))}})
	if err != nil {
		t.Fatal(err)
	}
	rel, size, err := f.s.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := f.s.store.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "/api/v1/mail/messages/" + msg.ID + "/attachments/0/prepare"
	f.call(-1, "POST", endpoint, "", 401)
	f.call(1, "POST", endpoint, "", 403)
	f.call(0, "POST", strings.Replace(endpoint, "/0/", "/99/", 1), "", 404)
	w := f.call(0, "POST", endpoint, "", 200)
	var ref outgoingAttachment
	if err = json.Unmarshal(w.Body.Bytes(), &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Name != "план.pdf" || !strings.HasPrefix(ref.Path, "mail-out/") || ref.ETag == "" {
		t.Fatalf("bad reference %+v", ref)
	}
	files, err := f.s.resolveOutgoingFiles(u.ID, []outgoingAttachment{ref})
	if err != nil || string(files[0].content) != "%PDF-original" {
		t.Fatal("changed bytes", err)
	}
	if _, err = f.s.resolveOutgoingFiles(f.users[1], []outgoingAttachment{ref}); err == nil {
		t.Fatal("cross-user file leak")
	}
	f.call(0, "PUT", "/api/v1/files/content?path="+ref.Path, "changed", 200)
	if _, err = f.s.resolveOutgoingFiles(u.ID, []outgoingAttachment{ref}); err == nil {
		t.Fatal("stale reference accepted")
	}
	if n, err := f.s.store.CountOutbound(ctx); err != nil || n != 0 {
		t.Fatal("preparing attachment sent mail", err, n)
	}
}
