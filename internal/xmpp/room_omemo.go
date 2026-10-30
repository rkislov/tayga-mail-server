package xmpp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrRoomChanged = errors.New("Состав группы изменился. Обновите устройства OMEMO и повторите отправку.")

type RoomOMEMOMember struct {
	JID  string `json:"jid"`
	Name string `json:"name"`
}
type RoomOMEMOContext struct {
	Room     string            `json:"room"`
	Revision string            `json:"revision"`
	Members  []RoomOMEMOMember `json:"members"`
}

func (s *Server) RoomOMEMOContext(ctx context.Context, actor, room string) (RoomOMEMOContext, error) {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	return s.roomOMEMOContext(ctx, actor, room)
}

// The snapshot includes account IDs and active membership, not merely addresses.
// Disabling/moving/recreating an account invalidates an in-flight web send too.
func (s *Server) roomOMEMOContext(ctx context.Context, actor, room string) (RoomOMEMOContext, error) {
	result := RoomOMEMOContext{Room: room, Members: []RoomOMEMOMember{}}
	if !s.isWebRoom(room) || ParseJID(room).Resource != "" {
		return result, ErrRoomAccess
	}
	if err := s.requireRoomMember(ctx, actor, room); err != nil {
		return result, err
	}
	var token string
	if err := s.db.QueryRowContext(ctx, s.rebind(`SELECT membership_token FROM web_chat_rooms WHERE jid=?`), room).Scan(&token); err != nil {
		return result, err
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT u.id,u.email,u.display_name FROM web_chat_members m
 JOIN web_chat_rooms r ON r.jid=m.room_jid JOIN users owner ON owner.email=r.owner_email JOIN users u ON u.email=m.email
 WHERE m.room_jid=? AND u.enabled=? AND owner.enabled=? AND u.tenant_id=owner.tenant_id AND u.auth_source<>'resource' ORDER BY u.email`), room, true, true)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	identities := [][2]string{}
	for rows.Next() {
		var id string
		var m RoomOMEMOMember
		if err = rows.Scan(&id, &m.JID, &m.Name); err != nil {
			return result, err
		}
		result.Members = append(result.Members, m)
		identities = append(identities, [2]string{id, m.JID})
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	raw, _ := json.Marshal([]any{room, token, identities})
	result.Revision = fmt.Sprintf("%x", sha256.Sum256(raw))
	return result, nil
}

func (s *Server) SendRoomOMEMO(ctx context.Context, actor, room, revision, raw string) (ChatEvent, error) {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	audience, err := s.roomOMEMOContext(ctx, actor, room)
	if err != nil {
		return ChatEvent{}, err
	}
	if revision == "" || revision != audience.Revision {
		return ChatEvent{}, ErrRoomChanged
	}
	cipher, err := validateWebCiphertext(raw)
	if err != nil {
		return ChatEvent{}, err
	}
	if err = s.checkOMEMOSender(ctx, actor, cipher); err != nil {
		return ChatEvent{}, err
	}
	return s.storeRoomMessage(ctx, actor, room, "", webRoomNick(actor), "", actor+"/web", cipher)
}

func roomCipherExtras(cipher string) string {
	if cipher == "" {
		return ""
	}
	return cipher + `<store xmlns='urn:xmpp:hints'/><encryption xmlns='urn:xmpp:eme:0' namespace='` + legacyOMEMO + `' name='OMEMO'/>`
}

// Also notify about offline members. Native clients use these updates to refresh
// the OMEMO recipient set; web senders additionally validate the snapshot above.
func (s *Server) notifyRoomAffiliation(ctx context.Context, room, email, affiliation string) {
	if affiliation == "none" {
		s.logBookmarkError(s.removeRoomBookmark(ctx, email, room))
	} else {
		s.logBookmarkError(s.syncRoomBookmarksLocked(ctx, email))
	}
	for session := range s.muc[room] {
		if s.requireRoomMember(ctx, session.JID.Bare(), room) != nil {
			continue
		}
		raw := fmt.Sprintf(`<message from='%s' to='%s'><x xmlns='%s#user'><item jid='%s' affiliation='%s' role='none'/></x></message>`, xmlEscape(room), xmlEscape(session.JID.Full()), mucNS, xmlEscape(email), xmlEscape(affiliation))
		session.SendRaw([]byte(raw))
	}
	members, err := s.roomMembers(ctx, room)
	if err == nil {
		for _, member := range members {
			s.hub.pushWeb(member, ChatEvent{Type: "roster"})
		}
	}
	if affiliation == "none" {
		s.hub.pushWeb(email, ChatEvent{Type: "roster"})
	}
}
