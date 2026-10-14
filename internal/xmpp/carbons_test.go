package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestCarbonsNamespacesEligibilityAndDirectRecipient(t *testing.T) {
	hub := NewHub()
	var phone, desktop bytes.Buffer
	hub.Register(&Session{JID: ParseJID("alice@example.com/phone"), bw: bufio.NewWriter(&phone), carbons: true})
	hub.Register(&Session{JID: ParseJID("alice@example.com/desktop"), bw: bufio.NewWriter(&desktop), carbons: true})
	raw := `<message from='bob@example.com/mobile' to='alice@example.com/phone' type='chat'><body>Hello</body></message>`
	hub.Route("alice@example.com/phone", []byte(raw))
	if strings.Contains(phone.String(), "<received") || !strings.Contains(desktop.String(), "<received") {
		t.Fatal(phone.String(), desktop.String())
	}
	var envelope struct {
		XMLName  xml.Name
		Received struct {
			Forwarded struct {
				Message struct {
					XMLName xml.Name
					Body    string `xml:"jabber:client body"`
				} `xml:"jabber:client message"`
			} `xml:"urn:xmpp:forward:0 forwarded"`
		} `xml:"urn:xmpp:carbons:2 received"`
	}
	if err := xml.Unmarshal(desktop.Bytes(), &envelope); err != nil || envelope.Received.Forwarded.Message.Body != "Hello" {
		t.Fatalf("invalid forwarded namespace: %s, %v", desktop.String(), err)
	}
	for _, inner := range []string{`<private xmlns='urn:xmpp:carbons:2'/>`, `<no-copy xmlns='urn:xmpp:hints'/>`, `<sent xmlns='urn:xmpp:carbons:2'/>`} {
		_, _, ok := carbonMessage(`<message type='chat'>` + inner + `<body>secret</body></message>`)
		if ok {
			t.Fatal("private or nested carbon copied", inner)
		}
	}
	if _, _, ok := carbonMessage(`<message type='groupchat'><body>room</body></message>`); ok {
		t.Fatal("groupchat copied")
	}
}
func TestWebMessagesReachOtherCarbonsEnabledDevices(t *testing.T) {
	srv, _, u := testServer(t)
	var target bytes.Buffer
	srv.hub.Register(&Session{JID: ParseJID(u.Email + "/phone"), bw: bufio.NewWriter(&target), carbons: true})
	if _, err := srv.SendChatMessage(context.Background(), u.Email, "bob@example.com", "web message"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(target.String(), "<sent") || !strings.Contains(target.String(), "web message") {
		t.Fatal(target.String())
	}
}
func TestCarbonsIQRejectsMalformedCommands(t *testing.T) {
	var out bytes.Buffer
	s := &Session{JID: ParseJID("alice@example.com/phone"), bw: bufio.NewWriter(&out)}
	for _, raw := range []string{`<enabled xmlns='urn:xmpp:carbons:2'/>`, `<enable xmlns='wrong'/>`, `<enable xmlns='urn:xmpp:carbons:2'><disable/></enable>`} {
		out.Reset()
		_ = s.handleCarbonsIQ("x", "set", raw)
		if s.carbons || !strings.Contains(out.String(), "bad-request") {
			t.Fatal(out.String())
		}
	}
	_ = s.handleCarbonsIQ("x", "set", `<enable xmlns='urn:xmpp:carbons:2'/>`)
	if !s.carbons {
		t.Fatal("not enabled")
	}
	_ = s.handleCarbonsIQ("x", "set", `<disable xmlns='urn:xmpp:carbons:2'/>`)
	if s.carbons {
		t.Fatal("not disabled")
	}
}
func TestComponentReplacementDoesNotDeadlock(t *testing.T) {
	h := NewHub()
	old := &ComponentConn{Domain: "bots.example.com"}
	old.Close = func() { h.UnregisterComponent(old.Domain, old) }
	h.RegisterComponent(old)
	replacement := &ComponentConn{Domain: old.Domain}
	done := make(chan struct{})
	go func() { h.RegisterComponent(replacement); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("component replacement deadlock")
	}
	if h.Component(old.Domain) != replacement {
		t.Fatal("replacement lost")
	}
}
