package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestWebCalendarEditPreservesImportedEventAndLocation(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	st := f.s.store
	cal, _ := st.EnsureCalendar(ctx, f.users[0], "personal", "Personal")
	owner, _ := st.GetUserByID(ctx, f.users[0])
	if _, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "room", DisplayName: "Room"}); err != nil {
		t.Fatal(err)
	}
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:imported-flight\r\nSUMMARY:Flight\r\nORGANIZER:mailto:owner@example.com\r\nDTSTART:20261023T161015Z\r\nDTEND:20261023T184015Z\r\nLOCATION:Airport\r\nGEO:55.97;37.41\r\nX-APPLE-STRUCTURED-LOCATION:geo:55.97,37.41\r\nX-TAYGA-TRAVEL:original\r\nRRULE:FREQ=DAILY;COUNT=2\r\nATTENDEE;ROLE=OPT-PARTICIPANT;CUTYPE=RESOURCE:mailto:room@example.com\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT30M\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	object, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, HrefName: "original.ics", UID: "imported-flight", Component: "VEVENT", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	w := f.call(0, "GET", "/api/v1/calendar/events/"+object.ID, "", 200)
	var record map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["geo"] != "55.97;37.41" || record["recurrence_organizer"] != true {
		t.Fatal(record)
	}
	if record["attendees"].([]any)[0].(map[string]any)["kind"] != "resource" {
		t.Fatal(record)
	}
	delete(record, "recurrence") // The event form keeps the original series rule.
	record["summary"], record["location"], record["geo"] = "Updated flight", "New airport", "56;38"
	body, _ := json.Marshal(record)
	path := "/api/v1/calendar/calendars/" + cal.ID + "/events"
	f.call(0, "PUT", path, string(body), 200)
	saved, _ := st.GetCalendarObjectByID(ctx, object.ID)
	p, _ := calutil.ParseICS(saved.Data)
	if saved.HrefName != object.HrefName || saved.UID != object.UID || p.Geo != "56;38" || p.Location != "New airport" || p.Start.Second() != 15 || p.Attendees[0].Role != "OPT-PARTICIPANT" || p.Attendees[0].CUType != "RESOURCE" {
		t.Fatal(saved.Data)
	}
	for _, keep := range []string{"RRULE:FREQ=DAILY;COUNT=2", "BEGIN:VALARM", "X-TAYGA-TRAVEL:original"} {
		if !strings.Contains(saved.Data, keep) {
			t.Fatal("lost", keep)
		}
	}
	if strings.Contains(saved.Data, "X-APPLE-STRUCTURED-LOCATION") {
		t.Fatal("retained outdated Apple pin")
	}
	objects, _ := st.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 1 {
		t.Fatal("edit created duplicate")
	}
	before := calendarSnapshot(t, f)
	f.call(0, "PUT", path, string(body), 409)
	if calendarSnapshot(t, f) != before {
		t.Fatal("stale edit changed calendar or invitations")
	}
}
