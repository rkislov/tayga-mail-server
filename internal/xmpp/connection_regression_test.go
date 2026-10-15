package xmpp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
)

func TestReplacingResourceDoesNotDeadlock(t *testing.T) {
	srv, store, u := testServer(t)
	srv.authn = auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	a, b := net.Pipe()
	defer b.Close()
	old := &Session{srv: srv, conn: a, bound: true, JID: ParseJID(u.Email + "/phone")}
	replacement := &Session{srv: srv, JID: old.JID}
	srv.hub.Register(old)
	done := make(chan struct{})
	go func() { srv.hub.Register(replacement); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("resource replacement deadlocked")
	}
	sessions := srv.hub.Sessions(u.Email)
	if len(sessions) != 1 || sessions[0] != replacement {
		t.Fatal("replacement was unregistered")
	}
	if !old.closed {
		t.Fatal("previous connection remains open")
	}
}

func TestRouteFullJIDOnlyToSelectedResource(t *testing.T) {
	h := NewHub()
	var phone, desktop bytes.Buffer
	h.Register(&Session{JID: ParseJID("alice@example.com/phone"), bw: bufio.NewWriter(&phone)})
	h.Register(&Session{JID: ParseJID("alice@example.com/desktop"), bw: bufio.NewWriter(&desktop)})
	if !h.Route("alice@example.com/phone", []byte("<message/>")) {
		t.Fatal("not delivered")
	}
	if phone.Len() == 0 || desktop.Len() != 0 {
		t.Fatal("full JID delivered to wrong resource")
	}
	if h.Route("alice@example.com/missing", []byte("<message/>")) {
		t.Fatal("missing resource reported delivered")
	}
}

func TestBindingRequiresAuthenticationAndCorrectNamespace(t *testing.T) {
	srv, _, u := testServer(t)
	for _, tc := range []struct {
		authed     bool
		body, want string
	}{
		{false, `<bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>phone</resource></bind>`, "not-authorized"},
		{true, `<bind xmlns='urn:wrong'><resource>phone</resource></bind>`, "bad-request"},
		{true, `<query xmlns='jabber:iq:roster'/>`, "bad-request"},
	} {
		var out bytes.Buffer
		s := &Session{srv: srv, authed: tc.authed, user: u, JID: ParseJID(u.Email), bw: bufio.NewWriter(&out)}
		s.dec = xml.NewDecoder(strings.NewReader(`<iq type='set' id='bind'>` + tc.body + `</iq>`))
		tok, err := s.dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		if err = s.handleBindOrSessionIQ(tok.(xml.StartElement), "example.com"); err != nil {
			t.Fatal(err)
		}
		if s.bound || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("unexpected binding response %s", out.String())
		}
	}
}

func TestIQDispatchUsesRootNamespaceAndIgnoresReplies(t *testing.T) {
	srv, _, u := testServer(t)
	for _, tc := range []struct{ typ, payload, want string }{
		{"get", `<ping xmlns='urn:xmpp:ping'/>`, "type='result'"},
		{"get", `<query xmlns='urn:unknown'><ping xmlns='urn:xmpp:ping'/></query>`, "feature-not-implemented"},
		{"get", `<ping xmlns='urn:wrong'>urn:xmpp:ping</ping>`, "feature-not-implemented"},
		{"get", `<query xmlns='urn:wrong'>jabber:iq:roster</query>`, "feature-not-implemented"},
		{"result", `<query xmlns='jabber:iq:roster'/>`, ""},
		{"error", `<ping xmlns='urn:xmpp:ping'/>`, ""},
		{"unknown", `<ping xmlns='urn:xmpp:ping'/>`, "bad-request"},
	} {
		t.Run(tc.typ+tc.payload, func(t *testing.T) {
			var out bytes.Buffer
			s := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/test"), bw: bufio.NewWriter(&out)}
			s.dec = xml.NewDecoder(strings.NewReader(`<iq type='` + tc.typ + `' id='test'>` + tc.payload + `</iq>`))
			token, err := s.dec.Token()
			if err != nil {
				t.Fatal(err)
			}
			if err = s.handleIQ(token.(xml.StartElement)); err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if out.Len() != 0 {
					t.Fatalf("reply loop: %s", out.String())
				}
			} else if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("got %s want %s", out.String(), tc.want)
			}
		})
	}
}
