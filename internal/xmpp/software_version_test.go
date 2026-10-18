package xmpp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/version"
)

func TestSoftwareVersionUsesQueriedDomain(t *testing.T) {
	srv, _, user := testServer(t)
	srv.domain = "mail.example.com"
	for _, tc := range []struct {
		to, from string
		ok       bool
	}{
		{"example.com", "example.com", true},
		{"mail.example.com", "mail.example.com", true},
		{"", "example.com", true},
		{"alice@example.com", "alice@example.com", false},
		{"remote.example.org", "remote.example.org", false},
	} {
		t.Run(tc.to, func(t *testing.T) {
			var out bytes.Buffer
			s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/monal"), bw: bufio.NewWriter(&out)}
			s.dec = xml.NewDecoder(strings.NewReader(`<iq type='get' id='version&amp;check' to='` + tc.to + `' from='forged.example.org'><query xmlns='jabber:iq:version'/></iq>`))
			token, err := s.dec.Token()
			if err != nil {
				t.Fatal(err)
			}
			if err := s.handleIQ(token.(xml.StartElement)); err != nil {
				t.Fatal(err)
			}
			var response struct {
				ID    string `xml:"id,attr"`
				Type  string `xml:"type,attr"`
				From  string `xml:"from,attr"`
				To    string `xml:"to,attr"`
				Query struct {
					Name    string `xml:"name"`
					Version string `xml:"version"`
					OS      string `xml:"os"`
				} `xml:"jabber:iq:version query"`
			}
			if err := xml.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.From != tc.from || response.To != s.JID.Full() || response.ID != "version&check" {
				t.Fatalf("invalid response addressing: %s", out.String())
			}
			if tc.ok {
				if response.Type != "result" || response.Query.Name != "Tayga Mail" || response.Query.Version != version.Version || response.Query.OS != "" {
					t.Fatalf("invalid server version: %s", out.String())
				}
			} else if response.Type != "error" || !strings.Contains(out.String(), "service-unavailable") || response.Query.Name != "" {
				t.Fatalf("answered for another entity: %s", out.String())
			}
		})
	}
}

func TestSoftwareVersionDiscoveryOnlyAtServer(t *testing.T) {
	srv, _, user := testServer(t)
	srv.domain = "mail.example.com"
	for _, target := range []string{"", "example.com", srv.domain, user.Email} {
		var out bytes.Buffer
		s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/monal"), bw: bufio.NewWriter(&out)}
		if err := s.writeDiscoInfo("features", target); err != nil {
			t.Fatal(err)
		}
		advertised := strings.Contains(out.String(), `<feature var='jabber:iq:version'/>`)
		if advertised != (target != user.Email) {
			t.Fatalf("target=%s: %s", target, out.String())
		}
	}
}
