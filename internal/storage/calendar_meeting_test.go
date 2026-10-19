package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/tayga/tms/internal/calutil"
)

func meetingStorageSnapshot(t *testing.T, s *Store) string {
	t.Helper()
	state := map[string][][]any{}
	for _, table := range []string{"calendars", "calendar_objects", "calendar_invites", "calendar_deliveries"} {
		rows, err := s.db.Query(`SELECT * FROM ` + table + ` ORDER BY id`)
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

func TestMeetingRollbackCopiesInvitesAndQueue(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := meetingWithParticipant(t, f, u.Email)
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	copyID := invites[0].CalendarObjectID
	for _, failure := range []struct{ table, operation, condition string }{
		{"calendar_objects", "UPDATE", `OLD.id='` + copyID + `'`},
		{"calendar_invites", "UPDATE", `OLD.attendee_email='` + u.Email + `'`},
		{"calendar_deliveries", "INSERT", ""},
	} {
		before := meetingStorageSnapshot(t, f.s)
		remove := failCalendarWrite(t, f.s, failure.table, failure.operation, failure.condition)
		if _, err := f.s.SaveCalendarMeeting(ctx, f.user.ID, changedMeeting(t, o, "Should rollback")); err == nil {
			t.Fatal("failure not injected")
		}
		if meetingStorageSnapshot(t, f.s) != before {
			t.Fatal("partial edit", failure.table)
		}
		if err := f.s.DeleteCalendarMeeting(ctx, f.user.ID, o); err == nil {
			t.Fatal("delete failure not injected")
		}
		if meetingStorageSnapshot(t, f.s) != before {
			t.Fatal("partial cancellation", failure.table)
		}
		remove()
	}
}

func TestMeetingNewerDetachedCopyRejectsUpdate(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := meetingWithParticipant(t, f, u.Email)
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	instance := "BEGIN:VEVENT\r\nUID:meeting\r\nRECURRENCE-ID:20261026T100000Z\r\nDTSTART:20261026T110000Z\r\nDTEND:20261026T120000Z\r\nSEQUENCE:99\r\nORGANIZER:mailto:owner@example.com\r\nEND:VEVENT\r\n"
	copy.Data = strings.Replace(copy.Data, "END:VCALENDAR", instance+"END:VCALENDAR", 1)
	if _, err := f.s.UpsertCalendarObject(ctx, copy); err != nil {
		t.Fatal(err)
	}
	before := meetingStorageSnapshot(t, f.s)
	if _, err := f.s.SaveCalendarMeeting(ctx, f.user.ID, changedMeeting(t, o, "Stale update")); err != ErrCalendarConflict {
		t.Fatal(err)
	}
	if meetingStorageSnapshot(t, f.s) != before {
		t.Fatal("newer exception overwritten")
	}
}

func meetingUser(t *testing.T, f bookingFixture, local string) *User {
	t.Helper()
	u, err := f.s.CreateUser(context.Background(), &User{TenantID: f.user.TenantID, DomainID: f.user.DomainID, Email: local + "@example.com", LocalPart: local, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func meetingWithParticipant(t *testing.T, f bookingFixture, email string) *CalendarObject {
	t.Helper()
	o := f.event(t, "meeting", 19, 3)
	p, _ := calutil.ParseICS(o.Data)
	p.Attendees = append(p.Attendees, calutil.Attendee{Email: email, PartStat: "NEEDS-ACTION"})
	var err error
	o.Data, err = calutil.BuildRequestICS(p.Event)
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.s.SaveCalendarMeeting(context.Background(), f.user.ID, o)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func changedMeeting(t *testing.T, o *CalendarObject, summary string) *CalendarObject {
	t.Helper()
	copy := *o
	p, err := calutil.ParseICS(copy.Data)
	if err != nil {
		t.Fatal(err)
	}
	p.Summary, p.Sequence = summary, calutil.NextSequence(copy.Data)
	for i := range p.Attendees {
		p.Attendees[i].PartStat = "NEEDS-ACTION"
		p.Attendees[i].RSVP = true
	}
	copy.Data, err = calutil.BuildRequestICS(p.Event)
	if err != nil {
		t.Fatal(err)
	}
	copy.ExpectedETag = o.ETag
	return &copy
}

func TestMeetingConcurrentEditsAcrossConnections(t *testing.T) {
	testConcurrentMeetingEdits(t, false)
}

func TestCalendarClientConcurrentEditsAcrossConnections(t *testing.T) {
	testConcurrentMeetingEdits(t, true)
}

func testConcurrentMeetingEdits(t *testing.T, client bool) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := meetingWithParticipant(t, f, u.Email)
	other, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var wg sync.WaitGroup
	gate, outcomes := make(chan struct{}), make(chan error, 8)
	for i := 0; i < 8; i++ {
		db := f.s
		if i%2 == 1 {
			db = other.(*Store)
		}
		edit := changedMeeting(t, o, fmt.Sprintf("Update %d", i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			save := db.SaveCalendarMeeting
			if client {
				save = db.SaveCalendarClientObject
			}
			_, err := save(ctx, f.user.ID, edit)
			outcomes <- err
		}()
	}
	close(gate)
	wg.Wait()
	close(outcomes)
	winners := 0
	for err := range outcomes {
		if err == nil {
			winners++
		} else if err != ErrCalendarConflict {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("committed %d edits", winners)
	}
	current, _ := f.s.GetCalendarObjectByID(ctx, o.ID)
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	a, _ := calutil.ParseICS(current.Data)
	b, _ := calutil.ParseICS(copy.Data)
	if a.Sequence != 1 || b.Sequence != 1 || a.Summary != b.Summary {
		t.Fatal("copies diverged")
	}
	var queued int
	if err := f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&queued); err != nil || queued != 4 {
		t.Fatal("duplicate/lost mail", queued, err)
	}
}

func TestMeetingReusesMovedCopyAndProtectsReassignedAlias(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	v := meetingUser(t, f, "another")
	alias, err := f.s.CreateAlias(ctx, u.DomainID, u.ID, "alias")
	if err != nil {
		t.Fatal(err)
	}
	o := meetingWithParticipant(t, f, "alias@example.com")
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	inv := invites[0]
	copy, _ := f.s.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
	work, err := f.s.EnsureCalendar(ctx, u.ID, "work", "Work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.Exec(f.s.rebind(`UPDATE calendar_objects SET calendar_id=?,href_name='moved.ics' WHERE id=?`), work.ID, copy.ID); err != nil {
		t.Fatal(err)
	}
	copy, _ = f.s.GetCalendarObjectByID(ctx, copy.ID)
	copy.Data = strings.Replace(copy.Data, "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Personal\r\nTRIGGER:-PT3M\r\nEND:VALARM\r\nEND:VEVENT", 1)
	if _, err = f.s.UpsertCalendarObject(ctx, copy); err != nil {
		t.Fatal(err)
	}
	o, err = f.s.SaveCalendarMeeting(ctx, f.user.ID, changedMeeting(t, o, "Moved copy kept"))
	if err != nil {
		t.Fatal(err)
	}
	inv, _ = f.s.GetCalendarInvite(ctx, inv.ID)
	copy, _ = f.s.GetCalendarObjectByID(ctx, copy.ID)
	if inv.CalendarObjectID != copy.ID || copy.CalendarID != work.ID || copy.HrefName != "moved.ics" || !strings.Contains(copy.Data, "TRIGGER:-PT3M") {
		t.Fatal("moved copy lost")
	}
	// Reassigning an address cannot silently relink the old participant's
	// invitation to another account or overwrite the old account's data.
	if _, err = f.s.db.Exec(f.s.rebind(`UPDATE aliases SET user_id=? WHERE id=?`), v.ID, alias.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveCalendarMeeting(ctx, f.user.ID, changedMeeting(t, o, "Reassigned")); err != ErrCalendarConflict {
		t.Fatal(err)
	}
	unchanged, _ := f.s.GetCalendarObjectByID(ctx, o.ID)
	if unchanged.ETag != o.ETag {
		t.Fatal("identity conflict changed source")
	}
}

func TestMeetingAliasRemovalKeepsRemainingAddress(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	if _, err := f.s.CreateAlias(ctx, u.DomainID, u.ID, "aaa"); err != nil {
		t.Fatal(err)
	}
	o := meetingWithParticipant(t, f, "aaa@example.com")
	edit := changedMeeting(t, o, "Two addresses")
	p, _ := calutil.ParseICS(edit.Data)
	p.Attendees = append(p.Attendees, calutil.Attendee{Email: u.Email, PartStat: "NEEDS-ACTION"})
	edit.Data, _ = calutil.BuildRequestICS(p.Event)
	var err error
	o, err = f.s.SaveCalendarMeeting(ctx, f.user.ID, edit)
	if err != nil {
		t.Fatal(err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	if len(invites) != 2 || invites[0].CalendarObjectID != invites[1].CalendarObjectID {
		t.Fatal("alias duplicated copy")
	}
	copyID := invites[0].CalendarObjectID
	edit = changedMeeting(t, o, "One address")
	p, _ = calutil.ParseICS(edit.Data)
	kept := p.Attendees[:0]
	for _, a := range p.Attendees {
		if a.Email != "aaa@example.com" {
			kept = append(kept, a)
		}
	}
	p.Attendees = kept
	edit.Data, _ = calutil.BuildRequestICS(p.Event)
	if _, err = f.s.SaveCalendarMeeting(ctx, f.user.ID, edit); err != nil {
		t.Fatal(err)
	}
	copy, _ := f.s.GetCalendarObjectByID(ctx, copyID)
	p, _ = calutil.ParseICS(copy.Data)
	if p.Status == "CANCELLED" || strings.Contains(copy.Data, "aaa@example.com") {
		t.Fatal("remaining address cancelled")
	}
	invites, _ = f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	for _, inv := range invites {
		if inv.CalendarObjectID != copyID {
			t.Fatal("alias reference changed")
		}
		if inv.AttendeeEmail == "aaa@example.com" && inv.PartStat != "DECLINED" {
			t.Fatal("removed alias still active")
		}
	}
}

func TestMeetingActorACLAndRecipientDeletion(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	u := meetingUser(t, f, "participant")
	o := meetingWithParticipant(t, f, u.Email)
	if _, err := f.s.SaveCalendarMeeting(ctx, u.ID, changedMeeting(t, o, "Not organizer")); err != ErrUnauthorized {
		t.Fatal(err)
	}
	invites, _ := f.s.ListCalendarInvitesForUser(ctx, u.ID, false)
	copy, _ := f.s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	// Sharing this participant calendar with the organizer does not turn the
	// recipient's deletion into organizer cancellation of the room booking.
	if _, err := f.s.db.Exec(f.s.rebind(`INSERT INTO calendar_acl(calendar_id,grantee_user_id,rights) VALUES(?,?,'write')`), copy.CalendarID, f.user.ID); err != nil {
		t.Fatal(err)
	}
	var before, after int
	if err := f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := f.s.DeleteCalendarMeeting(ctx, u.ID, copy); err != nil {
		t.Fatal(err)
	}
	current, _ := f.s.GetCalendarObjectByID(ctx, o.ID)
	if current.ETag != o.ETag {
		t.Fatal("recipient deletion changed organizer")
	}
	roomInvites, err := f.s.ListCalendarInvitesForUser(ctx, f.room.UserID, false)
	if err != nil || len(roomInvites) != 1 {
		t.Fatal("recipient deletion released booking", err)
	}
	if err := f.s.db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&after); err != nil || after != before {
		t.Fatal("recipient deletion cancelled meeting", err)
	}
	// Revoking a shared writer is checked inside storage, independently of HTTP.
	shared, err := f.s.EnsureCalendar(ctx, u.ID, "work", "Work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.Exec(f.s.rebind(`INSERT INTO calendar_acl(calendar_id,grantee_user_id,rights) VALUES(?,?,'write')`), shared.ID, f.user.ID); err != nil {
		t.Fatal(err)
	}
	candidate := f.event(t, "shared", 26, 0)
	candidate.CalendarID = shared.ID
	if _, err = f.s.db.Exec(f.s.rebind(`UPDATE calendar_acl SET rights='read' WHERE calendar_id=?`), shared.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveCalendarMeeting(ctx, f.user.ID, candidate); err != ErrUnauthorized {
		t.Fatal(err)
	}
}
