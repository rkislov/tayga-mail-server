package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage/migrations"
)

func TestOccurrenceProposalPersistenceAndConcurrentDecision(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	user := meetingUser(t, f, "participant")
	object := f.event(t, "occurrence-counter", 19, 3)
	object.Data = strings.Replace(object.Data, "END:VEVENT", "ATTENDEE:mailto:"+user.Email+"\r\nEND:VEVENT", 1)
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, object)
	if err != nil {
		t.Fatal(err)
	}
	invs, _ := f.s.ListCalendarInvitesForUser(ctx, user.ID, false)
	inv := invs[0]
	copy, _ := f.s.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	rid := "2026-10-26T10:00:00Z"
	if _, err = f.s.ProposeCalendarOccurrence(ctx, user.ID, inv.ID, rid, copy.ETag, start, end, "Move this date"); err != nil {
		t.Fatal(err)
	}
	p, err := f.s.GetCalendarOccurrenceProposal(ctx, inv.ID, rid)
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	loaded, err := other.GetCalendarOccurrenceProposal(ctx, inv.ID, rid)
	if err != nil || loaded.Revision != p.Revision || !loaded.ProposedStart.Equal(start) {
		t.Fatalf("lost proposal: %+v %v", loaded, err)
	}
	gate := make(chan struct{})
	results := make(chan struct {
		accept bool
		err    error
	}, 2)
	for i, driver := range []Driver{f.s, other} {
		go func(accept bool, driver Driver) {
			<-gate
			_, err := driver.DecideCalendarOccurrenceProposal(ctx, f.user.ID, inv.ID, source.ID, rid, p.Revision, source.ETag, accept)
			results <- struct {
				accept bool
				err    error
			}{accept, err}
		}(i == 0, driver)
	}
	close(gate)
	success := 0
	accepted := false
	for range 2 {
		result := <-results
		if result.err == nil {
			success++
			accepted = result.accept
		} else if !errors.Is(result.err, ErrCalendarConflict) {
			t.Fatal(result.err)
		}
	}
	if success != 1 {
		t.Fatalf("decisions committed: %d", success)
	}
	if _, err = other.GetCalendarOccurrenceProposal(ctx, inv.ID, rid); !errors.Is(err, ErrNotFound) {
		t.Fatal("proposal not removed", err)
	}
	for _, id := range []string{source.ID, copy.ID} {
		saved, _ := other.GetCalendarObjectByID(ctx, id)
		instance, err := calutil.GetOccurrence(saved.Data, rid)
		if err != nil {
			t.Fatal(err)
		}
		want := p.OriginalStart
		if accepted {
			want = start
		}
		if !instance.Start.Equal(want) {
			t.Fatalf("partial decision: %v want %v", instance.Start, want)
		}
	}
}

func TestOccurrenceProposalMigrationAndCascade(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	user := meetingUser(t, f, "participant")
	object := f.event(t, "migration-counter", 19, 3)
	object.Data = strings.Replace(object.Data, "END:VEVENT", "ATTENDEE:mailto:"+user.Email+"\r\nEND:VEVENT", 1)
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, object)
	if err != nil {
		t.Fatal(err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, user.ID, false)
	inv := invites[0]
	dialect := "sqlite"
	if f.s.dialect == DialectPostgres {
		dialect = "postgres"
	}
	raw, err := migrations.FS.ReadFile("062_calendar_occurrence_proposals." + dialect + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if err = execScript(ctx, f.s.DB(), parts[1]); err != nil {
		t.Fatal(err)
	}
	copy, err := f.s.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	if err != nil {
		t.Fatal("down lost attendee", err)
	}
	got, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	if got.Data != source.Data {
		t.Fatal("down changed calendar")
	}
	if err = execScript(ctx, f.s.DB(), strings.TrimPrefix(parts[0], "-- +goose Up")); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if _, err = f.s.ProposeCalendarOccurrence(ctx, user.ID, inv.ID, "2026-10-26T10:00:00Z", copy.ETag, start, start.Add(time.Hour), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.DB().ExecContext(ctx, f.s.rebind(`DELETE FROM calendar_invites WHERE id=?`), inv.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.GetCalendarOccurrenceProposal(ctx, inv.ID, "2026-10-26T10:00:00Z"); !errors.Is(err, ErrNotFound) {
		t.Fatal("missing cascade", err)
	}
}
