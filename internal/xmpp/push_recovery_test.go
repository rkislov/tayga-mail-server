package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

type pushUserLookup struct {
	storage.Driver
	err error
}

func (s pushUserLookup) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.Driver.GetUserByID(ctx, id)
}

func TestPushRecoversAfterExtendedGatewayOutage(t *testing.T) {
	for _, reply := range []string{"timeout", "wait", "malformed"} {
		t.Run(reply, func(t *testing.T) {
			srv, store, u := testServer(t)
			var logs, out bytes.Buffer
			srv.log = slog.New(slog.NewTextHandler(&logs, nil))
			srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{Domain: "push.example.com", Secret: "shared", Push: true}}
			var requests []string
			srv.hub.RegisterComponent(&ComponentConn{Domain: "push.example.com", Send: func(raw []byte) {
				var iq struct {
					ID string `xml:"id,attr"`
				}
				if err := xml.Unmarshal(raw, &iq); err != nil {
					t.Fatal(err)
				}
				requests = append(requests, iq.ID)
			}})
			session := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
			if err := session.handlePushIQ("enable", "set", "", `<enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='private-device-token'><x xmlns='jabber:x:data' type='submit'><field var='FORM_TYPE'><value>http://jabber.org/protocol/pubsub#publish-options</value></field><field var='secret'><value>private-publish-secret</value></field></x></enable>`); err != nil {
				t.Fatal(err)
			}
			event := ChatEvent{Type: "message", From: "bob@example.com/phone", Body: "private-message-body"}
			srv.enqueueNativePush(u.Email, event)
			now := time.Now().Truncate(time.Second)
			// Advance a fake clock through more failures than the former five-attempt
			// limit. No sleeps or real gateway notifications are needed.
			for i := 0; i < 10; i++ {
				srv.deliverPushBatch(context.Background(), now)
				if len(requests) != i+1 {
					t.Fatalf("attempt %d: registration/queue lost", i+1)
				}
				var due int64
				var attempts, registrations int
				if err := srv.db.QueryRow(`SELECT next_attempt,attempts FROM xmpp_push_jobs`).Scan(&due, &attempts); err != nil {
					t.Fatal(err)
				}
				if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_registrations`).Scan(&registrations); err != nil {
					t.Fatal(err)
				}
				delay := time.Unix(due, 0).Sub(now)
				if registrations != 1 || attempts != i+1 || delay < 30*time.Second || delay > 30*time.Minute {
					t.Fatalf("invalid retry state: registrations=%d attempts=%d delay=%s", registrations, attempts, delay)
				}
				if i >= 6 && delay != 30*time.Minute {
					t.Fatalf("long outage backoff = %s", delay)
				}
				if reply == "wait" {
					srv.acceptPushReply("push.example.com", u.Email, requests[i], "error", `<error type='wait'><service-unavailable xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error>`)
				} else if reply == "malformed" {
					srv.acceptPushReply("push.example.com", u.Email, requests[i], "error", `<error`)
				}
				srv.enqueueNativePush(u.Email, event) // arrivals must not reset the backoff
				srv.deliverPushBatch(context.Background(), time.Unix(due-1, 0))
				if len(requests) != i+1 {
					t.Fatal("retried before backoff expired")
				}
				if i == 5 {
					restarted := New(srv.cfg, srv.log, store, nil, nil)
					restarted.hub = srv.hub
					srv = restarted
				}
				now = time.Unix(due, 0)
			}
			// Recovery acknowledges the latest publication, while newer arrivals
			// still need one more publish. A stale acknowledgement cannot remove it.
			srv.acceptPushReply("push.example.com", u.Email, requests[0], "result", "")
			srv.acceptPushReply("push.example.com", u.Email, requests[len(requests)-1], "result", "")
			srv.deliverPushBatch(context.Background(), now)
			if len(requests) != 11 {
				t.Fatal("new arrivals lost after gateway recovered")
			}
			srv.acceptPushReply("push.example.com", u.Email, requests[10], "result", "")
			var jobs, registrations int
			if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_jobs`).Scan(&jobs); err != nil {
				t.Fatal(err)
			}
			if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_registrations`).Scan(&registrations); err != nil {
				t.Fatal(err)
			}
			if jobs != 0 || registrations != 1 {
				t.Fatalf("recovery: jobs=%d registrations=%d", jobs, registrations)
			}
			if !strings.Contains(logs.String(), "xmpp push accepted") {
				t.Fatal("gateway acceptance not logged")
			}
			for _, secret := range []string{event.Body, event.From, "private-device-token", "private-publish-secret"} {
				if strings.Contains(logs.String(), secret) {
					t.Fatalf("private value leaked to log: %q", secret)
				}
			}
		})
	}
}

func TestPushUserLookupFailureKeepsRegistration(t *testing.T) {
	srv, store, u := testServer(t)
	srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{Domain: "push.example.com", Secret: "x", Push: true}}
	var out bytes.Buffer
	session := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	srv.hub.RegisterComponent(&ComponentConn{Domain: "push.example.com", Send: func([]byte) { t.Fatal("published while account lookup failed") }})
	if err := session.handlePushIQ("enable", "set", "", `<enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='device'/>`); err != nil {
		t.Fatal(err)
	}
	srv.enqueueNativePush(u.Email, ChatEvent{Type: "message", Body: "test", From: "bob@example.com"})
	srv.store = pushUserLookup{Driver: store, err: errors.New("temporary database outage")}
	now := time.Now().Truncate(time.Second)
	srv.deliverPushBatch(context.Background(), now)
	var count int
	var due int64
	if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_registrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("temporary lookup error deleted registration")
	}
	if err := srv.db.QueryRow(`SELECT next_attempt FROM xmpp_push_jobs`).Scan(&due); err != nil {
		t.Fatal(err)
	}
	if due <= now.Unix() {
		t.Fatal("lookup failure has no backoff")
	}
	// A genuinely removed account must still invalidate the registration.
	srv.store = pushUserLookup{Driver: store, err: storage.ErrNotFound}
	srv.deliverPushBatch(context.Background(), time.Unix(due, 0))
	if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_registrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("missing account retained registration")
	}
}

func TestPushRetryCounterCannotOverflow(t *testing.T) {
	srv, _, u := testServer(t)
	srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{Domain: "push.example.com", Secret: "x", Push: true}}
	var out bytes.Buffer
	publications := 0
	srv.hub.RegisterComponent(&ComponentConn{Domain: "push.example.com", Send: func([]byte) { publications++ }})
	session := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	if err := session.handlePushIQ("enable", "set", "", `<enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='device'/>`); err != nil {
		t.Fatal(err)
	}
	srv.enqueueNativePush(u.Email, ChatEvent{Type: "message", Body: "test", From: "bob@example.com"})
	if _, err := srv.db.Exec(`UPDATE xmpp_push_jobs SET attempts=2147483647`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	srv.deliverPushBatch(context.Background(), now)
	var attempts int
	var due int64
	if err := srv.db.QueryRow(`SELECT attempts,next_attempt FROM xmpp_push_jobs`).Scan(&attempts, &due); err != nil {
		t.Fatal(err)
	}
	if publications != 1 || attempts != 2147483647 || due != now.Add(30*time.Minute).Unix() {
		t.Fatalf("overflow protection: published=%d attempts=%d due=%d", publications, attempts, due)
	}
	if err := srv.store.UpdateUserEnabled(context.Background(), u.ID, false); err != nil {
		t.Fatal(err)
	}
	srv.deliverPushBatch(context.Background(), time.Unix(due, 0))
	var registrations int
	if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_push_registrations`).Scan(&registrations); err != nil {
		t.Fatal(err)
	}
	if registrations != 0 || publications != 1 {
		t.Fatal("disabled account still receives push")
	}
}
