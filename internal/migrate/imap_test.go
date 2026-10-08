package migrate

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/memory"
	imapserver "github.com/emersion/go-imap/server"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestIMAPArchiveImportIndexedAndReadable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, _ := st.CreateTenant(ctx, "t")
	dom, _ := st.CreateDomain(ctx, tenant.ID, "ex.com")
	user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "u@ex.com", LocalPart: "u", Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	be := memory.New()
	source, _ := be.Login(nil, "username", "password")
	_ = source.CreateMailbox("Archive")
	box, _ := source.GetMailbox("Archive")
	raw := []byte("From: source@ex.com\r\nTo: u@ex.com\r\nSubject: Migrated archive\r\nMessage-ID: <archive@ex.com>\r\n\r\n" + string(bytes.Repeat([]byte("archive body "), 100)))
	if err = box.CreateMessage([]string{imap.SeenFlag}, time.Now(), bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	srv := imapserver.New(be)
	srv.AllowInsecureAuth = true
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(ln)
	cfg := config.Default()
	cfg.Mailstore.Root = filepath.Join(dir, "mail")
	cfg.Server.SecretsKey = "test archive key"
	ms := mailstore.New(cfg.Mailstore.Root)
	svc, err := New(cfg, nil, st, ms)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := encryptPassword(svc.key, "password")
	if err != nil {
		t.Fatal(err)
	}
	host, portText, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portText)
	job := &Job{ID: "test", UserID: user.ID, Host: host, Port: port, Username: "username", PasswordCiphertext: cipher, Options: `{"folders":["Archive"]}`}
	if err = svc.runIMAP(ctx, job); err != nil {
		t.Fatal(err)
	}
	if job.Copied != 1 || job.Errors != 0 {
		t.Fatalf("progress %+v", job)
	}
	mb, err := st.GetMailbox(ctx, user.ID, "Archive")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := st.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("messages %d: %v", len(msgs), err)
	}
	if !msgs[0].Archived || msgs[0].Subject != "Migrated archive" {
		t.Fatalf("headers %+v", msgs[0])
	}
	data, err := ms.Read(msgs[0].FilePath)
	if err != nil || !bytes.Equal(raw, data) {
		t.Fatalf("read archive %v", err)
	}
	// A second migration must skip the already imported compressed message.
	job.Copied = 0
	job.Skipped = 0
	if err = svc.runIMAP(ctx, job); err != nil {
		t.Fatal(err)
	}
	if job.Copied != 0 || job.Skipped != 1 {
		t.Fatalf("repeat copied=%d skipped=%d", job.Copied, job.Skipped)
	}
}
