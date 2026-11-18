package mmsettings

import (
	"context"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"path/filepath"
	"testing"
)

func TestEmailDraftScopeIsolationAndDisabledDelivery(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "email.db")
	st, e := storage.Open(ctx, cfg.Storage)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	a, _ := st.CreateTenant(ctx, "A")
	b, _ := st.CreateTenant(ctx, "B")
	one, _ := st.CreateDomain(ctx, a.ID, "one.test")
	two, _ := st.CreateDomain(ctx, a.ID, "two.test")
	foreign, _ := st.CreateDomain(ctx, b.ID, "foreign.test")
	channels := chat.New(st)
	makeChannel := func(d *storage.Domain) string {
		u, e := st.CreateUser(ctx, &storage.User{TenantID: d.TenantID, DomainID: d.ID, Email: "user@" + d.Name, LocalPart: "user", Enabled: true, AuthSource: "local"})
		if e != nil {
			t.Fatal(e)
		}
		if e = channels.EnsureDefault(ctx, u); e != nil {
			t.Fatal(e)
		}
		items, e := channels.Channels(ctx, u)
		if e != nil {
			t.Fatal(e)
		}
		return items[0].ID
	}
	channel := makeChannel(one)
	otherChannel := makeChannel(foreign)
	service := &Service{Store: st}
	scope := Scope{a.ID, one.ID}
	prefix := "[Team]"
	incoming := "inbox@one.test"
	from := "chat@one.test"
	if e = service.Save(ctx, Scope{TenantID: a.ID}, Overrides{Email: &EmailOverrides{SubjectPrefix: &prefix}}); e != nil {
		t.Fatal(e)
	}
	draft := EmailOverrides{IncomingAddress: &incoming, IncomingChannelID: &channel, OutgoingFrom: &from}
	if e = service.Save(ctx, scope, Overrides{Email: &draft}); e != nil {
		t.Fatal(e)
	}
	resolved, e := service.Resolve(ctx, scope)
	if e != nil {
		t.Fatal(e)
	}
	if resolved.Email.IncomingEnabled || resolved.Email.OutgoingEnabled || !resolved.Email.UserOptInRequired || resolved.Email.RecipientPolicy != "own_address" || resolved.Email.IncomingAddress != incoming || resolved.Email.SubjectPrefix != prefix {
		t.Fatal(resolved.Email)
	}
	sibling, e := service.Resolve(ctx, Scope{a.ID, two.ID})
	if e != nil {
		t.Fatal(e)
	}
	if sibling.Email.IncomingAddress != "" || sibling.Email.OutgoingFrom != "" || sibling.Email.IncomingChannelID != "" || sibling.Email.SubjectPrefix != prefix {
		t.Fatal("draft leaked into sibling", sibling.Email)
	}
	if e = service.Save(ctx, Scope{TenantID: a.ID}, Overrides{Email: &draft}); e == nil {
		t.Fatal("concrete parent route accepted")
	}
	bad := "chat@two.test"
	if e = service.Save(ctx, scope, Overrides{Email: &EmailOverrides{OutgoingFrom: &bad}}); e == nil {
		t.Fatal("foreign domain sender accepted")
	}
	if e = service.Save(ctx, scope, Overrides{Email: &EmailOverrides{IncomingChannelID: &otherChannel}}); e == nil {
		t.Fatal("foreign tenant channel accepted")
	}
	bad = "valid@one.test\r\nBcc: victim@other.test"
	if e = service.Save(ctx, scope, Overrides{Email: &EmailOverrides{IncomingAddress: &bad}}); e == nil {
		t.Fatal("header injection accepted")
	}
	if e = service.Save(ctx, scope, Overrides{}); e != nil {
		t.Fatal(e)
	}
	resolved, _ = service.Resolve(ctx, scope)
	if resolved.Email.IncomingAddress != "" || resolved.Email.SubjectPrefix != prefix {
		t.Fatal("reset failed")
	}
}
