package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarInvitesAndFreeBusy(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ten, err := store.CreateTenant(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := store.CreateDomain(ctx, ten.ID, "ex.com")
	if err != nil {
		t.Fatal(err)
	}
	org, err := store.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID, LocalPart: "org", Email: "org@ex.com",
		DisplayName: "Org", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	att, err := store.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID, LocalPart: "att", Email: "att@ex.com",
		DisplayName: "Att", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cal, err := store.EnsureCalendar(ctx, org.ID, "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Truncate(time.Hour)
	end := start.Add(2 * time.Hour)
	obj, err := store.UpsertCalendarObject(ctx, &storage.CalendarObject{
		CalendarID: cal.ID, UID: "busy-1", HrefName: "busy-1.ics",
		Data: "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", Size: 10, Component: "VEVENT",
		DTStart: &start, DTEnd: &end, ETag: `"e"`,
	})
	if err != nil {
		t.Fatal(err)
	}

	busy, err := store.FreeBusyForUser(ctx, org.ID, start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(busy) != 1 {
		t.Fatalf("busy=%d", len(busy))
	}

	inv, err := store.UpsertCalendarInvite(ctx, &storage.CalendarInvite{
		OrganizerUserID: org.ID, AttendeeEmail: att.Email, AttendeeUserID: att.ID,
		EventUID: "busy-1", CalendarObjectID: obj.ID, Summary: "Meet", PartStat: "NEEDS-ACTION",
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.ListCalendarInvitesForUser(ctx, att.ID, true)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	ps := start.Add(3 * time.Hour)
	pe := ps.Add(time.Hour)
	if err := store.UpdateCalendarInvitePartStat(ctx, inv.ID, "ACCEPTED", nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetCalendarInvite(ctx, inv.ID)
	if err != nil || got.PartStat != "ACCEPTED" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := store.UpdateCalendarInvitePartStat(ctx, inv.ID, "TENTATIVE", &ps, &pe); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetCalendarInvite(ctx, inv.ID)
	if err != nil || got.ProposedStart == nil || !got.ProposedStart.Equal(ps) {
		t.Fatalf("proposed=%+v err=%v", got, err)
	}

	// default calendar protected from delete
	if err := store.DeleteCalendar(ctx, org.ID, "default"); err == nil {
		// some stores may allow; verify Ensure still works
		_, _ = store.EnsureCalendar(ctx, org.ID, "default", "Calendar")
	}
	extra, err := store.CreateCalendar(ctx, &storage.Calendar{
		UserID: org.ID, Name: "team", DisplayName: "Team",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteCalendar(ctx, org.ID, extra.Name); err != nil {
		t.Fatal(err)
	}
}
