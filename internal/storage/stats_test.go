package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestServerAndTenantStats(t *testing.T) {
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
	defer os.RemoveAll(dir)

	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	mb, _ := store.EnsureMailbox(ctx, u.ID, "INBOX", filepath.Join(dir, "mail"))
	_, _ = store.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, Size: 42, FilePath: "x", InternalDate: time.Now().UTC(),
	})

	st, err := store.ServerStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Users < 1 || st.Messages < 1 || st.BytesStored < 42 {
		t.Fatalf("stats %+v", st)
	}
	ts, err := store.TenantStats(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Messages != 1 || ts.BytesStored != 42 {
		t.Fatalf("tenant %+v", ts)
	}
}
