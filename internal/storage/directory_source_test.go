package storage

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestDirectoryImportAtomicityAndModes(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	domain := f.user.DomainID
	manual, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: domain, Name: "Manual", LocalPart: "manual", MemberIDs: []string{f.user.ID}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := f.s.GetDirectorySource(ctx, domain)
	if err != nil || source.Mode != "manual" || source.Revision != 0 {
		t.Fatal(source, err)
	}
	source.Mode, source.BaseDN = "ldap", "ou=Groups,dc=example,dc=com"
	source, err = f.s.SaveDirectorySource(ctx, domain, *source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveDirectoryGroup(ctx, manual); !errors.Is(err, ErrDirectoryManaged) {
		t.Fatal("manual write in LDAP mode", err)
	}
	if err = f.s.DeleteDirectoryGroup(ctx, domain, manual.ID, manual.Revision); !errors.Is(err, ErrDirectoryManaged) {
		t.Fatal("manual delete in LDAP mode", err)
	}
	imported := []DirectoryImportGroup{
		{ExternalID: "root", LocalPart: "company", Name: "Company"},
		{ExternalID: "child", ParentExternalID: "root", LocalPart: "team", Name: "Team", MemberIDs: []string{f.user.ID, f.user.ID}},
	}
	before, _ := f.s.ListDirectoryGroups(ctx, domain)
	if err = f.s.ValidateDirectoryImport(ctx, domain, source.Revision, imported); err != nil {
		t.Fatal(err)
	}
	after, _ := f.s.ListDirectoryGroups(ctx, domain)
	afterSource, _ := f.s.GetDirectorySource(ctx, domain)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(source, afterSource) {
		t.Fatal("preview changed data")
	}
	source, err = f.s.ApplyDirectoryImport(ctx, domain, source.Revision, imported)
	if err != nil || source.LastSuccess.IsZero() {
		t.Fatal(source, err)
	}
	groups, err := f.s.ListDirectoryGroups(ctx, domain)
	if err != nil || len(groups) != 3 {
		t.Fatal(groups, err)
	}
	byExternal := map[string]*DirectoryGroup{}
	for _, g := range groups {
		byExternal[g.ExternalID] = g
	}
	if !reflect.DeepEqual(manual, byExternal[""]) || byExternal["child"].ParentID != byExternal["root"].ID || len(byExternal["child"].MemberIDs) != 1 {
		t.Fatal("invalid merge", groups)
	}
	recipients, err := f.s.ResolveRecipients(ctx, "company@example.com")
	if err != nil || len(recipients) != 1 || recipients[0].ID != f.user.ID {
		t.Fatal(recipients, err)
	}
	// Every invalid snapshot must leave both the old hierarchy and sync status.
	for name, bad := range map[string][]DirectoryImportGroup{
		"empty":             {},
		"manual collision":  {{ExternalID: "a", LocalPart: "manual", Name: "A"}},
		"mailbox collision": {{ExternalID: "a", LocalPart: f.user.LocalPart, Name: "A"}},
		"invalid member":    {{ExternalID: "a", LocalPart: "a", Name: "A", MemberIDs: []string{f.room.UserID}}},
		"foreign member":    {{ExternalID: "a", LocalPart: "a", Name: "A", MemberIDs: []string{"foreign-user"}}},
		"duplicate id":      {{ExternalID: "a", LocalPart: "a", Name: "A"}, {ExternalID: "a", LocalPart: "b", Name: "B"}},
		"duplicate address": {{ExternalID: "a", LocalPart: "a", Name: "A"}, {ExternalID: "b", LocalPart: "a", Name: "B"}},
		"cycle":             {{ExternalID: "a", ParentExternalID: "b", LocalPart: "a", Name: "A"}, {ExternalID: "b", ParentExternalID: "a", LocalPart: "b", Name: "B"}},
		"missing parent":    {{ExternalID: "a", ParentExternalID: "b", LocalPart: "a", Name: "A"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.s.ApplyDirectoryImport(ctx, domain, source.Revision, bad); err == nil {
				t.Fatal("bad snapshot accepted")
			}
			got, _ := f.s.ListDirectoryGroups(ctx, domain)
			current, _ := f.s.GetDirectorySource(ctx, domain)
			if !reflect.DeepEqual(groups, got) || !reflect.DeepEqual(source, current) {
				t.Fatal("bad snapshot modified existing tree")
			}
		})
	}
	// Stable LDAP identity preserves ID even if the address changes or swaps.
	imported[0].LocalPart, imported[1].LocalPart = "team", "company"
	oldRevision := source.Revision
	source, err = f.s.ApplyDirectoryImport(ctx, domain, source.Revision, imported)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := f.s.ListDirectoryGroups(ctx, domain)
	for _, g := range updated {
		if g.ID != byExternal[g.ExternalID].ID {
			t.Fatal("rename changed ID")
		}
	}
	if _, err = f.s.ApplyDirectoryImport(ctx, domain, oldRevision, imported); !errors.Is(err, ErrDirectoryConflict) {
		t.Fatal("stale snapshot overwrote newer sync", err)
	}
	// Obsolete LDAP groups are removed, manual groups remain.
	source, err = f.s.ApplyDirectoryImport(ctx, domain, source.Revision, imported[:1])
	if err != nil {
		t.Fatal(err)
	}
	updated, _ = f.s.ListDirectoryGroups(ctx, domain)
	if len(updated) != 2 {
		t.Fatal(updated)
	}
	source.Mode = "manual"
	source, err = f.s.SaveDirectorySource(ctx, domain, *source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.ApplyDirectoryImport(ctx, domain, source.Revision, imported); !errors.Is(err, ErrDirectoryConflict) {
		t.Fatal("import in manual mode", err)
	}
	manual.Name = "Editable again"
	if _, err = f.s.SaveDirectoryGroup(ctx, manual); err != nil {
		t.Fatal(err)
	}
}

func TestDirectorySourceConcurrentCAS(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	source := DefaultDirectorySource()
	source.Mode, source.BaseDN = "ldap", "ou=Groups,dc=example,dc=com"
	current, err := f.s.SaveDirectorySource(ctx, f.user.DomainID, source)
	if err != nil {
		t.Fatal(err)
	}
	changed := *current
	changed.Mode = "manual"
	start := make(chan struct{})
	out := make(chan error, 2)
	var wg sync.WaitGroup
	for _, operation := range []func() error{
		func() error {
			_, err := f.s.SaveDirectorySource(ctx, f.user.DomainID, changed)
			return err
		},
		func() error {
			_, err := f.s.ApplyDirectoryImport(ctx, f.user.DomainID, current.Revision, []DirectoryImportGroup{{ExternalID: "a", LocalPart: "team", Name: "Team"}})
			return err
		},
	} {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; out <- operation() }()
	}
	close(start)
	wg.Wait()
	close(out)
	success, conflict := 0, 0
	for err := range out {
		if err == nil {
			success++
		} else if errors.Is(err, ErrDirectoryConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	if err = f.s.RecordDirectoryImportError(ctx, f.user.DomainID, current.Revision, "old failure"); !errors.Is(err, ErrDirectoryConflict) {
		t.Fatal("old failure overwrote newer configuration", err)
	}
}
