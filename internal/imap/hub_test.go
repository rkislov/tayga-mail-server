package imapserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emersion/go-imap/backend"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestHubNotifyFanout(t *testing.T) {
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
	mb, err := store.EnsureMailbox(ctx, u.ID, "INBOX", filepath.Join(dir, "mail"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, Size: 10, FilePath: "x", InternalDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	hub := NewHub(store)
	a := hub.Updates()
	b := hub.Updates()
	hub.Notify("u@ex.com", "INBOX")

	for _, ch := range []<-chan backend.Update{a, b} {
		select {
		case upd := <-ch:
			mu, ok := upd.(*backend.MailboxUpdate)
			if !ok || mu.MailboxStatus.Messages != 1 {
				t.Fatalf("got %#v", upd)
			}
			select {
			case <-upd.Done():
				t.Fatal("listener shares completion with another listener")
			default:
			}
			close(upd.Done())
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for update")
		}
	}
}

type countingPublisher struct{ n int }

func (c *countingPublisher) Publish(email, mailbox string) { c.n++ }

func TestHubNotifyRemoteSkipsCluster(t *testing.T) {
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
	mb, err := store.EnsureMailbox(ctx, u.ID, "INBOX", filepath.Join(dir, "mail"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, Size: 10, FilePath: "x", InternalDate: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}

	hub := NewHub(store)
	pub := &countingPublisher{}
	hub.SetCluster(pub)
	ch := hub.Updates()
	hub.NotifyRemote("u@ex.com", "INBOX")
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if pub.n != 0 {
		t.Fatalf("NotifyRemote must not publish to cluster, got %d", pub.n)
	}
	hub.Notify("u@ex.com", "INBOX")
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if pub.n != 1 {
		t.Fatalf("Notify should publish once, got %d", pub.n)
	}
}
