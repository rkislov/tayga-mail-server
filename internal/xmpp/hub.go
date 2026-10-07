package xmpp

import (
	"strings"
	"sync"
)

// ComponentConn is an authenticated XEP-0114 component connection.
type ComponentConn struct {
	Domain string
	Send   func([]byte)
	Close  func()
}

// Hub tracks online C2S sessions, web SSE clients, and external components.
type Hub struct {
	mu         sync.RWMutex
	byBare     map[string]map[string]*Session // bare JID → resource → session
	components map[string]*ComponentConn      // component domain → conn
	web        map[string]map[*webSub]struct{}
}

func NewHub() *Hub {
	return &Hub{
		byBare:     make(map[string]map[string]*Session),
		components: make(map[string]*ComponentConn),
		web:        make(map[string]map[*webSub]struct{}),
	}
}

func (h *Hub) Register(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	bare := s.JID.Bare()
	if h.byBare[bare] == nil {
		h.byBare[bare] = make(map[string]*Session)
	}
	if old := h.byBare[bare][s.JID.Resource]; old != nil && old != s {
		old.Close()
	}
	h.byBare[bare][s.JID.Resource] = s
}

func (h *Hub) Unregister(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	bare := s.JID.Bare()
	m := h.byBare[bare]
	if m == nil {
		return
	}
	if m[s.JID.Resource] == s {
		delete(m, s.JID.Resource)
	}
	if len(m) == 0 {
		delete(h.byBare, bare)
	}
}

func (h *Hub) RegisterComponent(c *ComponentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	dom := strings.ToLower(c.Domain)
	if old := h.components[dom]; old != nil && old != c {
		if old.Close != nil {
			old.Close()
		}
	}
	h.components[dom] = c
}

func (h *Hub) UnregisterComponent(domain string, c *ComponentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	dom := strings.ToLower(domain)
	if h.components[dom] == c {
		delete(h.components, dom)
	}
}

func (h *Hub) Component(domain string) *ComponentConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.components[strings.ToLower(domain)]
}

func (h *Hub) HasComponentDomain(host string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	host = strings.ToLower(host)
	if _, ok := h.components[host]; ok {
		return true
	}
	// also match configured domains that are not yet connected
	return false
}

func (h *Hub) Sessions(bare string) []*Session {
	h.mu.RLock()
	defer h.mu.RUnlock()
	m := h.byBare[stringsToLowerBare(bare)]
	out := make([]*Session, 0, len(m))
	for _, s := range m {
		out = append(out, s)
	}
	return out
}

// Route delivers a stanza to local sessions and/or an XMPP component.
func (h *Hub) Route(to string, stanza []byte) bool {
	j := ParseJID(to)
	delivered := false

	// Component domain (user@bots.example.com or bare bots.example.com)
	h.mu.RLock()
	comp := h.components[j.Domain]
	h.mu.RUnlock()
	if comp != nil && comp.Send != nil {
		comp.Send(stanza)
		delivered = true
	}

	sessions := h.Sessions(j.Bare())
	for _, s := range sessions {
		s.SendRaw(stanza)
		delivered = true
	}
	return delivered
}

func stringsToLowerBare(s string) string {
	return ParseJID(s).Bare()
}
