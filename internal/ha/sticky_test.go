package ha_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/ha"
	"github.com/tayga/tms/internal/storage"
)

func TestStickyAllowWrite(t *testing.T) {
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

	a := ha.NewSticky(ha.StickyConfig{Store: store, NodeID: "a", TTL: time.Minute})
	b := ha.NewSticky(ha.StickyConfig{Store: store, NodeID: "b", TTL: time.Minute})
	if err := a.AllowWrite(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	err = b.AllowWrite(ctx, u.ID)
	if !errors.Is(err, ha.ErrStandby) {
		t.Fatalf("want ErrStandby, got %v", err)
	}
}

func TestClusterWriters(t *testing.T) {
	w := ha.ClusterWriters{Gate: stubGate{leader: false}}
	if err := w.AllowWrite(context.Background(), "x"); !errors.Is(err, ha.ErrStandby) {
		t.Fatal(err)
	}
	w = ha.ClusterWriters{Gate: stubGate{leader: true}}
	if err := w.AllowWrite(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
}
