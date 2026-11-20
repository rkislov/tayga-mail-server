// SPDX-License-Identifier: Apache-2.0
package dav

import (
	"context"
	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

type schedulingFixture struct {
	t       *testing.T
	ctx     context.Context
	store   storage.Driver
	users   []*storage.User
	handler http.Handler
}

func newSchedulingFixture(t *testing.T) *schedulingFixture {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "scheduling.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	tenant, _ := store.CreateTenant(ctx, "scheduling")
	domain, _ := store.CreateDomain(ctx, tenant.ID, "example.com")
	f := &schedulingFixture{t: t, ctx: ctx, store: store}
	for _, name := range []string{"organizer", "participant", "another"} {
		u, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: name + "@example.com", LocalPart: name, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = store.EnsureDAVDefaults(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		f.users = append(f.users, u)
	}
	f.handler = calendarScheduling(store, calendarRequestConditions(&caldav.Handler{Backend: &calBackend{store: store}, Prefix: calPrefix}))
	return f
}
func (f *schedulingFixture) call(who int, method, path, body string, status int, headers ...string) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r = r.WithContext(withUser(r.Context(), f.users[who]))
	r.Header.Set("Content-Type", "application/xml")
	if method == "PUT" || method == "POST" {
		r.Header.Set("Content-Type", "text/calendar")
	}
	for i := 0; i < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != status {
		f.t.Fatalf("%s %s: want %d got %d: %s", method, path, status, w.Code, w.Body.String())
	}
	return w
}
func (f *schedulingFixture) create() string {
	f.t.Helper()
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:caldav-scheduling\r\nDTSTAMP:20261105T100000Z\r\nDTSTART:20261110T100000Z\r\nDTEND:20261110T110000Z\r\nRRULE:FREQ=DAILY;COUNT=3\r\nSUMMARY:Private meeting\r\nORGANIZER:mailto:organizer@example.com\r\nATTENDEE:mailto:participant@example.com\r\nATTENDEE:mailto:another@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	path := calObjectPath(f.users[0].Email, "default", "meeting.ics")
	w := f.call(0, "PUT", path, data, 201, "If-None-Match", "*")
	if w.Header().Get("Schedule-Tag") == "" {
		f.t.Fatal("missing create Schedule-Tag")
	}
	return path
}
func (f *schedulingFixture) participantPath(who int) string {
	f.t.Helper()
	invites, err := f.store.ListCalendarInvitesForUser(f.ctx, f.users[who].ID, false)
	if err != nil || len(invites) == 0 {
		f.t.Fatal(invites, err)
	}
	obj, err := f.store.GetCalendarObjectByID(f.ctx, invites[0].CalendarObjectID)
	if err != nil {
		f.t.Fatal(err)
	}
	cal, err := f.store.GetCalendarByID(f.ctx, f.users[who].ID, obj.CalendarID)
	if err != nil {
		f.t.Fatal(err)
	}
	return calObjectPath(f.users[who].Email, cal.Name, obj.HrefName)
}
func (f *schedulingFixture) inbox(who int) []storage.CalendarSchedulingMessage {
	f.t.Helper()
	items, err := f.store.ListCalendarSchedulingInbox(f.ctx, f.users[who].ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return items
}

func TestCalDAVSchedulingDiscoveryAndInbox(t *testing.T) {
	f := newSchedulingFixture(t)
	w := f.call(0, "OPTIONS", calPrefix+"/", "", 204)
	if !strings.Contains(w.Header().Get("DAV"), "calendar-auto-schedule") {
		t.Fatal(w.Header())
	}
	props := `<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:schedule-inbox-URL/><c:schedule-outbox-URL/><c:calendar-user-address-set/><c:calendar-user-type/></d:prop></d:propfind>`
	w = f.call(0, "PROPFIND", calPrincipal(f.users[0].Email), props, 207, "Depth", "0")
	for _, value := range []string{"schedule-inbox-URL", calInbox(f.users[0].Email), calOutbox(f.users[0].Email), "mailto:" + f.users[0].Email, "INDIVIDUAL"} {
		if !strings.Contains(w.Body.String(), value) {
			t.Fatal(w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "404 Not Found") {
		t.Fatal("discovery property missing", w.Body.String())
	}
	f.call(1, "PROPFIND", calPrincipal(f.users[0].Email), props, 403, "Depth", "0")
	f.call(0, "PUT", calInbox(f.users[0].Email)+"injected.ics", "invalid", 405)
	path := f.create()
	_ = path
	inbox := f.inbox(1)
	if len(inbox) != 1 {
		t.Fatal(inbox)
	}
	propfind := `<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:resourcetype/><d:getetag/><c:calendar-data/><c:schedule-default-calendar-URL/></d:prop></d:propfind>`
	w = f.call(1, "PROPFIND", calInbox(f.users[1].Email), propfind, 207, "Depth", "1")
	if !strings.Contains(w.Body.String(), "schedule-inbox") || !strings.Contains(w.Body.String(), "METHOD:REQUEST") {
		t.Fatal(w.Body.String())
	}
	messagePath := calInbox(f.users[1].Email) + inbox[0].ID + ".ics"
	f.call(0, "GET", messagePath, "", 403)
	w = f.call(1, "GET", messagePath, "", 200)
	if w.Header().Get("ETag") != inbox[0].ETag {
		t.Fatal(w.Header())
	}
	f.call(1, "GET", messagePath, "", 304, "If-None-Match", inbox[0].ETag)
	multiget := `<c:calendar-multiget xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:d="DAV:"><d:prop><d:getetag/><c:calendar-data/></d:prop><d:href>` + messagePath + `</d:href></c:calendar-multiget>`
	w = f.call(1, "REPORT", calInbox(f.users[1].Email), multiget, 207)
	if !strings.Contains(w.Body.String(), "METHOD:REQUEST") {
		t.Fatal(w.Body.String())
	}
	query := `<c:calendar-query xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:d="DAV:"><d:prop><c:calendar-data/></d:prop><c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"/></c:comp-filter></c:filter></c:calendar-query>`
	w = f.call(1, "REPORT", calInbox(f.users[1].Email), query, 207)
	if !strings.Contains(w.Body.String(), "METHOD:REQUEST") {
		t.Fatal(w.Body.String())
	}
	participant := f.participantPath(1)
	before := f.call(1, "GET", participant, "", 200).Body.String()
	f.call(1, "DELETE", messagePath, "", 412, "If-Match", `"stale"`)
	f.call(1, "DELETE", messagePath, "", 204, "If-Match", inbox[0].ETag)
	if len(f.inbox(1)) != 0 || f.call(1, "GET", participant, "", 200).Body.String() != before {
		t.Fatal("inbox dismissal changed meeting")
	}
}

func TestCalDAVAttendeeReplyAndScheduleTagMerge(t *testing.T) {
	f := newSchedulingFixture(t)
	source := f.create()
	organizerBefore := f.call(0, "GET", source, "", 200)
	path := f.participantPath(1)
	before := f.call(1, "GET", path, "", 200)
	reply, err := calutil.WithAttendeeStatus(before.Body.String(), f.users[1].Email, "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	changed := f.call(1, "PUT", path, reply, 201, "If-Schedule-Tag-Match", before.Header().Get("Schedule-Tag"))
	if changed.Header().Get("Schedule-Tag") == before.Header().Get("Schedule-Tag") {
		t.Fatal("direct response did not rotate own tag")
	}
	after := f.call(0, "GET", source, "", 200)
	if after.Header().Get("ETag") == organizerBefore.Header().Get("ETag") || after.Header().Get("Schedule-Tag") != organizerBefore.Header().Get("Schedule-Tag") {
		t.Fatal("automatic reply changed organizer tag", after.Header())
	}
	parsed, _ := calutil.ParseICS(after.Body.String())
	if parsed.Sequence != 0 {
		t.Fatal("reply changed organizer sequence")
	}
	for _, a := range parsed.Attendees {
		if a.Email == f.users[1].Email && a.PartStat != "ACCEPTED" {
			t.Fatal("organizer response not updated")
		}
	}
	inbox := f.inbox(0)
	if len(inbox) != 1 || !strings.Contains(inbox[0].Data, "METHOD:REPLY") {
		t.Fatal("missing organizer REPLY", inbox)
	}
	// A stale ETag is fine with the matching Schedule-Tag; latest replies survive.
	personal := strings.Replace(organizerBefore.Body.String(), "END:VEVENT", "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\nEND:VEVENT", 1)
	f.call(0, "PUT", source, personal, 201, "If-Schedule-Tag-Match", organizerBefore.Header().Get("Schedule-Tag"))
	merged := f.call(0, "GET", source, "", 200)
	parsed, _ = calutil.ParseICS(merged.Body.String())
	for _, a := range parsed.Attendees {
		if a.Email == f.users[1].Email && a.PartStat != "ACCEPTED" {
			t.Fatal("stale client overwrote reply")
		}
	}
	f.call(0, "PUT", source, personal, 412, "If-Schedule-Tag-Match", organizerBefore.Header().Get("Schedule-Tag"))
	propfind := `<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:schedule-tag/></d:prop></d:propfind>`
	w := f.call(0, "PROPFIND", source, propfind, 207, "Depth", "0")
	if !strings.Contains(w.Body.String(), strings.Trim(merged.Header().Get("Schedule-Tag"), `"`)) || strings.Contains(w.Body.String(), "404") {
		t.Fatal(w.Body.String())
	}

	multi := `<c:calendar-multiget xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:d="DAV:"><d:prop><d:getetag/><c:schedule-tag/></d:prop><d:href>` + source + `</d:href></c:calendar-multiget>`
	report := f.call(0, "REPORT", calCollection(f.users[0].Email, "default"), multi, 207)
	if !strings.Contains(report.Body.String(), strings.Trim(merged.Header().Get("Schedule-Tag"), `"`)) || strings.Contains(report.Body.String(), "404") {
		t.Fatal(report.Body.String())
	}
	current := f.call(1, "GET", path, "", 200)
	invalid := strings.Replace(current.Body.String(), "20261110T100000Z", "20261110T120000Z", 1)
	f.call(1, "PUT", path, invalid, 403, "If-Schedule-Tag-Match", current.Header().Get("Schedule-Tag"))
	beforeCount := len(f.inbox(0))
	f.call(1, "DELETE", path, "", 204, "Schedule-Reply", "F")
	if len(f.inbox(0)) != beforeCount {
		t.Fatal("suppressed delete sent REPLY")
	}
	f.call(2, "DELETE", f.participantPath(2), "", 204)
	if len(f.inbox(0)) != beforeCount+1 {
		t.Fatal("default delete did not send DECLINED")
	}
	latest := f.call(0, "GET", source, "", 200)
	parsed, _ = calutil.ParseICS(latest.Body.String())
	for _, a := range parsed.Attendees {
		if a.Email == f.users[2].Email && a.PartStat != "DECLINED" {
			t.Fatal("delete did not decline participant")
		}
	}
}

func TestCalDAVOccurrenceReplyPreservesOriginalAndStaleMerge(t *testing.T) {
	f := newSchedulingFixture(t)
	source := f.create()
	original := f.call(0, "GET", source, "", 200)
	path := f.participantPath(1)
	before := f.call(1, "GET", path, "", 200)
	data, err := calutil.WithOccurrenceAttendeeStatus(before.Body.String(), "2026-11-11T10:00:00Z", f.users[1].Email, "DECLINED")
	if err != nil {
		t.Fatal(err)
	}
	f.call(1, "PUT", path, data, 201, "If-Schedule-Tag-Match", before.Header().Get("Schedule-Tag"))
	current := f.call(0, "GET", source, "", 200)
	if current.Header().Get("Schedule-Tag") != original.Header().Get("Schedule-Tag") {
		t.Fatal("occurrence reply changed organizer tag")
	}
	occurrence, err := calutil.GetOccurrence(current.Body.String(), "2026-11-11T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range occurrence.Attendees {
		if a.Email == f.users[1].Email && a.PartStat != "DECLINED" {
			t.Fatal("instance RSVP lost")
		}
	}
	inbox := f.inbox(0)
	if len(inbox) != 1 || !strings.Contains(inbox[0].Data, "RECURRENCE-ID") || strings.Contains(inbox[0].Data, "RRULE") {
		t.Fatal(inbox)
	}
	f.call(0, "PUT", source, original.Body.String(), 201, "If-Schedule-Tag-Match", original.Header().Get("Schedule-Tag"))
	merged := f.call(0, "GET", source, "", 200)
	occurrence, err = calutil.GetOccurrence(merged.Body.String(), "2026-11-11T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range occurrence.Attendees {
		if a.Email == f.users[1].Email && a.PartStat != "DECLINED" {
			t.Fatal("stale merge dropped response-only exception")
		}
	}
}

func TestCalDAVOutboxFreeBusyAndIsolation(t *testing.T) {
	f := newSchedulingFixture(t)
	f.create()
	query := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VFREEBUSY\r\nUID:freebusy-test\r\nDTSTAMP:20261105T100000Z\r\nDTSTART:20261110T000000Z\r\nDTEND:20261113T000000Z\r\nORGANIZER:mailto:organizer@example.com\r\nATTENDEE:mailto:participant@example.com\r\nATTENDEE:mailto:missing@remote.example\r\nEND:VFREEBUSY\r\nEND:VCALENDAR\r\n"
	out := f.call(0, "POST", calOutbox(f.users[0].Email), query, 200)
	for _, value := range []string{"schedule-response", "2.0;Success", "3.7;Invalid Calendar User", "VFREEBUSY", "FREEBUSY;FBTYPE=BUSY"} {
		if !strings.Contains(out.Body.String(), value) {
			t.Fatal(out.Body.String())
		}
	}
	if strings.Contains(out.Body.String(), "Private meeting") {
		t.Fatal("freebusy disclosed private subject")
	}
	f.call(1, "POST", calOutbox(f.users[0].Email), query, 403)
	f.call(0, "POST", calOutbox(f.users[0].Email), strings.Replace(query, "ORGANIZER:mailto:organizer@example.com", "ORGANIZER:mailto:participant@example.com", 1), 403)
	f.call(0, "POST", calOutbox(f.users[0].Email), strings.Replace(query, "METHOD:REQUEST", "METHOD:REPLY", 1), 403)
	f.call(0, "POST", calInbox(f.users[0].Email), query, 405)
	tenant, err := f.store.CreateTenant(f.ctx, "other")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := f.store.CreateDomain(f.ctx, tenant.ID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.CreateUser(f.ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "private", Email: "private@other.example", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	response := f.call(0, "POST", calOutbox(f.users[0].Email), strings.Replace(query, "mailto:participant@example.com", "mailto:private@other.example", 1), 200)
	if strings.Contains(response.Body.String(), "2.0;Success") || strings.Contains(response.Body.String(), "calendar-data") {
		t.Fatal("foreign tenant availability disclosed", response.Body.String())
	}

}

func TestCalDAVInboxFailureRollsBackMeetingAndReply(t *testing.T) {
	f := newSchedulingFixture(t)
	db := f.store.(*storage.Store).DB()
	if _, err := db.Exec(`CREATE TRIGGER fail_inbox BEFORE INSERT ON calendar_scheduling_inbox BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\nUID:rollback\r\nDTSTAMP:20261105T100000Z\r\nDTSTART:20261110T100000Z\r\nDTEND:20261110T110000Z\r\nORGANIZER:mailto:organizer@example.com\r\nATTENDEE:mailto:participant@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	f.call(0, "PUT", calObjectPath(f.users[0].Email, "default", "rollback.ics"), data, 500)
	cal, _ := f.store.GetCalendarByName(f.ctx, f.users[0].ID, "default")
	objects, _ := f.store.ListCalendarObjects(f.ctx, cal.ID)
	if len(objects) != 0 {
		t.Fatal("inbox failure committed event")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_inbox`); err != nil {
		t.Fatal(err)
	}
	source := f.create()
	before := f.call(0, "GET", source, "", 200)
	path := f.participantPath(1)
	old := f.call(1, "GET", path, "", 200)
	if _, err := db.Exec(`CREATE TRIGGER fail_inbox BEFORE INSERT ON calendar_scheduling_inbox BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	reply, err := calutil.WithAttendeeStatus(old.Body.String(), f.users[1].Email, "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	f.call(1, "PUT", path, reply, 500, "If-Schedule-Tag-Match", old.Header().Get("Schedule-Tag"))
	after := f.call(0, "GET", source, "", 200)
	own := f.call(1, "GET", path, "", 200)
	if after.Body.String() != before.Body.String() || own.Body.String() != old.Body.String() || own.Header().Get("Schedule-Tag") != old.Header().Get("Schedule-Tag") {
		t.Fatal("reply partially committed")
	}
}

func TestCalDAVAttendeeScheduleAgentClientDoesNotReply(t *testing.T) {
	f := newSchedulingFixture(t)
	source := f.create()
	path := f.participantPath(1)
	invs, _ := f.store.ListCalendarInvitesForUser(f.ctx, f.users[1].ID, false)
	object, _ := f.store.GetCalendarObjectByID(f.ctx, invs[0].CalendarObjectID)
	object.Data = strings.ReplaceAll(object.Data, "ORGANIZER:", "ORGANIZER;SCHEDULE-AGENT=CLIENT:")
	object.ExpectedETag = object.ETag
	if _, err := f.store.SaveCalendarClientObject(f.ctx, f.users[1].ID, object); err != nil {
		t.Fatal(err)
	}
	before := f.call(0, "GET", source, "", 200).Body.String()
	own := f.call(1, "GET", path, "", 200)
	reply, err := calutil.WithAttendeeStatus(own.Body.String(), f.users[1].Email, "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	f.call(1, "PUT", path, reply, 201, "If-Schedule-Tag-Match", own.Header().Get("Schedule-Tag"))
	if len(f.inbox(0)) != 0 || f.call(0, "GET", source, "", 200).Body.String() != before {
		t.Fatal("CLIENT agent initiated server scheduling")
	}
	f.call(1, "DELETE", path, "", 204)
	if len(f.inbox(0)) != 0 {
		t.Fatal("CLIENT delete sent reply")
	}
}

func TestCalDAVAttendeeAliasReply(t *testing.T) {
	f := newSchedulingFixture(t)
	if _, err := f.store.CreateAlias(f.ctx, f.users[1].DomainID, f.users[1].ID, "participant-alias"); err != nil {
		t.Fatal(err)
	}
	source := f.create()
	original := f.call(0, "GET", source, "", 200)
	invitation := strings.Replace(original.Body.String(), "mailto:participant@example.com", "mailto:participant-alias@example.com", -1)
	f.call(0, "PUT", source, invitation, 201, "If-Schedule-Tag-Match", original.Header().Get("Schedule-Tag"))
	path := f.participantPath(1)
	before := f.call(1, "GET", path, "", 200)
	data, err := calutil.WithAttendeeStatus(before.Body.String(), "participant-alias@example.com", "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	f.call(1, "PUT", path, data, 201, "If-Schedule-Tag-Match", before.Header().Get("Schedule-Tag"))
	got := f.call(0, "GET", source, "", 200)
	parsed, _ := calutil.ParseICS(got.Body.String())
	for _, a := range parsed.Attendees {
		if a.Email == "participant-alias@example.com" && a.PartStat != "ACCEPTED" {
			t.Fatal("alias response lost")
		}
	}
	f.call(1, "DELETE", path, "", 204)
}
