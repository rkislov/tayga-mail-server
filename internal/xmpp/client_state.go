package xmpp

import (
	"encoding/xml"
)

const csiNamespace = "urn:xmpp:csi:0"

type clientState struct {
	inactive bool
	presence map[string]string
	bytes    int
}

func (s *Session) handleClientState(start xml.StartElement) error {
	if err := s.dec.Skip(); err != nil {
		return err
	}
	if !s.authed || !s.bound {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch start.Name.Local {
	case "inactive":
		s.csi.inactive = true
	case "active":
		s.csi = clientState{}
	}
	return nil
}

// Suppress only identical, repeated availability updates while inactive. New
// presence, subscription requests and messages are always delivered immediately.
func (s *Session) redundantPresence(raw string) bool {
	if !s.csi.inactive {
		return false
	}
	var p struct {
		XMLName xml.Name
		From    string `xml:"from,attr"`
		Type    string `xml:"type,attr"`
	}
	if len(raw) > 8192 || xml.Unmarshal([]byte(raw), &p) != nil || p.XMLName.Local != "presence" || (p.XMLName.Space != "" && p.XMLName.Space != "jabber:client") || p.From == "" {
		return false
	}
	if p.Type != "" {
		s.csi.bytes -= len(s.csi.presence[p.From])
		delete(s.csi.presence, p.From)
		return false
	}
	if previous, ok := s.csi.presence[p.From]; ok && previous == raw {
		return true
	}
	if len(s.csi.presence) >= 128 || s.csi.bytes+len(raw) > 65536 {
		s.csi.presence = nil
		s.csi.bytes = 0
	}
	if s.csi.presence == nil {
		s.csi.presence = map[string]string{}
	}
	s.csi.bytes -= len(s.csi.presence[p.From])
	s.csi.presence[p.From] = raw
	s.csi.bytes += len(raw)
	return false
}
