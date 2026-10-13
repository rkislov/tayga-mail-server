package xmpp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Web rooms are local groups. They do not advertise XMPP MUC compatibility.
type ChatRoom struct {
	JID     string   `json:"jid"`
	Name    string   `json:"name"`
	Owner   string   `json:"owner"`
	Members []string `json:"members"`
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
	for i := range rooms {
		rooms[i].Members, err = s.roomMembers(ctx, rooms[i].JID)
		if err != nil {
			return nil, err
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
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM web_chat_members WHERE room_jid=? AND email=?`), jid, email).Scan(&n)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("group unavailable")
	}
	return nil
}
func (s *Server) CreateChatRoom(ctx context.Context, email, name string, members []string) (ChatRoom, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 || len(members) > 99 {
		return ChatRoom{}, fmt.Errorf("name (1–100 characters) and up to 99 participants required")
	}
	owner, err := s.store.GetUserByEmail(ctx, email)
	if err != nil || !owner.Enabled {
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
		if e != nil || !user.Enabled || user.TenantID != owner.TenantID {
			return ChatRoom{}, fmt.Errorf("participant unavailable: %s", member)
		}
		unique[member] = true
		all = append(all, member)
	}
	if len(all) < 2 {
		return ChatRoom{}, fmt.Errorf("choose at least one other participant")
	}
	room := ChatRoom{JID: randomID() + "@web-groups." + s.domain, Name: name, Owner: email, Members: all}
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
	}
	return room, nil
}
func (s *Server) LeaveChatRoom(ctx context.Context, email, jid string) error {
	if err := s.requireRoomMember(ctx, email, jid); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`DELETE FROM web_chat_members WHERE room_jid=? AND email=?`), jid, email)
	if err == nil {
		s.hub.pushWeb(email, ChatEvent{Type: "roster"})
	}
	return err
}
func (s *Server) roomHistory(ctx context.Context, email, jid string, limit int) ([]HistoryMsg, error) {
	if err := s.requireRoomMember(ctx, email, jid); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		limit = 80
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,sender,body,created_at FROM web_chat_messages WHERE room_jid=? ORDER BY created_at DESC,id DESC LIMIT ?`), jid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []HistoryMsg{}
	for rows.Next() {
		var m HistoryMsg
		var at string
		if err = rows.Scan(&m.ID, &m.From, &m.Body, &at); err != nil {
			return nil, err
		}
		m.To = jid
		m.At, _ = time.Parse(time.RFC3339Nano, at)
		messages = append(messages, m)
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, rows.Err()
}
func (s *Server) sendRoomMessage(ctx context.Context, email, jid, body string) (ChatEvent, error) {
	if err := s.requireRoomMember(ctx, email, jid); err != nil {
		return ChatEvent{}, err
	}
	if strings.TrimSpace(body) == "" || len(body) > 65536 {
		return ChatEvent{}, fmt.Errorf("message must contain 1–65536 bytes")
	}
	ev := ChatEvent{Type: "message", Room: jid, ID: randomID(), From: email, To: jid, Body: body, At: time.Now().UTC()}
	result, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO web_chat_messages(id,room_jid,sender,body,created_at) SELECT ?,?,?,?,? WHERE EXISTS(SELECT 1 FROM web_chat_members WHERE room_jid=? AND email=?)`), ev.ID, jid, email, body, ev.At.Format(time.RFC3339Nano), jid, email)
	if err != nil {
		return ChatEvent{}, err
	}
	if count, e := result.RowsAffected(); e != nil || count != 1 {
		return ChatEvent{}, fmt.Errorf("group unavailable")
	}
	members, err := s.roomMembers(ctx, jid)
	if err != nil {
		return ChatEvent{}, err
	}
	for _, member := range members {
		s.hub.pushWeb(member, ev)
	}
	return ev, nil
}
