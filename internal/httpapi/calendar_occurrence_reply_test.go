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

func TestCalendarOccurrenceRepliesIsolateDatesAndFreeBusy(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	cal, _ := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	created := f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"summary":"Weekly","start":"2026-10-19T08:00:00Z","end":"2026-10-19T09:00:00Z","timezone":"Europe/Berlin","recurrence":{"frequency":"WEEKLY","interval":1,"count":4},"attendees":[{"email":"reader@example.com"},{"email":"editor@example.com"}]}`, 200)
	var result struct{ ID string }
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	invites, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	inv := invites[0]
	path := "/api/v1/calendar/invites/" + inv.ID + "/reply"
	f.call(1, "POST", path, `{"action":"accept"}`, 200)
	inv, _ = f.s.store.GetCalendarInvite(ctx, inv.ID)
	original, _ := f.s.store.GetCalendarObjectByID(ctx, result.ID)
	// Move the second occurrence across a day while retaining its DST identity.
	rid := "2026-10-26T09:00:00Z"
	f.call(0, "POST", "/api/v1/calendar/events/"+original.ID+"/occurrences", fmt.Sprintf(`{"action":"update","etag":%q,"recurrence_id":%q,"start":"2026-10-27T09:00:00Z","end":"2026-10-27T10:00:00Z"}`, original.ETag, rid), 200)
	copy, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	copy.Data = strings.Replace(copy.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Private\r\nTRIGGER:-PT5M\r\nEND:VALARM\r\nEND:VEVENT", 1)
	copy, _ = f.s.store.UpsertCalendarObject(ctx, copy)
	readPath := "/api/v1/calendar/events/" + copy.ID + "?recurrence_id=" + rid
	response := f.call(1, "GET", readPath, "", 200)
	if !strings.Contains(response.Body.String(), `"occurrence_invite_id":"`+inv.ID+`"`) || !strings.Contains(response.Body.String(), `"occurrence_partstat":"ACCEPTED"`) {
		t.Fatal(response.Body.String())
	}
	f.call(1, "POST", path, `{"action":"counter","start":"2026-10-19T12:00:00Z","end":"2026-10-19T13:00:00Z"}`, 200)
	inv, _ = f.s.store.GetCalendarInvite(ctx, inv.ID)
	// Another account cannot reply using this participant's invitation.
	f.call(2, "POST", path, fmt.Sprintf(`{"action":"decline","etag":%q,"recurrence_id":%q}`, copy.ETag, rid), 403)
	stale := copy.ETag
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, step := range []struct {
		action, status string
		busy           int
	}{{"decline", "DECLINED", 3}, {"tentative", "TENTATIVE", 4}, {"accept", "ACCEPTED", 4}} {
		copy, _ = f.s.store.GetCalendarObjectByID(ctx, copy.ID)
		body := fmt.Sprintf(`{"action":%q,"etag":%q,"recurrence_id":%q}`, step.action, copy.ETag, rid)
		got := f.call(1, "POST", path, body, 200)
		if !strings.Contains(got.Body.String(), `"partstat":"`+step.status+`"`) || !strings.Contains(got.Body.String(), `"series_partstat":"ACCEPTED"`) {
			t.Fatal(got.Body.String())
		}
		if step.action == "decline" {
			f.call(1, "POST", path, fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":%q}`, stale, rid), 409)
		}
		for _, id := range []string{original.ID, copy.ID} {
			o, _ := f.s.store.GetCalendarObjectByID(ctx, id)
			master, _ := calutil.ParseICS(o.Data)
			instance, err := calutil.GetOccurrence(o.Data, rid)
			if err != nil || instance.Start.UTC().Format(time.RFC3339) != "2026-10-27T09:00:00Z" || instance.Sequence != 1 || master.Sequence != 0 {
				t.Fatalf("changed event: %+v %v", instance, err)
			}
			for _, attendee := range master.Attendees {
				if attendee.Email == "reader@example.com" && attendee.PartStat != "ACCEPTED" {
					t.Fatal("changed series RSVP")
				}
			}
			for _, attendee := range instance.Attendees {
				if attendee.Email == "reader@example.com" && attendee.PartStat != step.status {
					t.Fatal("missing occurrence RSVP")
				}
				if attendee.Email == "editor@example.com" && attendee.PartStat != "NEEDS-ACTION" {
					t.Fatal("changed another attendee")
				}
			}
			other, err := calutil.GetOccurrence(o.Data, "2026-11-02T09:00:00Z")
			if err != nil || other.Attendees[0].PartStat != "ACCEPTED" {
				t.Fatalf("changed another date: %+v %v", other, err)
			}
			if id == copy.ID && !strings.Contains(o.Data, "DESCRIPTION:Private") {
				t.Fatal("lost private reminder")
			}
		}
		for _, user := range []struct {
			id   string
			want int
		}{{f.users[1], step.busy}, {f.users[0], 4}, {f.users[2], 4}} {
			busy, err := f.s.store.FreeBusyForUser(ctx, user.id, from, from.AddDate(0, 2, 0))
			if err != nil || len(busy) != user.want {
				t.Fatalf("busy %s: %d want %d %v", user.id, len(busy), user.want, err)
			}
		}
		unchanged, _ := f.s.store.GetCalendarInvite(ctx, inv.ID)
		if unchanged.PartStat != inv.PartStat || !unchanged.UpdatedAt.Equal(inv.UpdatedAt) || unchanged.ProposedStart == nil || !unchanged.ProposedStart.Equal(*inv.ProposedStart) {
			t.Fatal("changed series invitation")
		}
		// A repeated confirmed response changes neither objects nor outbox.
		copy, _ = f.s.store.GetCalendarObjectByID(ctx, copy.ID)
		db := f.s.store.(*storage.Store).DB()
		var before, after int
		db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&before)
		f.call(1, "POST", path, fmt.Sprintf(`{"action":%q,"etag":%q,"recurrence_id":%q}`, step.action, copy.ETag, rid), 200)
		db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&after)
		check, _ := f.s.store.GetCalendarObjectByID(ctx, copy.ID)
		if after != before || check.ETag != copy.ETag {
			t.Fatal("duplicate response")
		}
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mailbox, _ := f.s.store.GetMailbox(ctx, f.users[0], "INBOX")
	messages, _ := f.s.store.ListMessages(ctx, mailbox.ID)
	instanceReplies := 0
	for _, message := range messages {
		raw, err := f.s.ms.Read(message.FilePath)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "RECURRENCE-ID") {
			continue
		}
		instanceReplies++
		if !strings.Contains(text, "METHOD:REPLY") || strings.Count(text, "BEGIN:VEVENT") != 1 || strings.Contains(text, "RRULE:") || strings.Contains(text, "BEGIN:VALARM") || !strings.Contains(text, "SEQUENCE:1") || !strings.Contains(text, "RECURRENCE-ID;TZID=Europe/Berlin:20261026T100000") {
			t.Fatalf("wrong reply payload: %s", text)
		}
	}
	if instanceReplies != 3 {
		t.Fatalf("reply mail count %d", instanceReplies)
	}
}

func TestCalendarOccurrenceReplyRejectsInvalidAndRollsBack(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	cal, _ := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"summary":"Series","start":"2026-10-19T00:00:00Z","end":"2026-10-20T00:00:00Z","all_day":true,"recurrence":{"frequency":"DAILY","interval":1,"count":3},"attendees":[{"email":"reader@example.com"}]}`, 200)
	invites, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	inv := invites[0]
	objects, _ := f.s.store.ListCalendarObjects(ctx, cal.ID)
	source := objects[0]
	copy, _ := f.s.store.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	path := "/api/v1/calendar/invites/" + inv.ID + "/reply"
	for _, tail := range []string{`"recurrence_id":""`, `"recurrence_id":null`, `"recurrence_id":12`, `"recurrence_id":"2026-10-20","action":"counter"`} {
		f.call(1, "POST", path, fmt.Sprintf(`{"action":"accept","etag":%q,%s}`, copy.ETag, tail), 400)
	}
	f.call(1, "POST", path, `{"action":"accept","recurrence_id":"2026-10-20"}`, 400)
	for _, rid := range []string{"2026-10-25", "not-a-date", "2026-10-20T00:00:00Z"} {
		f.call(1, "POST", path, fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":%q}`, copy.ETag, rid), 409)
	}
	body := fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":"2026-10-20"}`, copy.ETag)
	db := f.s.store.(*storage.Store).DB()
	for _, trigger := range []string{
		fmt.Sprintf(`CREATE TRIGGER fail_occ_reply BEFORE UPDATE ON calendar_objects WHEN OLD.id='%s' BEGIN SELECT RAISE(ABORT,'injected'); END`, source.ID),
		`CREATE TRIGGER fail_occ_reply BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		f.call(1, "POST", path, body, 500)
		for _, before := range []*storage.CalendarObject{source, copy} {
			after, _ := f.s.store.GetCalendarObjectByID(ctx, before.ID)
			if after.Data != before.Data || after.ETag != before.ETag {
				t.Fatal("partial response committed")
			}
		}
		if _, err := db.Exec(`DROP TRIGGER fail_occ_reply`); err != nil {
			t.Fatal(err)
		}
	}
	f.call(1, "POST", path, body, 200)
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, copy.ID)
	instance, err := calutil.GetOccurrence(copy.Data, "2026-10-20")
	if err != nil || !instance.AllDay || instance.Attendees[0].PartStat != "ACCEPTED" {
		t.Fatalf("all-day reply: %+v %v", instance, err)
	}
	// Accepting one date of a declined series must still make that date busy.
	f.call(1, "POST", path, `{"action":"decline"}`, 200)
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, copy.ID)
	f.call(1, "POST", path, fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":"2026-10-20"}`, copy.ETag), 200)
	from := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	busy, err := f.s.store.FreeBusyForUser(ctx, f.users[1], from, from.AddDate(0, 0, 4))
	if err != nil || len(busy) != 1 || !busy[0].Start.Equal(from.AddDate(0, 0, 1)) {
		t.Fatalf("accepted exception missing: %+v %v", busy, err)
	}
	// A cancelled occurrence cannot be accepted, even with the latest ETag.
	source, _ = f.s.store.GetCalendarObjectByID(ctx, source.ID)
	f.call(0, "POST", "/api/v1/calendar/events/"+source.ID+"/occurrences", fmt.Sprintf(`{"action":"cancel","etag":%q,"recurrence_id":"2026-10-20"}`, source.ETag), 200)
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, copy.ID)
	f.call(1, "POST", path, fmt.Sprintf(`{"action":"accept","etag":%q,"recurrence_id":"2026-10-20"}`, copy.ETag), 409)
}
