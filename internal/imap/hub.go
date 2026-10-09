package imapserver

import (
	"context"
	"sync"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/tayga/tms/internal/storage"
)

// ClusterPublisher fans mailbox notifications to other Tayga nodes (e.g. Postgres LISTEN/NOTIFY).
type ClusterPublisher interface {
	Publish(email, mailbox string)
}

// Hub broadcasts mailbox status changes to IDLE/NOOP clients (go-imap BackendUpdater).
// Each Updates() call creates a fan-out subscription (one per IMAP listener).
type Hub struct {
	store   storage.Driver
	mu      sync.Mutex
	subs    []chan backend.Update
	cluster ClusterPublisher
}

func NewHub(store storage.Driver) *Hub {
	return &Hub{store: store}
}

// SetCluster attaches an optional cross-node publisher. Local Notify still wakes this process;
// Publish is best-effort for peers.
func (h *Hub) SetCluster(p ClusterPublisher) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.cluster = p
	h.mu.Unlock()
}

func (h *Hub) Updates() <-chan backend.Update {
	ch := make(chan backend.Update, 64)
	h.mu.Lock()
	h.subs = append(h.subs, ch)
	h.mu.Unlock()
	return ch
}

// Notify pushes an EXISTS-capable mailbox status update for username@mailbox,
// then optionally publishes to peer nodes.
func (h *Hub) Notify(email, mailbox string) {
	if h == nil {
		return
	}
	h.notifyLocal(email, mailbox)
	h.mu.Lock()
	pub := h.cluster
	h.mu.Unlock()
	if pub != nil {
		pub.Publish(email, mailbox)
	}
}

// NotifyRemote applies a peer notification locally without rebroadcasting.
func (h *Hub) NotifyRemote(email, mailbox string) {
	h.notifyLocal(email, mailbox)
}

func (h *Hub) notifyLocal(email, mailbox string) {
	if h == nil {
		return
	}
	status, err := h.mailboxStatus(email, mailbox)
	if err != nil || status == nil {
		return
	}
	h.publish(&backend.MailboxUpdate{
		Update:        backend.NewUpdate(email, mailbox),
		MailboxStatus: status,
	})
}

func (h *Hub) publish(upd *backend.MailboxUpdate) {
	h.mu.Lock()
	subs := append([]chan backend.Update(nil), h.subs...)
	h.mu.Unlock()
	for _, ch := range subs {
		// Each listener closes Done after broadcasting. Never share that channel
		// between the plain IMAP and IMAPS servers.
		copy := &backend.MailboxUpdate{
			Update:        backend.NewUpdate(upd.Username(), upd.Mailbox()),
			MailboxStatus: upd.MailboxStatus,
		}
		select {
		case ch <- copy:
		default:
		}
	}
}

func (h *Hub) mailboxStatus(email, mailbox string) (*imap.MailboxStatus, error) {
	ctx := context.Background()
	u, err := h.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	mb, err := h.store.GetMailbox(ctx, u.ID, mailbox)
	if err != nil {
		return nil, err
	}
	msgs, err := h.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return nil, err
	}
	items := []imap.StatusItem{imap.StatusMessages, imap.StatusRecent, imap.StatusUidNext}
	status := imap.NewMailboxStatus(mailbox, items)
	status.Flags = []string{imap.SeenFlag, imap.AnsweredFlag, imap.FlaggedFlag, imap.DeletedFlag, imap.DraftFlag}
	status.PermanentFlags = []string{"\\*"}
	status.Messages = uint32(len(msgs))
	status.UidNext = uint32(mb.UIDNext)
	status.UidValidity = uint32(mb.UIDValidity)
	var recent uint32
	for _, msg := range msgs {
		if storage.MessageIsRecent(msg) {
			recent++
		}
	}
	status.Recent = recent
	status.UnseenSeqNum = unseenSeq(msgs)
	return status, nil
}
