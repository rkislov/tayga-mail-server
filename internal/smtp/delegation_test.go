package smtp

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

func TestDelegatedSMTPModesAndRevocationDuringDATA(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, _ := st.CreateTenant(ctx, "Delegation")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	create := func(name string) *storage.User {
		u, e := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: name, Email: name + "@example.com", Enabled: true})
		if e != nil {
			t.Fatal(e)
		}
		return u
	}
	owner, assistant := create("owner"), create("assistant")
	ms := mailstore.New(filepath.Join(dir, "mail"))
	out := &aliasCaptureSender{}
	session := &session{backend: &backend{store: st, mailstore: ms, sieve: sieve.New(st, ms, nil), out: out, log: slog.Default(), maxSize: 1 << 20}, user: assistant}
	grant := &storage.MailboxDelegate{OwnerID: owner.ID, DelegateID: assistant.ID, CanSendOnBehalf: true}
	if err = st.UpsertMailboxDelegate(ctx, grant); err != nil {
		t.Fatal(err)
	}
	envelope := func() {
		t.Helper()
		if err := session.Mail(owner.Email, &gosmtp.MailOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := session.Rcpt("remote@outside.example", nil); err != nil {
			t.Fatal(err)
		}
	}
	raw := func(sender string) string {
		return "From: owner@example.com\r\n" + sender + "To: remote@outside.example\r\n\r\nTest"
	}
	envelope()
	if err = session.Data(strings.NewReader(raw(""))); err == nil {
		t.Fatal("on behalf grant allowed send-as")
	}
	envelope()
	if err = session.Data(strings.NewReader(raw("Sender: assistant@example.com\r\n"))); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out.data), "Sender: assistant@example.com") {
		t.Fatal("lost actor", string(out.data))
	}
	for _, sender := range []string{"Sender: owner@example.com\r\n", "Sender: forged@outside.example\r\n", "Sender: assistant@example.com\r\nSender: assistant@example.com\r\n"} {
		envelope()
		if err = session.Data(strings.NewReader(raw(sender))); err == nil {
			t.Fatal("forged Sender accepted", sender)
		}
	}
	envelope()
	if err = st.DeleteMailboxDelegate(ctx, owner.ID, assistant.ID); err != nil {
		t.Fatal(err)
	}
	if err = session.Data(strings.NewReader(raw("Sender: assistant@example.com\r\n"))); err == nil {
		t.Fatal("revocation after MAIL not enforced")
	}
	grant.CanSendAs = true
	grant.CanSendOnBehalf = false
	if err = st.UpsertMailboxDelegate(ctx, grant); err != nil {
		t.Fatal(err)
	}
	envelope()
	if err = session.Data(strings.NewReader(raw(""))); err != nil {
		t.Fatal(err)
	}
	envelope()
	if err = session.Data(strings.NewReader(raw("Sender: assistant@example.com\r\n"))); err == nil {
		t.Fatal("send-as grant allowed on behalf")
	}
}
