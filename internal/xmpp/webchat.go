package xmpp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ChatEvent is pushed to web clients over SSE.
type ChatEvent struct {
	Type string    `json:"type"`
	ID   string    `json:"id,omitempty"`
	From string    `json:"from"`
	To   string    `json:"to"`
	Body string    `json:"body"`
	At   time.Time `json:"at"`
}

// RosterEntry is a roster row for the web messenger.
type RosterEntry struct {
	JID          string `json:"jid"`
	Name         string `json:"name"`
	Subscription string `json:"subscription"`
}

// HistoryMsg is a decoded MAM / chat history item.
type HistoryMsg struct {
	ID     string    `json:"id"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
	Stanza string    `json:"-"`
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
	unsub := func() {
		h.mu.Lock()
		if m := h.web[bare]; m != nil {
			delete(m, sub)
			if len(m) == 0 {
				delete(h.web, bare)
			}
		}
		h.mu.Unlock()
		close(sub.ch)
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
	h.mu.RUnlock()
	for _, s := range list {
		select {
		case s.ch <- ev:
		default:
		}
	}
}

func (s *Server) ListChatRoster(ctx context.Context, userID string) ([]RosterEntry, error) {
	items, err := s.listRoster(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]RosterEntry, 0, len(items))
	for _, it := range items {
		out = append(out, RosterEntry{JID: it.JID, Name: it.Name, Subscription: it.Subscription})
	}
	return out, nil
}

func (s *Server) UpsertChatRoster(ctx context.Context, userID, jid, name string) error {
	return s.upsertRoster(ctx, userID, rosterItem{
		JID: ParseJID(jid).Bare(), Name: name, Subscription: "both", GroupsJSON: "[]",
	})
}

func (s *Server) ChatHistory(ctx context.Context, ownerBare, withBare string, limit int) ([]HistoryMsg, error) {
	rows, err := s.queryMAM(ctx, ParseJID(ownerBare).Bare(), ParseJID(withBare).Bare(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryMsg, 0, len(rows))
	for _, r := range rows {
		from, to := parseMessageAddrs(r.Stanza)
		out = append(out, HistoryMsg{
			ID: r.StanzaID, From: from, To: to, Body: extractBody(r.Stanza),
			At: r.CreatedAt, Stanza: r.Stanza,
		})
	}
	return out, nil
}

func (s *Server) SendChatMessage(ctx context.Context, fromEmail, to, body string) (ChatEvent, error) {
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
	resource := "web"
	fromFull := from + "/" + resource
	stanza := fmt.Sprintf(
		`<message from='%s' to='%s' id='%s' type='chat'>%s</message>`,
		xmlEscape(fromFull), xmlEscape(toJ.Full()), xmlEscape(id), bodyXML,
	)
	b := []byte(stanza)
	delivered := s.hub.Route(to, b)
	if !delivered {
		delivered = s.hub.Route(toJ.Bare(), b)
	}
	if !delivered {
		_ = s.storeOffline(ctx, toJ.Bare(), string(b))
	}
	_ = s.enqueueBotInbox(ctx, toJ.Bare(), fromFull, string(b), body)
	_ = s.archiveMAM(ctx, from, toJ.Bare(), id, string(b))
	_ = s.archiveMAM(ctx, toJ.Bare(), from, id, string(b))

	ev := ChatEvent{Type: "message", ID: id, From: fromFull, To: toJ.Bare(), Body: body, At: time.Now().UTC()}
	s.hub.pushWeb(from, ev)
	s.hub.pushWeb(toJ.Bare(), ev)
	return ev, nil
}

func (s *Server) SubscribeChat(bare string) (<-chan ChatEvent, func()) {
	return s.hub.subscribeWeb(bare)
}

func parseMessageAddrs(stanza string) (from, to string) {
	from = attrValue(stanza, "from")
	to = attrValue(stanza, "to")
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
		ID:   attrValue(stanza, "id"),
		From: from,
		To:   ParseJID(to).Bare(),
		Body: extractBody(stanza),
		At:   time.Now().UTC(),
	}
	if ev.To != "" {
		s.hub.pushWeb(ev.To, ev)
	}
	if from != "" {
		s.hub.pushWeb(ParseJID(from).Bare(), ev)
	}
}

// Ensure Gateway still compiles with chat methods.
var _ Gateway = (*Server)(nil)