package xmpp

import (
	"context"
	"github.com/tayga/tms/internal/storage"
	"testing"
)

func TestDomainDirectoryIsOptInScopedAndPreservesPersonalRoster(t *testing.T) {
	srv, store, owner := testServer(t)
	ctx := context.Background()
	add := func(email, domain, tenant string, enabled bool) {
		t.Helper()
		_, err := store.CreateUser(ctx, &storage.User{Email: email, LocalPart: ParseJID(email).Local, DomainID: domain, TenantID: tenant, DisplayName: email, PasswordHash: "x", AuthSource: "local", Enabled: enabled})
		if err != nil {
			t.Fatal(err)
		}
	}
	add("bob@example.com", owner.DomainID, owner.TenantID, true)
	add("disabled@example.com", owner.DomainID, owner.TenantID, false)
	other, err := store.CreateDomain(ctx, owner.TenantID, "other.example.com")
	if err != nil {
		t.Fatal(err)
	}
	add("carol@other.example.com", other.ID, owner.TenantID, true)
	check := func(want int) []RosterEntry {
		t.Helper()
		rows, err := srv.ListChatRoster(ctx, owner.ID)
		if len(rows) > 0 && rows[0].Self {
			rows = rows[1:]
		}
		if err != nil || len(rows) != want {
			t.Fatalf("roster=%+v err=%v want=%d", rows, err, want)
		}
		return rows
	}
	check(0)
	if err := store.PutSetting(ctx, "domain.gal."+owner.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	check(0) // Mail GAL must not enable XMPP GAL.
	if err := store.PutSetting(ctx, "domain.xmpp_gal."+owner.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	rows := check(1)
	if rows[0].JID != "bob@example.com" || rows[0].Subscription != "none" {
		t.Fatalf("directory=%+v", rows)
	}
	if err := srv.UpsertChatRoster(ctx, owner.ID, "bob@example.com", "Personal Bob"); err != nil {
		t.Fatal(err)
	}
	if rows := check(1); rows[0].Name != "Personal Bob" {
		t.Fatal(rows)
	}
	if err := store.PutSetting(ctx, "domain.xmpp_gal."+owner.DomainID, "false"); err != nil {
		t.Fatal(err)
	}
	if rows := check(1); rows[0].Name != "Personal Bob" {
		t.Fatal(rows)
	}
}
