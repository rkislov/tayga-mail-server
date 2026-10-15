package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestPresenceSnapshotRequiresConsentAndFiltersResource(t *testing.T) {
	srv, store, alice := testServer(t)
	ctx := context.Background()
	bob, err := store.CreateUser(ctx, &storage.User{Email: "bob@example.com", LocalPart: "bob", DomainID: alice.DomainID, TenantID: alice.TenantID, Enabled: true, PasswordHash: "x", AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	srv.recordPresence(ctx, bob.Email+"/phone", "dnd", "Private & busy")
	srv.recordPresence(ctx, bob.Email+"/desktop", "away", "")
	var out bytes.Buffer
	send := func(raw []byte) { out.Write(raw) }
	srv.currentPresence(ctx, ParseJID(bob.Email), ParseJID(alice.Email+"/test"), send)
	if out.Len() != 0 {
		t.Fatal("presence disclosed without consent")
	}
	if err = srv.changeSubscription(ctx, bob.Email, alice.Email, false, true); err != nil {
		t.Fatal(err)
	}
	srv.currentPresence(ctx, ParseJID(bob.Email+"/phone"), ParseJID(alice.Email+"/test"), send)
	if !strings.Contains(out.String(), "Private &amp; busy") || strings.Contains(out.String(), "/desktop") {
		t.Fatalf("incorrect snapshot %s", out.String())
	}
	out.Reset()
	if err = srv.changeSubscription(ctx, bob.Email, alice.Email, false, false); err != nil {
		t.Fatal(err)
	}
	srv.currentPresence(ctx, ParseJID(bob.Email), ParseJID(alice.Email+"/test"), send)
	if out.Len() != 0 {
		t.Fatal("presence disclosed after revocation")
	}
}

func TestInitialPresenceRestoresSubscribedPeersAndStatus(t *testing.T) {
	srv, store, alice := testServer(t)
	ctx := context.Background()
	bob, err := store.CreateUser(ctx, &storage.User{Email: "bob@example.com", LocalPart: "bob", DomainID: alice.DomainID, TenantID: alice.TenantID, Enabled: true, PasswordHash: "x", AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	srv.recordPresence(ctx, bob.Email+"/phone", "dnd", "Working")
	if err = srv.changeSubscription(ctx, bob.Email, alice.Email, false, true); err != nil {
		t.Fatal(err)
	}
	if err = srv.changeSubscription(ctx, alice.Email, bob.Email, true, true); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	session := &Session{srv: srv, user: alice, JID: ParseJID(alice.Email + "/test"), bw: bufio.NewWriter(&out)}
	session.dec = xml.NewDecoder(strings.NewReader(`<presence id='p1'><show>away</show><status>Lunch &amp; tea</status><c xmlns='http://jabber.org/protocol/caps' node='test' ver='v1'/></presence>`))
	token, _ := session.dec.Token()
	if err = session.handlePresence(token.(xml.StartElement)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Working") {
		t.Fatalf("missing peer snapshot %s", out.String())
	}
	if !strings.Contains(out.String(), "http://jabber.org/protocol/caps") || !strings.Contains(out.String(), "id='p1'") {
		t.Fatalf("presence extensions lost: %s", out.String())
	}
	if got := srv.presenceFor(alice.Email); got.Status != "Lunch & tea" || got.State != "away" {
		t.Fatalf("lost status %+v", got)
	}
}
