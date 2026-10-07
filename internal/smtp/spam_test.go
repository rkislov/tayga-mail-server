package smtp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/spam"
	"github.com/tayga/tms/internal/storage"
)

type quarantineSpam struct{}

func (quarantineSpam) Name() string { return "test" }
func (quarantineSpam) Check(_ context.Context, _ spam.Meta, _ []byte) (*spam.Result, error) {
	return &spam.Result{Score: 20, Required: 15, Action: "no action", Scanner: "test"}, nil
}

func TestSpamQuarantine(t *testing.T) {
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
	ms := mailstore.New(filepath.Join(dir, "mail"))
	eng := sieve.New(store, ms, nil)
	sess := &session{
		backend: &backend{
			store: store, mailstore: ms, sieve: eng, hostname: "mail.test",
			spam: &spamPolicy{
				checker: quarantineSpam{},
				cfg:     spam.Config{QuarantineAbove: 10, Folder: "Junk"},
			},
		},
		from:   "a@ex.com",
		remote: "1.2.3.4:25",
	}
	msg := []byte("From: a@ex.com\r\nSubject: spam\r\n\r\nbody\r\n")
	if err := sess.deliver(ctx, u.Email, msg, "<2@ex.com>"); err != nil {
		t.Fatal(err)
	}
	mb, err := store.GetMailbox(ctx, u.ID, "Junk")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("junk msgs=%d err=%v", len(msgs), err)
	}
	raw, err := ms.Read(msgs[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "X-Spam-Status: Quarantined") {
		t.Fatalf("%s", raw)
	}
}
