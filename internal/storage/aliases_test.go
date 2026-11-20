package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestAliasesResolutionAndLifecycle(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	domainID := f.user.DomainID
	a, err := f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: domainID, Name: "Alias.EXAMPLE."})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "alias.example" {
		t.Fatal(a)
	}
	mailbox, err := f.s.CreateAlias(ctx, domainID, f.user.ID, "SALES")
	if err != nil {
		t.Fatal(err)
	}
	group, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: domainID, LocalPart: "team", Name: "Team", MemberIDs: []string{f.user.ID}})
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"OWNER@ALIAS.EXAMPLE", "sales@alias.example", mailbox.Email, f.user.Email, "team@alias.example", group.Email} {
		users, err := f.s.ResolveRecipients(ctx, address)
		if err != nil || len(users) != 1 || users[0].ID != f.user.ID {
			t.Fatal(address, users, err)
		}
	}
	if _, err = f.s.GetUserByEmail(ctx, "owner@alias.example"); err != ErrNotFound {
		t.Fatal("alias became login", err)
	}
	if _, err = f.s.GetDomainByName(ctx, "alias.example"); err != ErrNotFound {
		t.Fatal("alias became provisioning domain", err)
	}
	for _, address := range []string{"owner@alias.example", "sales@alias.example"} {
		tx, e := f.s.db.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		id, e := f.s.calendarRecipientTx(ctx, tx, address)
		tx.Rollback()
		if e != nil || id != f.user.ID {
			t.Fatal("calendar alias", id, e)
		}
	}
	mailbox.LocalPart = "support"
	if _, err = f.s.SaveAlias(ctx, mailbox); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ResolveRecipient(ctx, "sales@alias.example"); err != ErrNotFound {
		t.Fatal("old alias resolves", err)
	}
	if user, err := f.s.ResolveRecipient(ctx, "support@alias.example"); err != nil || user.ID != f.user.ID {
		t.Fatal(user, err)
	}
	list, err := f.s.ListAliases(ctx, f.user.ID)
	if err != nil || len(list) != 1 || list[0].Email != "support@example.com" {
		t.Fatal(list, err)
	}
	if err = f.s.UpdateUserEnabled(ctx, f.user.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"owner@alias.example", "support@alias.example"} {
		if _, err = f.s.ResolveRecipient(ctx, address); err != ErrNotFound {
			t.Fatal("disabled alias resolves", err)
		}
	}
	f.s.UpdateUserEnabled(ctx, f.user.ID, true)
	a.Name = "renamed.example"
	if _, err = f.s.SaveDomainAlias(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ResolveRecipient(ctx, "owner@alias.example"); err != ErrNotFound {
		t.Fatal(err)
	}
	if _, err = f.s.ResolveRecipient(ctx, "support@renamed.example"); err != nil {
		t.Fatal(err)
	}
	if err = f.s.DeleteAlias(ctx, domainID, f.user.ID, mailbox.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.s.DeleteAlias(ctx, domainID, f.user.ID, mailbox.ID); err != ErrNotFound {
		t.Fatal(err)
	}
	if err = f.s.DeleteDomainAlias(ctx, domainID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ResolveRecipient(ctx, "owner@renamed.example"); err != ErrNotFound {
		t.Fatal(err)
	}
	if _, err = f.s.GetUserByID(ctx, f.user.ID); err != nil {
		t.Fatal("alias removal removed mailbox", err)
	}
}

func TestAliasesValidationAndIsolation(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	domainID := f.user.DomainID
	other, err := f.s.CreateDomain(ctx, f.user.TenantID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := f.s.CreateTenant(ctx, "Other")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := f.s.CreateDomain(ctx, tenant.ID, "foreign.example")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "bad domain.example", "a..example", "-bad.example", "@test.example", "https://a.example", "a.example/"} {
		if _, err = f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: domainID, Name: name}); !errors.Is(err, ErrAliasInvalid) {
			t.Fatal(name, err)
		}
	}
	for _, name := range []string{"example.com", "OTHER.EXAMPLE"} {
		if _, err = f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: domainID, Name: name}); err != ErrAlreadyExists {
			t.Fatal(name, err)
		}
	}
	a, err := f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: domainID, Name: "alias.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CreateDomain(ctx, tenant.ID, "ALIAS.EXAMPLE"); err != ErrAlreadyExists {
		t.Fatal("alias stolen by real domain", err)
	}
	if _, err = f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: other.ID, Name: a.Name}); err != ErrAlreadyExists {
		t.Fatal(err)
	}
	if _, err = f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: other.ID, ID: a.ID, Name: "stolen.example"}); err != ErrNotFound {
		t.Fatal(err)
	}
	if err = f.s.DeleteDomainAlias(ctx, other.ID, a.ID); err != ErrNotFound {
		t.Fatal(err)
	}
	for _, local := range []string{"", "bad name", "a@b", ".bad", "bad..name", "bad."} {
		if _, err = f.s.CreateAlias(ctx, domainID, f.user.ID, local); err != ErrAliasInvalid {
			t.Fatal(local, err)
		}
	}
	if _, err = f.s.CreateAlias(ctx, domainID, f.user.ID, "OWNER"); err != ErrAlreadyExists {
		t.Fatal("mailbox shadowed", err)
	}
	group, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: domainID, LocalPart: "team", Name: "Team"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.CreateAlias(ctx, domainID, f.user.ID, group.LocalPart); err != ErrAlreadyExists {
		t.Fatal(err)
	}
	if _, err = f.s.CreateAlias(ctx, foreign.ID, f.user.ID, "bad"); err != ErrAliasInvalid {
		t.Fatal("cross tenant target", err)
	}
	if _, err = f.s.CreateAlias(ctx, domainID, f.room.UserID, "resource"); err != ErrAliasInvalid {
		t.Fatal("resource alias bypass", err)
	}
	alias, err := f.s.CreateAlias(ctx, other.ID, f.user.ID, "cross")
	if err != nil {
		t.Fatal(err)
	}
	if u, e := f.s.ResolveRecipient(ctx, "cross@other.example"); e != nil || u.ID != f.user.ID {
		t.Fatal(u, e)
	}
	if _, err = f.s.CreateUser(ctx, &User{DomainID: other.ID, TenantID: f.user.TenantID, Email: alias.Email, LocalPart: alias.LocalPart, Enabled: true}); err != ErrAlreadyExists {
		t.Fatal("alias shadowed", err)
	}
	if _, err = f.s.SaveAlias(ctx, &Alias{ID: alias.ID, DomainID: domainID, UserID: f.user.ID, LocalPart: "stolen"}); err != ErrNotFound {
		t.Fatal(err)
	}
}

func TestAliasesConcurrentAddressClaims(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	driver, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	peer := driver.(*Store)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, s := range []*Store{f.s, peer} {
		wg.Add(1)
		go func(i int, s *Store) {
			defer wg.Done()
			if i == 0 {
				_, e := s.SaveDomainAlias(ctx, &DomainAlias{DomainID: f.user.DomainID, Name: "race.example"})
				results <- e
			} else {
				_, e := s.CreateDomain(ctx, f.user.TenantID, "race.example")
				results <- e
			}
		}(i, s)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("namespace race", success)
	}
	var n int
	if err = f.s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM domains WHERE name='race.example')+(SELECT COUNT(*) FROM domain_aliases WHERE name='race.example')`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestAliasesConcurrentMailboxClaims(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	driver, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	peer := driver.(*Store)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, s := range []*Store{f.s, peer} {
		wg.Add(1)
		go func(i int, s *Store) {
			defer wg.Done()
			if i == 0 {
				_, e := s.CreateAlias(ctx, f.user.DomainID, f.user.ID, "race")
				results <- e
			} else {
				_, e := s.CreateUser(ctx, &User{DomainID: f.user.DomainID, TenantID: f.user.TenantID, LocalPart: "race", Email: "race@example.com", Enabled: true})
				results <- e
			}
		}(i, s)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("mailbox namespace race", success)
	}
	var n int
	if err = f.s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM users WHERE email='race@example.com')+(SELECT COUNT(*) FROM aliases WHERE local_part='race')`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestAliasSenderAddressesAndRevocation(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	a, err := f.s.CreateAlias(ctx, f.user.DomainID, f.user.ID, "sales")
	if err != nil {
		t.Fatal(err)
	}
	da, err := f.s.SaveDomainAlias(ctx, &DomainAlias{DomainID: f.user.DomainID, Name: "alias.example"})
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := f.s.SenderAddresses(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 4 || addresses[0] != f.user.Email {
		t.Fatal(addresses)
	}
	for _, address := range []string{f.user.Email, "sales@example.com", "owner@alias.example", "sales@alias.example"} {
		allowed, err := f.s.CanSendFrom(ctx, f.user.ID, address)
		if err != nil || !allowed {
			t.Fatal(address, allowed, err)
		}
	}
	peer := meetingUser(t, f, "peer")
	for _, address := range []string{peer.Email, "peer@alias.example", f.room.Email, "forged@outside.example", "bad\r\nFrom: owner@example.com"} {
		allowed, err := f.s.CanSendFrom(ctx, f.user.ID, address)
		if err != nil || allowed {
			t.Fatal("forged sender", address, allowed, err)
		}
	}
	if err = f.s.UpsertMailboxDelegate(ctx, &MailboxDelegate{OwnerID: peer.ID, DelegateID: f.user.ID, CanSendAs: true}); err != nil {
		t.Fatal(err)
	}
	if allowed, err := f.s.CanSendFrom(ctx, f.user.ID, "peer@alias.example"); err != nil || !allowed {
		t.Fatal("send-as grant", allowed, err)
	}
	if err = f.s.DeleteMailboxDelegate(ctx, peer.ID, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if allowed, err := f.s.CanSendFrom(ctx, f.user.ID, peer.Email); err != nil || allowed {
		t.Fatal("revoked delegation", allowed, err)
	}
	if err = f.s.DeleteAlias(ctx, a.DomainID, a.UserID, a.ID); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"sales@example.com", "sales@alias.example"} {
		if allowed, err := f.s.CanSendFrom(ctx, f.user.ID, address); err != nil || allowed {
			t.Fatal("deleted alias", address, allowed, err)
		}
	}
	if err = f.s.DeleteDomainAlias(ctx, da.DomainID, da.ID); err != nil {
		t.Fatal(err)
	}
	if allowed, err := f.s.CanSendFrom(ctx, f.user.ID, "owner@alias.example"); err != nil || allowed {
		t.Fatal("deleted domain alias", allowed, err)
	}
	if err = f.s.UpdateUserEnabled(ctx, f.user.ID, false); err != nil {
		t.Fatal(err)
	}
	if allowed, err := f.s.CanSendFrom(ctx, f.user.ID, f.user.Email); err != nil || allowed {
		t.Fatal("disabled sender", allowed, err)
	}
}
