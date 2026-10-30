package xmpp

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ChatEvent is pushed to web clients over SSE.
type ChatEvent struct {
	OMEMO      string          `json:"omemo,omitempty"`
	Room       string          `json:"room,omitempty"`
	Attachment *ChatAttachment `json:"attachment,omitempty"`
	Encrypted  bool            `json:"encrypted,omitempty"`
	Type       string          `json:"type"`
	ID         string          `json:"id,omitempty"`
	From       string          `json:"from"`
	To         string          `json:"to"`
	Body       string          `json:"body"`
	At         time.Time       `json:"at"`
}

// RosterEntry is a roster row for the web messenger.
type RosterEntry struct {
	Group               bool   `json:"group,omitempty"`
	JID                 string `json:"jid"`
	Name                string `json:"name"`
	Subscription        string `json:"subscription"`
	Presence            string `json:"presence"`
	SubscriptionRequest bool   `json:"subscription_request"`
}

// HistoryMsg is a decoded MAM / chat history item.
type HistoryMsg struct {
	SenderJID  string          `json:"-"`
	Nick       string          `json:"-"`
	OriginID   string          `json:"-"`
	OMEMO      string          `json:"omemo,omitempty"`
	Attachment *ChatAttachment `json:"attachment,omitempty"`
	Encrypted  bool            `json:"encrypted,omitempty"`
	ID         string          `json:"id"`
	From       string          `json:"from"`
	To         string          `json:"to"`
	Body       string          `json:"body"`
	At         time.Time       `json:"at"`
	Stanza     string          `json:"-"`
}

type webSub struct {
	ch chan ChatEvent
}

func (h *Hub) subscribeWeb(bare string) (<-chan ChatEvent, func()) {
	bare = ParseJID(bare).Bare()
	sub := &webSub{ch: make(chan ChatEvent, 32)}
	h.mu.Lock()
	if h.web == nil {
		h.web = make(map[string]map[*webSub]struct{})
	}
	if h.web[bare] == nil {
		h.web[bare] = make(map[*webSub]struct{})
	}
	h.web[bare][sub] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			if m := h.web[bare]; m != nil {
				delete(m, sub)
				if len(m) == 0 {
					delete(h.web, bare)
				}
			}
			close(sub.ch)
			h.mu.Unlock()
		})
	}
	return sub.ch, unsub
}

func (h *Hub) pushWeb(bare string, ev ChatEvent) {
	h.mu.RLock()
	subs := h.web[ParseJID(bare).Bare()]
	list := make([]*webSub, 0, len(subs))
	for s := range subs {
		list = append(list, s)
	}
	for _, s := range list {
		select {
		case s.ch <- ev:
		default:
		}
	}
	sink := h.notificationSink
	nativeSink := h.nativePushSink
	h.mu.RUnlock()
	if nativeSink != nil {
		nativeSink(ParseJID(bare).Bare(), ev)
	}
	if sink != nil && ev.Type == "message" && ParseJID(ev.From).Bare() != ParseJID(bare).Bare() {
		sink(ParseJID(bare).Bare(), ev)
	}

}

func (s *Server) ListChatRoster(ctx context.Context, userID string) ([]RosterEntry, error) {
	items, err := s.effectiveRoster(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Conversations remain visible even when the peer is not a saved contact.
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT with_bare_jid FROM xmpp_mam WHERE owner_bare_jid = ? GROUP BY with_bare_jid ORDER BY MAX(created_at) DESC`), ParseJID(user.Email).Bare())
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		seen := make(map[string]bool)
		for _, item := range items {
			seen[ParseJID(item.JID).Bare()] = true
		}
		for rows.Next() {
			var peer string
			if err := rows.Scan(&peer); err != nil {
				return nil, err
			}
			peer = ParseJID(peer).Bare()
			if peer != "" && peer != ParseJID(user.Email).Bare() && !seen[peer] {
				items = append(items, rosterItem{JID: peer, Subscription: "none"})
				seen[peer] = true
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	requests := map[string]bool{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT from_bare_jid FROM xmpp_presence_requests WHERE owner_bare_jid=?`), user.Email)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var peer string
			if err = rows.Scan(&peer); err != nil {
				rows.Close()
				return nil, err
			}
			requests[peer] = true
			found := false
			for _, item := range items {
				if item.JID == peer {
					found = true
				}
			}
			if !found {
				items = append(items, rosterItem{JID: peer, Subscription: "none"})
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	out := make([]RosterEntry, 0, len(items))
	for _, it := range items {
		name := s.rosterDisplayName(ctx, userID, it.JID, it.Name)
		presence := "unknown"
		if it.Subscription == "to" || it.Subscription == "both" {
			presence = s.presenceFor(it.JID).State
		}
		out = append(out, RosterEntry{JID: it.JID, Name: name, Subscription: it.Subscription, Presence: presence, SubscriptionRequest: requests[it.JID]})
	}
	rooms, err := s.ListChatRooms(ctx, user.Email)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		out = append(out, RosterEntry{JID: room.JID, Name: room.Name, Group: true})
	}
	return out, nil
}

func (s *Server) UpsertChatRoster(ctx context.Context, userID, jid, name string) error {
	peer := ParseJID(jid)
	if peer.Local == "" || peer.Domain == "" {
		return fmt.Errorf("invalid JID")
	}
	items, err := s.listRoster(ctx, userID)
	if err != nil {
		return err
	}
	item := rosterItem{JID: peer.Bare(), Subscription: "none", GroupsJSON: "[]"}
	for _, existing := range items {
		if existing.JID == peer.Bare() {
			item = existing
			break
		}
	}
	if name != "" {
		item.Name = name
	}
	return s.upsertRoster(ctx, userID, item)
}

func (s *Server) ChatHistory(ctx context.Context, ownerBare, withBare string, limit int) ([]HistoryMsg, error) {
	if s.isWebRoom(withBare) {
		s.mucMu.Lock()
		defer s.mucMu.Unlock()
		return s.roomHistory(ctx, ownerBare, withBare, limit)
	}
	rows, err := s.queryMAM(ctx, ParseJID(ownerBare).Bare(), ParseJID(withBare).Bare(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryMsg, 0, len(rows))
	for _, r := range rows {
		from, to := parseMessageAddrs(r.Stanza)
		attachment, encrypted := stanzaAttachment(r.Stanza)
		out = append(out, HistoryMsg{
			ID: r.StanzaID, From: from, To: to, Body: extractBody(r.Stanza), OMEMO: encryptedElement(r.Stanza),
			At: r.CreatedAt, Stanza: r.Stanza, Attachment: attachment, Encrypted: encrypted,
		})
	}
	return out, nil
}

func (s *Server) SendChatMessage(ctx context.Context, fromEmail, to, body string) (ChatEvent, error) {
	if s.isWebRoom(to) {
		return s.sendRoomMessage(ctx, fromEmail, to, body)
	}
	from := ParseJID(fromEmail).Bare()
	toJ := ParseJID(to)
	if toJ.Bare() == "" {
		return ChatEvent{}, fmt.Errorf("invalid recipient")
	}
	id := randomID()[:12]
	bodyXML := ""
	if body != "" {
		bodyXML = `<body>` + xmlEscape(body) + `</body>`
	}
	if attachment, _ := stanzaAttachment("<message>" + bodyXML + "</message>"); attachment != nil {
		bodyXML += `<x xmlns="jabber:x:oob"><url>` + xmlEscape(attachment.URL) + `</url></x>`
	}
	resource := "web"
	fromFull := from + "/" + resource
	stanza := fmt.Sprintf(
		`<message from='%s' to='%s' id='%s' type='chat'>%s</message>`,
		xmlEscape(fromFull), xmlEscape(toJ.Full()), xmlEscape(id), bodyXML,
	)
	stanza, id, err := s.prepareArchivedMessage(ctx, stanza)
	if err != nil {
		return ChatEvent{}, err
	}
	b := []byte(stanza)
	delivered := s.hub.Route(to, b)
	if !delivered {
		delivered = s.hub.Route(toJ.Bare(), b)
	}
	if !delivered {
		_ = s.storeOffline(ctx, toJ.Bare(), string(b))
	}
	_ = s.enqueueBotInbox(ctx, toJ.Bare(), fromFull, string(b), body)

	ev := ChatEvent{Type: "message", ID: id, From: fromFull, To: toJ.Bare(), Body: body, At: time.Now().UTC()}
	ev.Attachment, ev.Encrypted = stanzaAttachment(stanza)
	ev.OMEMO = encryptedElement(stanza)
	s.hub.fanoutCarbon(ParseJID(fromFull), "sent", stanza)
	s.hub.pushWeb(from, ev)
	if toJ.Bare() != from {
		s.hub.pushWeb(toJ.Bare(), ev)
	}
	return ev, nil
}

func (s *Server) SubscribeChat(bare string) (<-chan ChatEvent, func()) {
	s.webPresenceMu.Lock()
	defer s.webPresenceMu.Unlock()
	p := s.savedWebPresence(context.Background(), bare)
	s.recordPresence(context.Background(), bare+"/web", p.State, p.Status)
	ch, cancel := s.hub.subscribeWeb(bare)
	return ch, func() {
		s.webPresenceMu.Lock()
		defer s.webPresenceMu.Unlock()
		cancel()
		s.hub.mu.RLock()
		n := len(s.hub.web[ParseJID(bare).Bare()])
		s.hub.mu.RUnlock()
		if n == 0 {
			s.recordPresence(context.Background(), bare+"/web", "offline", "")
		}
	}
}

func parseMessageAddrs(stanza string) (from, to string) {
	var message struct {
		From string `xml:"from,attr"`
		To   string `xml:"to,attr"`
	}
	if xml.Unmarshal([]byte(stanza), &message) == nil {
		from, to = message.From, message.To
	}
	return from, to
}

func attrValue(stanza, name string) string {
	// cheap attribute scrape: name='…' or name="…"
	for _, q := range []string{`'`, `"`} {
		key := name + "=" + q
		i := strings.Index(stanza, key)
		if i < 0 {
			continue
		}
		rest := stanza[i+len(key):]
		j := strings.IndexByte(rest, q[0])
		if j >= 0 {
			return rest[:j]
		}
	}
	return ""
}

// notifyWebFromStanza is called when a C2S/component message is delivered.
func (s *Server) notifyWebFromStanza(stanza, fallbackTo string) {
	from, to := parseMessageAddrs(stanza)
	if to == "" {
		to = fallbackTo
	}
	ev := ChatEvent{
		Type: "message",
		ID:   stanzaEventID(stanza, ParseJID(to).Bare()),
		From: from,
		To:   ParseJID(to).Bare(),
		Body: extractBody(stanza),
		At:   time.Now().UTC(),
	}
	ev.Attachment, ev.Encrypted = stanzaAttachment(stanza)
	ev.OMEMO = encryptedElement(stanza)
	if ev.Body == "" && ev.Attachment == nil && !ev.Encrypted {
		return
	}
	if ev.To != "" {
		s.hub.pushWeb(ev.To, ev)
	}
	if sender := ParseJID(from).Bare(); sender != "" && sender != ev.To {
		ev.ID = stanzaEventID(stanza, sender)
		s.hub.pushWeb(sender, ev)
	}
}

// Ensure Gateway still compiles with chat methods.
var _ Gateway = (*Server)(nil)

func (s *Server) SetNotificationSink(sink func(string, ChatEvent)) {
	s.hub.mu.Lock()
	s.hub.notificationSink = sink
	s.hub.mu.Unlock()
}
