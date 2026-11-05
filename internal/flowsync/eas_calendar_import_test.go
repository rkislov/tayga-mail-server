package flowsync

import (
	"errors"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestEASImportExternalMeetingIsPrivateAndIdempotent(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	peer, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "invitee", Email: "invitee@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureDAVDefaults(ctx, peer.ID); err != nil {
		t.Fatal(err)
	}
	peerCal, _ := st.GetCalendarByName(ctx, peer.ID, "default")
	fields := `<ClientUid>forwarded-external</ClientUid><Subject>Imported external meeting</Subject><StartTime>20261106T060000Z</StartTime><EndTime>20261106T070000Z</EndTime><OrganizerEmail>organizer@outside.example</OrganizerEmail><OrganizerName>External organizer</OrganizerName><Attendees><Attendee><Email>invitee@example.com</Email><AttendeeStatus>3</AttendeeStatus></Attendee><Attendee><Email>other@outside.example</Email><AttendeeStatus>4</AttendeeStatus></Attendee></Attendees>`
	op := calendarOp(t, "add", "", fields)
	op.ClientID = "import-1"
	id, err := h.wbCalendar(ctx, u, cal.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := calutil.ParseICS(obj.Data)
	if err != nil || p.Organizer.Email != "organizer@outside.example" || p.Organizer.Name != "External organizer" || len(p.Attendees) != 2 || p.Attendees[0].PartStat != "ACCEPTED" || p.Attendees[1].PartStat != "DECLINED" {
		t.Fatal("imported identities/statuses changed", err, obj.Data)
	}
	op.ClientID = "import-retry"
	if retry, e := h.wbCalendar(ctx, u, cal.ID, op); e != nil || retry != id {
		t.Fatal("import duplicated", retry, e)
	}
	if objects, e := st.ListCalendarObjects(ctx, peerCal.ID); e != nil || len(objects) != 0 {
		t.Fatal("import created another participant copy", e)
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("import sent invitations")
	}
	if _, e := h.wbCalendar(ctx, u, peerCal.ID, calendarOp(t, "add", "", strings.Replace(fields, "forwarded-external", "foreign-calendar", 1))); !errors.Is(e, storage.ErrUnauthorized) {
		t.Fatal("foreign calendar accepted import", e)
	}
	for _, changed := range []string{`<OrganizerEmail>other-organizer@outside.example</OrganizerEmail>`, `<Attendees><Attendee><Email>extra@outside.example</Email></Attendee></Attendees>`} {
		if _, e := h.wbCalendar(ctx, u, cal.ID, calendarOp(t, "change", id, changed)); !errors.Is(e, storage.ErrUnauthorized) {
			t.Fatal("received identity could be changed", e)
		}
	}
	if _, e := h.wbCalendar(ctx, u, cal.ID, calendarOp(t, "delete", id, "")); e != nil {
		t.Fatal(e)
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("private deletion sent cancellation")
	}
}

func TestEASChangeCannotAssignExternalOrganizer(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	for _, organizer := range []string{"", `<OrganizerEmail>` + u.Email + `</OrganizerEmail>`} {
		id, err := h.wbCalendar(ctx, u, cal.ID, calendarOp(t, "add", "", `<StartTime>20261106T060000Z</StartTime><EndTime>20261106T070000Z</EndTime>`+organizer))
		if err != nil {
			t.Fatal(err)
		}
		before, _ := st.GetCalendarObjectByID(ctx, id)
		if _, err = h.wbCalendar(ctx, u, cal.ID, calendarOp(t, "change", id, `<OrganizerEmail>foreign@outside.example</OrganizerEmail>`)); !errors.Is(err, storage.ErrUnauthorized) {
			t.Fatal("changed organizer of existing event", err)
		}
		after, _ := st.GetCalendarObjectByID(ctx, id)
		if after.Data != before.Data || after.ETag != before.ETag {
			t.Fatal("rejected change modified event")
		}
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("rejected change sent mail")
	}
}
