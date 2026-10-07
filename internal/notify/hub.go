package notify

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const (
	KindMail     = "mail"
	KindCalendar = "calendar"
	KindSystem   = "system"
)

// Event is pushed to the web UI (SSE + toast / browser Notification).
type Event struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Href      string    `json:"href,omitempty"` // client route hint: mail / calendar
	Mailbox   string    `json:"mailbox,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type sub struct {
	ch chan Event
}

// Hub fans in-process notifications to online web sessions.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[*sub]struct{} // userID → set
	ring map[string][]Event           // recent per user
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[*sub]struct{}),
		ring: make(map[string][]Event),
	}
}

// Subscribe returns a channel of events for userID; call unsubscribe when done.
func (h *Hub) Subscribe(userID string) (<-chan Event, func()) {
	if h == nil || userID == "" {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}
	s := &sub{ch: make(chan Event, 16)}
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[*sub]struct{})
	}
	h.subs[userID][s] = struct{}{}
	h.mu.Unlock()
	return s.ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if set := h.subs[userID]; set != nil {
			delete(set, s)
			if len(set) == 0 {
				delete(h.subs, userID)
			}
		}
		close(s.ch)
	}
}

// OnlineUserIDs returns users with at least one SSE subscriber.
func (h *Hub) OnlineUserIDs() []string {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.subs))
	for id := range h.subs {
		out = append(out, id)
	}
	return out
}

// Recent returns the last buffered events for a user (newest last).
func (h *Hub) Recent(userID string, limit int) []Event {
	if h == nil {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	ring := h.ring[userID]
	if len(ring) == 0 {
		return nil
	}
	if len(ring) > limit {
		ring = ring[len(ring)-limit:]
	}
	out := make([]Event, len(ring))
	copy(out, ring)
	return out
}

// Publish sends an event to a user's subscribers and keeps a short ring buffer.
func (h *Hub) Publish(userID string, ev Event) {
	if h == nil || userID == "" {
		return
	}
	if ev.ID == "" {
		ev.ID = storage.NewID()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	h.mu.Lock()
	ring := append(h.ring[userID], ev)
	if len(ring) > 40 {
		ring = ring[len(ring)-40:]
	}
	h.ring[userID] = ring
	subs := make([]*sub, 0, len(h.subs[userID]))
	for s := range h.subs[userID] {
		subs = append(subs, s)
	}
	h.mu.Unlock()
	for _, s := range subs {
		select {
		case s.ch <- ev:
		default:
			// drop if slow consumer
		}
	}
}

// PublishMail notifies about a newly delivered message.
func (h *Hub) PublishMail(userID, mailbox, subject, from string) {
	title := "Новое письмо"
	if subject != "" {
		title = subject
	}
	body := mailbox
	if from != "" {
		body = from
		if mailbox != "" {
			body = from + " → " + mailbox
		}
	}
	h.Publish(userID, Event{
		Kind:    KindMail,
		Title:   title,
		Body:    body,
		Href:    "mail",
		Mailbox: mailbox,
	})
}

// PublishCalendar reminds about an upcoming event.
func (h *Hub) PublishCalendar(userID, title, when string) {
	if title == "" {
		title = "Событие"
	}
	h.Publish(userID, Event{
		Kind:  KindCalendar,
		Title: title,
		Body:  when,
		Href:  "calendar",
	})
}

// Marshal is a helper for SSE payloads.
func Marshal(ev Event) []byte {
	b, _ := json.Marshal(ev)
	return b
}
