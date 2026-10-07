package backup_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	ctx := context.Background()

	srcStore, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(srcDir, "src.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srcStore.Close()

	tenant, _ := srcStore.CreateTenant(ctx, "acme")
	dom, _ := srcStore.CreateDomain(ctx, tenant.ID, "acme.test")
	u, err := srcStore.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@acme.test", LocalPart: "u", AuthSource: "local",
		PasswordHash: "hash", Enabled: true, DisplayName: "User",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = srcStore.PutSieveScript(ctx, u.ID, "default", "keep;")
	_ = srcStore.SetActiveSieveScript(ctx, u.ID, "default")

	mailRoot := filepath.Join(srcDir, "mail")
	ms := mailstore.New(mailRoot)
	root, _ := ms.EnsureUser(u.Email)
	mb, _ := srcStore.EnsureMailbox(ctx, u.ID, "INBOX", root)
	rel, size, err := ms.Deliver(u.Email, "INBOX", []byte("From: a\r\nSubject: hi\r\n\r\nbody"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = srcStore.InsertMessage(ctx, &storage.Message{
		MailboxID: mb.ID, Size: size, FilePath: rel, InternalDate: time.Now().UTC(),
	})

	var buf bytes.Buffer
	if err := backup.WriteTarGz(ctx, srcStore, backup.Options{
		TenantID: tenant.ID, IncludeMail: true, MailRoot: mailRoot,
	}, &buf); err != nil {
		t.Fatal(err)
	}

	dstStore, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dstDir, "dst.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer dstStore.Close()
	dstMail := filepath.Join(dstDir, "mail")
	_ = os.MkdirAll(dstMail, 0o750)

	rep, err := backup.RestoreTarGz(ctx, dstStore, bytes.NewReader(buf.Bytes()), backup.RestoreOptions{
		MailRoot: dstMail, IncludeMail: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.UsersCreated != 1 || rep.ScriptsRestored < 1 {
		t.Fatalf("report %+v", rep)
	}
	got, err := dstStore.GetUserByEmail(ctx, "u@acme.test")
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordHash != "hash" {
		t.Fatalf("hash %q", got.PasswordHash)
	}
	mbs, _ := dstStore.ListMailboxes(ctx, got.ID)
	if len(mbs) == 0 {
		t.Fatal("expected mailbox")
	}
	msgs, _ := dstStore.ListMessages(ctx, mbs[0].ID)
	if len(msgs) < 1 {
		t.Fatalf("expected indexed messages, got %d", len(msgs))
	}
}
