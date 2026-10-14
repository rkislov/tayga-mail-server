package xmpp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestStreamManagementAcknowledgements(t *testing.T) {
	var out bytes.Buffer
	s := &Session{authed: true, bound: true, bw: bufio.NewWriter(&out)}
	command := func(raw string) error {
		s.dec = xml.NewDecoder(strings.NewReader(raw))
		tok, err := s.dec.Token()
		if err != nil {
			return err
		}
		return s.handleStreamManagement(tok.(xml.StartElement))
	}
	if err := command(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "resume='false'") {
		t.Fatal("advertised unsupported resumption")
	}
	out.Reset()
	for _, raw := range []string{`<message><body>one</body></message>`, `<iq type='result' id='i'/>`, `<presence/>`} {
		if err := s.writeString(raw); err != nil {
			t.Fatal(err)
		}
	}
	if s.sm.sent != 3 || strings.Count(out.String(), "<r ") != 3 {
		t.Fatalf("counters %+v output %s", s.sm, out.String())
	}
	if err := command(`<a xmlns='urn:xmpp:sm:3' h='2'/>`); err != nil {
		t.Fatal(err)
	}
	if s.sm.acknowledged != 2 {
		t.Fatal("ack not applied")
	}
	s.sm.received = 7
	out.Reset()
	if err := command(`<r xmlns='urn:xmpp:sm:3'/>`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "h='7'") || s.sm.sent != 3 {
		t.Fatal(out.String())
	}
	if err := command(`<a xmlns='urn:xmpp:sm:3' h='4'/>`); err == nil {
		t.Fatal("accepted acknowledgement beyond sent count")
	}
}

func TestStreamManagementWrapAndLimits(t *testing.T) {
	var out bytes.Buffer
	s := &Session{authed: true, bound: true, bw: bufio.NewWriter(&out), sm: streamManagement{enabled: true, sent: ^uint32(0), acknowledged: ^uint32(0)}}
	if err := s.writeString(`<message/>`); err != nil {
		t.Fatal(err)
	}
	if s.sm.sent != 0 {
		t.Fatal("counter did not wrap")
	}
	s.dec = xml.NewDecoder(strings.NewReader(`<a xmlns='urn:xmpp:sm:3' h='0'/>`))
	tok, _ := s.dec.Token()
	if err := s.handleStreamManagement(tok.(xml.StartElement)); err != nil {
		t.Fatal(err)
	}
	s.sm.sent = 1024
	s.sm.acknowledged = 0
	if err := s.writeString(`<message/>`); err == nil {
		t.Fatal("unbounded outstanding stanzas")
	}
	for _, raw := range []string{`<r xmlns='urn:xmpp:sm:3'/>`, `<message xmlns='urn:other'/>`, `<stream:features xmlns:stream='http://etherx.jabber.org/streams'/>`} {
		if isCountedStanza(raw) {
			t.Fatalf("counted non-stanza %s", raw)
		}
	}
}
