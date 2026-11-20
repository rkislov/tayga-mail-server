package storage_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestMailLogInsertAndSearch(t *testing.T) {
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

	e := &storage.MailLogEntry{
		TenantID:  "t1",
		Domain:    "example.com",
		Event:     "delivered",
		Direction: "inbound",
		Peer:      "1.2.3.4",
		MailFrom:  "alice@outside.test",
		RcptTo:    "bob@example.com",
		MessageID: "<msg-1@example.com>",
		Size:      1234,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.InsertMailLog(ctx, e); err != nil {
		t.Fatal(err)
	}
	if e.Line == "" || !strings.Contains(e.Line, "delivered") {
		t.Fatalf("expected formatted line, got %q", e.Line)
	}

	_ = store.InsertMailLog(ctx, &storage.MailLogEntry{
		TenantID: "t1", Domain: "other.com", Event: "queued", Direction: "outbound",
		MailFrom: "bob@example.com", RcptTo: "x@other.com", MessageID: "<msg-2>",
	})

	hits, err := store.SearchMailLog(ctx, storage.MailLogQuery{Q: "bob@example", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected search hits for bob@example")
	}

	domainHits, err := store.SearchMailLog(ctx, storage.MailLogQuery{
		Domains: []string{"example.com"},
		Q:       "delivered",
		Limit:   50,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range domainHits {
		if h.Domain != "example.com" {
			t.Fatalf("domain scope leak: %+v", h)
		}
	}
	if len(domainHits) == 0 {
		t.Fatal("expected domain-scoped hits")
	}
}

func TestDomainOfEmail(t *testing.T) {
	if got := storage.DomainOfEmail("Bob@Example.COM"); got != "example.com" {
		t.Fatalf("got %q", got)
	}
	if got := storage.DomainOfEmail("<a@b.c>"); got != "b.c" {
		t.Fatalf("got %q", got)
	}
	if storage.DomainOfEmail("nodomain") != "" {
		t.Fatal("expected empty")
	}
}

// Correlation must never widen the admin's scope, or mix another envelope recipient.
func TestMailLogExactHistoryScope(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx := "b9fde9ee-7700-4d52-8695-3179b5ea083f"
	base := storage.MailLogEntry{TenantID: "t1", Domain: "example.com", Event: "delivered", Direction: "inbound", RcptTo: "bob@example.com", MessageID: "<shared@example.com>", Detail: "transaction_id=" + tx + " accepted"}
	for i := 0; i < 7; i++ {
		e := base
		switch i {
		case 1:
			e.TenantID = "t2"
		case 2:
			e.Domain = "other.com"
		case 3:
			e.RcptTo = "other@example.com"
		case 4:
			e.MailFrom = "spoof@example.com"
		case 5:
			e.Direction = "outbound"
		case 6:
			e.Detail = "transaction_id=" + tx + "evil accepted"
		}
		if err := db.InsertMailLog(ctx, &e); err != nil {
			t.Fatal(err)
		}
	}
	sender, recipient := "", "bob@example.com"
	q := storage.MailLogQuery{TenantID: "t1", Domains: []string{"example.com"}, MessageID: base.MessageID, Direction: "inbound", MailFrom: &sender, RcptTo: &recipient, CorrelationKey: "transaction_id", CorrelationID: tx}
	for _, term := range []string{"", "accepted"} {
		q.Q = term
		hits, err := db.SearchMailLog(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].Detail != base.Detail {
			t.Fatalf("scope or exact token violated for %q: %+v", term, hits)
		}
	}
	q.CorrelationKey = "unsafe"
	if _, err := db.SearchMailLog(ctx, q); err == nil {
		t.Fatal("invalid correlation key accepted")
	}
}
