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

// Includes ctags and outbox rows: rollback must be invisible to sync clients
// and must not leave a scheduling message for a transaction that failed.
func calendarSnapshot(t *testing.T, f *fileFixture) string {
	t.Helper()
	state := map[string][][]any{}
	for _, table := range []string{"calendars", "calendar_objects", "calendar_invites", "calendar_deliveries"} {
		rows, err := f.s.store.(*storage.Store).DB().Query(`SELECT * FROM ` + table + ` ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			values, dest := make([]any, len(cols)), make([]any, len(cols))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				t.Fatal(err)
			}
			state[table] = append(state[table], values)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func meetingPayload(id, etag, title, attendees string) string {
	return fmt.Sprintf(`{"id":%q,"etag":%q,"uid":"atomic","summary":%q,"start":"2026-10-19T12:00:00Z","end":"2026-10-19T13:00:00Z","attendees":%s}`, id, etag, title, attendees)
}

func TestCalendarMeetingAtomicCreateEditRemoveAndDelete(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	st := f.s.store
	db := st.(*storage.Store).DB()
	cal, err := st.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := st.GetUserByID(ctx, f.users[0])
	_, err = st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "room", DisplayName: "Room", AutoAccept: true})
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/calendar/calendars/" + cal.ID + "/events"
	attendees := `[{"email":"reader@example.com"},{"email":"editor@example.com"},{"email":"room@example.com"},{"email":"outside@remote.example"}]`
	create := meetingPayload("", "", "First", attendees)
	// A failure on the last recipient rolls back source, room and earlier copies.
	for _, trigger := range []string{
		`CREATE TRIGGER fail_meeting BEFORE INSERT ON calendar_objects WHEN NEW.calendar_id IN (SELECT id FROM calendars WHERE user_id='` + f.users[1] + `') BEGIN SELECT RAISE(ABORT,'copy failure'); END`,
		`CREATE TRIGGER fail_meeting BEFORE INSERT ON calendar_invites WHEN NEW.attendee_email='reader@example.com' BEGIN SELECT RAISE(ABORT,'invite failure'); END`,
		`CREATE TRIGGER fail_meeting BEFORE INSERT ON calendar_deliveries WHEN NEW.recipient='reader@example.com' BEGIN SELECT RAISE(ABORT,'queue failure'); END`,
	} {
		before := calendarSnapshot(t, f)
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		f.call(0, "POST", path, create, 500)
		if after := calendarSnapshot(t, f); after != before {
			t.Fatal("partial create")
		}
		if _, err := db.Exec(`DROP TRIGGER fail_meeting`); err != nil {
			t.Fatal(err)
		}
	}
	w := f.call(0, "POST", path, create, 200)
	var event struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	invites, _ := st.ListCalendarInvitesForUser(ctx, f.users[1], false)
	copy, _ := st.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	copy.Data = strings.Replace(copy.Data, "END:VEVENT", "CATEGORIES:Private\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT7M\r\nDESCRIPTION:Mine\r\nEND:VALARM\r\nEND:VEVENT", 1)
	if _, err := st.UpsertCalendarObject(ctx, copy); err != nil {
		t.Fatal(err)
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"counter","start":"2026-10-19T14:00:00Z","end":"2026-10-19T15:00:00Z"}`, 200)
	source, _ := st.GetCalendarObjectByID(ctx, event.ID)
	edit := meetingPayload(event.ID, source.ETag, "Updated", attendees)
	for _, trigger := range []string{
		`CREATE TRIGGER fail_meeting BEFORE UPDATE ON calendar_objects WHEN OLD.id='` + copy.ID + `' BEGIN SELECT RAISE(ABORT,'copy failure'); END`,
		`CREATE TRIGGER fail_meeting BEFORE UPDATE ON calendar_invites WHEN OLD.attendee_email='reader@example.com' BEGIN SELECT RAISE(ABORT,'invite failure'); END`,
		`CREATE TRIGGER fail_meeting BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'queue failure'); END`,
	} {
		before := calendarSnapshot(t, f)
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		f.call(0, "PUT", path, edit, 500)
		if after := calendarSnapshot(t, f); after != before {
			t.Fatal("partial edit")
		}
		if _, err := db.Exec(`DROP TRIGGER fail_meeting`); err != nil {
			t.Fatal(err)
		}
	}
	f.call(0, "PUT", path, edit, 200)
	before := calendarSnapshot(t, f)
	f.call(0, "PUT", path, edit, 409)
	if calendarSnapshot(t, f) != before {
		t.Fatal("stale write changed state")
	}
	copy, _ = st.GetCalendarObjectByID(ctx, copy.ID)
	if !strings.Contains(copy.Data, "TRIGGER:-PT7M") || !strings.Contains(copy.Data, "CATEGORIES:Private") || !strings.Contains(copy.Data, "SUMMARY:Updated") {
		t.Fatal(copy.Data)
	}
	inv, _ := st.GetCalendarInvite(ctx, invites[0].ID)
	if inv.ProposedStart != nil || inv.PartStat != "NEEDS-ACTION" {
		t.Fatal("stale RSVP")
	}
	// Exclusion cancels the old personal copy; adding the same participant back
	// reuses it and its reminders rather than making a second event.
	removed := `[{"email":"editor@example.com"},{"email":"room@example.com"}]`
	source, _ = st.GetCalendarObjectByID(ctx, event.ID)
	f.call(0, "PUT", path, meetingPayload(event.ID, source.ETag, "Removed", removed), 200)
	copy, _ = st.GetCalendarObjectByID(ctx, copy.ID)
	p, _ := calutil.ParseICS(copy.Data)
	if p.Status != "CANCELLED" {
		t.Fatal("removed copy not cancelled")
	}
	f.call(1, "POST", "/api/v1/calendar/invites/"+inv.ID+"/reply", `{"action":"accept"}`, 409)
	source, _ = st.GetCalendarObjectByID(ctx, event.ID)
	f.call(0, "PUT", path, meetingPayload(event.ID, source.ETag, "Restored", attendees), 200)
	inv, _ = st.GetCalendarInvite(ctx, inv.ID)
	if inv.CalendarObjectID != copy.ID {
		t.Fatal("copy replaced")
	}
	copy, _ = st.GetCalendarObjectByID(ctx, copy.ID)
	if !strings.Contains(copy.Data, "TRIGGER:-PT7M") || strings.Contains(copy.Data, "STATUS:CANCELLED") {
		t.Fatal(copy.Data)
	}
	before = calendarSnapshot(t, f)
	if _, err := db.Exec(`CREATE TRIGGER fail_meeting BEFORE UPDATE ON calendar_objects WHEN OLD.id='` + copy.ID + `' BEGIN SELECT RAISE(ABORT,'cancel failure'); END`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "DELETE", "/api/v1/calendar/events/"+event.ID, "", 500)
	if calendarSnapshot(t, f) != before {
		t.Fatal("partial deletion")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_meeting`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "DELETE", "/api/v1/calendar/events/"+event.ID, "", 200)
	copy, _ = st.GetCalendarObjectByID(ctx, copy.ID)
	p, _ = calutil.ParseICS(copy.Data)
	if p.Status != "CANCELLED" {
		t.Fatal("deletion did not cancel recipient")
	}
}

func TestCalendarProposalRollsBackOnCopyFailure(t *testing.T) {
	f, id := queueCalendarFixture(t, "reader@example.com")
	ctx := context.Background()
	invites, _ := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	inv := invites[0]
	f.call(1, "POST", "/api/v1/calendar/invites/"+inv.ID+"/reply", `{"action":"counter","start":"2026-10-19T14:00:00Z","end":"2026-10-19T15:00:00Z"}`, 200)
	inv, _ = f.s.store.GetCalendarInvite(ctx, inv.ID)
	source, _ := f.s.store.GetCalendarObjectByID(ctx, id)
	body, _ := json.Marshal(map[string]string{"action": "accept", "event_id": id, "etag": source.ETag, "revision": inv.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	db := f.s.store.(*storage.Store).DB()
	before := calendarSnapshot(t, f)
	if _, err := db.Exec(`CREATE TRIGGER fail_copy BEFORE UPDATE ON calendar_objects WHEN OLD.id='` + inv.CalendarObjectID + `' BEGIN SELECT RAISE(ABORT,'copy failure'); END`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/calendar/invites/"+inv.ID+"/decision", string(body), 500)
	if calendarSnapshot(t, f) != before {
		t.Fatal("partial proposal decision")
	}
}
