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

func TestCalendarReplyAndCounterDoNotReschedule(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	calendar, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/calendar/calendars/" + calendar.ID + "/events"
	response := f.call(0, "POST", base, `{"uid":"meeting","summary":"Weekly meeting","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"}]}`, 200)
	var created struct{ ID string }
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	invites, err := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if err != nil || len(invites) != 1 {
		t.Fatalf("%+v %v", invites, err)
	}
	inv := invites[0]
	// Simulate an imported recurrence in both copies, including timezone and alarms.
	for _, id := range []string{created.ID, inv.CalendarObjectID} {
		o, err := f.s.store.GetCalendarObjectByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		o.Data = strings.Replace(o.Data, "END:VEVENT", "RRULE:FREQ=WEEKLY;COUNT=5\r\nEXDATE:20261102T100000Z\r\nX-PRIVATE:retained\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Alarm\r\nEND:VALARM\r\nEND:VEVENT", 1)
		o.Data = strings.Replace(o.Data, "SEQUENCE:0", "SEQUENCE:7", 1)
		if _, err := f.s.store.UpsertCalendarObject(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v1/calendar/invites/" + inv.ID + "/reply"
	attCopy, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	f.call(2, "POST", path, `{"action":"accept"}`, 403)
	f.call(1, "POST", path, `{"action":"counter","start":"2026-10-20T12:00:00Z","end":"2026-10-20T11:00:00Z"}`, 400)
	f.call(1, "POST", path, `{"action":"accept","etag":"stale"}`, 409)
	f.call(1, "POST", path, `{"action":"accept","etag":`+fmt.Sprintf("%q", attCopy.ETag)+`}`, 200)
	checkCopies := func(want string) {
		t.Helper()
		for _, id := range []string{created.ID, inv.CalendarObjectID} {
			o, _ := f.s.store.GetCalendarObjectByID(ctx, id)
			p, _ := calutil.ParseICS(o.Data)
			if p.Sequence != 7 || p.Start.Day() != 19 || p.Start.Hour() != 10 || p.Attendees[0].PartStat != want {
				t.Fatalf("changed meeting: %+v", p)
			}
			for _, text := range []string{"RRULE:FREQ=WEEKLY;COUNT=5", "EXDATE:20261102T100000Z", "X-PRIVATE:retained", "BEGIN:VALARM"} {
				if !strings.Contains(o.Data, text) {
					t.Fatalf("lost %s", text)
				}
			}
		}
	}
	checkCopies("ACCEPTED")
	attCopy, _ = f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	f.call(1, "POST", path, `{"action":"counter","start":"2026-10-20T12:00:00Z","end":"2026-10-20T13:00:00Z","comment":"Can we move it?","etag":`+fmt.Sprintf("%q", attCopy.ETag)+`}`, 200)
	checkCopies("ACCEPTED")
	got, _ := f.s.store.GetCalendarInvite(ctx, inv.ID)
	if got.ProposedStart == nil || got.ProposedStart.Day() != 20 || got.PartStat != "ACCEPTED" {
		t.Fatalf("proposal %+v", got)
	}
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mb, _ := f.s.store.GetMailbox(ctx, owner.ID, "INBOX")
	messages, err := f.s.store.ListMessages(ctx, mb.ID)
	if err != nil || len(messages) != 2 {
		t.Fatalf("responses: %d %v", len(messages), err)
	}
	methods := map[string]bool{}
	for _, m := range messages {
		raw, err := f.s.ms.Read(m.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "SEQUENCE:7") {
			t.Fatalf("wrong reply sequence: %s", text)
		}
		for _, method := range []string{"REPLY", "COUNTER"} {
			if strings.Contains(text, "METHOD:"+method) {
				methods[method] = true
			}
		}
	}
	if !methods["REPLY"] || !methods["COUNTER"] {
		t.Fatal(methods)
	}
	// Updating the original reuses the same invite, preserves series data and
	// invalidates the proposal/previous ETag rather than creating another inbox row.
	f.call(0, "PUT", base, `{"id":"`+created.ID+`","summary":"Renamed","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"}]}`, 200)
	invites, _ = f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if len(invites) != 1 || invites[0].ID != inv.ID || invites[0].ProposedStart != nil {
		t.Fatalf("duplicate/stale invite: %+v", invites)
	}
	updated, _ := f.s.store.GetCalendarObjectByID(ctx, created.ID)
	if !strings.Contains(updated.Data, "RRULE:") || !strings.Contains(updated.Data, "BEGIN:VALARM") {
		t.Fatal(updated.Data)
	}
	f.call(1, "POST", path, `{"action":"accept","etag":`+fmt.Sprintf("%q", attCopy.ETag)+`}`, 409)
	f.call(0, "DELETE", "/api/v1/calendar/events/"+created.ID, "", 200)
	f.call(1, "POST", path, `{"action":"accept"}`, 409)
	cancelled, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	p, _ := calutil.ParseICS(cancelled.Data)
	if p.Status != "CANCELLED" {
		t.Fatalf("cancel not applied: %+v", p)
	}
	busy, err := f.s.store.FreeBusyForUser(ctx, f.users[1], time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC))
	if err != nil || len(busy) != 0 {
		t.Fatalf("cancelled meeting still busy: %+v %v", busy, err)
	}
}

func TestCalendarSameUIDHasSeparateOrganizers(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	for _, index := range []int{0, 2} {
		cal, err := f.s.store.EnsureCalendar(ctx, f.users[index], "default", "Calendar")
		if err != nil {
			t.Fatal(err)
		}
		f.call(index, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"uid":"same","summary":"Independent","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"}]}`, 200)
	}
	invites, err := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if err != nil || len(invites) != 2 || invites[0].CalendarObjectID == invites[1].CalendarObjectID {
		t.Fatalf("colliding meetings: %+v %v", invites, err)
	}
	var byOrganizer = map[string]*storage.CalendarInvite{}
	for _, inv := range invites {
		byOrganizer[inv.OrganizerUserID] = inv
		o, err := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.s.store.GetCalendarByID(ctx, f.users[1], o.CalendarID); err != nil {
			t.Fatal("invite points at organizer, not a delivered attendee copy")
		}
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+byOrganizer[f.users[0]].ID+"/reply", `{"action":"accept"}`, 200)
	o, _ := f.s.store.GetCalendarObjectByID(ctx, byOrganizer[f.users[2]].CalendarObjectID)
	p, _ := calutil.ParseICS(o.Data)
	if p.Attendees[0].PartStat != "NEEDS-ACTION" {
		t.Fatal("reply changed different organizer's meeting")
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+byOrganizer[f.users[2]].ID+"/reply", `{"action":"accept"}`, 200)
}

func TestCalendarSharedOrganizerAndLegacyCopy(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	shared, err := f.s.store.EnsureCalendar(ctx, f.users[2], "work", "Shared")
	if err != nil {
		t.Fatal(err)
	}
	f.call(2, "PUT", "/api/v1/calendar/calendars/"+shared.ID+"/acl", `{"email":"owner@example.com","rights":"write"}`, 200)
	attCal, err := f.s.store.EnsureCalendar(ctx, f.users[1], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data, err := calutil.BuildRequestICS(calutil.Event{UID: "legacy", Summary: "Old", Start: &start, End: &end, Organizer: calutil.Attendee{Email: "owner@example.com"}, Attendees: []calutil.Attendee{{Email: "reader@example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := f.s.store.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: attCal.ID, UID: "legacy", HrefName: "legacy.ics", Data: data, Component: "VEVENT", DTStart: &start, DTEnd: &end})
	if err != nil {
		t.Fatal(err)
	}
	response := f.call(0, "POST", "/api/v1/calendar/calendars/"+shared.ID+"/events", `{"uid":"legacy","summary":"Updated","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"}]}`, 200)
	var event struct{ ID string }
	if err := json.Unmarshal(response.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	invites, err := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if err != nil || len(invites) != 1 || invites[0].CalendarObjectID != legacy.ID {
		t.Fatalf("legacy reference lost: %+v %v", invites, err)
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"accept"}`, 200)
	o, err := f.s.store.GetCalendarObjectByID(ctx, event.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := calutil.ParseICS(o.Data)
	if p.Attendees[0].PartStat != "ACCEPTED" {
		t.Fatal("shared organizer did not receive RSVP")
	}
	copies, err := f.s.store.ListCalendarObjects(ctx, attCal.ID)
	if err != nil || len(copies) != 1 {
		t.Fatalf("duplicate legacy calendar copy: %+v %v", copies, err)
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"counter","start":"2026-10-19T12:00:00Z","end":"2026-10-19T13:00:00Z"}`, 200)
	proposals, err := f.s.store.ListCalendarProposals(ctx, f.users[0])
	if err != nil || len(proposals) != 1 {
		t.Fatalf("shared proposal: %+v %v", proposals, err)
	}
	proposal := proposals[0]
	payload, _ := json.Marshal(map[string]string{"action": "accept", "event_id": proposal.Object.ID, "revision": proposal.Invite.UpdatedAt.UTC().Format(time.RFC3339Nano), "etag": proposal.Object.ETag})
	path := "/api/v1/calendar/invites/" + invites[0].ID + "/decision"
	f.call(2, "PUT", "/api/v1/calendar/calendars/"+shared.ID+"/acl", `{"email":"owner@example.com","rights":"read"}`, 200)
	f.call(0, "POST", path, string(payload), 409)
	proposals, err = f.s.store.ListCalendarProposals(ctx, f.users[0])
	if err != nil || len(proposals) != 0 {
		t.Fatal("read-only calendar accepted proposal")
	}
	f.call(2, "PUT", "/api/v1/calendar/calendars/"+shared.ID+"/acl", `{"email":"owner@example.com","rights":"write"}`, 200)
	f.call(0, "POST", path, string(payload), 200)
}
