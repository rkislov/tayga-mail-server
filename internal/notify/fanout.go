package notify

import "github.com/tayga/tms/internal/sieve"

// Fanout implements sieve.MailNotifier and optional DeliveryNotifier,
// waking IMAP IDLE and pushing web toasts.
type Fanout struct {
	Idle sieve.MailNotifier
	Hub  *Hub
	// ResolveUserID maps email → user UUID for hub targeting.
	ResolveUserID func(email string) string
}

func (f Fanout) Notify(email, mailbox string) {
	if f.Idle != nil {
		f.Idle.Notify(email, mailbox)
	}
	f.publish(email, mailbox, "", "")
}

// NotifyDelivery is called when sieve has header context.
func (f Fanout) NotifyDelivery(email, mailbox, subject, from string) {
	if f.Idle != nil {
		f.Idle.Notify(email, mailbox)
	}
	f.publish(email, mailbox, subject, from)
}

func (f Fanout) publish(email, mailbox, subject, from string) {
	if f.Hub == nil {
		return
	}
	uid := ""
	if f.ResolveUserID != nil {
		uid = f.ResolveUserID(email)
	}
	if uid == "" {
		return
	}
	f.Hub.PublishMail(uid, mailbox, subject, from)
}
