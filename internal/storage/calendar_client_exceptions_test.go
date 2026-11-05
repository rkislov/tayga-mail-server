// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"errors"
	"github.com/tayga/tms/internal/calutil"
	"strings"
	"testing"
)

func TestCalendarExceptionBatchAtomicCopiesResourcesAndOutbox(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	participant := meetingUser(t, f, "exception-participant")
	object := f.event(t, "exception-batch", 19, 3)
	object.Data = strings.Replace(object.Data, "END:VEVENT", "ATTENDEE:mailto:"+participant.Email+"\r\nEND:VEVENT", 1)
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, object)
	if err != nil {
		t.Fatal(err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, participant.ID, false)
	copy, err := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if err != nil {
		t.Fatal(err)
	}
	countJobs := func() int {
		var n int
		if err := f.s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM calendar_deliveries").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	beforeJobs := countJobs()
	summary := "One date only"
	edits := []CalendarOccurrenceEdit{{RecurrenceID: "2026-10-26T10:00:00Z", Change: calutil.OccurrenceChange{Summary: &summary, ResetResponses: true}}, {RecurrenceID: "2030-01-01T10:00:00Z", Change: calutil.OccurrenceChange{Cancel: true}}}
	if _, err = f.s.ChangeCalendarOccurrences(ctx, f.user.ID, source.ID, source.ETag, edits); err == nil {
		t.Fatal("invalid second date accepted")
	}
	for _, o := range []*CalendarObject{source, copy} {
		saved, _ := f.s.GetCalendarObjectByID(ctx, o.ID)
		if saved.ETag != o.ETag || saved.Data != o.Data {
			t.Fatal("partial batch committed")
		}
	}
	if _, err = f.s.UpsertCalendarObject(ctx, f.event(t, "original-still-booked", 26, 0)); err == nil {
		t.Fatal("rollback released resource")
	}
	if countJobs() != beforeJobs {
		t.Fatal("rolled-back jobs leaked")
	}
	edits[1].RecurrenceID = "2026-11-02T10:00:00Z"
	saved, err := f.s.ChangeCalendarOccurrences(ctx, f.user.ID, source.ID, source.ETag, edits)
	if err != nil {
		t.Fatal(err)
	}
	if countJobs() != beforeJobs+4 {
		t.Fatal("expected one delivery per exception and participant", beforeJobs, countJobs())
	}
	for _, id := range []string{saved.ID, copy.ID} {
		o, _ := f.s.GetCalendarObjectByID(ctx, id)
		first, err := calutil.GetOccurrence(o.Data, edits[0].RecurrenceID)
		if err != nil || first.Summary != summary {
			t.Fatal("lost edit", err)
		}
		second, err := calutil.GetOccurrence(o.Data, edits[1].RecurrenceID)
		if err != nil || second.Status != "CANCELLED" {
			t.Fatal("lost cancellation", err)
		}
	}
	if _, err = f.s.UpsertCalendarObject(ctx, f.event(t, "released-by-batch-cancellation", 33, 0)); err != nil {
		t.Fatal("cancelled date remains booked", err)
	}
	if _, err = f.s.ChangeCalendarOccurrences(ctx, f.user.ID, source.ID, source.ETag, edits[:1]); !errors.Is(err, ErrCalendarConflict) {
		t.Fatal("stale version accepted", err)
	}
	if _, err = f.s.ChangeCalendarOccurrences(ctx, participant.ID, copy.ID, saved.ETag, edits[:1]); err == nil {
		t.Fatal("recipient changed organizer occurrence")
	}
}
