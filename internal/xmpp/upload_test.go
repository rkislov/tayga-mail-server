package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestUploadDiscoveryAndSlot(t *testing.T) {
	srv, _, u := testServer(t)
	srv.domain = "mail.example.com"
	srv.SetUploadHandler(func(ctx context.Context, uid, name string, size int64, ct string) (UploadSlot, error) {
		if uid != u.ID || name != "photo.jpg" || size != 1300000 {
			t.Fatal("wrong slot request")
		}
		return UploadSlot{Put: "https://mail.example.com/put", Get: "https://mail.example.com/get"}, nil
	})
	var out bytes.Buffer
	s := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	for _, tc := range []struct{ to, body, want string }{{"example.com", `<query xmlns='http://jabber.org/protocol/disco#items'/>`, "upload.mail.example.com"}, {"upload.mail.example.com", `<query xmlns='http://jabber.org/protocol/disco#info'/>`, UploadNamespace}, {"upload.mail.example.com", `<request xmlns='urn:xmpp:http:upload:0' filename='photo.jpg' size='1300000' content-type='image/jpeg'/>`, "https://mail.example.com/put"}} {
		out.Reset()
		handled, err := s.uploadIQ("id", "get", tc.to, tc.body)
		if !handled || err != nil || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%t %v %s", handled, err, out.String())
		}
	}
}
func TestChatAttachmentOOBAndEncrypted(t *testing.T) {
	a, e := stanzaAttachment(`<message><x xmlns='jabber:x:oob'><url>https://example.com/photo.jpg?a=1&amp;b=2</url></x></message>`)
	if e || a == nil || a.Name != "photo.jpg" || !strings.Contains(a.URL, "&b=2") {
		t.Fatal(a, e)
	}
	a, e = stanzaAttachment(`<message><encrypted xmlns='eu.siacs.conversations.axolotl'><payload>ciphertext</payload></encrypted></message>`)
	if !e || a != nil {
		t.Fatal(a, e)
	}
	a, _ = stanzaAttachment(`<message><body>javascript:alert(1)</body></message>`)
	if a != nil {
		t.Fatal("unsafe URL")
	}
}
