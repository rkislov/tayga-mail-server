package storage

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/travel"
)

func ticketFixture() *travel.Ticket {
	return travel.Parse("Flight:SU1234\nFrom:Moscow\nTo:Delhi\nDeparture:2026-10-25 23:30 UTC+3\nArrival:2026-10-26 07:00 +05:30\nPassenger:TEST PERSON\nBooking:TEST123\nSeat:12A")
}
func travelMessage(t *testing.T, f bookingFixture, box string) *Message {
	t.Helper()
	ctx := context.Background()
	mb, err := f.s.EnsureMailbox(ctx, f.user.ID, box, "/unused")
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.s.InsertMessage(ctx, &Message{MailboxID: mb.ID, FilePath: "unused", Size: 100})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func candidate(t *testing.T, f bookingFixture, msg *Message, trip *travel.Ticket) TravelCandidate {
	t.Helper()
	items, err := f.s.RecordTravelCandidates(context.Background(), f.user.ID, msg.ID, []*travel.Ticket{trip})
	if err != nil || len(items) == 0 {
		t.Fatal(items, err)
	}
	for _, item := range items {
		if item.Ticket.Fingerprint() == trip.Fingerprint() {
			return item
		}
	}
	t.Fatal("missing candidate")
	return TravelCandidate{}
}
func TestTravelAtomicReplayAndPrivateCalendar(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	trip := ticketFixture()
	msg := travelMessage(t, f, "INBOX")
	c := candidate(t, f, msg, trip)
	start, end, _ := trip.Times()
	undo := failCalendarWrite(t, f.s, "travel_events", "INSERT", "")
	if _, err := f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start, end, "", false); err == nil {
		t.Fatal("expected rollback")
	}
	undo()
	if _, err := f.s.GetCalendarByName(ctx, f.user.ID, "personal"); err != ErrNotFound {
		t.Fatal("partial calendar", err)
	}
	o, err := f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start, end, "", false)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := f.s.GetCalendarByID(ctx, f.user.ID, o.CalendarID)
	if err != nil || cal.Name != "personal" {
		t.Fatal(cal, err)
	}
	if strings.Contains(o.Data, "METHOD:") || strings.Contains(o.Data, "ATTENDEE:") {
		t.Fatal("trip became invitation")
	}
	// Repeated save is idempotent and does not bump ETag/ctag.
	again, err := f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start.Add(time.Hour), end.Add(time.Hour), "", false)
	if err != nil || again.ID != o.ID || again.ETag != o.ETag {
		t.Fatal("repeated save rewrote event", err)
	}
	after, _ := f.s.GetCalendarByID(ctx, f.user.ID, o.CalendarID)
	if after.CTag != cal.CTag {
		t.Fatal("replay changed ctag")
	}
	if err = f.s.DeleteMessage(ctx, msg.ID); err != nil {
		t.Fatal(err)
	}
	msg = travelMessage(t, f, "INBOX")
	duplicate := *trip
	duplicate.Source = "ticket.pdf"
	againCandidate := candidate(t, f, msg, &duplicate)
	if againCandidate.ID != c.ID || againCandidate.State != "saved" {
		t.Fatal("reimport lost receipt", againCandidate)
	}
	if _, err = f.s.GetTravelCandidate(ctx, f.room.UserID, c.ID); err != ErrNotFound {
		t.Fatal("candidate leaked", err)
	}
	if _, err = f.s.SaveTravelEvent(ctx, f.room.UserID, c.ID, start, end, "", false); err != ErrNotFound {
		t.Fatal("cross-user write", err)
	}
	if err = f.s.DeleteCalendarObject(ctx, o.CalendarID, o.HrefName); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start, end, "", false); err != ErrCalendarConflict {
		t.Fatal("deleted trip resurrected", err)
	}
}
func TestTravelRevisionPreservesNotesAndRequiresReview(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	_ = f.s.SetTravelMode(ctx, f.user.ID, "auto")
	trip := ticketFixture()
	c := candidate(t, f, travelMessage(t, f, "INBOX"), trip)
	start, end, _ := trip.Times()
	o, err := f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start, end, "", true)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := calutil.ParseICS(o.Data)
	parsed.Description += "\nMy private note"
	o.Data, _ = calutil.BuildRequestICS(parsed.Event)
	o.Data = strings.Replace(o.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT45M\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\nEND:VEVENT", 1)
	o.CreateOnly = false
	o.ExpectedETag = o.ETag
	o, err = f.s.UpsertCalendarObject(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	changed := *trip
	changed.Arrival = "2026-10-26T08:00"
	changed.Seat = "14A"
	c2 := candidate(t, f, travelMessage(t, f, "INBOX"), &changed)
	a, b, _ := changed.Times()
	if c2.EventID != o.ID {
		t.Fatal("revision lost event link")
	}
	if _, err = f.s.SaveTravelEvent(ctx, f.user.ID, c2.ID, a, b, "", true); err != ErrCalendarConflict {
		t.Fatal("auto overwrite", err)
	}
	if _, err = f.s.SaveTravelEvent(ctx, f.user.ID, c2.ID, a, b, "stale", false); err != ErrCalendarConflict {
		t.Fatal("stale update", err)
	}
	updated, err := f.s.SaveTravelEvent(ctx, f.user.ID, c2.ID, a, b, o.ETag, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.Data, "My private note") || !strings.Contains(updated.Data, "TRIGGER:-PT45M") || !strings.Contains(updated.Data, "Seat: 14A") || strings.Contains(updated.Data, "Seat: 12A") {
		t.Fatal("notes/reminder/seat lost", updated.Data)
	}
	replay, err := f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start, end, "", false)
	if err != nil || replay.ETag != updated.ETag {
		t.Fatal("old mail replay reverted schedule", err)
	}
	changed.Cancelled = true
	changed.ReviewRequired = true
	c3 := candidate(t, f, travelMessage(t, f, "INBOX"), &changed)
	cancelled, err := f.s.SaveTravelEvent(ctx, f.user.ID, c3.ID, a, b, updated.ETag, false)
	if err != nil || !strings.Contains(cancelled.Data, "STATUS:CANCELLED") {
		t.Fatal("cancel", err)
	}
}
func TestTravelQueueModesLeaseRestartAndPagination(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	travelMessage(t, f, "INBOX")
	if _, err := f.s.ClaimTravelJob(ctx); err != ErrNotFound {
		t.Fatal("off still queued", err)
	}
	if err := f.s.SetTravelMode(ctx, f.user.ID, "suggest"); err != nil {
		t.Fatal(err)
	}
	for _, box := range []string{"Junk", "Trash", "Drafts", "Sent"} {
		travelMessage(t, f, box)
	}
	if _, err := f.s.ClaimTravelJob(ctx); err != ErrNotFound {
		t.Fatal("excluded mailbox queued", err)
	}
	msg := travelMessage(t, f, "INBOX")
	j, err := f.s.ClaimTravelJob(ctx)
	if err != nil || j.MessageID != msg.ID {
		t.Fatal(j, err)
	}
	if _, err = f.s.ClaimTravelJob(ctx); err != ErrNotFound {
		t.Fatal("lease stolen", err)
	}
	// A new storage handle simulates a process restart. Expire the abandoned lease.
	_, err = f.s.db.Exec(f.s.rebind(`UPDATE travel_jobs SET lease_until=0 WHERE message_id=?`), msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	driver, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	reopened := driver.(*Store)
	next, err := reopened.ClaimTravelJob(ctx)
	if err != nil || next.Token == j.Token {
		t.Fatal(next, err)
	}
	if err = f.s.FinishTravelJob(ctx, j, "", false); err != nil {
		t.Fatal(err)
	}
	var state string
	_ = f.s.db.QueryRow(f.s.rebind(`SELECT state FROM travel_jobs WHERE message_id=?`), msg.ID).Scan(&state)
	if state != "pending" {
		t.Fatal("stale worker completed job")
	}
	if err = reopened.FinishTravelJob(ctx, next, "", false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		trip := ticketFixture()
		trip.Number = NewID()
		candidate(t, f, msg, trip)
	}
	items, total, err := f.s.ListTravelCandidates(ctx, f.user.ID, "", 10, 10)
	if err != nil || total != 12 || len(items) != 2 {
		t.Fatal(len(items), total, err)
	}
}
func TestTravelConcurrentAutomaticAndIgnore(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	_ = f.s.SetTravelMode(ctx, f.user.ID, "auto")
	trip := ticketFixture()
	c := candidate(t, f, travelMessage(t, f, "INBOX"), trip)
	start, end, _ := trip.Times()
	var wg sync.WaitGroup
	errch := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.s.SaveTravelEvent(ctx, f.user.ID, c.ID, start, end, "", true)
			errch <- err
		}()
	}
	wg.Wait()
	close(errch)
	for err := range errch {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n int
	_ = f.s.db.QueryRow(`SELECT COUNT(*) FROM travel_events`).Scan(&n)
	if n != 1 {
		t.Fatal("duplicate receipt")
	}
	_ = f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_objects`).Scan(&n)
	if n != 1 {
		t.Fatal("duplicate event")
	}
	trip.Number = "SU1235"
	c2 := candidate(t, f, travelMessage(t, f, "INBOX"), trip)
	if err := f.s.IgnoreTravelCandidate(ctx, f.user.ID, c2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.SaveTravelEvent(ctx, f.user.ID, c2.ID, start, end, "", true); err != ErrCalendarConflict {
		t.Fatal("ignored trip saved", err)
	}
}
