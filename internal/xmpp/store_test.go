package xmpp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func testServer(t *testing.T) (*Server, *storage.Store, *storage.User) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sqlStore := store.(*storage.Store)
	ten, err := sqlStore.CreateTenant(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := sqlStore.CreateDomain(ctx, ten.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	u, err := sqlStore.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID,
		Email: "alice@example.com", LocalPart: "alice",
		DisplayName: "Alice", PasswordHash: "x", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Server:  config.ServerConfig{Hostname: "example.com"},
		Storage: config.StorageConfig{Driver: "sqlite"},
		XMPP:    config.XMPPConfig{Enabled: true, Listen: ":0", RequireTLS: false},
	}
	srv := New(cfg, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})), store, nil, nil)
	return srv, sqlStore, u
}

func TestPEPPublishGet(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	bare := u.Email
	node := "urn:xmpp:omemo:2:devices"
	payload := `<list xmlns='urn:xmpp:omemo:2'><device id='42'/></list>`
	if err := srv.publishPEP(ctx, bare, node, "current", payload); err != nil {
		t.Fatal(err)
	}
	items, err := srv.getPEPItems(ctx, bare, node)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Payload != payload {
		t.Fatalf("items=%+v", items)
	}
}

func TestRosterAndMAM(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	if err := srv.upsertRoster(ctx, u.ID, rosterItem{
		JID: "bob@example.com", Name: "Bob", Subscription: "both", GroupsJSON: `["Friends"]`,
	}); err != nil {
		t.Fatal(err)
	}
	items, err := srv.listRoster(ctx, u.ID)
	if err != nil || len(items) != 1 || items[0].JID != "bob@example.com" {
		t.Fatalf("roster=%+v err=%v", items, err)
	}

	stanza := `<message from='alice@example.com/a' to='bob@example.com'><body>hi</body></message>`
	if err := srv.archiveMAM(ctx, "alice@example.com", "bob@example.com", "sid1", stanza); err != nil {
		t.Fatal(err)
	}
	rows, err := srv.queryMAM(ctx, "alice@example.com", "bob@example.com", 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("mam=%+v err=%v", rows, err)
	}
}

func TestOfflineFlush(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	_ = srv.storeOffline(ctx, u.Email, `<message><body>offline</body></message>`)
	var got []string
	if err := srv.flushOffline(ctx, u.ID, func(b []byte) { got = append(got, string(b)) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	// second flush empty
	got = nil
	_ = srv.flushOffline(ctx, u.ID, func(b []byte) { got = append(got, string(b)) })
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}
