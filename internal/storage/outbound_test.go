package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestOutboundQueueRoundTrip(t *testing.T) {
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

	it, err := store.EnqueueOutbound(ctx, &storage.OutboundItem{
		EnvelopeFrom: "a@ex.com",
		EnvelopeTo:   "b@remote.test",
		MessageID:    "<1@ex.com>",
		Data:         []byte("From: a\r\n\r\nbody"),
		MaxAttempts:  3,
	})
	if err != nil {
		t.Fatal(err)
	}
	n, err := store.CountOutbound(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	claimed, err := store.ClaimOutboundDue(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != it.ID {
		t.Fatalf("%+v", claimed)
	}
	next := time.Now().UTC().Add(time.Hour)
	if err := store.RescheduleOutbound(ctx, it.ID, 1, next, "temp fail"); err != nil {
		t.Fatal(err)
	}
	claimed, err = store.ClaimOutboundDue(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatal("should not be due yet")
	}
	if err := store.DeleteOutbound(ctx, it.ID); err != nil {
		t.Fatal(err)
	}
	n, _ = store.CountOutbound(ctx)
	if n != 0 {
		t.Fatal(n)
	}
}
