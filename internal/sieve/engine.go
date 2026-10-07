package sieve

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/textproto"
	"strings"
	"time"

	gosieve "github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

var (
	ErrRejected = errors.New("sieve rejected message")
)

// Result is the outcome of running a script against a message.
type Result struct {
	Rejected   bool
	RejectMsg  string
	Discarded  bool
	Redirects  []string
	Deliveries []Delivery // folders to file into (empty folder = INBOX keep)
}

type Delivery struct {
	Mailbox string
	Flags   []string
	Copy    bool
}

// MailNotifier is called after a message is filed into a mailbox (IMAP IDLE wakeups).
type MailNotifier interface {
	Notify(email, mailbox string)
}

// DeliveryNotifier is an optional richer hook with message subject/from.
type DeliveryNotifier interface {
	NotifyDelivery(email, mailbox, subject, from string)
}

// Engine runs active Sieve scripts for local delivery.
type Engine struct {
	Store     storage.Driver
	Mailstore *mailstore.Store
	Log       *slog.Logger
	Notifier  MailNotifier
}

func New(store storage.Driver, ms *mailstore.Store, log *slog.Logger) *Engine {
	return &Engine{Store: store, Mailstore: ms, Log: log}
}

// CheckScript validates script syntax without executing it.
func CheckScript(script string) error {
	_, err := gosieve.Load(strings.NewReader(script), gosieve.DefaultOptions())
	return err
}

// Deliver runs the user's active script (if any) and files the message accordingly.
func (e *Engine) Deliver(ctx context.Context, user *storage.User, envelopeFrom, envelopeTo string, data []byte, msgid string) error {
	script, err := e.Store.GetActiveSieveScript(ctx, user.ID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	var result Result
	if script != nil && script.Script != "" {
		result, err = Execute(script.Script, envelopeFrom, envelopeTo, user.Email, data)
		if err != nil {
			e.Log.Warn("sieve execute failed; falling back to INBOX", "user", user.Email, "err", err)
			result = Result{Deliveries: []Delivery{{Mailbox: "INBOX"}}}
		}
	} else {
		result = Result{Deliveries: []Delivery{{Mailbox: "INBOX"}}}
	}

	if result.Rejected {
		return fmt.Errorf("%w: %s", ErrRejected, result.RejectMsg)
	}
	if result.Discarded && len(result.Deliveries) == 0 && len(result.Redirects) == 0 {
		e.Log.Info("sieve discarded message", "user", user.Email)
		return nil
	}

	for _, addr := range result.Redirects {
		if err := e.redirectLocal(ctx, user, addr, data, msgid); err != nil {
			e.Log.Warn("sieve redirect skipped", "to", addr, "err", err)
		}
	}

	if len(result.Deliveries) == 0 && !result.Discarded {
		result.Deliveries = []Delivery{{Mailbox: "INBOX"}}
	}

	for _, d := range result.Deliveries {
		folder := d.Mailbox
		if folder == "" {
			folder = "INBOX"
		}
		if err := e.FileInto(ctx, user, folder, d.Flags, data, msgid); err != nil {
			return err
		}
	}
	return nil
}

// FileInto delivers a message into a mailbox folder (used by Sieve and quarantine).
func (e *Engine) FileInto(ctx context.Context, user *storage.User, folder string, flags []string, data []byte, msgid string) error {
	if err := storage.EnsureQuota(ctx, e.Store, user, int64(len(data))); err != nil {
		return err
	}
	if _, err := e.Mailstore.EnsureUser(user.Email); err != nil {
		return err
	}
	path, err := e.Mailstore.EnsureFolder(user.Email, folder)
	if err != nil {
		return err
	}
	mb, err := e.Store.EnsureMailbox(ctx, user.ID, folder, path)
	if err != nil {
		return err
	}
	rel, size, err := e.Mailstore.Deliver(user.Email, folder, data)
	if err != nil {
		return err
	}
	flagStr := storage.NormalizeFlags(flags)
	msg := &storage.Message{
		MailboxID:    mb.ID,
		Size:         size,
		Flags:        flagStr,
		InternalDate: time.Now().UTC(),
		FilePath:     rel,
		MessageID:    msgid,
		Archived:     strings.EqualFold(folder, "Archive"),
	}
	mailsearch.ApplyHeaders(msg, data)
	inserted, err := e.Store.InsertMessage(ctx, msg)
	if err == nil {
		_ = mailsearch.Index(ctx, e.Store, inserted.ID, data)
		if dn, ok := e.Notifier.(DeliveryNotifier); ok {
			dn.NotifyDelivery(user.Email, folder, msg.Subject, msg.FromAddr)
		} else if e.Notifier != nil {
			e.Notifier.Notify(user.Email, folder)
		}
	}
	return err
}

func (e *Engine) redirectLocal(ctx context.Context, fromUser *storage.User, addr string, data []byte, msgid string) error {
	addr = strings.ToLower(strings.Trim(addr, "<> "))
	u, err := e.Store.ResolveRecipient(ctx, addr)
	if err != nil {
		return fmt.Errorf("non-local redirect not supported yet: %w", err)
	}
	_ = fromUser
	return e.FileInto(ctx, u, "INBOX", nil, data, msgid)
}

// Execute loads and runs a Sieve script, returning normalized actions.
func Execute(script, envFrom, envTo, authUser string, raw []byte) (Result, error) {
	loaded, err := gosieve.Load(strings.NewReader(script), gosieve.DefaultOptions())
	if err != nil {
		return Result{}, err
	}

	hdr, err := textproto.NewReader(bufio.NewReader(bytes.NewReader(raw))).ReadMIMEHeader()
	if err != nil {
		return Result{}, err
	}

	rd := gosieve.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: envFrom, To: envTo, Auth: authUser},
		interp.MessageStatic{Size: len(raw), Header: hdr, RawMessage: raw},
	)
	if err := loaded.Execute(context.Background(), rd); err != nil {
		return Result{}, err
	}
	return resultFromRuntime(rd), nil
}

func resultFromRuntime(rd *interp.RuntimeData) Result {
	var res Result
	for _, a := range rd.AppliedActions {
		switch v := a.(type) {
		case interp.ActionReject:
			res.Rejected = true
			res.RejectMsg = v.Reason
		case interp.ActionEReject:
			res.Rejected = true
			res.RejectMsg = v.Reason
		case interp.ActionDiscard:
			res.Discarded = true
		case interp.ActionRedirect:
			res.Redirects = append(res.Redirects, v.Address)
		case interp.ActionFileInto:
			res.Deliveries = append(res.Deliveries, Delivery{
				Mailbox: normalizeMailbox(v.Mailbox),
				Flags:   flagsToSlice(v.Flags),
				Copy:    v.Copy,
			})
		case interp.ActionKeep:
			res.Deliveries = append(res.Deliveries, Delivery{
				Mailbox: "INBOX",
				Flags:   flagsToSlice(v.Flags),
			})
		}
	}
	if res.Rejected {
		return res
	}
	if rd.ImplicitKeep && !res.Discarded && !hasInbox(res.Deliveries) {
		res.Deliveries = append(res.Deliveries, Delivery{
			Mailbox: "INBOX",
			Flags:   append([]string(nil), rd.Flags...),
		})
	}
	return res
}

func hasInbox(ds []Delivery) bool {
	for _, d := range ds {
		if d.Mailbox == "" || strings.EqualFold(d.Mailbox, "INBOX") {
			return true
		}
	}
	return false
}

func normalizeMailbox(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "INBOX.")
	name = strings.TrimPrefix(name, "INBOX/")
	if name == "" || strings.EqualFold(name, "INBOX") {
		return "INBOX"
	}
	return name
}

func flagsToSlice(f interp.Flags) []string {
	return []string(f)
}
