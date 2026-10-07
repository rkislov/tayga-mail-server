package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestUserWriterLease(t *testing.T) {
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
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ok, holder, err := store.TryAcquireUserWriter(ctx, u.ID, "node-a", time.Minute)
	if err != nil || !ok || holder != "node-a" {
		t.Fatalf("a: ok=%v holder=%s err=%v", ok, holder, err)
	}
	ok, holder, err = store.TryAcquireUserWriter(ctx, u.ID, "node-b", time.Minute)
	if err != nil || ok || holder != "node-a" {
		t.Fatalf("b should fail: ok=%v holder=%s err=%v", ok, holder, err)
	}
	// Renew for same node.
	ok, _, err = store.TryAcquireUserWriter(ctx, u.ID, "node-a", time.Minute)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if err := store.ReleaseUserWriter(ctx, u.ID, "node-a"); err != nil {
		t.Fatal(err)
	}
	ok, holder, err = store.TryAcquireUserWriter(ctx, u.ID, "node-b", time.Minute)
	if err != nil || !ok || holder != "node-b" {
		t.Fatalf("after release: ok=%v holder=%s err=%v", ok, holder, err)
	}
}

func TestUserWriterLeaseExpiry(t *testing.T) {
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
	u, _ := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local", Enabled: true,
	})

	ok, _, err := store.TryAcquireUserWriter(ctx, u.ID, "node-a", time.Millisecond)
	if err != nil || !ok {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	ok, holder, err := store.TryAcquireUserWriter(ctx, u.ID, "node-b", time.Minute)
	if err != nil || !ok || holder != "node-b" {
		t.Fatalf("expired: ok=%v holder=%s err=%v", ok, holder, err)
	}
}
