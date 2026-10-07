package settings

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestHubPutGetSpam(t *testing.T) {
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

	boot := config.Default()
	boot.Server.Hostname = "mail.test"
	boot.Storage.Driver = "sqlite"
	boot.Storage.SQLite.Path = filepath.Join(dir, "t.db")
	boot.Mailstore.Root = filepath.Join(dir, "mail")

	h := NewHub(store, boot)
	if err := h.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if h.Config().Spam.Enabled {
		t.Fatal("expected spam off")
	}

	body := []byte(`{"enabled":true,"backend":"rspamd","url":"http://127.0.0.1:11333","fail_open":true,"folder":"Junk","follow_rspamd":true}`)
	if err := h.PutSection(ctx, "spam", body); err != nil {
		t.Fatal(err)
	}
	if !h.Config().Spam.Enabled {
		t.Fatal("expected spam on after put")
	}
	if !h.RestartRequired() {
		t.Fatal("expected restart_required")
	}
	raw, err := h.GetSection("spam")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 10 {
		t.Fatalf("%s", raw)
	}

	// Bootstrap storage path must stay
	if h.Config().Storage.SQLite.Path != boot.Storage.SQLite.Path {
		t.Fatal("storage path changed")
	}
}

func TestHubPreserveRelayPassword(t *testing.T) {
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

	boot := config.Default()
	boot.Server.Hostname = "mail.test"
	boot.Storage.SQLite.Path = filepath.Join(dir, "t.db")
	boot.Mailstore.Root = filepath.Join(dir, "mail")
	boot.SMTP.Relay.Host = "smtp.example.com:587"
	boot.SMTP.Relay.Password = "secret"

	h := NewHub(store, boot)
	_ = h.Load(ctx)

	// First persist smtp with password
	if err := h.PutSection(ctx, "smtp", []byte(`{"relay":{"host":"smtp.example.com:587","password":"secret"},"queue":{"enabled":true,"workers":1,"poll_interval":"5s","max_attempts":8,"batch_size":10}}`)); err != nil {
		t.Fatal(err)
	}
	if err := h.PutSection(ctx, "smtp", []byte(`{"relay":{"host":"smtp.example.com:587","password":"***"},"queue":{"enabled":true,"workers":1,"poll_interval":"5s","max_attempts":8,"batch_size":10}}`)); err != nil {
		t.Fatal(err)
	}
	if h.Config().SMTP.Relay.Password != "secret" {
		t.Fatalf("password=%q", h.Config().SMTP.Relay.Password)
	}
}
