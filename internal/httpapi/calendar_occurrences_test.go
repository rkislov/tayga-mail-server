package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func addTestRecurrence(t *testing.T, f *fileFixture, objects []*storage.CalendarObject) {
	t.Helper()
	for _, o := range objects {
		o.Data = strings.Replace(o.Data, "END:VEVENT", "RRULE:FREQ=WEEKLY;COUNT=4\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\nEND:VEVENT", 1)
		if _, err := f.s.store.UpsertCalendarObject(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCalendarOccurrenceTransactionAndNotification(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	cal, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"uid":"occurrence","summary":"Weekly","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"},{"email":"editor@example.com"}]}`, 200)
	copies, err := f.s.store.ListCalendarObjects(ctx, cal.ID)
	if err != nil {
		t.Fatal(err)
	}
	source := copies[0]
	for _, u := range f.users[1:] {
		invs, _ := f.s.store.ListCalendarInvitesForUser(ctx, u, false)
		o, _ := f.s.store.GetCalendarObjectByID(ctx, invs[0].CalendarObjectID)
		copies = append(copies, o)
	}
	addTestRecurrence(t, f, copies)
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	original := map[string]string{}
	for _, o := range copies {
		stored, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
		original[o.ID] = stored.Data
	}
	rid := "2026-10-26T10:00:00Z"
	path := "/api/v1/calendar/events/" + source.ID + "/occurrences"
	request := func(etag, action string) string {
		b, _ := json.Marshal(map[string]any{"action": action, "etag": etag, "recurrence_id": rid, "start": "2026-10-27T12:00:00Z", "end": "2026-10-27T13:30:00Z", "summary": "Moved instance"})
		return string(b)
	}
	f.call(1, "POST", path, request(source.ETag, "update"), 403)
	f.call(1, "POST", "/api/v1/calendar/events/"+copies[1].ID+"/occurrences", request(copies[1].ETag, "update"), 403)
	f.call(0, "POST", path, request("stale", "update"), 409)
	f.call(0, "POST", path, strings.Replace(request(source.ETag, "update"), rid, "2026-10-28T10:00:00Z", 1), 400)
	db := f.s.store.(*storage.Store).DB()
	if _, err := db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_occurrence BEFORE UPDATE ON calendar_objects WHEN OLD.id='%s' BEGIN SELECT RAISE(ABORT,'injected'); END`, copies[1].ID)); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", path, request(source.ETag, "update"), 500)
	for _, o := range copies {
		stored, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
		if stored.Data != original[o.ID] {
			t.Fatal("partial occurrence update")
		}
	}
	if _, err := db.Exec(`DROP TRIGGER fail_occurrence`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", path, request(source.ETag, "update"), 200)
	f.call(0, "POST", path, request(source.ETag, "update"), 409)
	for _, o := range copies {
		stored, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
		parsed, _ := calutil.ParseICS(stored.Data)
		if parsed.Sequence != 0 || parsed.Summary != "Weekly" || parsed.Start.Day() != 19 {
			t.Fatalf("changed master %+v", parsed)
		}
		instance, err := calutil.GetOccurrence(stored.Data, rid)
		if err != nil || instance.Start.Day() != 27 || instance.End.Minute() != 30 || instance.Summary != "Moved instance" || instance.Sequence != 1 {
			t.Fatalf("instance %+v %v", instance, err)
		}
		if strings.Count(stored.Data, "BEGIN:VALARM") != 2 {
			t.Fatal("recipient reminders lost")
		}
	}
	listPath := "/api/v1/calendar/calendars/" + cal.ID + "/events?expand=true&from=2026-10-01T00:00:00Z&to=2026-12-01T00:00:00Z"
	w := f.call(0, "GET", listPath, "", 200)
	var list struct {
		Events   []map[string]any
		Warnings []map[string]string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Events) != 4 || len(list.Warnings) != 0 {
		t.Fatalf("%s %v", w.Body.String(), err)
	}
	getPath := "/api/v1/calendar/events/" + source.ID + "?" + url.Values{"recurrence_id": {rid}}.Encode()
	got := f.call(0, "GET", getPath, "", 200)
	if !strings.Contains(got.Body.String(), "2026-10-27T12:00:00Z") {
		t.Fatal(got.Body.String())
	}
	f.call(0, "GET", "/api/v1/calendar/calendars/"+cal.ID+"/events?expand=true", "", 400)
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	f.call(0, "POST", path, request(source.ETag, "cancel"), 200)
	for _, o := range copies {
		stored, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
		items, err := calutil.ExpandOccurrences(stored.Data, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
		if err != nil || len(items) != 3 {
			t.Fatalf("cancel removed series: %d %v", len(items), err)
		}
	}
	for _, u := range f.users[1:] {
		if err := f.s.drainCalendarDeliveries(ctx); err != nil {
			t.Fatal(err)
		}
		mb, _ := f.s.store.GetMailbox(ctx, u, "INBOX")
		messages, _ := f.s.store.ListMessages(ctx, mb.ID)
		if len(messages) != 3 {
			t.Fatalf("wrong invite count %d", len(messages))
		}
		counts := map[string]int{}
		for _, message := range messages {
			raw, _ := f.s.ms.Read(message.FilePath)
			text := string(raw)
			if !strings.Contains(text, "RECURRENCE-ID:20261026T100000Z") {
				continue
			}
			if strings.Contains(text, "RRULE") || strings.Contains(text, "VALARM") || strings.Count(text, "BEGIN:VEVENT") != 1 {
				t.Fatalf("whole series/alarms sent: %s", text)
			}
			for _, method := range []string{"REQUEST", "CANCEL"} {
				if strings.Contains(text, "METHOD:"+method) {
					counts[method]++
				}
			}
		}
		if counts["REQUEST"] != 1 || counts["CANCEL"] != 1 {
			t.Fatalf("missing instance mail: %v", counts)
		}
	}
}

func TestCalendarOccurrenceRoomIncludesOtherRepeats(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	room, err := f.s.store.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "room", DisplayName: "Room", Kind: "room", AutoAccept: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cal, _ := f.s.store.EnsureCalendar(ctx, owner.ID, "default", "Calendar")
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"uid":"room-series","summary":"Weekly room","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"room@example.com"}]}`, 200)
	objs, _ := f.s.store.ListCalendarObjects(ctx, cal.ID)
	roomCal, _ := f.s.store.GetCalendarByName(ctx, room.UserID, "default")
	roomObjs, _ := f.s.store.ListCalendarObjects(ctx, roomCal.ID)
	addTestRecurrence(t, f, append(objs, roomObjs...))
	source, _ := f.s.store.GetCalendarObjectByID(ctx, objs[0].ID)
	path := "/api/v1/calendar/events/" + source.ID + "/occurrences"
	payload := func(start, end string) string {
		b, _ := json.Marshal(map[string]any{"action": "update", "recurrence_id": "2026-10-26T10:00:00Z", "etag": source.ETag, "start": start, "end": end})
		return string(b)
	}
	// Another occurrence of the same series remains a genuine room conflict.
	f.call(0, "POST", path, payload("2026-11-02T10:30:00Z", "2026-11-02T11:30:00Z"), 409)
	// The instance being moved does not conflict with its own old interval.
	f.call(0, "POST", path, payload("2026-10-26T10:30:00Z", "2026-10-26T11:30:00Z"), 200)
	stored, _ := f.s.store.GetCalendarObjectByID(ctx, roomObjs[0].ID)
	instance, err := calutil.GetOccurrence(stored.Data, "2026-10-26T10:00:00Z")
	if err != nil || instance.Start.Minute() != 30 {
		t.Fatalf("resource copy: %+v %v", instance, err)
	}
}
