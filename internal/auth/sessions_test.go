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
