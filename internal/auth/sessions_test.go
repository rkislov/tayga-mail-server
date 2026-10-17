package auth

import (
	"context"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestHTTPClientSessionGroupsAndCancelsRequests(t *testing.T) {
	a := &Layer{}
	u := &storage.User{ID: "user", Email: "user@example.com"}
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	doneFirst := a.TrackRequest(u, "EWS", "127.0.0.1", "Outlook", "user-client", cancelFirst)
	doneSecond := a.TrackRequest(u, "EWS", "127.0.0.1", "Outlook", "user-client", cancelSecond)
	defer doneFirst()
	defer doneSecond()
	sessions := a.Sessions()
	if len(sessions) != 1 || sessions[0].Protocol != "EWS" || sessions[0].Agent != "Outlook" {
		t.Fatalf("sessions: %+v", sessions)
	}
	if err := a.CloseSession(sessions[0].ID); err != nil {
		t.Fatal(err)
	}
	if first.Err() == nil || second.Err() == nil {
		t.Fatal("did not cancel all active requests")
	}
	if len(a.Sessions()) != 0 {
		t.Fatal("closed session retained")
	}
}

func TestLiveSessionTouchDoesNotResurrectClosedConnection(t *testing.T) {
	a := &Layer{}
	u := &storage.User{ID: "u", Email: "u@example.com"}
	id := a.TrackSession(u, "XMPP", "192.0.2.1:1234", "XMPP", "stream", true, func() error { return nil })
	before := a.Sessions()[0]
	a.TouchSession(id, "XMPP · phone")
	after := a.Sessions()[0]
	if after.Agent != "XMPP · phone" || !after.Started.Equal(before.Started) || after.Seen.Before(before.Seen) {
		t.Fatal(after)
	}
	a.CloseSession(id)
	a.TouchSession(id, "XMPP · phone")
	if len(a.Sessions()) != 0 {
		t.Fatal("resurrected closed connection")
	}
}
