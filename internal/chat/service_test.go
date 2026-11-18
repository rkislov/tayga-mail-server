package chat

import "testing"

func TestSubscriberCleanup(t *testing.T) {
	s := &Service{subscribers: make(map[chan Event]struct{})}
	ch, cleanup := s.Subscribe()
	cleanup()
	cleanup()
	if _, ok := <-ch; ok {
		t.Fatal("closed subscriber still active")
	}
	if len(s.subscribers) != 0 {
		t.Fatal("subscriber retained")
	}
}
