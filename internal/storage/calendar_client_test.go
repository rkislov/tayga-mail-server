package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
)

func calendarJobCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCalendarClientMeetingLifecycle(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := f.event(t, "client-meeting", 19, 3)
	o.Data = strings.Replace(o.Data, "END:VEVENT", "ATTENDEE:mailto:"+u.Email+"\r\nEND:VEVENT", 1)
	o, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, o)
	if err != nil {
		t.Fatal(err)
	}
	jobs := calendarJobCount(t, f.s)
	if jobs != 2 {
		t.Fatalf("jobs=%d", jobs)
	}
	invites, err := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	if err != nil || len(invites) != 1 {
		t.Fatal(invites, err)
	}
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if copy == nil || !strings.Contains(copy.Data, "RRULE:") {
		t.Fatal("no participant recurrence copy")
	}
	// Echoes and private reminders must not fan out or change accepted rooms.
	for _, alarm := range []bool{false, true} {
		edit := *o
		edit.ExpectedETag = o.ETag
		if alarm {
			edit.Data = strings.Replace(edit.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT30M\r\nDESCRIPTION:Private\r\nEND:VALARM\r\nEND:VEVENT", 1)
		}
		o, err = f.s.SaveCalendarClientObject(ctx, f.user.ID, &edit)
		if err != nil {
			t.Fatal(err)
		}
		if calendarJobCount(t, f.s) != jobs {
			t.Fatal("duplicate invitations")
		}
		p, _ := calutil.ParseICS(o.Data)
		if p.Attendees[0].PartStat != "ACCEPTED" {
			t.Fatal("room acceptance lost")
		}
	}
	before := meetingStorageSnapshot(t, f.s)
	stale := *o
	stale.ExpectedETag = "stale"
	if _, err = f.s.SaveCalendarClientObject(ctx, f.user.ID, &stale); err != ErrCalendarConflict {
		t.Fatal(err)
	}
	if meetingStorageSnapshot(t, f.s) != before {
		t.Fatal("stale write leaked")
	}
	// A client edit without a sequence bump must still send an update.
	edit := *o
	edit.ExpectedETag = o.ETag
	edit.Data = strings.Replace(edit.Data, "SUMMARY:Booking", "SUMMARY:Changed", 1)
	o, err = f.s.SaveCalendarClientObject(ctx, f.user.ID, &edit)
	if err != nil {
		t.Fatal(err)
	}
	if calendarJobCount(t, f.s) != jobs+2 {
		t.Fatal("update not queued")
	}
	// Copying the organizer object to another href must not make a second
	// scheduling source that could later cancel the first one.
	duplicate := *o
	duplicate.ID, duplicate.HrefName, duplicate.CreateOnly, duplicate.ExpectedETag = "", "duplicate.ics", true, ""
	if _, err = f.s.SaveCalendarClientObject(ctx, f.user.ID, &duplicate); err != ErrCalendarConflict {
		t.Fatal("duplicate scheduling source", err)
	}
	copy, _ = f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	p, _ := calutil.ParseICS(copy.Data)
	if p.Summary != "Changed" || p.Sequence != 1 || strings.Contains(copy.Data, "Private") {
		t.Fatal("bad participant update", copy.Data)
	}
	received := *copy
	if _, err = f.s.db.Exec(f.s.rebind(`INSERT INTO calendar_acl(calendar_id,grantee_user_id,rights) VALUES(?,?,'write')`), copy.CalendarID, f.user.ID); err != nil {
		t.Fatal(err)
	}
	roomInvites, _ := f.s.ListCalendarInvitesForUser(ctx, f.room.UserID, false)
	roomBefore, _ := f.s.GetCalendarObjectByID(ctx, roomInvites[0].CalendarObjectID)
	received.ExpectedETag = copy.ETag
	received.Data = strings.Replace(copy.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT5M\r\nDESCRIPTION:Mine\r\nEND:VALARM\r\nEND:VEVENT", 1)
	copy, err = f.s.SaveCalendarClientObject(ctx, u.ID, &received)
	if err != nil || calendarJobCount(t, f.s) != jobs+2 {
		t.Fatal("recipient edit sent invitations", err)
	}
	roomAfter, _ := f.s.GetCalendarObjectByID(ctx, roomBefore.ID)
	if roomAfter.ETag != roomBefore.ETag {
		t.Fatal("recipient edit changed room booking")
	}
	// Deleting a received copy must not cancel the source or room bookings.
	if err = f.s.DeleteCalendarMeeting(ctx, u.ID, copy); err != nil {
		t.Fatal(err)
	}
	if calendarJobCount(t, f.s) != jobs+2 {
		t.Fatal("participant cancelled meeting")
	}
	if err = f.s.DeleteCalendarMeeting(ctx, f.user.ID, o); err != nil {
		t.Fatal(err)
	}
	if calendarJobCount(t, f.s) <= jobs+2 {
		t.Fatal("cancellation not queued")
	}
}

func TestCalendarClientAtomicFailureAndAuthorization(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := meetingWithParticipant(t, f, u.Email)
	for _, fail := range []struct{ table, operation string }{
		{"calendar_objects", "UPDATE"}, {"calendar_invites", "UPDATE"}, {"calendar_deliveries", "INSERT"},
	} {
		before := meetingStorageSnapshot(t, f.s)
		remove := failCalendarWrite(t, f.s, fail.table, fail.operation, "")
		if _, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, changedMeeting(t, o, "Rollback")); err == nil {
			t.Fatal("failure not injected")
		}
		if meetingStorageSnapshot(t, f.s) != before {
			t.Fatal("partial client edit", fail.table)
		}
		remove()
	}
	if _, err := f.s.SaveCalendarClientObject(ctx, u.ID, changedMeeting(t, o, "Unauthorized")); err != ErrUnauthorized {
		t.Fatal(err)
	}
}

func TestCalendarClientSchedulingAgents(t *testing.T) {
	for _, agent := range []string{"CLIENT", "NONE"} {
		t.Run(agent, func(t *testing.T) {
			f := newBookingFixture(t)
			ctx := context.Background()
			o := f.event(t, "agent", 19, 0)
			o.Data = strings.ReplaceAll(o.Data, "ATTENDEE;", "ATTENDEE;SCHEDULE-AGENT="+agent+";")
			o, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, o)
			if err != nil {
				t.Fatal(err)
			}
			if calendarJobCount(t, f.s) != 0 {
				t.Fatal("client-managed mail sent")
			}
			if err = f.s.DeleteCalendarMeeting(ctx, f.user.ID, o); err != nil {
				t.Fatal(err)
			}
			if calendarJobCount(t, f.s) != 0 {
				t.Fatal("client-managed cancellation sent")
			}
		})
	}
}

func TestCalendarClientScheduleAgentTransition(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := meetingWithParticipant(t, f, u.Email)
	before := calendarJobCount(t, f.s)
	edit := *o
	edit.ExpectedETag = o.ETag
	edit.Data = strings.ReplaceAll(edit.Data, "ATTENDEE;", "ATTENDEE;SCHEDULE-AGENT=CLIENT;")
	o, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, &edit)
	if err != nil {
		t.Fatal(err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	if len(invites) != 1 {
		t.Fatal("missing cancelled invitation")
	}
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	p, _ := calutil.ParseICS(copy.Data)
	if p.Status != "CANCELLED" {
		t.Fatal("old server invitation not cancelled")
	}
	rooms, err := f.s.ListCalendarInvitesForUser(ctx, f.room.UserID, false)
	if err != nil || len(rooms) != 0 {
		t.Fatal("room not released", err)
	}
	after := calendarJobCount(t, f.s)
	if after <= before {
		t.Fatal("no cancellation mail")
	}
	if err = f.s.DeleteCalendarMeeting(ctx, f.user.ID, o); err != nil {
		t.Fatal(err)
	}
	if calendarJobCount(t, f.s) != after {
		t.Fatal("client-managed delete sent mail")
	}
}
