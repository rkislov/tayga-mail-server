package flowsync

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestMeetingResponseInstanceValidationAndIsolation(t *testing.T) {
	for _, version := range []string{"14.1", "16.0", "16.1"} {
		t.Run(version, func(t *testing.T) {
			f := newMeetingFixture(t)
			for _, entry := range []struct{ value, want string }{{"20261029T170000Z", "104"}, {"", "104"}, {"2026-10-29T17:00:00.000Z", "146"}} {
				got := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", version, "<InstanceId>"+entry.value+"</InstanceId>")
				if got.value("Status") != entry.want {
					t.Fatal(got.render())
				}
			}
			for _, object := range []*storage.CalendarObject{f.original, f.received} {
				object.Data = strings.Replace(object.Data, "END:VEVENT", "RRULE:FREQ=WEEKLY;COUNT=3\r\nEND:VEVENT", 1)
				if _, err := f.st.UpsertCalendarObject(f.ctx, object); err != nil {
					t.Fatal(err)
				}
			}
			extra := "<InstanceId>2026-10-29T17:00:00.000Z</InstanceId>"
			if version != "14.1" {
				extra += "<SendResponse/>"
			}
			got := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "3", version, extra)
			if got.value("Status") != "1" || got.child("CalendarId") != nil {
				t.Fatal(got.render())
			}
			if version == "14.1" && got.child("InstanceId") != nil || version != "14.1" && got.value("InstanceId") != "2026-10-29T17:00:00.000Z" {
				t.Fatal("wrong instance echo", got.render())
			}
			for _, object := range []*storage.CalendarObject{f.original, f.received} {
				saved, _ := f.st.GetCalendarObjectByID(f.ctx, object.ID)
				p, _ := calutil.ParseICS(saved.Data)
				instance, err := calutil.GetOccurrence(saved.Data, "2026-10-29T17:00:00Z")
				if err != nil || instance.Attendees[0].PartStat != "DECLINED" || p.Attendees[0].PartStat != "NEEDS-ACTION" {
					t.Fatalf("wrong RSVP: %+v %v", instance, err)
				}
			}
			inv, _ := f.st.GetCalendarInvite(f.ctx, f.invite.ID)
			if inv.PartStat != "NEEDS-ACTION" {
				t.Fatal("changed series invitation")
			}
			jobs := drainCalendarJobs(t, f.ctx, f.st)
			want := 1
			if version == "14.1" {
				want = 0
			}
			if len(jobs) != want {
				t.Fatalf("mail count %d", len(jobs))
			}
			if want > 0 && (!bytes.Contains(jobs[0].Data, []byte("RECURRENCE-ID:20261029T170000Z")) || bytes.Contains(jobs[0].Data, []byte("RRULE:"))) {
				t.Fatal("not an instance reply")
			}
			f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "3", version, extra)
			if len(drainCalendarJobs(t, f.ctx, f.st)) != 0 {
				t.Fatal("duplicate instance reply")
			}
			from := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
			busy, err := f.st.FreeBusyForUser(f.ctx, f.guest.ID, from, from.AddDate(0, 0, 21))
			if err != nil || len(busy) != 2 {
				t.Fatalf("decline removes only selected busy date: %v %v", busy, err)
			}
			for _, status := range []string{"2", "1"} {
				got = f.respond(t, f.guest, f.received.CalendarID, f.received.ID, status, version, extra)
				if got.value("Status") != "1" || got.value("CalendarId") != f.received.ID {
					t.Fatal(got.render())
				}
			}
		})
	}
}
