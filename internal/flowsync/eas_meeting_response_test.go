package flowsync

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

type meetingFixture struct {
	ctx                context.Context
	st                 storage.Driver
	owner, guest       *storage.User
	h                  *easHandler
	original, received *storage.CalendarObject
	invite             *storage.CalendarInvite
	mail               *storage.Message
}

func newMeetingFixture(t *testing.T) meetingFixture {
	t.Helper()
	ctx, st, owner, h, _, _ := syncFixture(t)
	guest, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, LocalPart: "bob", Email: "bob@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureDAVDefaults(ctx, guest.ID); err != nil {
		t.Fatal(err)
	}
	cal, _ := st.GetCalendarByName(ctx, owner.ID, "default")
	obj, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "rsvp-test", HrefName: "rsvp.ics", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:rsvp-test\r\nDTSTART:20261022T170000Z\r\nDTEND:20261022T180000Z\r\nSUMMARY:Тест\r\nGEO:56.8;60.6\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.wbCalendar(ctx, owner, cal.ID, calendarOp(t, "change", obj.ID, `<Attendees><Attendee><Email>bob@example.com</Email><Name>Bob</Name></Attendee></Attendees>`)); err != nil {
		t.Fatal(err)
	}
	jobs := drainCalendarJobs(t, ctx, st)
	if len(jobs) != 1 {
		t.Fatal("missing invitation", len(jobs))
	}
	mail := composeSource(t, ctx, h, guest, jobs[0].Data)
	invites, err := st.ListCalendarInvitesForUser(ctx, guest.ID, false)
	if err != nil || len(invites) != 1 {
		t.Fatal(err, invites)
	}
	received, _ := st.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	obj, _ = st.GetCalendarObjectByID(ctx, obj.ID)
	return meetingFixture{ctx, st, owner, guest, h, obj, received, invites[0], mail}
}

func (f meetingFixture) respond(t *testing.T, u *storage.User, collection, id, response, version, extra string) *protocolNode {
	t.Helper()
	// Actual iOS command layout, encoded with code page 8 (no Sync wrapper).
	raw := `<MeetingResponse xmlns="MeetingResponse:"><Request><UserResponse>` + response + `</UserResponse><CollectionId>` + collection + `</CollectionId><RequestId>` + id + `</RequestId>` + extra + `</Request></MeetingResponse>`
	wire, err := encodeWire(raw)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=MeetingResponse&DeviceId=ipad&DeviceType=iPad", bytes.NewReader(wire)).WithContext(withUser(f.ctx, u))
	req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
	req.Header.Set("MS-ASProtocolVersion", version)
	w := httptest.NewRecorder()
	f.h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("MS-ASProtocolCommands"), "MeetingResponse") {
		t.Fatal("not advertised")
	}
	decoded, err := decodeWire(w.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML(decoded)
	if err != nil {
		t.Fatal(err)
	}
	return doc.find("Result")
}

func TestMeetingResponseUpdatesBothCalendarsAndIncrementalSync(t *testing.T) {
	for _, version := range []string{"14.1", "16.0", "16.1"} {
		t.Run(version, func(t *testing.T) {
			f := newMeetingFixture(t)
			dev, _ := f.st.EnsureFlowSyncDevice(f.ctx, f.owner.ID, "organizer", "iPhone")
			sync := func(key string) *protocolNode {
				t.Helper()
				result, err := f.h.syncCollections(f.ctx, f.owner, dev, []byte(`<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>`+key+`</SyncKey><CollectionId>`+f.original.CalendarID+`</CollectionId></Collection></Collections></Sync>`))
				if err != nil {
					t.Fatal(err)
				}
				doc, err := parseProtocolXML([]byte(result))
				if err != nil {
					t.Fatal(err)
				}
				return doc.find("Collection")
			}
			page := sync(sync("0").value("SyncKey"))
			if page.find("Add") == nil {
				t.Fatal(page.render())
			}
			result := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", version, "")
			if result.value("Status") != "1" || result.value("CalendarId") != f.received.ID {
				t.Fatal(result.render())
			}
			for _, obj := range []*storage.CalendarObject{f.original, f.received} {
				saved, _ := f.st.GetCalendarObjectByID(f.ctx, obj.ID)
				p, _ := calutil.ParseICS(saved.Data)
				if len(p.Attendees) != 1 || p.Attendees[0].PartStat != "ACCEPTED" || saved.UID != obj.UID || p.Geo != "56.8;60.6" || saved.ETag == obj.ETag {
					t.Fatal(saved.Data)
				}
			}
			page = sync(page.value("SyncKey"))
			if page.find("Change") == nil || page.find("Add") != nil || page.find("Delete") != nil || page.find("AttendeeStatus").Text != "3" || page.find("ResponseType").Text != "1" {
				t.Fatal(page.render())
			}
			inv, _ := f.st.GetCalendarInvite(f.ctx, f.invite.ID)
			if inv.PartStat != "ACCEPTED" {
				t.Fatal(inv.PartStat)
			}
			recipient, _, _ := f.h.syncCalendar(f.ctx, f.guest, f.received.CalendarID, "unused", false, nil)
			d, _ := parseProtocolXML([]byte(recipient))
			if d.find("ResponseType").Text != "3" {
				t.Fatal(recipient)
			}
			before, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
			f.h = &easHandler{store: f.st, ms: f.h.ms} // retry after a server restart
			if got := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", version, ""); got.value("Status") != "1" {
				t.Fatal(got.render())
			}
			after, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
			if before.ETag != after.ETag || !before.UpdatedAt.Equal(after.UpdatedAt) {
				t.Fatal("retry changed event")
			}
			if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
				t.Fatal("unrequested email", len(jobs))
			}
		})
	}
}

func TestMeetingResponseMailSendAndRetry(t *testing.T) {
	f := newMeetingFixture(t)
	result := f.respond(t, f.guest, f.mail.MailboxID, f.mail.ID, "2", "16.1", `<SendResponse><Body xmlns="AirSyncBase:"><Type>1</Type><Data>Буду позже</Data></Body></SendResponse>`)
	if result.value("Status") != "1" || result.value("CalendarId") != f.received.ID {
		t.Fatal(result.render())
	}
	jobs := drainCalendarJobs(t, f.ctx, f.st)
	if len(jobs) != 1 || jobs[0].Sender != f.guest.Email || jobs[0].Recipient != f.owner.Email || !bytes.Contains(jobs[0].Data, []byte("METHOD:REPLY")) {
		t.Fatal("missing reply", len(jobs))
	}
	f.respond(t, f.guest, f.mail.MailboxID, f.mail.ID, "2", "16.1", `<SendResponse/>`)
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("duplicate reply")
	}
	result = f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "3", "16.1", `<SendResponse/>`)
	if result.value("Status") != "1" || result.child("CalendarId") != nil {
		t.Fatal(result.render())
	}
	inv, _ := f.st.GetCalendarInvite(f.ctx, f.invite.ID)
	if inv.PartStat != "DECLINED" {
		t.Fatal(inv.PartStat)
	}
	recipient, _, _ := f.h.syncCalendar(f.ctx, f.guest, f.received.CalendarID, "unused", false, nil)
	if strings.Contains(recipient, f.received.ID) {
		t.Fatal("declined item still synchronized")
	}
	from := time.Date(2026, 10, 22, 16, 0, 0, 0, time.UTC)
	busy, err := f.st.FreeBusyForUser(f.ctx, f.guest.ID, from, from.Add(3*time.Hour))
	if err != nil || len(busy) != 0 {
		t.Fatal("decline blocks guest availability", busy, err)
	}
	busy, err = f.st.FreeBusyForUser(f.ctx, f.owner.ID, from, from.Add(3*time.Hour))
	if err != nil || len(busy) != 1 {
		t.Fatal("decline removed organizer availability", busy, err)
	}
	result = f.respond(t, f.guest, f.mail.MailboxID, f.mail.ID, "1", "16.1", "")
	if result.value("CalendarId") != f.received.ID {
		t.Fatal("reacceptance created another item", result.render())
	}
	busy, err = f.st.FreeBusyForUser(f.ctx, f.guest.ID, from, from.Add(3*time.Hour))
	if err != nil || len(busy) != 1 {
		t.Fatal("acceptance did not restore availability", busy, err)
	}
}

func TestMeetingResponseRejectsInvalidTargetsWithoutWrites(t *testing.T) {
	f := newMeetingFixture(t)
	cases := []struct {
		u                                        *storage.User
		collection, id, response, version, extra string
	}{
		{f.owner, f.received.CalendarID, f.received.ID, "1", "16.1", ""},
		{f.owner, f.original.CalendarID, f.original.ID, "1", "16.1", ""},
		{f.guest, f.received.CalendarID, f.original.ID, "1", "16.1", ""},
		{f.guest, f.mail.MailboxID, f.received.ID, "1", "16.1", ""},
		{f.guest, f.received.CalendarID, f.received.ID, "9", "16.1", ""},
		{f.guest, f.received.CalendarID, f.received.ID, "1", "14.0", ""},
		{f.guest, f.received.CalendarID, f.received.ID, "1", "14.1", "<SendResponse/>"},
		{f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", "<InstanceId>20261022T170000Z</InstanceId>"},
		{f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", "<UserResponse>3</UserResponse>"},
		{f.guest, f.received.CalendarID, f.received.ID, "2", "16.1", "<SendResponse><ProposedStartTime>20261022T180000Z</ProposedStartTime></SendResponse>"},
	}
	for i, c := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			got := f.respond(t, c.u, c.collection, c.id, c.response, c.version, c.extra)
			if got.value("Status") != "2" || got.child("CalendarId") != nil {
				t.Fatal(got.render())
			}
		})
	}
	for _, obj := range []*storage.CalendarObject{f.original, f.received} {
		saved, _ := f.st.GetCalendarObjectByID(f.ctx, obj.ID)
		if saved.ETag != obj.ETag {
			t.Fatal("invalid request changed event")
		}
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("invalid request sent mail")
	}
}

func TestMeetingResponseRollbackAndCancelled(t *testing.T) {
	f := newMeetingFixture(t)
	db := f.st.(*storage.Store).DB()
	if _, err := db.Exec(`CREATE TRIGGER fail_mobile_rsvp BEFORE UPDATE ON calendar_invites BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	got := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", "<SendResponse/>")
	if got.value("Status") != "4" {
		t.Fatal(got.render())
	}
	for _, obj := range []*storage.CalendarObject{f.original, f.received} {
		saved, _ := f.st.GetCalendarObjectByID(f.ctx, obj.ID)
		if saved.ETag != obj.ETag {
			t.Fatal("partial RSVP committed")
		}
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("rolled-back RSVP sent mail")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_mobile_rsvp`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.h.wbCalendar(f.ctx, f.owner, f.original.CalendarID, clientOp{Kind: "delete", ServerID: f.original.ID}); err != nil {
		t.Fatal(err)
	}
	got = f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", "")
	if got.value("Status") != "2" {
		t.Fatal(got.render())
	}
}
