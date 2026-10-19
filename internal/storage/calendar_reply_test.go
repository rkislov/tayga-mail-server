package storage

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage/migrations"
)

func TestCalendarInviteMigrationDeduplicatesAndRollback(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "calendar.db")}}
	st, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if st != nil {
			st.Close()
		}
	}()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ten, err := st.CreateTenant(ctx, "org")
	check(err)
	dom, err := st.CreateDomain(ctx, ten.ID, "example.com")
	check(err)
	owner, err := st.CreateUser(ctx, &User{TenantID: ten.ID, DomainID: dom.ID, Email: "owner@example.com", LocalPart: "owner", Enabled: true})
	check(err)
	attendee, err := st.CreateUser(ctx, &User{TenantID: ten.ID, DomainID: dom.ID, Email: "reader@example.com", LocalPart: "reader", Enabled: true})
	check(err)
	start := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data, err := calutil.BuildRequestICS(calutil.Event{UID: "one", Summary: "Meeting", Start: &start, End: &end, Organizer: calutil.Attendee{Email: owner.Email}, Attendees: []calutil.Attendee{{Email: attendee.Email}}})
	check(err)
	var copies []*CalendarObject
	for _, user := range []*User{owner, attendee} {
		cal, err := st.EnsureCalendar(ctx, user.ID, "default", "Calendar")
		check(err)
		o, err := st.UpsertCalendarObject(ctx, &CalendarObject{CalendarID: cal.ID, UID: "one", HrefName: "one.ics", Data: data, Component: "VEVENT", DTStart: &start, DTEnd: &end})
		check(err)
		copies = append(copies, o)
	}
	inv, err := st.UpsertCalendarInvite(ctx, &CalendarInvite{OrganizerUserID: owner.ID, AttendeeUserID: attendee.ID, AttendeeEmail: attendee.Email, EventUID: "one", CalendarObjectID: copies[1].ID})
	check(err)
	raw, err := migrations.FS.ReadFile("049_calendar_invite_identity.sqlite.sql")
	check(err)
	check(execScript(ctx, st.DB(), strings.Split(string(raw), "-- +goose Down")[1]))
	_, err = st.DB().Exec(`DELETE FROM schema_migrations WHERE version='049_calendar_invite_identity.sqlite'`)
	check(err)
	// Old versions inserted another inbox row for every organizer update.
	_, err = st.DB().Exec(`INSERT INTO calendar_invites (id, organizer_user_id, attendee_email, attendee_user_id, event_uid, calendar_object_id, summary, partstat, created_at, updated_at)
		SELECT 'old-duplicate',organizer_user_id,attendee_email,attendee_user_id,event_uid,calendar_object_id,'old','NEEDS-ACTION','2020-01-01 00:00:00','2020-01-01 00:00:00' FROM calendar_invites WHERE id=?`, inv.ID)
	check(err)
	st.Close()
	st, err = Open(ctx, cfg)
	check(err)
	rows, err := st.ListCalendarInvitesForUser(ctx, attendee.ID, false)
	check(err)
	if len(rows) != 1 || rows[0].ID != inv.ID {
		t.Fatalf("dedup: %+v", rows)
	}
	// Simulate a write failure after calendar edits but before RSVP commit.
	_, err = st.DB().Exec(`CREATE TRIGGER fail_rsvp BEFORE UPDATE ON calendar_invites BEGIN SELECT RAISE(ABORT,'test failure'); END`)
	check(err)
	if _, err := st.ReplyCalendarInvite(ctx, attendee.ID, inv.ID, "ACCEPTED", nil, nil, copies[1].ETag); err == nil {
		t.Fatal("expected injected error")
	}
	for _, copy := range copies {
		o, err := st.GetCalendarObjectByID(ctx, copy.ID)
		check(err)
		if o.Data != copy.Data || o.ETag != copy.ETag {
			t.Fatal("partial RSVP committed")
		}
	}
	_, err = st.DB().Exec(`DROP TRIGGER fail_rsvp`)
	check(err)
	_, err = st.ReplyCalendarInvite(ctx, attendee.ID, inv.ID, "ACCEPTED", nil, nil, copies[1].ETag)
	check(err)
	// Conditional writes prevent lost updates from a stale event editor.
	stale := *copies[0]
	stale.ExpectedETag = copies[0].ETag
	if _, err := st.UpsertCalendarObject(ctx, &stale); !errors.Is(err, ErrCalendarConflict) {
		t.Fatalf("stale write: %v", err)
	}
	stale.ExpectedETag = ""
	stale.CreateOnly = true
	if _, err := st.UpsertCalendarObject(ctx, &stale); !errors.Is(err, ErrCalendarConflict) {
		t.Fatalf("duplicate creation: %v", err)
	}
	// The down migration leaves calendar objects and the surviving RSVP intact.
	check(execScript(ctx, st.DB(), strings.Split(string(raw), "-- +goose Down")[1]))
	got, err := st.GetCalendarInvite(ctx, inv.ID)
	check(err)
	if got.PartStat != "ACCEPTED" {
		t.Fatal(got.PartStat)
	}
}
