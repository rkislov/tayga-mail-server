package notify

import (
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
