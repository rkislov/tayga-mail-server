package smtp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/scan"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

type infectedScanner struct{}

func (infectedScanner) Name() string { return "test" }
func (infectedScanner) Scan(context.Context, []byte) (*scan.Result, error) {
	return &scan.Result{Clean: false, Virus: "Eicar", Scanner: "test"}, nil
}

func TestScanQuarantine(t *testing.T) {
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
			store: store, mailstore: ms, sieve: eng, log: nil,
			scan: &scanPolicy{
				scanner: infectedScanner{}, action: scan.ActionQuarantine, folder: "Quarantine",
			},
		},
	}
	msg := []byte("From: a@ex.com\r\nSubject: x\r\n\r\nbody\r\n")
	err = sess.deliver(ctx, u.Email, msg, "<1@ex.com>")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := store.GetMailbox(ctx, u.ID, "Quarantine")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("quarantine msgs=%d err=%v", len(msgs), err)
	}
	raw, err := ms.Read(msgs[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "X-Virus-Status: Quarantined") {
		t.Fatalf("%s", raw)
	}
}
