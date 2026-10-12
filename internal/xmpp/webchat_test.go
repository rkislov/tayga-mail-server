package xmpp

import (
	"context"
	"testing"
	"time"
)

func TestWebChatSendHistorySubscribe(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()

	ch, unsub := srv.SubscribeChat(u.Email)
	defer unsub()

	ev, err := srv.SendChatMessage(ctx, u.Email, "bob@example.com", "hello web")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Body != "hello web" {
		t.Fatalf("ev=%+v", ev)
	}

	select {
	case got := <-ch:
		if got.Body != "hello web" {
			t.Fatalf("sse=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for chat event")
	}

	_ = srv.UpsertChatRoster(ctx, u.ID, "bob@example.com", "Bob")
	roster, err := srv.ListChatRoster(ctx, u.ID)
	if err != nil || len(roster) != 1 {
		t.Fatalf("roster=%+v err=%v", roster, err)
	}

	hist, err := srv.ChatHistory(ctx, u.Email, "bob@example.com", 20)
	if err != nil || len(hist) == 0 {
		t.Fatalf("hist=%+v err=%v", hist, err)
	}
	if hist[len(hist)-1].Body != "hello web" {
		t.Fatalf("last=%+v", hist[len(hist)-1])
	}
}

func TestIncomingConversationVisibleWithoutSavedContact(t *testing.T) {
	srv, _, user := testServer(t)
	ctx := context.Background()
	if _, err := srv.SendChatMessage(ctx, "bob@example.com", user.Email, "incoming"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		roster, err := srv.ListChatRoster(ctx, user.ID)
		if err != nil || len(roster) != 1 || roster[0].JID != "bob@example.com" {
			t.Fatalf("roster=%+v err=%v", roster, err)
		}
	}
	saved, err := srv.listRoster(ctx, user.ID)
	if err != nil || len(saved) != 0 {
		t.Fatalf("conversation must not grant a roster subscription: %+v %v", saved, err)
	}
}

func TestWebSubscriptionConcurrentClose(t *testing.T) {
	srv, _, user := testServer(t)
	for i := 0; i < 100; i++ {
		_, cancel := srv.SubscribeChat(user.Email)
		done := make(chan struct{})
		go func() { defer close(done); srv.hub.pushWeb(user.Email, ChatEvent{Type: "message"}) }()
		cancel()
		cancel()
		<-done
	}
}
