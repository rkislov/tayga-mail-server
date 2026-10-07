package smtp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestGreylistDeferThenPass(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(context.Background(), config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	p := newGreylistPolicy(store, 50*time.Millisecond, time.Hour, 32, nil)
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err == nil {
		t.Fatal("expected defer")
	}
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err == nil {
		t.Fatal("still too early")
	}
	time.Sleep(60 * time.Millisecond)
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err != nil {
		t.Fatal(err)
	}
	if err := p.check("203.0.113.1", "a@ex.com", "b@ex.com"); err != nil {
		t.Fatal(err)
	}
}

func TestGreylistIPv4Net(t *testing.T) {
	if greylistKey("203.0.113.10", "a@e", "b@e", 24) != greylistKey("203.0.113.99", "a@e", "b@e", 24) {
		t.Fatal("same /24 should share key")
	}
	if greylistKey("203.0.113.10", "a@e", "b@e", 32) == greylistKey("203.0.113.99", "a@e", "b@e", 32) {
		t.Fatal("exact IP should differ")
	}
}

func TestGreylistCleanupDB(t *testing.T) {
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

	_, _ = store.GreylistTouch(ctx, "1.2.3.4", "a@e", "b@e", time.Hour)
	n, err := store.DeleteExpiredGreylist(ctx, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted=%d", n)
	}
}
