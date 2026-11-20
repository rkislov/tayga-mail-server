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

type occurrenceCounterFixture struct {
	f       *fileFixture
	source  *storage.CalendarObject
	invites map[int]*storage.CalendarInvite
	room    *storage.CalendarResource
}

func newOccurrenceCounterFixture(t *testing.T, allDay, room bool) occurrenceCounterFixture {
	t.Helper()
	ctx := context.Background()
	f := newFileFixture(t)
	cal, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	attendees := []map[string]string{{"email": "reader@example.com"}, {"email": "editor@example.com"}}
	var resource *storage.CalendarResource
	if room {
		owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
		resource, err = f.s.store.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "counter-room", DisplayName: "Room", Kind: "room", AutoAccept: true, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		attendees = append(attendees, map[string]string{"email": resource.Email})
	}
	start, end := "2026-10-19T08:00:00Z", "2026-10-19T09:00:00Z"
	if allDay {
		start, end = "2026-10-19T00:00:00Z", "2026-10-20T00:00:00Z"
	}
	body, _ := json.Marshal(map[string]any{"summary": "Series", "start": start, "end": end, "all_day": allDay, "timezone": "Europe/Berlin", "recurrence": map[string]any{"frequency": "WEEKLY", "interval": 1, "count": 4}, "attendees": attendees})
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", string(body), 200)
	objects, _ := f.s.store.ListCalendarObjects(ctx, cal.ID)
	out := occurrenceCounterFixture{f: f, source: objects[0], invites: map[int]*storage.CalendarInvite{}, room: resource}
	for _, user := range []int{1, 2} {
		invs, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[user], false)
		out.invites[user] = invs[0]
	}
	return out
}

func (x occurrenceCounterFixture) propose(t *testing.T, user int, rid, start, end string, want int) {
	t.Helper()
	inv := x.invites[user]
	copy, _ := x.f.s.store.GetCalendarObjectByID(context.Background(), inv.CalendarObjectID)
	x.f.call(user, "POST", "/api/v1/calendar/invites/"+inv.ID+"/reply", fmt.Sprintf(`{"action":"counter","etag":%q,"recurrence_id":%q,"start":%q,"end":%q}`, copy.ETag, rid, start, end), want)
}

func (x occurrenceCounterFixture) list(t *testing.T, user int) []map[string]any {
	t.Helper()
	var out struct{ Proposals []map[string]any }
	w := x.f.call(user, "GET", "/api/v1/calendar/invites", "", 200)
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Proposals
}

func (x occurrenceCounterFixture) proposal(t *testing.T, rid string) map[string]any {
	t.Helper()
	for _, p := range x.list(t, 0) {
		if p["id"] == x.invites[1].ID && p["recurrence_id"] == rid {
			return p
		}
	}
	t.Fatalf("missing proposal %s", rid)
	return nil
}

func counterDecision(t *testing.T, p map[string]any, action string) string {
	t.Helper()
	body := map[string]any{"action": action, "event_id": p["event_id"], "revision": p["revision"], "etag": p["etag"], "recurrence_id": p["recurrence_id"]}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOccurrenceCounterProposalLifecycleAndIsolation(t *testing.T) {
	x := newOccurrenceCounterFixture(t, false, false)
	f := x.f
	ctx := context.Background()
	inv := x.invites[1]
	reply := "/api/v1/calendar/invites/" + inv.ID + "/reply"
	decision := "/api/v1/calendar/invites/" + inv.ID + "/decision"
	f.call(1, "POST", reply, `{"action":"accept"}`, 200)
	f.call(1, "POST", reply, `{"action":"counter","start":"2026-10-19T12:00:00Z","end":"2026-10-19T13:00:00Z"}`, 200)
	seriesInvite, _ := f.s.store.GetCalendarInvite(ctx, inv.ID)
	source, _ := f.s.store.GetCalendarObjectByID(ctx, x.source.ID)
	rid := "2026-10-26T09:00:00Z"
	otherRID := "2026-11-02T09:00:00Z"
	f.call(0, "POST", "/api/v1/calendar/events/"+source.ID+"/occurrences", fmt.Sprintf(`{"action":"update","etag":%q,"recurrence_id":%q,"start":"2026-10-27T09:00:00Z","end":"2026-10-27T10:00:00Z"}`, source.ETag, rid), 200)
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	initial := source.Data
	copy, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	copy.Data = strings.Replace(copy.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Private\r\nTRIGGER:-PT5M\r\nEND:VALARM\r\nEND:VEVENT", 1)
	copy, _ = f.s.store.UpsertCalendarObject(ctx, copy)
	x.propose(t, 1, rid, "2026-11-03T12:00:00Z", "2026-11-03T13:00:00Z", 200)
	p := x.proposal(t, rid)
	db := f.s.store.(*storage.Store).DB()
	var before, after int
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&before)
	x.propose(t, 1, rid, "2026-11-03T12:00:00Z", "2026-11-03T13:00:00Z", 200)
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&after)
	if after != before || x.proposal(t, rid)["revision"] != p["revision"] {
		t.Fatal("duplicate COUNTER/revision")
	}
	if len(x.list(t, 1)) != 0 || len(x.list(t, 2)) != 0 {
		t.Fatal("proposal leaked to participant")
	}
	for _, obj := range []*storage.CalendarObject{source, copy} {
		saved, _ := f.s.store.GetCalendarObjectByID(ctx, obj.ID)
		if saved.Data != obj.Data || saved.ETag != obj.ETag {
			t.Fatal("proposal moved event or RSVP")
		}
	}
	x.propose(t, 1, otherRID, "2026-11-02T14:00:00Z", "2026-11-02T15:00:00Z", 200)
	if len(x.list(t, 0)) != 3 {
		t.Fatal("series/date proposals are not independent")
	}
	// Replacing one date has a new revision without changing the event ETag.
	x.propose(t, 1, rid, "2026-11-03T12:30:00Z", "2026-11-03T13:30:00Z", 200)
	f.call(0, "POST", decision, counterDecision(t, p, "accept"), 409)
	p = x.proposal(t, rid)
	f.call(2, "POST", decision, counterDecision(t, p, "accept"), 403)
	f.call(0, "POST", decision, counterDecision(t, p, "decline"), 200)
	f.call(0, "POST", decision, counterDecision(t, p, "decline"), 409)
	unchanged, _ := f.s.store.GetCalendarObjectByID(ctx, source.ID)
	if unchanged.Data != initial {
		t.Fatal("decline changed meeting")
	}
	if len(x.list(t, 0)) != 2 {
		t.Fatal("decline removed other proposals")
	}
	x.propose(t, 1, rid, "2026-11-03T12:00:00Z", "2026-11-03T13:00:00Z", 200)
	p = x.proposal(t, rid)
	bad := counterDecision(t, p, "accept")
	stale := make(map[string]any)
	for k, v := range p {
		stale[k] = v
	}
	stale["etag"] = "stale"
	f.call(0, "POST", decision, counterDecision(t, stale, "accept"), 409)
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&before)
	f.call(0, "POST", decision, bad, 200)
	f.call(0, "POST", decision, bad, 409)
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&after)
	if after != before+2 {
		t.Fatalf("REQUEST count %d", after-before)
	}
	for _, id := range []string{source.ID, copy.ID, x.invites[2].CalendarObjectID} {
		o, _ := f.s.store.GetCalendarObjectByID(ctx, id)
		master, _ := calutil.ParseICS(o.Data)
		instance, err := calutil.GetOccurrence(o.Data, rid)
		if err != nil || instance.Start.UTC().Format(time.RFC3339) != "2026-11-03T12:00:00Z" || instance.Sequence != 2 || master.Sequence != 0 || master.Start.UTC().Format(time.RFC3339) != "2026-10-19T08:00:00Z" {
			t.Fatalf("wrong accepted instance: %+v %v", instance, err)
		}
		for _, a := range instance.Attendees {
			if a.PartStat != "NEEDS-ACTION" {
				t.Fatal("instance RSVP not reset")
			}
		}
		other, err := calutil.GetOccurrence(o.Data, otherRID)
		if err != nil || other.Start.UTC().Format(time.RFC3339) != otherRID {
			t.Fatal("moved another date")
		}
		if id == copy.ID && !strings.Contains(o.Data, "DESCRIPTION:Private") {
			t.Fatal("lost reminder")
		}
	}
	got, _ := f.s.store.GetCalendarInvite(ctx, inv.ID)
	if got.PartStat != seriesInvite.PartStat || got.ProposedStart == nil || !got.ProposedStart.Equal(*seriesInvite.ProposedStart) || !got.UpdatedAt.Equal(seriesInvite.UpdatedAt) {
		t.Fatal("changed whole-series RSVP/proposal")
	}
	if len(x.list(t, 0)) != 2 {
		t.Fatal("accept removed other date or series proposal")
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	for _, user := range []int{0, 1} {
		mb, _ := f.s.store.GetMailbox(ctx, f.users[user], "INBOX")
		messages, _ := f.s.store.ListMessages(ctx, mb.ID)
		found := false
		for _, m := range messages {
			raw, _ := f.s.ms.Read(m.FilePath)
			text := string(raw)
			if !strings.Contains(text, "RECURRENCE-ID;TZID=Europe/Berlin:20261026T100000") {
				continue
			}
			if strings.Contains(text, "METHOD:COUNTER") && user == 0 || strings.Contains(text, "METHOD:DECLINECOUNTER") && user == 1 {
				found = true
				if strings.Count(text, "BEGIN:VEVENT") != 1 || strings.Contains(text, "RRULE:") || strings.Contains(text, "BEGIN:VALARM") {
					t.Fatal("wrong single-instance payload")
				}
			}
		}
		if !found {
			t.Fatalf("missing counter/decline mail for %d", user)
		}
	}
}

func TestOccurrenceCounterBookingAndRollback(t *testing.T) {
	x := newOccurrenceCounterFixture(t, false, true)
	f := x.f
	ctx := context.Background()
	inv := x.invites[1]
	rid := "2026-10-26T09:00:00Z"
	decision := "/api/v1/calendar/invites/" + inv.ID + "/decision"
	x.propose(t, 1, rid, "2026-11-02T09:00:00Z", "2026-11-02T10:00:00Z", 200)
	p := x.proposal(t, rid)
	f.call(0, "POST", decision, counterDecision(t, p, "accept"), 409)
	source, _ := f.s.store.GetCalendarObjectByID(ctx, x.source.ID)
	if source.ETag != x.source.ETag {
		t.Fatal("room conflict partially committed")
	}
	x.propose(t, 1, rid, "2026-10-27T12:00:00Z", "2026-10-27T13:00:00Z", 200)
	p = x.proposal(t, rid)
	db := f.s.store.(*storage.Store).DB()
	copy, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	for _, trigger := range []string{
		`CREATE TRIGGER fail_counter BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`,
		`CREATE TRIGGER fail_counter BEFORE DELETE ON calendar_occurrence_proposals BEGIN SELECT RAISE(ABORT,'injected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		f.call(0, "POST", decision, counterDecision(t, p, "accept"), 500)
		for _, o := range []*storage.CalendarObject{source, copy} {
			after, _ := f.s.store.GetCalendarObjectByID(ctx, o.ID)
			if after.Data != o.Data || after.ETag != o.ETag {
				t.Fatal("partial accepted proposal")
			}
		}
		if x.proposal(t, rid)["revision"] != p["revision"] {
			t.Fatal("lost proposal on rollback")
		}
		if _, err := db.Exec(`DROP TRIGGER fail_counter`); err != nil {
			t.Fatal(err)
		}
	}
	f.call(0, "POST", decision, counterDecision(t, p, "accept"), 200)
	roomCal, _ := f.s.store.GetCalendarByName(ctx, x.room.UserID, "default")
	bookings, _ := f.s.store.ListCalendarObjects(ctx, roomCal.ID)
	instance, err := calutil.GetOccurrence(bookings[0].Data, rid)
	if err != nil || instance.Start.UTC().Hour() != 12 {
		t.Fatalf("room not moved: %+v %v", instance, err)
	}
	// Failed proposal creation rolls back its COUNTER job as well.
	var before, after int
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&before)
	if _, err := db.Exec(`CREATE TRIGGER fail_counter BEFORE INSERT ON calendar_occurrence_proposals BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	x.propose(t, 1, rid, "2026-10-28T12:00:00Z", "2026-10-28T13:00:00Z", 500)
	db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&after)
	if before != after {
		t.Fatal("orphan COUNTER job")
	}
}

func TestOccurrenceCounterAllDayStaleAndWithdraw(t *testing.T) {
	x := newOccurrenceCounterFixture(t, true, false)
	f := x.f
	ctx := context.Background()
	inv := x.invites[1]
	rid := "2026-10-26"
	decision := "/api/v1/calendar/invites/" + inv.ID + "/decision"
	reply := "/api/v1/calendar/invites/" + inv.ID + "/reply"
	x.propose(t, 1, rid, "2026-10-27T12:00:00Z", "2026-10-27T13:00:00Z", 200)
	p := x.proposal(t, rid)
	for _, bad := range []string{`null`, `""`, `123`} {
		body := strings.Replace(counterDecision(t, p, "accept"), `"recurrence_id":"2026-10-26"`, `"recurrence_id":`+bad, 1)
		f.call(0, "POST", decision, body, 400)
	}
	f.call(0, "POST", decision, counterDecision(t, p, "accept"), 200)
	source, _ := f.s.store.GetCalendarObjectByID(ctx, x.source.ID)
	master, _ := calutil.ParseICS(source.Data)
	instance, err := calutil.GetOccurrence(source.Data, rid)
	if err != nil || instance.AllDay || !master.AllDay || instance.Start.UTC().Format(time.RFC3339) != "2026-10-27T12:00:00Z" {
		t.Fatalf("date proposal lost hours/identity: %+v %v", instance, err)
	}
	// Explicit RSVP withdraws only that date's proposal, including same status.
	x.propose(t, 1, rid, "2026-10-28T12:00:00Z", "2026-10-28T13:00:00Z", 200)
	copy, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	f.call(1, "POST", reply, fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":%q}`, copy.ETag, rid), 200)
	if len(x.list(t, 0)) != 0 {
		t.Fatal("RSVP did not withdraw date proposal")
	}
	x.propose(t, 1, rid, "2026-10-28T12:00:00Z", "2026-10-28T13:00:00Z", 200)
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	f.call(1, "POST", reply, fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":%q}`, copy.ETag, rid), 200)
	if len(x.list(t, 0)) != 0 {
		t.Fatal("same-status RSVP did not withdraw proposal")
	}
	x.propose(t, 1, rid, "2026-10-28T12:00:00Z", "2026-10-28T13:00:00Z", 200)
	p = x.proposal(t, rid)
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	f.call(0, "POST", "/api/v1/calendar/events/"+source.ID+"/occurrences", fmt.Sprintf(`{"action":"cancel","etag":%q,"recurrence_id":%q}`, source.ETag, rid), 200)
	if len(x.list(t, 0)) != 0 {
		t.Fatal("cancelled proposal visible")
	}
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	p["etag"] = source.ETag
	f.call(0, "POST", decision, counterDecision(t, p, "accept"), 409)
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	f.call(1, "POST", reply, fmt.Sprintf(`{"action":"counter","etag":%q,"recurrence_id":%q,"start":"2026-10-28T12:00:00Z","end":"2026-10-28T13:00:00Z"}`, copy.ETag, rid), 409)
}
