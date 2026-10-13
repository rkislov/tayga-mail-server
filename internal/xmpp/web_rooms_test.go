package xmpp

import (
	"context"
	"github.com/tayga/tms/internal/storage"
	"testing"
)

func TestWebGroupsMembershipHistoryAndDelivery(t *testing.T) {
	srv, store, alice := testServer(t)
	ctx := context.Background()
	bob, err := store.CreateUser(ctx, &storage.User{Email: "bob@example.com", LocalPart: "bob", DomainID: alice.DomainID, TenantID: alice.TenantID, DisplayName: "Bob", PasswordHash: "x", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ch, unsubscribe := srv.SubscribeChat(bob.Email)
	defer unsubscribe()
	room, err := srv.CreateChatRoom(ctx, alice.Email, "Team", []string{bob.Email, bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	if len(room.Members) != 2 {
		t.Fatal(room)
	}
	<-ch // roster update
	ev, err := srv.SendChatMessage(ctx, alice.Email, room.JID, "Hello team")
	if err != nil {
		t.Fatal(err)
	}
	received := <-ch
	if received.ID != ev.ID || received.Room != room.JID {
		t.Fatal(received)
	}
	history, err := srv.ChatHistory(ctx, bob.Email, room.JID, 80)
	if err != nil || len(history) != 1 || history[0].Body != "Hello team" {
		t.Fatal(history, err)
	}
	if _, err = srv.ChatHistory(ctx, "outsider@example.com", room.JID, 80); err == nil {
		t.Fatal("outsider can read")
	}
	if _, err = srv.SendChatMessage(ctx, "outsider@example.com", room.JID, "attack"); err == nil {
		t.Fatal("outsider can write")
	}
	roster, err := srv.ListChatRoster(ctx, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range roster {
		if r.JID == room.JID && r.Group {
			found = true
		}
	}
	if !found {
		t.Fatal("room missing")
	}
	if err = srv.LeaveChatRoom(ctx, bob.Email, room.JID); err != nil {
		t.Fatal(err)
	}
	if _, err = srv.ChatHistory(ctx, bob.Email, room.JID, 80); err == nil {
		t.Fatal("former member can read")
	}
	if _, err = srv.CreateChatRoom(ctx, alice.Email, "Invalid", []string{"missing@example.com"}); err == nil {
		t.Fatal("unknown member accepted")
	}
	tenant, err := store.CreateTenant(ctx, "other")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := store.CreateDomain(ctx, tenant.ID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateUser(ctx, &storage.User{Email: "other@other.example", LocalPart: "other", DomainID: domain.ID, TenantID: tenant.ID, PasswordHash: "x", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = srv.CreateChatRoom(ctx, alice.Email, "Invalid", []string{"other@other.example"}); err == nil {
		t.Fatal("cross tenant member accepted")
	}
}
