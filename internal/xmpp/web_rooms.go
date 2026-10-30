package xmpp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Local persistent, members-only rooms are shared by web clients and XMPP MUC.
type ChatRoom struct {
	Affiliations map[string]string `json:"affiliations"`
	JID          string            `json:"jid"`
	Name         string            `json:"name"`
	Owner        string            `json:"owner"`
	Members      []string          `json:"members"`
}

func (s *Server) isWebRoom(jid string) bool { return ParseJID(jid).Domain == "web-groups."+s.domain }
func (s *Server) ListChatRooms(ctx context.Context, email string) ([]ChatRoom, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT r.jid,r.name,r.owner_email FROM web_chat_rooms r JOIN web_chat_members m ON m.room_jid=r.jid WHERE m.email=? ORDER BY r.name`), email)
	if err != nil {
		return nil, err
	}
	rooms := []ChatRoom{}
	for rows.Next() {
		var r ChatRoom
		if err = rows.Scan(&r.JID, &r.Name, &r.Owner); err != nil {
			rows.Close()
			return nil, err
		}
		rooms = append(rooms, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	visible := rooms[:0]
	for _, room := range rooms {
		if s.requireRoomMember(ctx, email, room.JID) == nil {
			visible = append(visible, room)
		}
	}
	rooms = visible
	for i := range rooms {
		rooms[i].Members, err = s.roomMembers(ctx, rooms[i].JID)
		if err != nil {
			return nil, err
		}
		rooms[i].Affiliations = map[string]string{}
		for _, member := range rooms[i].Members {
			rooms[i].Affiliations[member] = s.roomAffiliation(ctx, mucRoom{JID: rooms[i].JID, Owner: rooms[i].Owner}, member)
		}
	}
	return rooms, nil
}
func (s *Server) roomMembers(ctx context.Context, jid string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT email FROM web_chat_members WHERE room_jid=? ORDER BY email`), jid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []string{}
	for rows.Next() {
		var email string
		if err = rows.Scan(&email); err != nil {
			return nil, err
		}
		members = append(members, email)
	}
	return members, rows.Err()
}
func (s *Server) requireRoomMember(ctx context.Context, email, jid string) error {
	var n int
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM web_chat_members m
 JOIN web_chat_rooms r ON r.jid=m.room_jid JOIN users u ON u.email=m.email JOIN users owner ON owner.email=r.owner_email
 WHERE m.room_jid=? AND m.email=? AND u.enabled=? AND owner.enabled=? AND u.tenant_id=owner.tenant_id AND u.auth_source<>'resource'`), jid, email, true, true).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrRoomAccess
	}
	return nil
}

func (s *Server) CreateChatRoom(ctx context.Context, email, name string, members []string) (ChatRoom, error) {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	return s.createChatRoom(ctx, email, name, members, "")
}
func (s *Server) createChatRoom(ctx context.Context, email, name string, members []string, requested string) (ChatRoom, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 || len(members) > 99 {
		return ChatRoom{}, fmt.Errorf("name (1–100 characters) and up to 99 participants required")
	}
	owner, err := s.store.GetUserByEmail(ctx, email)
	if err != nil || !owner.Enabled || owner.AuthSource == "resource" {
		return ChatRoom{}, fmt.Errorf("account unavailable")
	}
	unique := map[string]bool{email: true}
	all := []string{email}
	for _, member := range members {
		member = strings.ToLower(strings.TrimSpace(member))
		if unique[member] {
			continue
		}
		user, e := s.store.GetUserByEmail(ctx, member)
		if e != nil || !user.Enabled || user.TenantID != owner.TenantID || user.AuthSource == "resource" {
			return ChatRoom{}, fmt.Errorf("participant unavailable: %s", member)
		}
		unique[member] = true
		all = append(all, member)
	}
	if len(all) < 2 && requested == "" {
		return ChatRoom{}, fmt.Errorf("choose at least one other participant")
	}
	room := ChatRoom{JID: randomID() + "@web-groups." + s.domain, Name: name, Owner: email, Members: all}
	if requested != "" {
		room.JID = requested
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ChatRoom{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO web_chat_rooms(jid,name,owner_email) VALUES(?,?,?)`), room.JID, name, email); err != nil {
		return ChatRoom{}, err
	}
	for _, member := range all {
		if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO web_chat_members(room_jid,email) VALUES(?,?)`), room.JID, member); err != nil {
			return ChatRoom{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return ChatRoom{}, err
	}
	for _, member := range all {
		s.hub.pushWeb(member, ChatEvent{Type: "roster"})
		if member != email {
			s.inviteRoomMember(email, member, room.JID)
		}
	}
	return room, nil
}
func (s *Server) LeaveChatRoom(ctx context.Context, email, jid string) error {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	room, err := s.getRoom(ctx, jid)
	if err != nil {
		return err
	}
	if room.Owner == email {
		return fmt.Errorf("transfer ownership or delete the group before leaving")
	}
	if err := s.requireRoomMember(ctx, email, jid); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, s.rebind(`DELETE FROM web_chat_members WHERE room_jid=? AND email=?`), jid, email)
	if err == nil {
		s.evictRoomMember(jid, email, "321")
		s.hub.pushWeb(email, ChatEvent{Type: "roster"})
	}
	return err
}
func (s *Server) roomHistory(ctx context.Context, email, jid string, limit int) ([]HistoryMsg, error) {
	if limit < 1 || limit > 200 {
		limit = 80
	}
	end := ""
	page, err := s.roomArchivePage(ctx, email, jid, mamFilter{Before: &end, Limit: limit})
	return page.Messages, err
}

func (s *Server) sendRoomMessage(ctx context.Context, email, jid, body string) (ChatEvent, error) {
	s.mucMu.Lock()
	defer s.mucMu.Unlock()
	return s.sendRoomMessageAs(ctx, email, jid, body, email, "", email)
}
func (s *Server) sendRoomMessageAs(ctx context.Context, email, jid, body, nick, originID, senderJID string) (ChatEvent, error) {
	if err := s.requireRoomMember(ctx, email, jid); err != nil {
		return ChatEvent{}, err
	}
	if strings.TrimSpace(body) == "" || len(body) > 65536 {
		return ChatEvent{}, fmt.Errorf("message must contain 1–65536 bytes")
	}
	ev := ChatEvent{Type: "message", Room: jid, ID: randomID(), From: email, To: jid, Body: body, At: time.Now().UTC()}
	if sender, e := s.store.GetUserByEmail(ctx, email); e == nil {
		ev.SenderName = strings.TrimSpace(sender.DisplayName)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ChatEvent{}, err
	}
	defer tx.Rollback()
	seq, err := s.roomArchiveSequence(ctx, tx, jid)
	if err != nil {
		return ChatEvent{}, err
	}
	seq++
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE web_chat_rooms SET next_archive_seq=? WHERE jid=?`), seq, jid); err != nil {
		return ChatEvent{}, err
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO web_chat_messages(id,room_jid,sender,body,created_at,nick,origin_id,sender_jid,archive_seq) VALUES(?,?,?,?,?,?,?,?,?)`), ev.ID, jid, email, body, ev.At.Format(roomStampLayout), nick, originID, senderJID, seq); err != nil {
		return ChatEvent{}, err
	}
	if err = tx.Commit(); err != nil {
		return ChatEvent{}, err
	}

	members, err := s.roomMembers(ctx, jid)
	if err != nil {
		return ChatEvent{}, err
	}
	for _, member := range members {
		if s.requireRoomMember(ctx, member, jid) == nil {
			s.hub.pushWeb(member, ev)
		}
	}
	s.broadcastRoomMessage(ev, nick, originID)
	return ev, nil
}
