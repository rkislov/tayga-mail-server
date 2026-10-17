package xmpp

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const smNamespace = "urn:xmpp:sm:3"

// Counters wrap modulo 2^32 as specified by XEP-0198. All access is under mu.
type streamManagement struct {
	resumable    bool
	token        string
	pending      []string
	bytes        int
	enabled      bool
	received     uint32
	sent         uint32
	acknowledged uint32
}

func isCountedStanza(raw string) bool {
	dec := xml.NewDecoder(strings.NewReader(raw))
	token, err := dec.Token()
	if err != nil {
		return false
	}
	start, ok := token.(xml.StartElement)
	if !ok || (start.Name.Space != "" && start.Name.Space != "jabber:client") {
		return false
	}
	return start.Name.Local == "message" || start.Name.Local == "presence" || start.Name.Local == "iq"
}

// writeLocked serializes stanza accounting with socket writes. SM elements are
// never counted; replay writes bypass this function to preserve sequence numbers.
func (s *Session) writeLocked(raw string) error {
	// A stalled mobile client must not hold routing or group locks indefinitely.
	if s.conn != nil {
		_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		defer s.conn.SetWriteDeadline(time.Time{})
	}
	counted := s.sm.enabled && isCountedStanza(raw)
	if counted && s.sm.sent-s.sm.acknowledged >= 1024 {
		s.sm.resumable = false
		return fmt.Errorf("stream management: acknowledgement window exhausted")
	}
	if counted && s.sm.resumable {
		if s.sm.bytes+len(raw) > 1<<20 {
			s.sm.resumable = false
			return fmt.Errorf("stream management: replay budget exhausted")
		}
		s.sm.pending = append(s.sm.pending, raw)
		s.sm.bytes += len(raw)
	}
	if counted {
		s.sm.sent++
	}
	if _, err := s.bw.WriteString(raw); err != nil {
		return err
	}
	if counted {
		if _, err := s.bw.WriteString(`<r xmlns='urn:xmpp:sm:3'/>`); err != nil {
			return err
		}
	}
	return s.bw.Flush()
}

func (s *Session) handleStreamManagement(start xml.StartElement) error {
	if err := s.dec.Skip(); err != nil {
		return err
	}
	if start.Name.Local == "resume" {
		return s.resumeStream(start)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fail := func(condition string) error {
		return s.writeLocked(`<failed xmlns='urn:xmpp:sm:3'><` + condition + ` xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></failed>`)
	}
	switch start.Name.Local {
	case "enable":
		if !s.authed || !s.bound {
			return fail("unexpected-request")
		}
		if s.sm.enabled {
			return fail("unexpected-request")
		}
		resume := attr(start, "resume")
		if resume != "" && resume != "true" && resume != "false" && resume != "1" && resume != "0" {
			return fail("bad-request")
		}
		s.sm = streamManagement{enabled: true}
		if (resume == "true" || resume == "1") && s.srv != nil && s.user != nil {
			s.sm.resumable = true
			s.sm.token = randomID()
			return s.writeLocked(fmt.Sprintf(`<enabled xmlns="urn:xmpp:sm:3" resume="true" id="%s" max="120"/>`, xmlEscape(s.sm.token)))
		}
		return s.writeLocked(`<enabled xmlns='urn:xmpp:sm:3' resume='false'/>`)
	case "resume":
		return fail("item-not-found")
	case "r":
		if !s.sm.enabled {
			return fail("unexpected-request")
		}
		return s.writeLocked(fmt.Sprintf(`<a xmlns='urn:xmpp:sm:3' h='%d'/>`, s.sm.received))
	case "a":
		if !s.sm.enabled {
			return fail("unexpected-request")
		}
		h, err := strconv.ParseUint(attr(start, "h"), 10, 32)
		if err != nil || uint32(h)-s.sm.acknowledged > s.sm.sent-s.sm.acknowledged {
			s.sm.resumable = false
			_ = s.writeLocked(`<stream:error><policy-violation xmlns='urn:ietf:params:xml:ns:xmpp-streams'/></stream:error>`)
			return fmt.Errorf("stream management: invalid acknowledgement")
		}
		s.sm.ack(uint32(h))
		return nil
	default:
		return fail("bad-request")
	}
}
