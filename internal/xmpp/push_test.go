package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
)

func TestPushRegistrationPublishingAndAcknowledgements(t *testing.T) {
	srv, _, u := testServer(t)
	var out bytes.Buffer
	session := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	call := func(body, want string) {
		t.Helper()
		out.Reset()
		if err := session.handlePushIQ("p", "set", u.Email, body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), want) {
			t.Fatalf("want %s got %s", want, out.String())
		}
	}
	enable := `<enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='device'><x xmlns='jabber:x:data' type='submit'><field var='FORM_TYPE'><value>http://jabber.org/protocol/pubsub#publish-options</value></field><field var='secret'><value>token&amp;secret</value></field></x></enable>`
	call(enable, "service-unavailable")
	srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{Domain: "push.example.com", Secret: "shared", Push: true}}
	var delivered []string
	comp := &ComponentConn{Domain: "push.example.com", Send: func(raw []byte) { delivered = append(delivered, string(raw)) }}
	srv.hub.RegisterComponent(comp)
	if session.pushDiscoveryFeature(u.Email) == "" || session.pushDiscoveryFeature("bob@example.com") != "" {
		t.Fatal("discovery scope")
	}
	call(enable, "result")
	count := func(table string) int {
		t.Helper()
		var n int
		if err := srv.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	srv.hub.Register(session)
	event := ChatEvent{Type: "message", Body: "private message", From: "bob@example.com/phone", To: u.Email}
	srv.hub.pushWeb(u.Email, event)
	if count("xmpp_push_jobs") != 0 {
		t.Fatal("active device got push")
	}
	session.csi.inactive = true
	srv.hub.pushWeb(u.Email, event)
	if count("xmpp_push_jobs") != 1 {
		t.Fatal("missing queued push")
	}
	srv.deliverPushBatch(context.Background(), time.Now())
	if len(delivered) != 1 || strings.Contains(delivered[0], event.Body) || strings.Contains(delivered[0], event.From) || !strings.Contains(delivered[0], "token&amp;secret") {
		t.Fatalf("payload privacy/options: %s", delivered)
	}
	var iq struct {
		ID string `xml:"id,attr"`
	}
	xml.Unmarshal([]byte(delivered[0]), &iq)
	srv.acceptPushReply("forged.example.com", u.Email, iq.ID, "result", "")
	if count("xmpp_push_jobs") != 1 {
		t.Fatal("spoofed reply accepted")
	}
	srv.hub.pushWeb(u.Email, event)
	srv.acceptPushReply("push.example.com", u.Email, iq.ID, "result", "")
	if count("xmpp_push_jobs") != 1 {
		t.Fatal("new event lost while awaiting ack")
	}
	srv.deliverPushBatch(context.Background(), time.Now())
	xml.Unmarshal([]byte(delivered[1]), &iq)
	srv.acceptPushReply("push.example.com", u.Email, iq.ID, "result", "")
	if count("xmpp_push_jobs") != 0 {
		t.Fatal("ack did not dequeue")
	}
	srv.hub.pushWeb(u.Email, event)
	srv.deliverPushBatch(context.Background(), time.Now())
	xml.Unmarshal([]byte(delivered[2]), &iq)
	srv.acceptPushReply("push.example.com", u.Email, iq.ID, "error", `<error type='cancel'><item-not-found xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error>`)
	if count("xmpp_push_registrations") != 0 || count("xmpp_push_jobs") != 0 {
		t.Fatal("rejected registration retained")
	}
	call(enable, "result")
	srv.hub.pushWeb(u.Email, event)
	call(`<disable xmlns='urn:xmpp:push:0' jid='push.example.com'/>`, "result")
	if count("xmpp_push_registrations") != 0 || count("xmpp_push_jobs") != 0 {
		t.Fatal("disable failed")
	}
}
func TestPushRenewalPersistenceAndMalformedRequests(t *testing.T) {
	srv, store, u := testServer(t)
	srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{Domain: "push.example.com", Secret: "x", Push: true}}
	var wire, out bytes.Buffer
	srv.hub.RegisterComponent(&ComponentConn{Domain: "push.example.com", Send: func(b []byte) { wire.Write(b) }})
	session := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&out)}
	enable := `<enable xmlns='urn:xmpp:push:0' jid='push.example.com' node='n'/>`
	for _, bad := range []string{`<enable xmlns='urn:xmpp:push:0'/>`, `<enable xmlns='urn:xmpp:push:0' jid='push.example.com'><x xmlns='jabber:x:data' type='submit'/></enable>`, `<enable xmlns='urn:xmpp:push:0' jid='push.example.com'><wrong/></enable>`} {
		out.Reset()
		session.handlePushIQ("p", "set", "", bad)
		if !strings.Contains(out.String(), "bad-request") {
			t.Fatal(out.String())
		}
	}
	out.Reset()
	session.handlePushIQ("p", "set", "bob@example.com", enable)
	if !strings.Contains(out.String(), "forbidden") {
		t.Fatal(out.String())
	}
	session.handlePushIQ("p", "set", "", enable)
	event := ChatEvent{Type: "message", Encrypted: true, From: "bob@example.com/phone"}
	srv.enqueueNativePush(u.Email, event)
	// Reconstruct a server over the same database; queue and registration survive.
	restarted := New(srv.cfg, srv.log, store, nil, nil)
	restarted.hub = srv.hub
	restarted.deliverPushBatch(context.Background(), time.Now())
	var iq struct {
		ID string `xml:"id,attr"`
	}
	xml.Unmarshal(wire.Bytes(), &iq)
	if iq.ID == "" {
		t.Fatal("lost queued notification")
	}
	session.handlePushIQ("p", "set", "", enable) // re-enabling replaces old identity
	restarted.acceptPushReply("push.example.com", u.Email, iq.ID, "error", `<error type='cancel'/>`)
	var n int
	restarted.db.QueryRow("SELECT COUNT(*) FROM xmpp_push_registrations").Scan(&n)
	if n != 1 {
		t.Fatal("old reply removed renewed registration")
	}
	srv.enqueueNativePush(u.Email, event)
	for i := 0; i < 6; i++ {
		srv.deliverPushBatch(context.Background(), time.Now().Add(time.Duration(i)*time.Hour))
	}
	restarted.db.QueryRow("SELECT COUNT(*) FROM xmpp_push_registrations").Scan(&n)
	if n != 1 {
		t.Fatal("gateway timeout revoked registration")
	}
}
func TestPeerIQRouting(t *testing.T) {
	srv, _, u := testServer(t)
	var phone, other, sender bytes.Buffer
	source := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/desktop"), bw: bufio.NewWriter(&sender)}
	srv.hub.Register(&Session{JID: ParseJID("bob@example.com/phone"), bw: bufio.NewWriter(&phone)})
	srv.hub.Register(&Session{JID: ParseJID("bob@example.com/other"), bw: bufio.NewWriter(&other)})
	route := func(raw string) {
		t.Helper()
		source.dec = xml.NewDecoder(strings.NewReader(raw))
		tok, _ := source.dec.Token()
		if err := source.handleIQ(tok.(xml.StartElement)); err != nil {
			t.Fatal(err)
		}
	}
	route(`<iq type='get' id='device' from='mallory@example.com' to='bob@example.com/phone'><query xmlns='http://jabber.org/protocol/disco#info'/></iq>`)
	if phone.Len() == 0 || other.Len() != 0 || sender.Len() != 0 || strings.Contains(phone.String(), "mallory") || !strings.Contains(phone.String(), source.JID.Full()) {
		t.Fatal("wrong IQ route/sender", phone.String(), sender.String())
	}
	phone.Reset()
	route(`<iq type='result' id='device' to='bob@example.com/phone'><query xmlns='urn:test'/></iq>`)
	if phone.Len() == 0 {
		t.Fatal("reply not forwarded")
	}
	sender.Reset()
	route(`<iq type='get' id='missing' to='bob@example.com/absent'><ping xmlns='urn:xmpp:ping'/></iq>`)
	if !strings.Contains(sender.String(), "service-unavailable") {
		t.Fatal(sender.String())
	}
	sender.Reset()
	route(`<iq type='error' id='missing' to='bob@example.com/absent'/>`)
	if sender.Len() != 0 {
		t.Fatal("reply loop")
	}
	phone.Reset()
	route(`<iq type='get' id='many' to='bob@example.com/phone'><a/><b/></iq>`)
	if phone.Len() != 0 {
		t.Fatal("multi-payload request forwarded")
	}
}
