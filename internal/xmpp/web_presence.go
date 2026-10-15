package xmpp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type ChatPresence struct {
	State  string `json:"state"`
	Status string `json:"status"`
}
type presenceState struct {
	ChatPresence
	Seen    time.Time
	Payload string
	ID      string
}

func (s *Server) presenceFor(bare string) ChatPresence {
	s.mu.Lock()
	defer s.mu.Unlock()
	best := ChatPresence{State: "offline"}
	for _, p := range s.presence[bare] {
		if p.State != "offline" {
			best = p.ChatPresence
			if p.State == "online" {
				break
			}
		}
	}
	return best
}
func (s *Server) recordPresence(ctx context.Context, full, state, status string) {
	s.recordPresencePayload(ctx, full, state, status, "", "")
}
func (s *Server) recordPresencePayload(ctx context.Context, full, state, status, payload, id string) {
	jid := ParseJID(full)
	s.mu.Lock()
	if s.presence == nil {
		s.presence = map[string]map[string]presenceState{}
	}
	if s.presence[jid.Bare()] == nil {
		s.presence[jid.Bare()] = map[string]presenceState{}
	}
	if state == "offline" {
		delete(s.presence[jid.Bare()], jid.Resource)
	} else {
		s.presence[jid.Bare()][jid.Resource] = presenceState{ChatPresence: ChatPresence{state, status}, Seen: time.Now(), Payload: payload, ID: id}
	}
	s.mu.Unlock()
	user, err := s.store.GetUserByEmail(ctx, jid.Bare())
	if err != nil {
		return
	}
	roster, err := s.listRoster(ctx, user.ID)
	if err != nil {
		return
	}
	p := s.presenceFor(jid.Bare())
	stanza := `<presence from='` + xmlEscape(full) + `'>`
	if state == "offline" {
		stanza = `<presence from='` + xmlEscape(full) + `' type='unavailable'>`
	} else if state != "online" {
		stanza += `<show>` + xmlEscape(state) + `</show>`
	}
	stanza += `<status>` + xmlEscape(status) + `</status></presence>`
	if payload != "" && state != "offline" {
		stanza = `<presence from='` + xmlEscape(full) + `'` + attrIf("id", id) + `>` + payload + `</presence>`
	}
	for _, it := range roster {
		if it.Subscription == "both" || it.Subscription == "from" {
			s.hub.Route(it.JID, []byte(stanza))
			s.hub.pushWeb(it.JID, ChatEvent{Type: "presence", From: jid.Bare(), To: it.JID, Body: p.State})
		}
	}
	s.hub.pushWeb(jid.Bare(), ChatEvent{Type: "presence", From: jid.Bare(), Body: p.State})
}
func (s *Server) SetChatPresence(ctx context.Context, userID, state, status string) error {
	if state != "online" && state != "away" && state != "dnd" && state != "offline" {
		return fmt.Errorf("invalid presence")
	}
	if len(status) > 280 {
		return fmt.Errorf("status too long")
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(ChatPresence{state, status})
	if err := s.store.PutSetting(ctx, "xmpp.presence."+user.ID, string(raw)); err != nil {
		return err
	}
	s.recordPresence(ctx, user.Email+"/web", state, status)
	return nil
}
func (s *Server) ChatSubscription(ctx context.Context, userID, peer, action string) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	to := ParseJID(peer)
	if to.Local == "" || to.Domain == "" {
		return fmt.Errorf("invalid JID")
	}
	switch action {
	case "request":
		if err = s.pendingSubscription(ctx, to.Bare(), user.Email); err != nil {
			return err
		}
		s.routeSubscription(ctx, to.Bare(), `<presence from='`+xmlEscape(user.Email)+`' to='`+xmlEscape(to.Bare())+`' type='subscribe'/>`)
	case "accept", "reject":
		var n int
		if err = s.db.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM xmpp_presence_requests WHERE owner_bare_jid=? AND from_bare_jid=?`), user.Email, to.Bare()).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("request unavailable")
		}
		typ := "unsubscribed"
		if action == "accept" {
			if err = s.changeSubscription(ctx, user.Email, to.Bare(), false, true); err != nil {
				return err
			}
			_ = s.changeSubscription(ctx, to.Bare(), user.Email, true, true)
			typ = "subscribed"
		}
		if _, err = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_presence_requests WHERE owner_bare_jid=? AND from_bare_jid=?`), user.Email, to.Bare()); err != nil {
			return err
		}
		s.routeSubscription(ctx, to.Bare(), `<presence from='`+xmlEscape(user.Email)+`' to='`+xmlEscape(to.Bare())+`' type='`+typ+`'/>`)
		s.mu.Lock()
		current, online := s.presence[user.Email]["web"]
		s.mu.Unlock()
		if online {
			s.recordPresence(ctx, user.Email+"/web", current.State, current.Status)
		}
	default:
		return fmt.Errorf("invalid action")
	}
	s.hub.pushWeb(user.Email, ChatEvent{Type: "roster", From: user.Email})
	return nil
}
func (s *Server) pendingSubscription(ctx context.Context, owner, from string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_presence_requests(owner_bare_jid,from_bare_jid) VALUES(?,?) ON CONFLICT(owner_bare_jid,from_bare_jid) DO NOTHING`), owner, from)
	if err == nil {
		s.hub.pushWeb(owner, ChatEvent{Type: "roster", From: from, To: owner})
	}
	return err
}

func extractPresenceShow(inner string) string {
	var p struct {
		Show string `xml:"show"`
	}
	_ = xml.Unmarshal([]byte("<presence>"+inner+"</presence>"), &p)
	if p.Show == "away" || p.Show == "xa" {
		return "away"
	}
	if p.Show == "dnd" {
		return "dnd"
	}
	return "online"
}
func (s *Server) rosterDisplayName(ctx context.Context, ownerID, jid, fallback string) string {
	if strings.TrimSpace(fallback) != "" && fallback != jid {
		return fallback
	}
	owner, e := s.store.GetUserByID(ctx, ownerID)
	if e != nil {
		return fallback
	}
	user, e := s.store.GetUserByEmail(ctx, jid)
	if e != nil || user.TenantID != owner.TenantID || !user.Enabled {
		return fallback
	}
	if strings.TrimSpace(user.DisplayName) != "" {
		return user.DisplayName
	}
	return fallback
}

func (s *Server) routeSubscription(ctx context.Context, to, stanza string) {
	if !s.hub.Route(to, []byte(stanza)) {
		_ = s.storeOffline(ctx, to, stanza)
	}
}
func (s *Server) clearPresenceRequest(ctx context.Context, owner, from string) {
	_, _ = s.db.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_presence_requests WHERE owner_bare_jid=? AND from_bare_jid=?`), owner, from)
	s.hub.pushWeb(owner, ChatEvent{Type: "roster", From: owner})
}
func (s *Server) savedWebPresence(ctx context.Context, bare string) ChatPresence {
	p := ChatPresence{State: "online"}
	if u, err := s.store.GetUserByEmail(ctx, bare); err == nil {
		if raw, ok, err := s.store.GetSetting(ctx, "xmpp.presence."+u.ID); err == nil && ok {
			_ = json.Unmarshal([]byte(raw), &p)
		}
	}
	return p
}
