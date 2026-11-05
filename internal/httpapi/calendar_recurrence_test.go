package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarRecurrenceCreateAndTransaction(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	cal, _ := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	path := "/api/v1/calendar/calendars/" + cal.ID + "/events"
	req := `{"summary":"Weekly","start":"2026-10-19T08:00:00Z","end":"2026-10-19T09:00:00Z","timezone":"Europe/Berlin","recurrence":{"frequency":"WEEKLY","interval":1,"weekdays":["MO"],"count":4},"attendees":[{"email":"reader@example.com"},{"email":"editor@example.com"}]}`
	f.call(0, "POST", path, strings.Replace(req, `"count":4`, `"count":-1`, 1), 400)
	f.call(0, "POST", path, req, 200)
	objects, _ := f.s.store.ListCalendarObjects(ctx, cal.ID)
	source := objects[0]
	copies := []*storage.CalendarObject{source}
	for _, userID := range f.users[1:] {
		invs, _ := f.s.store.ListCalendarInvitesForUser(ctx, userID, false)
		copy, _ := f.s.store.GetCalendarObjectByID(ctx, invs[0].CalendarObjectID)
		copy.Data = strings.Replace(copy.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Private reminder\r\nTRIGGER:-PT5M\r\nEND:VALARM\r\nEND:VEVENT", 1)
		copy, _ = f.s.store.UpsertCalendarObject(ctx, copy)
		copies = append(copies, copy)
		if err := f.s.store.UpdateCalendarInvitePartStat(ctx, invs[0].ID, "ACCEPTED", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range copies {
		if !strings.Contains(o.Data, "RRULE:FREQ=WEEKLY") || !strings.Contains(o.Data, "TZID=Europe/Berlin") {
			t.Fatalf("missing series: %s", o.Data)
		}
	}
	get := f.call(0, "GET", "/api/v1/calendar/events/"+source.ID, "", 200)
	if !strings.Contains(get.Body.String(), `"editable":true`) || !strings.Contains(get.Body.String(), `"count":4`) {
		t.Fatal(get.Body.String())
	}
	updatePath := "/api/v1/calendar/events/" + source.ID + "/recurrence"
	body := func(etag string, count int) string {
		b, _ := json.Marshal(map[string]any{"etag": etag, "rule": calutil.RecurrenceRule{Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"MO"}, Count: count}})
		return string(b)
	}
	f.call(1, "PUT", updatePath, body(source.ETag, 6), 403)
	f.call(1, "PUT", "/api/v1/calendar/events/"+copies[1].ID+"/recurrence", body(copies[1].ETag, 6), 403)
	f.call(0, "PUT", updatePath, body("stale", 6), 409)
	f.call(0, "PUT", updatePath, body("", 6), 400)
	db := f.s.store.(*storage.Store).DB()
	var oldJobs int
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&oldJobs)
	for _, target := range []string{"copy", "queue"} {
		statement := fmt.Sprintf(`CREATE TRIGGER fail_rule BEFORE UPDATE ON calendar_objects WHEN OLD.id='%s' BEGIN SELECT RAISE(ABORT,'injected'); END`, copies[1].ID)
		if target == "queue" {
			statement = `CREATE TRIGGER fail_rule BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		f.call(0, "PUT", updatePath, body(source.ETag, 6), 500)
		for _, o := range copies {
			got, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
			if got.Data != o.Data {
				t.Fatal("partial update")
			}
		}
		var jobs int
		db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&jobs)
		if jobs != oldJobs {
			t.Fatal("partial mail queue")
		}
		invs, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
		if invs[0].PartStat != "ACCEPTED" {
			t.Fatal("partial RSVP reset")
		}
		if _, err := db.Exec(`DROP TRIGGER fail_rule`); err != nil {
			t.Fatal(err)
		}
	}
	f.call(0, "PUT", updatePath, body(source.ETag, 6), 200)
	f.call(0, "PUT", updatePath, body(source.ETag, 6), 409)
	for i, o := range copies {
		got, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
		info := calutil.ReadRecurrence(got.Data)
		p, _ := calutil.ParseICS(got.Data)
		if info.Rule.Count != 6 || p.Sequence != 1 || got.ETag == o.ETag {
			t.Fatalf("%+v %+v", info, p)
		}
		if i > 0 && !strings.Contains(got.Data, "Private reminder") {
			t.Fatal("lost private reminder")
		}
		instances, err := calutil.ExpandOccurrences(got.Data, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
		if err != nil || len(instances) != 6 {
			t.Fatalf("%d %v", len(instances), err)
		}
	}
	invs, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if invs[0].PartStat != "NEEDS-ACTION" {
		t.Fatal("RSVP not reset")
	}
	var jobs int
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&jobs)
	if jobs != oldJobs+2 {
		t.Fatalf("jobs %d", jobs)
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mb, _ := f.s.store.GetMailbox(ctx, f.users[1], "INBOX")
	messages, _ := f.s.store.ListMessages(ctx, mb.ID)
	if len(messages) != 2 {
		t.Fatalf("mail count %d", len(messages))
	}
	updated := false
	for _, m := range messages {
		raw, _ := f.s.ms.Read(m.FilePath)
		if strings.Contains(string(raw), "COUNT=6") {
			updated = true
		}
	}
	if !updated {
		t.Fatal("updated rule absent from invitation")
	}
}

func TestCalendarRecurrenceProtectsPrivacyAndResources(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	cal, _ := f.s.store.EnsureCalendar(ctx, owner.ID, "default", "Calendar")
	room, err := f.s.store.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "room", DisplayName: "Room", Kind: "room", AutoAccept: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	request := fmt.Sprintf(`{"summary":"Room series","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","recurrence":{"frequency":"WEEKLY","interval":1,"count":2},"attendees":[{"email":%q}]}`, room.Email)
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", request, 200)
	objects, _ := f.s.store.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 1 {
		t.Fatal("room booking was not saved")
	}
	roomCal, _ := f.s.store.GetCalendarByName(ctx, room.UserID, "default")
	bookings, err := f.s.store.ListCalendarObjects(ctx, roomCal.ID)
	if err != nil || len(bookings) != 1 {
		t.Fatalf("room copies: %v %v", bookings, err)
	}
	occurrences, err := calutil.BookingOccurrences(bookings[0].Data)
	if err != nil || len(occurrences) != 2 || occurrences[0].Attendees[0].PartStat != "ACCEPTED" {
		t.Fatalf("unchecked series: %v %v", occurrences, err)
	}
	// Unbounded bookings are rejected without disturbing the existing series.
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", strings.Replace(request, `,"count":2`, "", 1), 409)
	data, err := calutil.BuildSimpleVEVENT(calutil.Event{UID: "privacy", Summary: "Private rule"})
	if err != nil {
		t.Fatal(err)
	}
	o := &storage.CalendarObject{ID: "o", Data: data}
	p, _ := calutil.ParseICS(data)
	item := calendarEventJSON(o, p, &storage.Calendar{Name: "personal", UserID: owner.ID}, &authUser{ID: f.users[1]})
	if item["recurrence"] != nil || item["recurrence_organizer"] != nil {
		t.Fatal("private recurrence disclosed")
	}
}

func TestCalendarOrdinalRecurrenceAndRoomConflict(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	cal, _ := f.s.store.EnsureCalendar(ctx, owner.ID, "default", "Calendar")
	path := "/api/v1/calendar/calendars/" + cal.ID + "/events"
	request := `{"summary":"Third Tuesday","start":"2026-10-20T10:00:00Z","end":"2026-10-20T11:00:00Z","recurrence":{"frequency":"MONTHLY","interval":1,"weekdays":["TU"],"week_ordinal":3,"count":3},"attendees":[{"email":"reader@example.com"}]}`
	for _, invalid := range []string{
		strings.Replace(request, `"week_ordinal":3`, `"week_ordinal":6`, 1),
		strings.Replace(request, `"weekdays":["TU"]`, `"weekdays":["MO"]`, 1),
		strings.Replace(request, `"week_ordinal":3`, `"month_day":-1`, 1),
	} {
		f.call(0, "POST", path, invalid, 400)
	}
	objects, _ := f.s.store.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 0 {
		t.Fatal("invalid pattern saved an event")
	}
	f.call(0, "POST", path, request, 200)
	objects, _ = f.s.store.ListCalendarObjects(ctx, cal.ID)
	source := objects[0]
	body, _ := json.Marshal(map[string]any{"etag": source.ETag, "rule": calutil.RecurrenceRule{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: 3, Count: 4}})
	updatePath := "/api/v1/calendar/events/" + source.ID + "/recurrence"
	f.call(0, "PUT", updatePath, string(body), 200)
	f.call(0, "PUT", updatePath, string(body), 409)
	invites, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	copy, err := f.s.store.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if err != nil {
		t.Fatal(err)
	}
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	for _, object := range []*storage.CalendarObject{source, copy} {
		info := calutil.ReadRecurrence(object.Data)
		if !info.Editable || info.Rule.WeekOrdinal != 3 || info.Rule.Count != 4 {
			t.Fatalf("%+v", info)
		}
		items, err := calutil.ExpandOccurrences(object.Data, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC))
		if err != nil || len(items) != 4 {
			t.Fatalf("%d %v", len(items), err)
		}
		for i, want := range []string{"2026-10-20", "2026-11-17", "2026-12-15", "2027-01-19"} {
			if items[i].Start.Format("2006-01-02") != want {
				t.Fatalf("wrong date: %v", items[i].Start)
			}
		}
	}
	room, err := f.s.store.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "ordinal-room", DisplayName: "Room", Kind: "room", AutoAccept: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	busy := fmt.Sprintf(`{"summary":"Already booked","start":"2026-11-17T10:00:00Z","end":"2026-11-17T11:00:00Z","attendees":[{"email":%q}]}`, room.Email)
	f.call(0, "POST", path, busy, 200)
	withRoom := strings.Replace(request, `reader@example.com`, room.Email, 1)
	f.call(0, "POST", path, withRoom, 409)
	objects, _ = f.s.store.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 2 {
		t.Fatal("conflicting ordinal series partially saved")
	}
}
