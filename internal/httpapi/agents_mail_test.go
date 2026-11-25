package httpapi

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestAgentMailToolsUseOnlyCurrentMailbox(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.HTTP.Admins = nil
	cfg.FlowSync.Enabled = false
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "mail.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant, _ := store.CreateTenant(ctx, "test")
	domain, _ := store.CreateDomain(ctx, tenant.ID, "agents.test")
	alice, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "alice@agents.test", LocalPart: "alice", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "bob@agents.test", LocalPart: "bob", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "messages"))
	server := New(cfg, nil, store, auth.NewLayer(store, cfg.LDAP, cfg.MFA, cfg.OIDC), ms, nil, nil)
	var bobMessage *storage.Message
	for _, user := range []*storage.User{alice, bob} {
		box, err := store.EnsureMailbox(ctx, user.ID, "INBOX", user.Email)
		if err != nil {
			t.Fatal(err)
		}
		raw := []byte("From: test@agents.test\r\nTo: " + user.Email + "\r\nSubject: PRIVATE-" + user.LocalPart + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nprivate body " + user.LocalPart)
		name := filepath.Join(dir, user.LocalPart+".eml")
		if err = os.WriteFile(name, raw, 0600); err != nil {
			t.Fatal(err)
		}
		m, err := store.InsertMessage(ctx, &storage.Message{MailboxID: box.ID, FilePath: name, Size: int64(len(raw)), InternalDate: time.Now(), Subject: "PRIVATE-" + user.LocalPart, FromAddr: "test@agents.test", ToAddr: user.Email})
		if err != nil {
			t.Fatal(err)
		}
		if err = store.UpsertMessageSearch(ctx, m.ID, m.Subject, m.FromAddr, m.ToAddr, "private body "+user.LocalPart); err != nil {
			t.Fatal(err)
		}
		if user.ID == bob.ID {
			bobMessage = m
		}
	}
	hits, err := server.agentMailTool(ctx, alice, "mail_search", map[string]string{"query": "private"})
	if err != nil {
		t.Fatal(err)
	}
	rows := hits.([]map[string]string)
	if len(rows) != 1 || rows[0]["subject"] != "PRIVATE-alice" {
		t.Fatalf("mail isolation failed: %#v", rows)
	}
	if _, err = server.agentMailTool(ctx, alice, "mail_read", map[string]string{"message_id": bobMessage.ID}); err == nil {
		t.Fatal("read another user's mail")
	}
	read, err := server.agentMailTool(ctx, bob, "mail_read", map[string]string{"message_id": bobMessage.ID})
	if err != nil {
		t.Fatal(err)
	}
	if read.(map[string]string)["text"] != "private body bob" {
		t.Fatal("missing body")
	}
	// Sending goes through the same mail composition/delivery path as the web UI.
	if _, err = server.agentMailTool(ctx, alice, "mail_send", map[string]string{"to": bob.Email, "subject": "fixture-confirmed", "text": "synthetic only"}); err != nil {
		t.Fatal(err)
	}
	if err = store.UpdateUserEnabled(ctx, bob.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = server.agentMailTool(ctx, bob, "mail_read", map[string]string{"message_id": bobMessage.ID}); err == nil {
		t.Fatal("disabled session read mail")
	}
}
