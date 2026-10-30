package xmpp

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// Web participants use a reserved nick: a native client cannot impersonate the
// web occupant and bind another account's Signal session to that nickname.
const webRoomNickPrefix = "tayga-web-"

func webRoomNick(email string) string {
	sum := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%s%x", webRoomNickPrefix, sum[:16])
}

type webRoomOccupant struct {
	session  *Session
	presence mucOccupant
}

// Caller holds mucMu. A web occupant lives until the last SSE stream closes.
// HTTP-only senders join for this send and leave immediately after delivery.
func (s *Server) ensureWebRoomPresence(ctx context.Context, room, email string) (bool, error) {
	if _, ok := s.mucWeb[room][email]; ok {
		return false, nil
	}
	r, err := s.getRoom(ctx, room)
	if err != nil {
		return false, err
	}
	aff := s.roomAffiliation(ctx, r, email)
	if aff == "none" {
		return false, ErrRoomAccess
	}
	entry := webRoomOccupant{session: &Session{JID: ParseJID(email + "/web")}, presence: mucOccupant{Nick: webRoomNick(email), Affiliation: aff}}
	if s.mucWeb == nil {
		s.mucWeb = map[string]map[string]webRoomOccupant{}
	}
	if s.mucWeb[room] == nil {
		s.mucWeb[room] = map[string]webRoomOccupant{}
	}
	s.mucWeb[room][email] = entry
	for other := range s.muc[room] {
		if s.requireRoomMember(ctx, other.JID.Bare(), room) == nil {
			other.SendRaw([]byte(mucPresence(room, entry.session, entry.presence, other.JID.Full(), false, "")))
		}
	}
	s.hub.mu.RLock()
	subscribed := len(s.hub.web[email]) > 0
	s.hub.mu.RUnlock()
	return !subscribed, nil
}
func (s *Server) removeWebRoomPresence(room, email string, removed bool) {
	entry, ok := s.mucWeb[room][email]
	if !ok {
		return
	}
	delete(s.mucWeb[room], email)
	if len(s.mucWeb[room]) == 0 {
		delete(s.mucWeb, room)
	}
	if removed {
		entry.presence.Affiliation = "none"
	}
	for other := range s.muc[room] {
		if s.requireRoomMember(context.Background(), other.JID.Bare(), room) == nil {
			other.SendRaw([]byte(mucPresence(room, entry.session, entry.presence, other.JID.Full(), true, "")))
		}
	}
}
func (s *Server) leaveWebRooms(email string) {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	for room := range s.mucWeb {
		s.removeWebRoomPresence(room, email, false)
	}
}
