package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/config"
)

type bookingFixture struct {
	s    *Store
	cfg  config.StorageConfig
	user *User
	cal  *Calendar
	room *CalendarResource
}

func newBookingFixture(t *testing.T) bookingFixture {
	t.Helper()
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "bookings.db")}}
	if pg := bookingPostgresConfig(t); pg != nil {
		cfg = *pg
	}
	driver, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := driver.(*Store)
	t.Cleanup(func() { s.Close() })
	tenant, err := s.CreateTenant(ctx, "Bookings")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := s.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, Email: "owner@example.com", LocalPart: "owner", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cal, err := s.EnsureCalendar(ctx, u.ID, "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	room, err := s.CreateCalendarResource(ctx, &CalendarResource{DomainID: domain.ID, LocalPart: "room", DisplayName: "Room 201", AutoAccept: true})
	if err != nil {
		t.Fatal(err)
	}
	return bookingFixture{s, cfg, u, cal, room}
}
func (f bookingFixture) event(t *testing.T, uid string, day, count int) *CalendarObject {
	t.Helper()
	start := time.Date(2026, 10, day, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data, err := calutil.BuildRequestICS(calutil.Event{UID: uid, Summary: "Booking", Start: &start, End: &end, Organizer: calutil.Attendee{Email: f.user.Email}, Attendees: []calutil.Attendee{{Email: f.room.Email, CUType: "RESOURCE"}, {Email: "external@elsewhere.example"}}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		data, err = calutil.WithRecurrence(data, calutil.RecurrenceRule{Frequency: "WEEKLY", Interval: 1, Count: count}, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	return &CalendarObject{CalendarID: f.cal.ID, UID: uid, HrefName: uid + ".ics", Component: "VEVENT", Data: data, DTStart: &start, DTEnd: &end, CreateOnly: true}
}
func requireBookingError(t *testing.T, err error) {
	t.Helper()
	var b *CalendarBookingError
	if !errors.As(err, &b) {
		t.Fatalf("wanted booking error, got %v", err)
	}
}

func TestBookingWholeSeriesAtomicConflictAndRelease(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	// Only the SECOND recurrence conflicts.
	blocker := f.event(t, "block", 26, 0)
	if _, err := f.s.UpsertCalendarObject(ctx, blocker); err != nil {
		t.Fatal(err)
	}
	candidate := f.event(t, "series", 19, 3)
	p, _ := calutil.ParseICS(candidate.Data)
	mails, _ := CalendarEventMail(f.user.Email, "", p.Event, false)
	_, err := f.s.SaveCalendarObjectWithMail(ctx, candidate, mails)
	requireBookingError(t, err)
	if _, err = f.s.GetCalendarObject(ctx, f.cal.ID, "series.ics"); err != ErrNotFound {
		t.Fatal("partial source saved")
	}
	var jobs int
	f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&jobs)
	if jobs != 0 {
		t.Fatal("conflict queued invites")
	}
	if err = f.s.DeleteCalendarObject(ctx, f.cal.ID, "block.ics"); err != nil {
		t.Fatal(err)
	}
	candidate = f.event(t, "series", 19, 3)
	source, err := f.s.SaveCalendarObjectWithMail(ctx, candidate, mails)
	if err != nil {
		t.Fatal(err)
	}
	roomCal, _ := f.s.GetCalendarByName(ctx, f.room.UserID, "default")
	copies, _ := f.s.ListCalendarObjects(ctx, roomCal.ID)
	if len(copies) != 1 {
		t.Fatalf("copies %d", len(copies))
	}
	items, err := calutil.BookingOccurrences(copies[0].Data)
	if err != nil || len(items) != 3 {
		t.Fatalf("%v %v", items, err)
	}
	parsed, _ := calutil.ParseICS(source.Data)
	if parsed.Attendees[0].PartStat != "ACCEPTED" {
		t.Fatal("not accepted")
	}
	f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&jobs)
	if jobs != 1 {
		t.Fatalf("jobs %d", jobs)
	}
	// Cancelling one instance releases precisely its interval.
	source, err = f.s.ChangeCalendarOccurrence(ctx, f.user.ID, source.ID, source.ETag, "2026-10-26T10:00:00Z", calutil.OccurrenceChange{Cancel: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.UpsertCalendarObject(ctx, f.event(t, "replacement", 26, 0)); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.UpsertCalendarObject(ctx, f.event(t, "still-busy", 19, 0))
	requireBookingError(t, err)
	// A cancelled instance survives a rule change and stays unoccupied.
	source, err = f.s.ChangeCalendarRecurrence(ctx, f.user.ID, source.ID, source.ETag, calutil.RecurrenceRule{Frequency: "WEEKLY", Interval: 1, Count: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.DeleteCalendarObjectWithMail(ctx, source, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.UpsertCalendarObject(ctx, f.event(t, "after-delete", 19, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestBookingCompetingConnections(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	second, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	ready := make(chan struct{})
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		event := f.event(t, fmt.Sprintf("race-%d", i), 19, 4)
		var s Driver = f.s
		if i%2 != 0 {
			s = second
		}
		wg.Add(1)
		go func() { defer wg.Done(); <-ready; _, e := s.UpsertCalendarObject(ctx, event); results <- e }()
	}
	close(ready)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			requireBookingError(t, err)
		}
	}
	if successes != 1 {
		t.Fatalf("accepted %d overlapping bookings", successes)
	}
	objects, _ := f.s.ListCalendarObjects(ctx, f.cal.ID)
	if len(objects) != 1 {
		t.Fatalf("partial sources: %d", len(objects))
	}
}

func TestBookingRollbackRoomAndQueue(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	room2, err := f.s.CreateCalendarResource(ctx, &CalendarResource{DomainID: f.user.DomainID, LocalPart: "z-room", DisplayName: "Second room", AutoAccept: true})
	if err != nil {
		t.Fatal(err)
	}
	event := f.event(t, "rollback", 19, 3)
	p, _ := calutil.ParseICS(event.Data)
	p.Attendees = append(p.Attendees, calutil.Attendee{Email: room2.Email, CUType: "RESOURCE"})
	event.Data, _ = calutil.BuildRequestICS(p.Event)
	mails, _ := CalendarEventMail(f.user.Email, "", p.Event, false)
	// Failure after room writes but before commit must undo EVERY copy.
	removeFailure := failCalendarWrite(t, f.s, "calendar_deliveries", "INSERT", "")
	if _, err = f.s.SaveCalendarObjectWithMail(ctx, event, mails); err == nil {
		t.Fatal("injection failed")
	}
	var count int
	f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_objects`).Scan(&count)
	if count != 0 {
		t.Fatalf("partial objects %d", count)
	}
	f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_invites`).Scan(&count)
	if count != 0 {
		t.Fatal("partial invites")
	}
	removeFailure()
	// Unknown availability is not interpreted as an empty room.
	roomCal, _ := f.s.GetCalendarByName(ctx, f.room.UserID, "default")
	_, err = f.s.db.Exec(f.s.rebind(`INSERT INTO calendar_objects(id,calendar_id,uid,href_name,etag,data,size,component,updated_at) VALUES ('broken',?,'broken','broken.ics','x','not an ics',10,'VEVENT',?)`), roomCal.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.UpsertCalendarObject(ctx, f.event(t, "unsafe", 19, 2))
	requireBookingError(t, err)
}

func TestBookingRuleConflictAndDirectWrite(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	source, err := f.s.UpsertCalendarObject(ctx, f.event(t, "series", 19, 2))
	if err != nil {
		t.Fatal(err)
	}
	block := f.event(t, "later", 33, 0) // Nov 2, after existing series.
	if _, err = f.s.UpsertCalendarObject(ctx, block); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.ChangeCalendarRecurrence(ctx, f.user.ID, source.ID, source.ETag, calutil.RecurrenceRule{Frequency: "WEEKLY", Interval: 1, Count: 3})
	requireBookingError(t, err)
	saved, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	if saved.ETag != source.ETag {
		t.Fatal("partial rule")
	}
	roomCal, _ := f.s.GetCalendarByName(ctx, f.room.UserID, "default")
	direct := f.event(t, "direct", 19, 0)
	direct.CalendarID = roomCal.ID
	_, err = f.s.UpsertCalendarObject(ctx, direct)
	requireBookingError(t, err)
	// Turning a booked series into an unbounded personal series releases rooms.
	p, _ := calutil.ParseICS(source.Data)
	p.Attendees = nil
	personal, _ := calutil.BuildSimpleVEVENT(p.Event)
	personal = strings.Replace(personal, "COUNT=2", "UNTIL=20301019T100000Z", 1)
	saved.Data = personal
	saved.ExpectedETag = saved.ETag
	if _, err = f.s.UpsertCalendarObject(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.UpsertCalendarObject(ctx, f.event(t, "freed", 19, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestBookingIdentityAdjacentAndManualResources(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	first, err := f.s.UpsertCalendarObject(ctx, f.event(t, "same-uid", 19, 0))
	if err != nil {
		t.Fatal(err)
	}
	secondUser, err := f.s.CreateUser(ctx, &User{TenantID: f.user.TenantID, DomainID: f.user.DomainID, Email: "second@example.com", LocalPart: "second", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	secondCal, err := f.s.EnsureCalendar(ctx, secondUser.ID, "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	other := f
	other.user = secondUser
	other.cal = secondCal
	// Same UID with another organizer is not an exclusion from the busy check.
	_, err = f.s.UpsertCalendarObject(ctx, other.event(t, "same-uid", 19, 0))
	requireBookingError(t, err)
	if _, err = f.s.UpsertCalendarObject(ctx, other.event(t, "same-uid", 20, 0)); err != nil {
		t.Fatal(err)
	}
	cals, err := f.s.ListCalendars(ctx, f.room.UserID)
	if err != nil || len(cals) != 2 {
		t.Fatalf("UID isolation: %v %v", cals, err)
	}
	adjacent := f.event(t, "adjacent", 19, 0)
	p, _ := calutil.ParseICS(adjacent.Data)
	a := p.Start.Add(time.Hour)
	b := p.End.Add(time.Hour)
	p.Start, p.End = &a, &b
	adjacent.DTStart, adjacent.DTEnd = &a, &b
	adjacent.Data, _ = calutil.BuildRequestICS(p.Event)
	if _, err = f.s.UpsertCalendarObject(ctx, adjacent); err != nil {
		t.Fatal(err)
	}
	// Cancelling/removing one organizer's calendar doesn't free the other copy.
	if err = f.s.DeleteCalendar(ctx, f.user.ID, "default"); err != nil {
		t.Fatal(err)
	}
	busy, err := f.s.FreeBusyForUser(ctx, f.room.UserID, *first.DTStart, first.DTStart.Add(48*time.Hour))
	if err != nil || len(busy) != 1 || busy[0].Start.Day() != 20 {
		t.Fatalf("release isolation %v %v", busy, err)
	}
	f.room.AutoAccept = false
	if err = f.s.UpdateCalendarResource(ctx, f.room); err != nil {
		t.Fatal(err)
	}
	manual, err := f.s.UpsertCalendarObject(ctx, other.event(t, "manual", 21, 2))
	if err != nil {
		t.Fatal(err)
	}
	p, _ = calutil.ParseICS(manual.Data)
	if p.Attendees[0].PartStat != "NEEDS-ACTION" {
		t.Fatal("manual resource silently accepted")
	}
	_, err = f.s.UpsertCalendarObject(ctx, other.event(t, "manual-blocked", 21, 0))
	requireBookingError(t, err)
}

func TestBookingDisabledRemovalAndIdentityGuard(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	source, err := f.s.UpsertCalendarObject(ctx, f.event(t, "protected", 19, 2))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := calutil.ParseICS(source.Data)
	p.Organizer.Email = "elsewhere@example.net"
	replacement := *source
	replacement.Data, _ = calutil.BuildRequestICS(p.Event)
	replacement.ExpectedETag = source.ETag
	_, err = f.s.UpsertCalendarObject(ctx, &replacement)
	requireBookingError(t, err)
	stored, _ := f.s.GetCalendarObjectByID(ctx, source.ID)
	if stored.ETag != source.ETag {
		t.Fatal("changed organizer stranded a booking")
	}
	f.room.Enabled = false
	if err = f.s.UpdateCalendarResource(ctx, f.room); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.UpsertCalendarObject(ctx, f.event(t, "disabled", 20, 2))
	requireBookingError(t, err)
	if err = f.s.DeleteCalendarObject(ctx, f.cal.ID, source.HrefName); err != nil {
		t.Fatal(err)
	}
	roomCal, _ := f.s.GetCalendarByName(ctx, f.room.UserID, "default")
	copies, _ := f.s.ListCalendarObjects(ctx, roomCal.ID)
	if len(copies) != 0 {
		t.Fatal("disabled booking was not released")
	}
}
