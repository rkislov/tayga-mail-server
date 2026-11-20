// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"errors"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage/migrations"
	"strings"
	"testing"
)

func TestCalDAVScheduleTagConcurrentWritesAndReply(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	peer := meetingUser(t, f, "dav-reply")
	object := f.event(t, "dav-tag-concurrency", 19, 3)
	object.Data = strings.Replace(object.Data, "END:VEVENT", "ATTENDEE:mailto:"+peer.Email+"\r\nEND:VEVENT", 1)
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, object)
	if err != nil {
		t.Fatal(err)
	}
	source, err = f.s.GetCalendarObjectByID(ctx, source.ID)
	if err != nil || source.ScheduleTag == "" {
		t.Fatal(source, err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, peer.ID, false)
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if _, err = f.s.RespondCalendarInvite(ctx, peer.ID, invites[0].ID, "ACCEPTED", copy.ETag, true, ""); err != nil {
		t.Fatal(err)
	}
	after, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	if after.ETag == source.ETag || after.ScheduleTag != source.ScheduleTag {
		t.Fatal("reply did not preserve organizer tag")
	}
	other, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	start := make(chan struct{})
	results := make(chan error, 2)
	for i, driver := range []Driver{f.s, other} {
		go func(i int, driver Driver) {
			<-start
			edit := *source
			edit.Data = strings.Replace(source.Data, "SUMMARY:Booking", "SUMMARY:Concurrent "+string(rune('A'+i)), 1)
			edit.ExpectedETag = source.ETag
			edit.ExpectedScheduleTag = source.ScheduleTag
			_, err := driver.SaveCalendarClientObject(WithCalDAVScheduling(ctx, true), f.user.ID, &edit)
			results <- err
		}(i, driver)
	}
	close(start)
	success, conflict := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrCalendarConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	// A regular If-Match remains strict even when a Schedule-Tag is also supplied.
	current, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	edit := *current
	edit.ExpectedScheduleTag = current.ScheduleTag
	edit.ExpectedETag = source.ETag
	edit.StrictETag = true
	if _, err = f.s.SaveCalendarClientObject(WithCalDAVScheduling(ctx, true), f.user.ID, &edit); !errors.Is(err, ErrCalendarConflict) {
		t.Fatal("stale If-Match bypassed", err)
	}
}

func TestCalDAVSchedulingMigrationSeedsExistingTagsAndCascades(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, f.event(t, "dav-migration", 19, 0))
	if err != nil {
		t.Fatal(err)
	}
	file := "058_caldav_scheduling." + f.s.DialectName() + ".sql"
	raw, err := migrations.FS.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	down := strings.SplitN(string(raw), "-- +goose Down", 2)[1]
	if err = execScript(ctx, f.s.db, down); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.ExecContext(ctx, f.s.rebind(`DELETE FROM schema_migrations WHERE version=?`), strings.TrimSuffix(file, ".sql")); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.GetCalendarObjectByID(ctx, source.ID)
	if err != nil || saved.ScheduleTag != source.ETag {
		t.Fatal("legacy tag not seeded", saved, err)
	}
	if err = reopened.DeleteCalendarMeeting(WithoutCalendarMail(ctx), f.user.ID, saved); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.s.db.QueryRowContext(ctx, f.s.rebind(`SELECT COUNT(*) FROM calendar_schedule_tags WHERE object_id=?`), source.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("orphaned tag", count, err)
	}
}

func TestCalDAVAttendeePersonalAndResponseBatchRollback(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	peer := meetingUser(t, f, "dav-atomic")
	object := f.event(t, "dav-atomic", 19, 3)
	object.Data = strings.Replace(object.Data, "END:VEVENT", "ATTENDEE:mailto:"+peer.Email+"\r\nEND:VEVENT", 1)
	source, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, object)
	if err != nil {
		t.Fatal(err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, peer.ID, false)
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	data, err := calutil.WithAttendeeStatus(copy.Data, peer.Email, "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	data, err = calutil.WithOccurrenceAttendeeStatus(data, "2026-11-02T10:00:00Z", peer.Email, "DECLINED")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	// Fail the second (instance) REPLY after the master response was written.
	cleanup := failCalendarWrite(t, f.s, "calendar_scheduling_inbox", "INSERT", "NEW.data LIKE '%RECURRENCE-ID%'")
	edit := *copy
	edit.Data = data
	edit.ExpectedETag = copy.ETag
	edit.ExpectedScheduleTag = copy.ScheduleTag
	if _, err = f.s.SaveCalendarClientObject(WithCalDAVScheduling(ctx, true), peer.ID, &edit); err == nil {
		t.Fatal("injected failure accepted")
	}
	cleanup()
	after, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	own, _ := f.s.GetCalendarObjectByID(ctx, copy.ID)
	if after.Data != before.Data || after.ScheduleTag != before.ScheduleTag || own.Data != copy.Data || own.ScheduleTag != copy.ScheduleTag {
		t.Fatal("partial RSVP committed")
	}
	saved, err := f.s.SaveCalendarClientObject(WithCalDAVScheduling(ctx, true), peer.ID, &edit)
	if err != nil {
		t.Fatal(err)
	}
	master, _ := calutil.ParseICS(saved.Data)
	for _, a := range master.Attendees {
		if a.Email == peer.Email && a.PartStat != "ACCEPTED" {
			t.Fatal("master RSVP lost")
		}
	}
	instance, err := calutil.GetOccurrence(saved.Data, "2026-11-02T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range instance.Attendees {
		if a.Email == peer.Email && a.PartStat != "DECLINED" {
			t.Fatal("instance RSVP lost")
		}
	}
	messages, err := f.s.ListCalendarSchedulingInbox(ctx, f.user.ID)
	if err != nil || len(messages) != 2 {
		t.Fatal("expected master and instance REPLY", messages, err)
	}

}
