// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func calendarOp(t *testing.T, kind, id, fields string) clientOp {
	t.Helper()
	doc, err := parseProtocolXML([]byte(`<ApplicationData xmlns="Calendar:">` + fields + `</ApplicationData>`))
	if err != nil {
		t.Fatal(err)
	}
	return clientOp{Kind: kind, ServerID: id, Fields: extractAppDataFields(fields), ApplicationData: doc.child("ApplicationData")}
}
func drainCalendarJobs(t *testing.T, ctx context.Context, st storage.Driver) []*storage.CalendarDelivery {
	t.Helper()
	var jobs []*storage.CalendarDelivery
	for {
		j, err := st.ClaimCalendarDelivery(ctx)
		if errors.Is(err, storage.ErrNotFound) {
			return jobs
		}
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, j)
	}
}
func TestEASCalendarAttendeeEditSchedulesInvitationsAndPreservesEvent(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	peer, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureDAVDefaults(ctx, peer.ID); err != nil {
		t.Fatal(err)
	}
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//Calendar//EN\r\nBEGIN:VEVENT\r\nUID:flight\r\nDTSTART:20261023T161000Z\r\nDTEND:20261023T184000Z\r\nSUMMARY:Flight SU1418\r\nGEO:55.9726;37.4146\r\nX-TAYGA-TRAVEL:keep\r\nRRULE:FREQ=DAILY;COUNT=2\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT30M\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	obj, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "flight", HrefName: "flight.ics", Component: "VEVENT", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	attendees := `<OrganizerEmail>alice@example.com</OrganizerEmail><Attendees><Attendee><Email>bob@example.com</Email><Name>Bob</Name><AttendeeType>1</AttendeeType></Attendee><Attendee><Email>guest@outside.example</Email><Name>Guest &amp; friend</Name><AttendeeType>2</AttendeeType></Attendee></Attendees>`
	op := calendarOp(t, "change", obj.ID, attendees)
	if _, err = h.wbCalendar(ctx, u, cal.ID, op); err != nil {
		t.Fatal(err)
	}
	saved, _ := st.GetCalendarObjectByID(ctx, obj.ID)
	parsed, err := calutil.ParseICS(saved.Data)
	if err != nil || len(parsed.Attendees) != 2 || parsed.Organizer.Email != u.Email || parsed.Sequence != 1 {
		t.Fatal(saved.Data, err)
	}
	for _, keep := range []string{"GEO:55.9726;37.4146", "X-TAYGA-TRAVEL:keep", "RRULE:FREQ=DAILY;COUNT=2", "BEGIN:VALARM"} {
		if !strings.Contains(saved.Data, keep) {
			t.Fatal("lost", keep)
		}
	}
	jobs := drainCalendarJobs(t, ctx, st)
	if len(jobs) != 2 {
		t.Fatalf("invitations=%d", len(jobs))
	}
	for _, j := range jobs {
		if !bytes.Contains(j.Data, []byte("METHOD:REQUEST")) || j.Sender != u.Email {
			t.Fatal("invalid invitation", j)
		}
	}
	invites, err := st.ListCalendarInvitesForUser(ctx, peer.ID, false)
	if err != nil || len(invites) != 1 {
		t.Fatal(invites, err)
	}
	copy, _ := st.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if copy == nil || copy.UID != obj.UID {
		t.Fatal("missing recipient copy")
	}
	// Replayed edits and recipient-side echoes cannot generate more invitations.
	if _, err = h.wbCalendar(ctx, u, cal.ID, op); err != nil {
		t.Fatal(err)
	}
	if jobs = drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("duplicate invitation")
	}
	received := calendarOp(t, "change", copy.ID, attendees)
	if _, err = h.wbCalendar(ctx, peer, copy.CalendarID, received); err != nil {
		t.Fatal(err)
	}
	if jobs = drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("recipient impersonated organizer")
	}
	// Serialized metadata reaches both clients with the correct meeting role.
	for _, tc := range []struct{ viewer, want string }{{u.Email, "1"}, {peer.Email, "3"}} {
		wire, err := encodeWire(`<ApplicationData xmlns="AirSync:">` + calendarMeetingXML(saved.Data, tc.viewer) + `</ApplicationData>`)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeWire(wire)
		if err != nil {
			t.Fatal(err)
		}
		doc, _ := parseProtocolXML(decoded)
		if doc.find("MeetingStatus").Text != tc.want || len(doc.find("Attendees").Children) != 2 {
			t.Fatal(string(decoded))
		}
	}
	// Removing an attendee queues a cancellation, and a server-side Delete
	// cancels the remaining invitation without destroying another user's event.
	remove := calendarOp(t, "change", obj.ID, `<Attendees><Attendee><Email>bob@example.com</Email><Name>Bob</Name></Attendee></Attendees>`)
	if _, err = h.wbCalendar(ctx, u, cal.ID, remove); err != nil {
		t.Fatal(err)
	}
	jobs = drainCalendarJobs(t, ctx, st)
	cancelled := false
	for _, j := range jobs {
		if j.Recipient == "guest@outside.example" && bytes.Contains(j.Data, []byte("METHOD:CANCEL")) {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatal("removed guest did not receive cancellation")
	}
	if _, err = h.wbCalendar(ctx, u, cal.ID, clientOp{Kind: "delete", ServerID: obj.ID}); err != nil {
		t.Fatal(err)
	}
	jobs = drainCalendarJobs(t, ctx, st)
	if len(jobs) != 1 || !bytes.Contains(jobs[0].Data, []byte("METHOD:CANCEL")) {
		t.Fatal("missing cancellation")
	}
}

func TestEASCalendarWBXMLAddReplayAndNoDuplicate(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	call := func(key, commands string) string {
		t.Helper()
		raw := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + cal.ID + `</CollectionId>` + commands + `</Collection></Collections></Sync>`
		wire, err := encodeWire(raw)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=test&DeviceType=test", bytes.NewReader(wire)).WithContext(withUser(ctx, u))
		r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		decoded, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		return string(decoded)
	}
	first := call("0", "")
	key := extractTag(first, "SyncKey")
	fields := `<Subject>New flight</Subject><StartTime>20261023T161000Z</StartTime><EndTime>20261023T184000Z</EndTime><Attendees><Attendee><Email>guest@outside.example</Email><Name>Guest</Name></Attendee></Attendees>`
	commands := `<Commands><Add><ClientId>stable-add</ClientId><ApplicationData xmlns:c="Calendar:">` + regexp.MustCompile(`(<\/?)([A-Za-z]+)`).ReplaceAllString(fields, `${1}c:${2}`) + `</ApplicationData></Add></Commands>`
	response := call(key, commands)
	if replay := call(key, commands); replay != response {
		t.Fatal("different retry response")
	}
	doc, _ := parseProtocolXML([]byte(response))
	for _, n := range doc.find("Commands").Children {
		if n.Name.Local == "Add" {
			t.Fatal("server echoed duplicate Add")
		}
	}
	objects, _ := st.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 1 {
		t.Fatal(objects)
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 1 {
		t.Fatal(len(jobs))
	}
	// Retry even after loss of the response cache retains the original object.
	op := calendarOp(t, "add", "", fields)
	op.ClientID = "stable-add"
	got, err := h.wbCalendar(context.WithValue(ctx, taskDeviceKey{}, dev.ID), u, cal.ID, op)
	if err != nil || got != objects[0].ID {
		t.Fatal(got, err)
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("replayed invitation")
	}
}

func TestEASCalendarRejectsForeignOrganizerAndInvalidAttendee(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	for _, fields := range []string{`<OrganizerEmail>other@example.com</OrganizerEmail>`, `<Attendees><Attendee><Email>bad address</Email></Attendee></Attendees>`, `<BusyStatus>999</BusyStatus>`} {
		op := calendarOp(t, "add", "", `<StartTime>20261023T160000Z</StartTime><EndTime>20261023T170000Z</EndTime>`+fields)
		if _, err := h.wbCalendar(ctx, u, cal.ID, op); err == nil {
			t.Fatal("accepted", fields)
		}
	}
	objects, _ := st.ListCalendarObjects(ctx, cal.ID)
	if len(objects) != 0 {
		t.Fatal("partial write")
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("unauthorized invitation")
	}
}
