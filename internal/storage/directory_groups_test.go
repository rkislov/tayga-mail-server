package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestDirectoryGroupsHierarchyAndDelivery(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	peer := meetingUser(t, f, "colleague")
	save := func(group *DirectoryGroup) *DirectoryGroup {
		t.Helper()
		group.DomainID = f.user.DomainID
		result, err := f.s.SaveDirectoryGroup(ctx, group)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	root := save(&DirectoryGroup{LocalPart: "Company", Name: "Company", MemberIDs: []string{f.user.ID}})
	dept := save(&DirectoryGroup{LocalPart: "engineering", Name: "Engineering", ParentID: root.ID, MemberIDs: []string{peer.ID, peer.ID, f.user.ID}})
	child := save(&DirectoryGroup{LocalPart: "support", Name: "Support", ParentID: dept.ID})
	if root.Email != "company@example.com" || len(dept.MemberIDs) != 2 || root.Revision != 1 {
		t.Fatal(root, dept)
	}
	got, err := f.s.ResolveRecipients(ctx, "COMPANY@example.com")
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	if _, err = f.s.ResolveRecipients(ctx, child.Email); err != ErrNoRecipients {
		t.Fatal("empty group", err)
	}
	if _, err = f.s.ResolveRecipient(ctx, root.Email); err != ErrNotFound {
		t.Fatal("group is not a login identity", err)
	}
	if err = f.s.UpdateUserEnabled(ctx, peer.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err = f.s.ResolveRecipients(ctx, root.Email)
	if err != nil || len(got) != 1 || got[0].ID != f.user.ID {
		t.Fatal(got, err)
	}
	loop := *root
	loop.ParentID = child.ID
	if _, err = f.s.SaveDirectoryGroup(ctx, &loop); !errors.Is(err, ErrDirectoryInvalid) {
		t.Fatal("cycle accepted", err)
	}
	stale := *dept
	dept.Description = "Updated"
	dept = save(dept)
	if _, err = f.s.SaveDirectoryGroup(ctx, &stale); err != ErrDirectoryConflict {
		t.Fatal("stale write", err)
	}
	if err = f.s.DeleteDirectoryGroup(ctx, root.DomainID, root.ID, root.Revision); err != ErrDirectoryChildren {
		t.Fatal("deleted parent", err)
	}
	if err = f.s.DeleteDirectoryGroup(ctx, child.DomainID, child.ID, child.Revision); err != nil {
		t.Fatal(err)
	}
	if err = f.s.DeleteDirectoryGroup(ctx, dept.DomainID, dept.ID, stale.Revision); err != ErrDirectoryConflict {
		t.Fatal("stale delete", err)
	}
	if err = f.s.DeleteDirectoryGroup(ctx, dept.DomainID, dept.ID, dept.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.GetUserByID(ctx, peer.ID); err != nil {
		t.Fatal("deleting department deleted mailbox", err)
	}
	groups, err := f.s.ListDirectoryGroups(ctx, root.DomainID)
	if err != nil || len(groups) != 1 || len(groups[0].MemberIDs) != 1 {
		t.Fatal(groups, err)
	}
}

func TestDirectoryGroupsIsolationAndAddresses(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	other, err := f.s.CreateDomain(ctx, f.user.TenantID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: other.ID, Name: "Other", LocalPart: "other"})
	if err != nil {
		t.Fatal(err)
	}
	g := &DirectoryGroup{DomainID: f.user.DomainID, Name: "Team", LocalPart: "team", MemberIDs: []string{f.user.ID}}
	group, err := f.s.SaveDirectoryGroup(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	bad := *group
	bad.ParentID = foreign.ID
	if _, err = f.s.SaveDirectoryGroup(ctx, &bad); !errors.Is(err, ErrDirectoryInvalid) {
		t.Fatal(err)
	}
	bad = *foreign
	bad.MemberIDs = []string{f.user.ID}
	if _, err = f.s.SaveDirectoryGroup(ctx, &bad); !errors.Is(err, ErrDirectoryInvalid) {
		t.Fatal(err)
	}
	bad = *group
	bad.MemberIDs = []string{f.room.UserID}
	if _, err = f.s.SaveDirectoryGroup(ctx, &bad); !errors.Is(err, ErrDirectoryInvalid) {
		t.Fatal(err)
	}
	bad = *group
	bad.DomainID = other.ID
	if _, err = f.s.SaveDirectoryGroup(ctx, &bad); err != ErrNotFound {
		t.Fatal(err)
	}
	groups, _ := f.s.ListDirectoryGroups(ctx, group.DomainID)
	if len(groups) != 1 || groups[0].Revision != 1 || len(groups[0].MemberIDs) != 1 {
		t.Fatal("failed validation did not roll back", groups)
	}
	if _, err = f.s.CreateUser(ctx, &User{DomainID: group.DomainID, TenantID: f.user.TenantID, LocalPart: "TEAM", Email: group.Email}); err != ErrAlreadyExists {
		t.Fatal("mailbox collision", err)
	}
	if _, err = f.s.CreateAlias(ctx, group.DomainID, f.user.ID, "TEAM"); err != ErrAlreadyExists {
		t.Fatal("alias collision", err)
	}
	for _, local := range []string{"OWNER", "room"} {
		g.LocalPart = local
		if _, err = f.s.SaveDirectoryGroup(ctx, g); err != ErrAlreadyExists {
			t.Fatal("group collision", local, err)
		}
	}
	if _, err = f.s.CreateAlias(ctx, group.DomainID, f.user.ID, "alias"); err != nil {
		t.Fatal(err)
	}
	g.LocalPart = "alias"
	if _, err = f.s.SaveDirectoryGroup(ctx, g); err != ErrAlreadyExists {
		t.Fatal(err)
	}
	for _, local := range []string{"a@b", "..a", "a.", "x\r\nbcc", "<x>"} {
		g.LocalPart = local
		if _, err = f.s.SaveDirectoryGroup(ctx, g); !errors.Is(err, ErrDirectoryInvalid) {
			t.Fatal(local, err)
		}
	}
	if err = f.s.DeleteDomain(ctx, f.user.TenantID, other.ID); err == nil {
		t.Fatal("deleted domain with departments")
	}
}

func TestDirectoryConcurrentHierarchyEdits(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	a, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: f.user.DomainID, LocalPart: "a", Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: f.user.DomainID, LocalPart: "b", Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	a.ParentID, b.ParentID = b.ID, a.ID
	start := make(chan struct{})
	out := make(chan error, 2)
	var wg sync.WaitGroup
	for _, g := range []*DirectoryGroup{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := f.s.SaveDirectoryGroup(ctx, g)
			out <- e
		}()
	}
	close(start)
	wg.Wait()
	close(out)
	success, rejected := 0, 0
	for err := range out {
		if err == nil {
			success++
		} else if errors.Is(err, ErrDirectoryInvalid) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("success=%d rejected=%d", success, rejected)
	}
}
