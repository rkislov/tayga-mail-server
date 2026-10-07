package smtp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

type flakySender struct {
	fails int
	sent  int
}

func (f *flakySender) Send(from string, to []string, data []byte) error {
	if f.fails > 0 {
		f.fails--
		return errors.New("421 try later")
	}
	f.sent++
	return nil
}

type failSender struct{}

func (failSender) Send(from string, to []string, data []byte) error {
	return errors.New("550 user unknown")
}

func TestOutboundQueueRetryThenDeliver(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sender := &flakySender{fails: 1}
	q := newOutboundQueue(store, sender, nil, nil, "mail.ex.com", QueueConfig{
		Enabled: true, MaxAttempts: 5, BatchSize: 5, PollInterval: time.Millisecond,
	}, nil)
	if err := q.Enqueue(ctx, "a@ex.com", []string{"b@remote.test"}, []byte("From: a\r\n\r\nx"), "<1@ex.com>"); err != nil {
		t.Fatal(err)
	}
	items, _ := store.ClaimOutboundDue(ctx, 5)
	if len(items) != 1 {
		t.Fatal(len(items))
	}
	q.process(ctx, items[0])
	n, _ := store.CountOutbound(ctx)
	if n != 1 {
		t.Fatalf("expected rescheduled, count=%d", n)
	}
	// Force due now
	_ = store.RescheduleOutbound(ctx, items[0].ID, 1, time.Now().UTC().Add(-time.Second), "421")
	items, _ = store.ClaimOutboundDue(ctx, 5)
	q.process(ctx, items[0])
	if sender.sent != 1 {
		t.Fatalf("sent=%d", sender.sent)
	}
	n, _ = store.CountOutbound(ctx)
	if n != 0 {
		t.Fatal(n)
	}
}

func TestOutboundQueueDSNOnFinalFailure(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "sender@ex.com", LocalPart: "sender", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	eng := sieve.New(store, ms, nil)
	q := newOutboundQueue(store, failSender{}, nil, eng, "mail.ex.com", QueueConfig{
		Enabled: true, MaxAttempts: 1, BatchSize: 5,
	}, nil)
	msg := []byte("From: sender@ex.com\r\nTo: nowhere@remote.test\r\nSubject: hi\r\n\r\nbody\r\n")
	if err := q.Enqueue(ctx, "sender@ex.com", []string{"nowhere@remote.test"}, msg, "<x@ex.com>"); err != nil {
		t.Fatal(err)
	}
	items, _ := store.ClaimOutboundDue(ctx, 5)
	q.process(ctx, items[0])
	n, _ := store.CountOutbound(ctx)
	if n != 0 {
		t.Fatal("queue should be empty after final failure")
	}
	mb, err := store.EnsureMailbox(ctx, u.ID, "INBOX", ms.UserRoot(u.Email))
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(ctx, mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected DSN in inbox, got %d", len(msgs))
	}
	raw, err := ms.Read(msgs[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Undelivered Mail Returned to Sender") {
		t.Fatalf("DSN missing: %s", raw)
	}
	if !strings.Contains(string(raw), "nowhere@remote.test") {
		t.Fatal("DSN should mention failed recipient")
	}
}

func TestBackoff(t *testing.T) {
	if backoff(1) != time.Minute {
		t.Fatal(backoff(1))
	}
	if backoff(8) != 8*time.Hour {
		t.Fatal(backoff(8))
	}
	if backoff(99) != 8*time.Hour {
		t.Fatal(backoff(99))
	}
}
