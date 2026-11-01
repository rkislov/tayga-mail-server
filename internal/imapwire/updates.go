package server

import (
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/responses"
)

type updateReceipt struct {
	done   <-chan struct{}
	closed <-chan struct{}
}

type queuedUpdate struct {
	update    backend.Update
	selection uint64
	silent    bool
	done      chan struct{}
}

// The queue is per connection and bounded. A client that cannot consume it
// reconnects/resynchronizes instead of stalling every other user's IDLE.
func (c *conn) queueUpdate(update backend.Update) updateReceipt {
	u := queuedUpdate{update: update, selection: c.selection.Load(), silent: c.silentVal.Load(), done: make(chan struct{})}
	if target := update.Username(); target != "" {
		if user := c.username.Load(); user == nil || *user != target {
			close(u.done)
			return updateReceipt{done: u.done, closed: c.closed}
		}
	}
	select {
	case c.updates <- u:
	case <-c.closed:
	default:
		c.Close()
	}
	return updateReceipt{done: u.done, closed: c.closed}
}

func (c *conn) sendUpdates() {
	for {
		select {
		case u := <-c.updates:
			c.deliverUpdate(u)
			close(u.done)
		case <-c.closed:
			return
		}
	}
}

func (c *conn) deliverUpdate(u queuedUpdate) {
	c.contextMu.Lock()
	defer c.contextMu.Unlock()
	select {
	case <-c.closed:
		return
	default:
	}
	ctx, update := c.ctx, u.update
	if ctx.State == imap.LogoutState || u.selection != c.selection.Load() {
		return
	}
	if update.Username() != "" && (ctx.User == nil || ctx.User.Username() != update.Username()) {
		return
	}
	if update.Mailbox() != "" && (ctx.Mailbox == nil || ctx.Mailbox.Name() != update.Mailbox()) {
		return
	}
	if _, message := update.(*backend.MessageUpdate); message && u.silent {
		return
	}
	// Build a separate response for each recipient. FETCH/LIST/EXPUNGE
	// response channels are consumable, so they cannot be shared by clients.
	var res imap.WriterTo
	switch update := update.(type) {
	case *backend.StatusUpdate:
		res = update.StatusResp
	case *backend.MailboxUpdate:
		res = &responses.Select{Mailbox: update.MailboxStatus}
	case *backend.MailboxInfoUpdate:
		ch := make(chan *imap.MailboxInfo, 1)
		ch <- update.MailboxInfo
		close(ch)
		res = &responses.List{Mailboxes: ch}
	case *backend.MessageUpdate:
		ch := make(chan *imap.Message, 1)
		ch <- update.Message
		close(ch)
		res = &responses.Fetch{Messages: ch}
	case *backend.ExpungeUpdate:
		ch := make(chan uint32, 1)
		ch <- update.SeqNum
		close(ch)
		res = &responses.Expunge{SeqNums: ch}
	default:
		c.s.ErrorLog.Printf("unhandled update: %T", update)
	}
	if res != nil {
		_ = c.WriteResp(res)
	}
}
