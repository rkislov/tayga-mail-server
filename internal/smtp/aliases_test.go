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

func TestDomainAliasSMTPDelivery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, _ := st.CreateTenant(ctx, "Test")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	u, err := st.CreateUser(ctx, &storage.User{DomainID: domain.ID, TenantID: tenant.ID, LocalPart: "owner", Email: "owner@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.SaveDomainAlias(ctx, &storage.DomainAlias{DomainID: domain.ID, Name: "alias.example"}); err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateAlias(ctx, domain.ID, u.ID, "sales"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: domain.ID, LocalPart: "team", Name: "Team", MemberIDs: []string{u.ID}}); err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	engine := sieve.New(st, ms, nil)
	// The original RCPT must reach Sieve even when it names a domain alias.
	if _, err = st.PutSieveScript(ctx, u.ID, "alias", `require ["envelope", "fileinto"]; if envelope :is "to" "sales@alias.example" { fileinto "Alias"; stop; } keep;`); err != nil {
		t.Fatal(err)
	}
	if err = st.SetActiveSieveScript(ctx, u.ID, "alias"); err != nil {
		t.Fatal(err)
	}
	session := &session{backend: &backend{store: st, mailstore: ms, sieve: engine, log: slog.Default(), maxSize: 1 << 20}}
	if err = session.Mail("external@outside.example", &gosmtp.MailOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"sales@alias.example", "owner@alias.example", u.Email, "team@alias.example"} {
		if err = session.Rcpt(address, nil); err != nil {
			t.Fatal(address, err)
		}
	}
	if len(session.toLocal) != 1 || session.toLocal[0] != "sales@alias.example" {
		t.Fatal(session.toLocal)
	}
	if err = session.Data(strings.NewReader("From: external@outside.example\r\nTo: sales@alias.example\r\nSubject: Alias\r\n\r\nTest\r\n")); err != nil {
		t.Fatal(err)
	}
	box, err := st.GetMailbox(ctx, u.ID, "Alias")
	if err != nil {
		t.Fatal("Sieve lost alias envelope", err)
	}
	messages, err := st.ListMessages(ctx, box.ID)
	if err != nil || len(messages) != 1 {
		t.Fatal(messages, err)
	}
	if err = session.Rcpt("missing@alias.example", nil); err == nil {
		t.Fatal("accepted unknown alias mailbox")
	}
	if err = st.UpdateUserEnabled(ctx, u.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = session.Rcpt("sales@alias.example", nil); err == nil {
		t.Fatal("accepted disabled alias mailbox")
	}
}

func TestAliasSMTPSubmissionAuthorization(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, _ := st.CreateTenant(ctx, "Test")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	u, err := st.CreateUser(ctx, &storage.User{DomainID: domain.ID, TenantID: tenant.ID, LocalPart: "owner", Email: "owner@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	alias, err := st.CreateAlias(ctx, domain.ID, u.ID, "sales")
	if err != nil {
		t.Fatal(err)
	}
	da, err := st.SaveDomainAlias(ctx, &storage.DomainAlias{DomainID: domain.ID, Name: "alias.example"})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	out := &aliasCaptureSender{}
	session := &session{backend: &backend{store: st, mailstore: ms, sieve: sieve.New(st, ms, nil), out: out, log: slog.Default(), maxSize: 1 << 20}, user: u}
	for _, from := range []string{"sales@example.com", "owner@alias.example", "sales@alias.example"} {
		if err = session.Mail(from, &gosmtp.MailOptions{}); err != nil {
			t.Fatal(from, err)
		}
		if err = session.Rcpt("remote@outside.example", nil); err != nil {
			t.Fatal(err)
		}
		if err = session.Data(strings.NewReader("From: " + from + "\r\nTo: remote@outside.example\r\nSubject: Send as\r\n\r\nTest\r\n")); err != nil {
			t.Fatal(err)
		}
		if out.from != from || !strings.Contains(string(out.data), "From: "+from) {
			t.Fatal("alias rewritten", out.from, string(out.data))
		}
	}
	if err = session.Mail("forged@outside.example", &gosmtp.MailOptions{}); err == nil {
		t.Fatal("forged envelope")
	}
	if err = session.Mail(u.Email, &gosmtp.MailOptions{}); err != nil {
		t.Fatal(err)
	}
	session.Rcpt("remote@outside.example", nil)
	if err = session.Data(strings.NewReader("From: forged@outside.example\r\n\r\nTest")); err == nil {
		t.Fatal("forged From header")
	}
	// Alias deleted between MAIL and DATA must be rechecked before local/remote delivery.
	if err = session.Mail("sales@alias.example", &gosmtp.MailOptions{}); err != nil {
		t.Fatal(err)
	}
	session.Rcpt("remote@outside.example", nil)
	if err = st.DeleteAlias(ctx, alias.DomainID, alias.UserID, alias.ID); err != nil {
		t.Fatal(err)
	}
	if err = session.Data(strings.NewReader("From: sales@alias.example\r\n\r\nTest")); err == nil {
		t.Fatal("revoked alias From")
	}
	if err = st.DeleteDomainAlias(ctx, da.DomainID, da.ID); err != nil {
		t.Fatal(err)
	}
	if err = session.Mail("owner@alias.example", &gosmtp.MailOptions{}); err == nil {
		t.Fatal("revoked domain From")
	}
}

type aliasCaptureSender struct {
	from string
	data []byte
}

func (s *aliasCaptureSender) Send(from string, to []string, data []byte) error {
	s.from = from
	s.data = append([]byte{}, data...)
	return nil
}
