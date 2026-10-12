package metrics

import (
	"context"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryPersistsWithoutBrowser(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "metrics.db")}}
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := store.CreateTenant(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old := storage.MetricSample{At: now.Add(-26 * time.Hour).UnixMilli(), Boot: "old", Values: map[string]float64{"heap": 1}}
	if err = store.SaveMetricSample(ctx, "server", old); err != nil {
		t.Fatal(err)
	}
	if err = collectHistory(ctx, store, "boot", now); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, scope := range []string{"server", "tenant:" + tenant.ID} {
		rows, err := store.MetricHistory(ctx, scope, 0)
		if err != nil || len(rows) != 1 || rows[0].Boot != "boot" {
			t.Fatalf("scope=%s rows=%v err=%v", scope, rows, err)
		}
	}
	rows, err := store.MetricHistory(ctx, "tenant:other", 0)
	if err != nil || len(rows) != 0 {
		t.Fatal("scope leak")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	done := make(chan struct{})
	go func() { RunHistory(ctx, store, slog.New(slog.NewTextHandler(io.Discard, nil))); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector did not stop")
	}
}
