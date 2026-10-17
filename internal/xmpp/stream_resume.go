package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"time"
)

const resumeTTL = 2 * time.Minute
const resumeBudget = 16 << 20

type resumableStream struct {
	presence *ChatPresence
	state    streamManagement
	jid      JID
	userID   string
	carbons  bool
	expires  time.Time
}

// retainStream is called after the reader and socket have stopped.
// Offline delivery remains in the database while the resource is disconnected.
func (s *Session) retainStream() {
	s.mu.Lock()
	if !s.sm.resumable || !s.bound || s.user == nil {
		s.mu.Unlock()
		return
	}
	snapshot := resumableStream{state: s.sm, jid: s.JID, userID: s.user.ID, carbons: s.carbons, expires: time.Now().Add(resumeTTL)}
	if s.lastPresence != nil {
		p := *s.lastPresence
		snapshot.presence = &p
	}
	s.mu.Unlock()
	srv := s.srv
	srv.resumeMu.Lock()
	defer srv.resumeMu.Unlock()
	if srv.resumable == nil {
		srv.resumable = make(map[string]resumableStream)
	}
	size, count := 0, 0
	for token, r := range srv.resumable {
		if time.Now().After(r.expires) {
			delete(srv.resumable, token)
			continue
		}
		size += r.state.bytes
		if r.userID == snapshot.userID {
			count++
		}
	}
	if count >= 4 || len(srv.resumable) >= 256 || size+snapshot.state.bytes > resumeBudget {
		return
	}
	srv.resumable[snapshot.state.token] = snapshot
	token := snapshot.state.token
	time.AfterFunc(resumeTTL, func() {
		srv.resumeMu.Lock()
		defer srv.resumeMu.Unlock()
		if current, ok := srv.resumable[token]; ok && !time.Now().Before(current.expires) {
			delete(srv.resumable, token)
		}
	})
}

func (s *Session) resumeStream(start xml.StartElement) error {
	fail := func(condition string) error {
		return s.writeString(`<failed xmlns='urn:xmpp:sm:3'><` + condition + ` xmlns='urn:ietf:params:xml:ns:xmpp-stanzas'/></failed>`)
	}
	if !s.authed || s.user == nil || s.bound || s.srv == nil {
		return fail("unexpected-request")
	}
	n, err := strconv.ParseUint(attr(start, "h"), 10, 32)
	if err != nil {
		return fail("bad-request")
	}
	token := attr(start, "previd")
	s.srv.resumeMu.Lock()
	old, ok := s.srv.resumable[token]
	if !ok || old.userID != s.user.ID || time.Now().After(old.expires) {
		s.srv.resumeMu.Unlock()
		return fail("item-not-found")
	}
	if uint32(n)-old.state.acknowledged > old.state.sent-old.state.acknowledged {
		s.srv.resumeMu.Unlock()
		return fail("bad-request")
	}
	delete(s.srv.resumable, token)
	s.srv.resumeMu.Unlock()
	// A fresh bind of this resource supersedes a disconnected stream.
	for _, current := range s.srv.hub.Sessions(old.jid.Bare()) {
		if current.JID.Resource == old.jid.Resource {
			return fail("item-not-found")
		}
	}
	s.mu.Lock()
	s.JID = old.jid
	s.sm = old.state
	s.carbons = old.carbons
	s.lastPresence = old.presence
	s.bound = true
	s.sm.ack(uint32(n))
	_, err = s.bw.WriteString(fmt.Sprintf(`<resumed xmlns='urn:xmpp:sm:3' previd='%s' h='%d'/>`, xmlEscape(token), s.sm.received))
	if err == nil {
		for _, raw := range s.sm.pending {
			if _, err = s.bw.WriteString(raw); err != nil {
				break
			}
		}
	}
	if err == nil {
		_, err = s.bw.WriteString(`<r xmlns='urn:xmpp:sm:3'/>`)
	}
	if err == nil {
		err = s.bw.Flush()
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.srv.hub.Register(s)
	s.touchSession()
	if old.presence != nil {
		s.srv.recordPresence(context.Background(), s.JID.Full(), old.presence.State, old.presence.Status)
	}
	// These are new deliveries, so they receive fresh outgoing sequence numbers.
	return s.srv.flushOffline(context.Background(), s.user.ID, s.SendRaw)
}

func (sm *streamManagement) ack(h uint32) {
	count := int(h - sm.acknowledged)
	if sm.resumable && count <= len(sm.pending) {
		for _, raw := range sm.pending[:count] {
			sm.bytes -= len(raw)
		}
		copy(sm.pending, sm.pending[count:])
		clear(sm.pending[len(sm.pending)-count:])
		sm.pending = sm.pending[:len(sm.pending)-count]
	}
	sm.acknowledged = h
}
