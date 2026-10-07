package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestEnsureQuota(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "t.db")
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: dbPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer os.Remove(dbPath)

	tenant, err := store.CreateTenant(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := store.CreateDomain(ctx, tenant.ID, "ex.com")
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local", Enabled: true,
		QuotaBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	mb, err := store.EnsureMailbox(ctx, u.ID, "INBOX", "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, Size: 80, FilePath: "a", InternalDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := storage.EnsureQuota(ctx, store, u, 10); err != nil {
		t.Fatalf("10 bytes should fit: %v", err)
	}
	if err := storage.EnsureQuota(ctx, store, u, 30); !errors.Is(err, storage.ErrQuotaExceeded) {
		t.Fatalf("expected quota exceeded, got %v", err)
	}

	u.QuotaBytes = 0
	if err := storage.EnsureQuota(ctx, store, u, 1_000_000); err != nil {
		t.Fatalf("unlimited quota: %v", err)
	}
}
