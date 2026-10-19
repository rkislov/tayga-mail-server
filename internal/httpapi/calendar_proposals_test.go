package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarProposalDecisions(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	cal, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"uid":"decision","summary":"Meeting","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"},{"email":"editor@example.com"}]}`, 200)
	invites, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	inv := invites[0]
	reply := "/api/v1/calendar/invites/" + inv.ID + "/reply"
	decision := "/api/v1/calendar/invites/" + inv.ID + "/decision"
	propose := func() {
		f.call(1, "POST", reply, `{"action":"counter","start":"2026-10-19T10:30:00Z","end":"2026-10-19T11:30:00Z"}`, 200)
	}
	list := func(user int) []map[string]any {
		t.Helper()
		w := f.call(user, "GET", "/api/v1/calendar/invites", "", 200)
		var result struct{ Proposals []map[string]any }
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Proposals
	}
	body := func(p map[string]any, action string) string {
		t.Helper()
		b, err := json.Marshal(map[string]any{"action": action, "event_id": p["event_id"], "revision": p["revision"], "etag": p["etag"]})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	f.call(1, "POST", reply, `{"action":"accept"}`, 200)
	propose()
	if len(list(1)) != 0 || len(list(2)) != 0 || len(list(0)) != 1 {
		t.Fatal("proposal visibility")
	}
	p := list(0)[0]
	o, _ := f.s.store.GetCalendarObjectByID(ctx, p["event_id"].(string))
	initialData := o.Data
	decline := body(p, "decline")
	f.call(2, "POST", decision, decline, 403)
	f.call(0, "POST", decision, strings.Replace(decline, p["revision"].(string), "stale", 1), 409)
	f.call(0, "POST", decision, decline, 200)
	f.call(0, "POST", decision, decline, 409)
	o, _ = f.s.store.GetCalendarObjectByID(ctx, o.ID)
	got, _ := f.s.store.GetCalendarInvite(ctx, inv.ID)
	if o.Data != initialData || got.PartStat != "ACCEPTED" || got.ProposedStart != nil || len(list(0)) != 0 {
		t.Fatal("decline altered meeting/RSVP")
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mb, _ := f.s.store.GetMailbox(ctx, f.users[1], "INBOX")
	messages, _ := f.s.store.ListMessages(ctx, mb.ID)
	found := false
	for _, message := range messages {
		raw, _ := f.s.ms.Read(message.FilePath)
		if strings.Contains(string(raw), "METHOD:DECLINECOUNTER") {
			found = true
			if !strings.Contains(string(raw), "SEQUENCE:0") || strings.Contains(string(raw), "mailto:editor@example.com") {
				t.Fatal("wrong decline payload")
			}
		}
	}
	if !found {
		t.Fatal("missing DECLINECOUNTER mail")
	}
	propose()
	p = list(0)[0]
	accept := body(p, "accept")
	// A replaced proposal is rejected even when the event ETag stayed the same.
	propose()
	f.call(0, "POST", decision, accept, 409)
	p = list(0)[0]
	accept = body(p, "accept")
	db := f.s.store.(*storage.Store).DB()
	if _, err := db.Exec(`CREATE TRIGGER fail_decision BEFORE UPDATE ON calendar_invites BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", decision, accept, 500)
	o, _ = f.s.store.GetCalendarObjectByID(ctx, o.ID)
	if o.Data != initialData || len(list(0)) != 1 {
		t.Fatal("partial decision")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_decision`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", decision, accept, 200)
	f.call(0, "POST", decision, accept, 409)
	if len(list(0)) != 0 {
		t.Fatal("proposal not cleared")
	}
	for _, userID := range f.users {
		cals, _ := f.s.store.ListCalendars(ctx, userID)
		seen := false
		for _, c := range cals {
			objects, _ := f.s.store.ListCalendarObjects(ctx, c.ID)
			for _, object := range objects {
				if object.UID != "decision" {
					continue
				}
				seen = true
				event, _ := calutil.ParseICS(object.Data)
				if event.Sequence != 1 || event.Start.Minute() != 30 || event.End.Hour() != 11 {
					t.Fatalf("not rescheduled %+v", event)
				}
				for _, a := range event.Attendees {
					if a.PartStat != "NEEDS-ACTION" {
						t.Fatal(a)
					}
				}
			}
		}
		if !seen {
			t.Fatal("missing local copy")
		}
	}
	for _, userID := range f.users[1:] {
		if err := f.s.drainCalendarDeliveries(ctx); err != nil {
			t.Fatal(err)
		}
		mb, _ := f.s.store.GetMailbox(ctx, userID, "INBOX")
		messages, _ := f.s.store.ListMessages(ctx, mb.ID)
		count := 0
		for _, message := range messages {
			raw, _ := f.s.ms.Read(message.FilePath)
			if strings.Contains(string(raw), "METHOD:REQUEST") && strings.Contains(string(raw), "SEQUENCE:1") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("updated invites: %d", count)
		}
	}
}

func TestCalendarProposalRoomConflictAndReschedule(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	room, err := f.s.store.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "room", DisplayName: "Room", Kind: "room", AutoAccept: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cal, _ := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"uid":"room-decision","summary":"Room meeting","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":"reader@example.com"},{"email":"room@example.com"}]}`, 200)
	invites, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	f.call(1, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"counter","start":"2026-10-19T10:30:00Z","end":"2026-10-19T11:30:00Z"}`, 200)
	proposals, err := f.s.store.ListCalendarProposals(ctx, owner.ID)
	if err != nil || len(proposals) != 1 {
		t.Fatalf("%v %v", proposals, err)
	}
	p := proposals[0]
	payload, _ := json.Marshal(map[string]string{"action": "accept", "event_id": p.Object.ID, "etag": p.Object.ETag, "revision": p.Invite.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	path := "/api/v1/calendar/invites/" + p.Invite.ID + "/decision"
	roomCal, _ := f.s.store.GetCalendarByName(ctx, room.UserID, "default")
	otherCal, _ := f.s.store.EnsureCalendar(ctx, room.UserID, "other", "Other meetings")
	start := time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data, _ := calutil.BuildSimpleVEVENT(calutil.Event{UID: "room-decision", Organizer: calutil.Attendee{Email: "editor@example.com"}, Start: &start, End: &end})
	_, err = f.s.store.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: otherCal.ID, UID: "room-decision", HrefName: "other.ics", Component: "VEVENT", Data: data, DTStart: &start, DTEnd: &end})
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", path, string(payload), 409)
	unchanged, _ := f.s.store.GetCalendarObjectByID(ctx, p.Object.ID)
	if unchanged.ETag != p.Object.ETag {
		t.Fatal("room conflict moved meeting")
	}
	if err := f.s.store.DeleteCalendarObject(ctx, otherCal.ID, "other.ics"); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", path, string(payload), 200)
	objects, _ := f.s.store.ListCalendarObjects(ctx, roomCal.ID)
	if len(objects) != 1 || objects[0].DTStart.Minute() != 30 {
		t.Fatalf("room booking not moved: %+v", objects)
	}
	o, _ := f.s.store.GetCalendarObjectByID(ctx, p.Object.ID)
	event, _ := calutil.ParseICS(o.Data)
	for _, a := range event.Attendees {
		if a.Email == room.Email && a.PartStat != "ACCEPTED" {
			t.Fatal("booking conflicted with itself")
		}
	}
}
