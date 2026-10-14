package xmpp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestInactiveClientOnlySuppressesDuplicateAvailability(t *testing.T) {
	var out bytes.Buffer
	s := &Session{authed: true, bound: true, bw: bufio.NewWriter(&out)}
	state := func(name string) {
		s.dec = xml.NewDecoder(strings.NewReader(`<` + name + ` xmlns='urn:xmpp:csi:0'/>`))
		tok, _ := s.dec.Token()
		if err := s.handleClientState(tok.(xml.StartElement)); err != nil {
			t.Fatal(err)
		}
	}
	state("inactive")
	available := []byte(`<presence from='bob@example.com/phone'><show>away</show></presence>`)
	s.SendRaw(available)
	s.SendRaw(available)
	if strings.Count(out.String(), "<presence") != 1 {
		t.Fatal(out.String())
	}
	s.SendRaw([]byte(`<message from='bob@example.com/phone'><body>hello</body></message>`))
	s.SendRaw([]byte(`<presence from='bob@example.com/phone' type='subscribe'/>`))
	s.SendRaw(available)
	if strings.Count(out.String(), "<presence") != 3 || !strings.Contains(out.String(), "hello") {
		t.Fatal(out.String())
	}
	state("active")
	s.SendRaw(available)
	if strings.Count(out.String(), "<presence") != 4 {
		t.Fatal(out.String())
	}
}
