package mailstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestSyncObjectsPushPull(t *testing.T) {
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

	mailRoot := filepath.Join(dir, "mail")
	ms := New(mailRoot)
	blob := NewMemBlob()
	ms.SetObjectStore(blob, nil)

	rel, _, err := ms.Deliver(u.Email, "INBOX", []byte("msg-one"))
	if err != nil {
		t.Fatal(err)
	}
	mb, err := store.EnsureMailbox(ctx, u.ID, "INBOX", ms.UserRoot(u.Email))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, Size: 7, FilePath: rel, InternalDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wipe blob and re-push via sync.
	blob.m = map[string][]byte{}
	rep, err := SyncObjects(ctx, store, ms, blob, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pushed != 1 {
		t.Fatalf("pushed=%d report=%+v", rep.Pushed, rep)
	}

	// Wipe local, pull back.
	if err := os.Remove(ms.Abs(rel)); err != nil {
		t.Fatal(err)
	}
	rep, err = SyncObjects(ctx, store, ms, blob, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Pulled != 1 {
		t.Fatalf("pulled=%d report=%+v", rep.Pulled, rep)
	}
	data, err := os.ReadFile(ms.Abs(rel))
	if err != nil || string(data) != "msg-one" {
		t.Fatalf("cache %q %v", data, err)
	}

	rep, err = SyncObjects(ctx, store, ms, blob, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Unchanged != 1 {
		t.Fatalf("unchanged=%d report=%+v", rep.Unchanged, rep)
	}
}