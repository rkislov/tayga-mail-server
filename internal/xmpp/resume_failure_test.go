package xmpp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

type failedReplayConn struct {
	net.Conn
	deadline time.Time
	bounded  bool
}

func (c *failedReplayConn) SetWriteDeadline(d time.Time) error { c.deadline = d; return nil }
func (c *failedReplayConn) Write([]byte) (int, error) {
	c.bounded = c.deadline.After(time.Now()) && time.Until(c.deadline) <= stanzaWriteTimeout
	return 0, os.ErrDeadlineExceeded
}

func TestFailedReplayKeepsMessagesAndRoomStateForNextResume(t *testing.T) {
	srv, _, u := testServer(t)
	raw := `<message id='pending'><body>one</body></message>`
	room := mucResumeState{JID: "room@conference.example.com", Nick: "Alice", Since: time.Now().UTC()}
	srv.resumable = map[string]resumableStream{"retry": {
		userID: u.ID, jid: ParseJID(u.Email + "/phone"), expires: time.Now().Add(resumeTTL), rooms: []mucResumeState{room},
		state: streamManagement{enabled: true, resumable: true, token: "retry", received: 7, sent: 1, pending: []string{raw}, bytes: len(raw)},
	}}
	conn := &failedReplayConn{}
	s := &Session{srv: srv, user: u, authed: true, conn: conn, bw: bufio.NewWriter(conn)}
	start := xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "previd"}, Value: "retry"}, {Name: xml.Name{Local: "h"}, Value: "0"}}}
	if err := s.resumeStream(start); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("replay failure: %v", err)
	}
	if !conn.bounded || !conn.deadline.IsZero() {
		t.Fatal("replay did not bound/reset write deadline")
	}
	if len(srv.hub.Sessions(u.Email)) != 0 {
		t.Fatal("failed replay registered as online")
	}
	s.retainStream()
	snapshot, ok := srv.resumable["retry"]
	if !ok || snapshot.state.sent != 1 || snapshot.state.received != 7 || len(snapshot.state.pending) != 1 || snapshot.state.pending[0] != raw {
		t.Fatal("failed replay lost counters or queued stanza")
	}
	if len(snapshot.rooms) != 1 || snapshot.rooms[0] != room {
		t.Fatal("failed replay lost rooms")
	}
}

func TestResumeCounterWrapReplaysOnlyUnhandled(t *testing.T) {
	srv, _, u := testServer(t)
	one, two, three := `<message id='one'/>`, `<message id='two'/>`, `<message id='three'/>`
	srv.resumable = map[string]resumableStream{"wrap": {
		userID: u.ID, jid: ParseJID(u.Email + "/phone"), expires: time.Now().Add(resumeTTL),
		state: streamManagement{enabled: true, resumable: true, token: "wrap", acknowledged: ^uint32(0) - 1, sent: 1, pending: []string{one, two, three}, bytes: len(one + two + three)},
	}}
	var wire bytes.Buffer
	s := &Session{srv: srv, user: u, authed: true, bw: bufio.NewWriter(&wire)}
	start := xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "previd"}, Value: "wrap"}, {Name: xml.Name{Local: "h"}, Value: "0"}}}
	if err := s.resumeStream(start); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(wire.String(), one) || strings.Contains(wire.String(), two) || strings.Count(wire.String(), three) != 1 {
		t.Fatal("invalid replay across counter wrap", wire.String())
	}
	if s.sm.sent != 1 || s.sm.acknowledged != 0 || len(s.sm.pending) != 1 {
		t.Fatal("counter wrap changed sequence state")
	}
}

func TestResumeWaitsForUnregisteredReaderAndRejectsOtherAccount(t *testing.T) {
	srv, _, u := testServer(t)
	serverConn, peer := net.Pipe()
	defer serverConn.Close()
	defer peer.Close()
	old := newSession(srv, serverConn, true)
	old.user, old.authed, old.bound, old.JID = u, true, true, ParseJID(u.Email+"/phone")
	old.sm = streamManagement{enabled: true, resumable: true, token: "closing-token"}
	// Close has already unregistered the resource, but room cleanup has not
	// finished and the reader has not yet published the resumable snapshot.
	srv.closingStreams = map[*Session]struct{}{old: {}}
	start := xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "previd"}, Value: "closing-token"}, {Name: xml.Name{Local: "h"}, Value: "0"}}}
	otherUser := *u
	otherUser.ID, otherUser.Email = "other-user", "bob@example.com"
	var foreign bytes.Buffer
	other := &Session{srv: srv, user: &otherUser, authed: true, bw: bufio.NewWriter(&foreign)}
	if err := other.resumeStream(start); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(foreign.String(), "item-not-found") {
		t.Fatal("foreign stream accepted", foreign.String())
	}
	peer.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	var b [1]byte
	if _, err := peer.Read(b[:]); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("foreign request interrupted owner's transport: %v", err)
	}
	var wire bytes.Buffer
	fresh := &Session{srv: srv, user: u, authed: true, bw: bufio.NewWriter(&wire)}
	result := make(chan error, 1)
	workDone := make(chan struct{})
	go func() { result <- fresh.resumeStream(start); close(workDone) }()
	t.Cleanup(func() {
		select {
		case <-old.done:
		default:
			close(old.done)
		}
		select {
		case <-workDone:
		case <-time.After(2 * time.Second):
			t.Error("resume did not stop")
		}
	})
	peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(b[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("old transport not closed: %v", err)
	}
	select {
	case err := <-result:
		t.Fatalf("resume did not wait for cleanup: %v", err)
	default:
	}
	old.retainStream()
	srv.resumeMu.Lock()
	delete(srv.closingStreams, old)
	srv.resumeMu.Unlock()
	close(old.done)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("resume did not finish after cleanup")
	}
	if !strings.Contains(wire.String(), "<resumed") {
		t.Fatal(wire.String())
	}
}
