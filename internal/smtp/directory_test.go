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

func TestDistributionGroupSMTPDelivery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, _ := st.CreateTenant(ctx, "Test")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	var users []*storage.User
	for _, name := range []string{"alice", "bob"} {
		u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: name, Email: name + "@example.com", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u)
	}
	root, err := st.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: domain.ID, LocalPart: "all", Name: "All", MemberIDs: []string{users[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: domain.ID, ParentID: root.ID, LocalPart: "support", Name: "Support", MemberIDs: []string{users[0].ID, users[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	session := &session{backend: &backend{store: st, mailstore: ms, sieve: sieve.New(st, ms, nil), log: slog.Default(), maxSize: 1 << 20}}
	if err = session.Mail("external@example.net", &gosmtp.MailOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{root.Email, child.Email, users[0].Email} {
		if err = session.Rcpt(address, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err = session.Data(strings.NewReader("From: external@example.net\r\nTo: all@example.com\r\nSubject: Test\r\n\r\nTeam message\r\n")); err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
		if err != nil {
			t.Fatal(err)
		}
		messages, err := st.ListMessages(ctx, mb.ID)
		if err != nil || len(messages) != 1 {
			t.Fatal("expected one copy per employee", u.Email, len(messages), err)
		}
	}
	empty, err := st.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: domain.ID, LocalPart: "empty", Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	if err = session.Rcpt(empty.Email, nil); err == nil {
		t.Fatal("accepted empty distribution group")
	}
	// Keep the original alias for Sieve envelope rules, but deduplicate aliases
	// and group expansions by mailbox. RSET and a fresh MAIL must clear this set.
	if _, err = st.CreateAlias(ctx, domain.ID, users[0].ID, "sales"); err != nil {
		t.Fatal(err)
	}
	session.Reset()
	for attempt := 0; attempt < 2; attempt++ {
		if err = session.Mail("external@example.net", &gosmtp.MailOptions{}); err != nil {
			t.Fatal(err)
		}
		for _, address := range []string{"sales@example.com", users[0].Email, root.Email} {
			if err = session.Rcpt(address, nil); err != nil {
				t.Fatal(err)
			}
		}
		if len(session.toLocal) != 2 || session.toLocal[0] != "sales@example.com" {
			t.Fatal("alias envelope or mailbox deduplication lost", session.toLocal)
		}
	}
	session.Reset()
	if len(session.toLocal) != 0 || len(session.localSeen) != 0 {
		t.Fatal("RSET did not clear recipients")
	}
}
