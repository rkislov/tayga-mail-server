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
	rooms        []mucResumeState
	presence     *ChatPresence
	state        streamManagement
	jid          JID
	userID       string
	credentialID string
	carbons      bool
	pepInterest  map[string]bool
	pepCaps      string
	pepDiscoID   string
	pepDiscoNode string
	pepDiscoAt   time.Time
	expires      time.Time
}

func (s *Session) retainDisconnectedStream() {
	// Keep the reader discoverable after Close unregisters its resource and
	// while presence/MUC cleanup is still running. A reconnect in that window
	// must wait for the snapshot instead of getting a spurious item-not-found.
	s.srv.resumeMu.Lock()
	if s.srv.closingStreams == nil {
		s.srv.closingStreams = make(map[*Session]struct{})
	}
	s.srv.closingStreams[s] = struct{}{}
	s.srv.resumeMu.Unlock()
	defer func() {
		s.srv.resumeMu.Lock()
		delete(s.srv.closingStreams, s)
		s.srv.resumeMu.Unlock()
	}()
	s.Close()
	s.retainStream()
}

// retainStream is called after the reader and socket have stopped.
// Offline delivery remains in the database while the resource is disconnected.
func (s *Session) retainStream() {
	s.mu.Lock()
	if !s.sm.resumable || !s.bound || s.user == nil {
		s.mu.Unlock()
		return
	}
	snapshot := resumableStream{state: s.sm, jid: s.JID, userID: s.user.ID, credentialID: s.credentialID, carbons: s.carbons, expires: time.Now().Add(resumeTTL)}
	snapshot.pepInterest = s.pepInterest // replaced, never mutated after publication
	snapshot.pepCaps, snapshot.pepDiscoID, snapshot.pepDiscoNode, snapshot.pepDiscoAt = s.pepCaps, s.pepDiscoID, s.pepDiscoNode, s.pepDiscoAt
	snapshot.rooms = append([]mucResumeState(nil), s.mucResume...)
	if s.lastPresence != nil {
		p := *s.lastPresence
		snapshot.presence = &p
	}
	s.mu.Unlock()
	_ = s.withDeviceAccess(func() error {
		s.srv.retainSnapshot(snapshot)
		return nil
	})
}

func (srv *Server) retainSnapshot(snapshot resumableStream) {
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
	_, ok := s.srv.resumable[token]
	s.srv.resumeMu.Unlock()
	if !ok {
		// On a network switch the new connection can arrive before TCP detects
		// the old one is gone. Finish that reader before taking its SM snapshot;
		// copying live counters would race with an in-flight stanza handler.
		if condition := s.finishPreviousStream(token, uint32(n)); condition != "" {
			return fail(condition)
		}
	}
	return s.withDeviceAccess(func() error { return s.resumeStoredStream(token, uint32(n), fail) })
}

func (s *Session) resumeStoredStream(token string, n uint32, fail func(string) error) error {
	// Serialize the archive boundary check and room rejoin with new messages.
	// Never confirm a resumed stream that would omit encrypted room history.
	s.srv.mucMu.Lock()
	roomsLocked := true
	defer func() {
		if roomsLocked {
			s.srv.mucMu.Unlock()
		}
	}()
	s.srv.resumeMu.Lock()
	old, ok := s.srv.resumable[token]
	if !ok || old.userID != s.user.ID || old.credentialID != s.credentialID || time.Now().After(old.expires) {
		s.srv.resumeMu.Unlock()
		return fail("item-not-found")
	}
	if uint32(n)-old.state.acknowledged > old.state.sent-old.state.acknowledged {
		s.srv.resumeMu.Unlock()
		return fail("bad-request")
	}
	delete(s.srv.resumable, token)
	s.srv.resumeMu.Unlock()
	for _, room := range old.rooms {
		var missed bool
		err := s.srv.db.QueryRowContext(context.Background(), s.srv.rebind(`SELECT EXISTS(SELECT 1 FROM web_chat_messages WHERE room_jid=? AND archive_seq>? AND omemo<>'')`), room.JID, room.ArchiveSeq).Scan(&missed)
		if err != nil || missed {
			// A full bind/join triggers the client's MAM catchup, whose forwarded
			// envelope carries the authoritative sender JID. A legacy join replay
			// cannot safely reconstruct OMEMO identities from reused nicknames.
			return fail("item-not-found")
		}
	}
	// A fresh bind of this resource supersedes a disconnected stream.
	for _, current := range s.srv.hub.Sessions(old.jid.Bare()) {
		if current.JID.Resource == old.jid.Resource {
			return fail("item-not-found")
		}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("connection closed")
	}
	s.JID = old.jid
	s.sm = old.state
	s.carbons = old.carbons
	s.lastPresence = old.presence
	s.pepInterest = old.pepInterest
	s.pepCaps, s.pepDiscoID, s.pepDiscoNode, s.pepDiscoAt = old.pepCaps, old.pepDiscoID, old.pepDiscoNode, old.pepDiscoAt
	s.bound = true
	s.sm.ack(uint32(n))
	replayed := len(s.sm.pending)
	err := s.replayStreamLocked(token)
	if err != nil {
		// A second disconnect during replay must not discard room memberships
		// that have not yet been re-established on this connection.
		s.mucResume = append([]mucResumeState(nil), old.rooms...)
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	for _, room := range old.rooms {
		if err := s.handleMUCPresenceLocked(room.JID+"/"+room.Nick, "", `<x xmlns='`+mucNS+`'><history maxstanzas='200' since='`+room.Since.Format(time.RFC3339Nano)+`'/></x>`); err != nil {
			return err
		}
	}
	s.srv.mucMu.Unlock()
	roomsLocked = false
	// Hub registration may close a superseded resource and acquire mucMu.
	if err := s.registerBoundSession(); err != nil {
		return err
	}
	s.touchSession()
	if old.presence != nil {
		s.srv.recordPresence(context.Background(), s.JID.Full(), old.presence.State, old.presence.Status)
	}
	if s.srv.log != nil {
		s.srv.log.Info("xmpp stream resumed", "user_id", s.user.ID, "resource", s.JID.Resource, "replayed_stanzas", replayed)
	}
	// These are new deliveries, so they receive fresh outgoing sequence numbers.
	return s.srv.flushOffline(context.Background(), s.user.ID, s.SendRaw)
}

// finishPreviousStream only considers a token belonging to this authenticated
// account. It closes the obsolete transport without waiting for a write to a
// possibly blackholed socket. The reader owns final counters and room cleanup.
func (s *Session) finishPreviousStream(token string, h uint32) string {
	candidates := s.srv.hub.Sessions(s.user.Email)
	s.srv.resumeMu.Lock()
	for previous := range s.srv.closingStreams {
		candidates = append(candidates, previous)
	}
	s.srv.resumeMu.Unlock()
	for _, previous := range candidates {
		if previous == s {
			continue
		}
		previous.mu.Lock()
		matches := previous.user != nil && previous.user.ID == s.user.ID && previous.credentialID == s.credentialID && previous.sm.resumable && previous.sm.token == token && previous.done != nil
		invalid := h-previous.sm.acknowledged > previous.sm.sent-previous.sm.acknowledged
		conn, done := previous.conn, previous.done
		previous.mu.Unlock()
		if !matches {
			continue
		}
		if invalid {
			return "bad-request"
		}
		if conn == nil {
			return "item-not-found"
		}
		_ = conn.Close()
		timer := time.NewTimer(stanzaWriteTimeout)
		defer timer.Stop()
		select {
		case <-done:
			return ""
		case <-timer.C:
			return "resource-constraint"
		}
	}
	return ""
}

// Replay uses one deadline for the complete batch. It bypasses writeLocked so
// retransmission neither advances counters nor inserts duplicate queue entries.
func (s *Session) replayStreamLocked(token string) error {
	if s.conn != nil {
		s.setWriteDeadline(time.Now().Add(stanzaWriteTimeout))
		defer s.setWriteDeadline(time.Time{})
	}
	if err := s.writeElementLocked(fmt.Sprintf(`<resumed xmlns='urn:xmpp:sm:3' previd='%s' h='%d'/>`, xmlEscape(token), s.sm.received)); err != nil {
		return err
	}
	for _, raw := range s.sm.pending {
		if err := s.writeElementLocked(raw); err != nil {
			return err
		}
	}
	if err := s.writeElementLocked(`<r xmlns='urn:xmpp:sm:3'/>`); err != nil {
		return err
	}
	return s.flushElementsLocked()
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

// DropDeviceStreams is called after credential revocation (or legacy-mode
// retirement). A concurrent retain is serialized by the auth access gate.
func (srv *Server) DropDeviceStreams(userID, credentialID string) {
	srv.resumeMu.Lock()
	defer srv.resumeMu.Unlock()
	for token, stream := range srv.resumable {
		if stream.userID == userID && stream.credentialID == credentialID {
			delete(srv.resumable, token)
		}
	}
}

func (s *Session) withDeviceAccess(fn func() error) error {
	if s.srv.authn == nil {
		return fn()
	} // Internal protocol fixtures without an auth layer.
	if s.user == nil {
		return fmt.Errorf("unauthenticated connection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.srv.authn.WithXMPPAccess(ctx, s.user.ID, s.credentialID, fn)
}
