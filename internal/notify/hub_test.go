package notify

import (
	"context"
	"testing"
	"time"
)

func TestHubPublishSubscribe(t *testing.T) {
	h := NewHub()
	ch, unsub := h.Subscribe("u1")
	defer unsub()
	h.PublishMail("u1", "INBOX", "Hello", "a@ex.com")
	select {
	case ev := <-ch:
		if ev.Kind != KindMail || ev.Title != "Hello" {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	recent := h.Recent("u1", 5)
	if len(recent) != 1 {
		t.Fatalf("recent=%d", len(recent))
	}
}

func TestConcurrentPublishUnsubscribe(t *testing.T) {
	h := NewHub()
	for i := 0; i < 200; i++ {
		_, unsub := h.Subscribe("user")
		done := make(chan struct{})
		go func() {
			defer close(done)
			for n := 0; n < 20; n++ {
				h.Publish("user", Event{Title: "test"})
			}
		}()
		unsub()
		unsub()
		<-done
	}
}

func TestReminderRecipientsIncludeOfflinePushSubscribers(t *testing.T) {
	h := NewHub()
	_, unsub := h.Subscribe("online")
	defer unsub()
	r := &ReminderLoop{Hub: h, AdditionalUsers: func(context.Context) ([]string, error) { return []string{"offline", "online", "offline", ""}, nil }}
	ids := r.recipientIDs(context.Background())
	if len(ids) != 2 {
		t.Fatalf("recipients=%v", ids)
	}
	found := false
	for _, id := range ids {
		found = found || id == "offline"
	}
	if !found {
		t.Fatal("offline subscriber missing")
	}
}
