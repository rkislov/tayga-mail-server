package backup_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestWriteTarGz(t *testing.T) {
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
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: "x", Enabled: true,
	})
	_, _ = store.PutSieveScript(ctx, u.ID, "default", "keep;")
	_ = store.SetActiveSieveScript(ctx, u.ID, "default")

	var buf bytes.Buffer
	if err := backup.WriteTarGz(ctx, store, backup.Options{TenantID: tenant.ID}, &buf); err != nil {
		t.Fatal(err)
	}
	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	data, _ := io.ReadAll(gr)
	if len(data) < 100 {
		t.Fatalf("archive too small %d", len(data))
	}
}
