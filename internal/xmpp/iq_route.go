package xmpp

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Full-JID IQs belong to the addressed client, not the server command dispatcher.
// The sender is always the authenticated stream JID; client-supplied from is
// ignored. Bare account IQs remain server-side (PEP, vCard and discovery).
func (s *Session) routePeerIQ(id, typ, to, inner string) (bool, error) {
	if to == "" {
		return false, nil
	}
	j, err := NormalizeJID(to)
	if err != nil {
		if typ == "result" || typ == "error" {
			return true, nil
		}
		return true, s.writeString(iqBadRequest(id))
	}
	comp := s.srv.hub.Component(j.Domain)
	if !j.IsFull() && comp == nil {
		return false, nil
	}
	stanza := []byte(fmt.Sprintf(`<iq xmlns='jabber:client' from='%s' to='%s' id='%s' type='%s'>%s</iq>`, xmlEscape(s.JID.Full()), xmlEscape(j.Full()), xmlEscape(id), xmlEscape(typ), inner))
	if s.srv.hub.Route(j.Full(), stanza) {
		return true, nil
	}
	if typ == "result" || typ == "error" {
		return true, nil
	}
	return true, s.writeString(fmt.Sprintf(`<iq type='error' from='%s' to='%s' id='%s'>%s<error type='cancel'><service-unavailable xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></error></iq>`, xmlEscape(j.Full()), xmlEscape(s.JID.Full()), xmlEscape(id), inner))
}
func iqPayloadName(inner string) (xml.Name, bool) {
	d := xml.NewDecoder(strings.NewReader(inner))
	var name xml.Name
	count := 0
	for {
		tok, e := d.Token()
		if e != nil {
			return name, e == io.EOF && count == 1
		}
		if start, ok := tok.(xml.StartElement); ok {
			count++
			name = start.Name
			if e = d.Skip(); e != nil {
				return name, false
			}
		} else if text, ok := tok.(xml.CharData); ok && strings.TrimSpace(string(text)) != "" {
			return name, false
		}
	}
}
