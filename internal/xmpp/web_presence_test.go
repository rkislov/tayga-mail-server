package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestWebPresenceConsentNamesAndReconnect(t *testing.T) {
	srv, store, alice := testServer(t)
	ctx := context.Background()
	bob, err := store.CreateUser(ctx, &storage.User{Email: "bob@example.com", LocalPart: "bob", DomainID: alice.DomainID, TenantID: alice.TenantID, DisplayName: "Борис Иванов", PasswordHash: "x", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.UpsertChatRoster(ctx, alice.ID, bob.Email, ""); err != nil {
		t.Fatal(err)
	}
	if err := srv.SetChatPresence(ctx, bob.ID, "dnd", "Работаю"); err != nil {
		t.Fatal(err)
	}
	rows, err := srv.ListChatRoster(ctx, alice.ID)
	if err != nil || len(rows) != 2 || rows[1].Name != "Борис Иванов" || rows[1].Presence != "unknown" {
		t.Fatalf("unapproved roster: %+v %v", rows, err)
	}
	if err := srv.ChatSubscription(ctx, alice.ID, bob.Email, "request"); err != nil {
		t.Fatal(err)
	}
	requests, err := srv.ListChatRoster(ctx, bob.ID)
	if err != nil || len(requests) != 2 || !requests[1].SubscriptionRequest {
		t.Fatalf("request: %+v %v", requests, err)
	}
	if err := srv.ChatSubscription(ctx, bob.ID, alice.Email, "accept"); err != nil {
		t.Fatal(err)
	}
	rows, _ = srv.ListChatRoster(ctx, alice.ID)
	if rows[1].Presence != "dnd" || rows[1].Subscription != "to" {
		t.Fatalf("approved: %+v", rows)
	}
	requests, _ = srv.ListChatRoster(ctx, bob.ID)
	if requests[1].SubscriptionRequest {
		t.Fatal("accepted request remains")
	}
	if err := srv.ChatSubscription(ctx, bob.ID, alice.Email, "accept"); err == nil {
		t.Fatal("accepted missing request")
	}
	_, closeWeb := srv.SubscribeChat(bob.Email)
	if srv.presenceFor(bob.Email).State != "dnd" {
		t.Fatal("reconnect reset selected presence")
	}
	closeWeb()
	if srv.presenceFor(bob.Email).State != "offline" {
		t.Fatal("closed web session remains online")
	}
	_, closeWeb = srv.SubscribeChat(bob.Email)
	defer closeWeb()
	if srv.presenceFor(bob.Email).State != "dnd" {
		t.Fatal("selection not restored")
	}
	if err := srv.SetChatPresence(ctx, bob.ID, "invalid", ""); err == nil {
		t.Fatal("invalid state accepted")
	}
}

func TestRosterEditCannotGrantPresence(t *testing.T) {
	srv, _, user := testServer(t)
	var out bytes.Buffer
	session := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/test"), bw: bufio.NewWriter(&out)}
	if err := session.handleRosterIQ(context.Background(), "test", "set", `<query xmlns='jabber:iq:roster'><item jid='bob@example.com' name='Bob' subscription='both'/></query>`); err != nil {
		t.Fatal(err)
	}
	rows, err := srv.listRoster(context.Background(), user.ID)
	if err != nil || len(rows) != 1 || rows[0].Subscription != "none" {
		t.Fatalf("client granted presence: %+v %v", rows, err)
	}
}
