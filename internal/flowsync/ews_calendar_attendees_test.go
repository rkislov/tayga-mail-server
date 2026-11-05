// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func standardMeetingCreate(folder, mode, fields string) string {
	return `<m:CreateItem SendMeetingInvitations="` + mode + `"><m:SavedItemFolderId><t:FolderId Id="` + folder + `"/></m:SavedItemFolderId><m:Items><t:CalendarItem><t:Subject>Standard EWS meeting</t:Subject><t:Start>2026-11-10T09:00:00Z</t:Start><t:End>2026-11-10T10:00:00Z</t:End>` + fields + `</t:CalendarItem></m:Items></m:CreateItem>`
}
func standardAttendee(email, name string) string {
	return `<t:Attendee><t:Mailbox><t:Name>` + xmlEscape(name) + `</t:Name><t:EmailAddress>` + xmlEscape(email) + `</t:EmailAddress><t:RoutingType>SMTP</t:RoutingType></t:Mailbox></t:Attendee>`
}
func standardMeetingUpdate(id, key, mode, updates string) string {
	return `<m:UpdateItem SendMeetingInvitationsOrCancellations="` + mode + `"><m:ItemChanges><t:ItemChange><t:ItemId Id="` + id + `" ChangeKey="` + xmlEscape(key) + `"/><t:Updates>` + updates + `</t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
}
func standardMeetingField(kind, field, value string) string {
	uri := "calendar:" + field
	if field == "Subject" || field == "Body" {
		uri = "item:" + field
	}
	inner := `<t:FieldURI FieldURI="` + uri + `"/>`
	if kind != "DeleteItemField" {
		inner += `<t:CalendarItem><t:` + field + `>` + value + `</t:` + field + `></t:CalendarItem>`
	}
	return `<t:` + kind + `>` + inner + `</t:` + kind + `>`
}
func standardMeetingPeer(t *testing.T, ctx context.Context, st storage.Driver, owner *storage.User, local string) *storage.User {
	t.Helper()
	u, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, LocalPart: local, Email: local + "@example.com", DisplayName: local, AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureDAVDefaults(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	return u
}
func TestEWSStandardCreateSchedulesParticipantsAndResource(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	peer := standardMeetingPeer(t, ctx, st, owner, "standard-peer")
	room, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "ews-room", DisplayName: "EWS room", AutoAccept: true})
	if err != nil {
		t.Fatal(err)
	}
	calendar, _ := st.GetCalendarByName(ctx, owner.ID, "default")
	fields := `<t:Body BodyType="HTML">&lt;b&gt;Meeting details&lt;/b&gt;</t:Body><t:RequiredAttendees>` + standardAttendee(peer.Email, "Peer & friend") + `</t:RequiredAttendees><t:OptionalAttendees>` + standardAttendee("outside@external.example", "Optional guest") + `</t:OptionalAttendees><t:Resources>` + standardAttendee(room.Email, "Room") + `</t:Resources>`
	response := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(calendar.ID, "SendOnlyToAll", fields))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	id, key := extractAttr(response.render(), "ItemId", "Id"), extractAttr(response.render(), "ItemId", "ChangeKey")
	saved, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	event, _ := calutil.ParseICS(saved.Data)
	if event.Organizer.Email != owner.Email || len(event.Attendees) != 3 || event.Description != "Meeting details" || key != ewsCalendarChangeKey(saved, nil) {
		t.Fatal(saved.Data, key)
	}
	roles := map[string]calutil.Attendee{}
	for _, a := range event.Attendees {
		roles[a.Email] = a
	}
	if roles["outside@external.example"].Role != "OPT-PARTICIPANT" || roles[room.Email].CUType != "RESOURCE" || roles[room.Email].PartStat != "ACCEPTED" {
		t.Fatal(saved.Data)
	}
	invites, _ := st.ListCalendarInvitesForUser(ctx, peer.ID, false)
	if len(invites) != 1 {
		t.Fatal("participant copy missing", invites)
	}
	copy, _ := st.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	if copy.UID != saved.UID || !strings.Contains(copy.Data, "Meeting details") {
		t.Fatal(copy.Data)
	}
	jobs := drainCalendarJobs(t, ctx, st)
	if len(jobs) != 2 {
		t.Fatal("wrong invitation count", len(jobs))
	}
	for _, job := range jobs {
		if job.Sender != owner.Email || job.Recipient == room.Email || !strings.Contains(string(job.Data), "METHOD:REQUEST") {
			t.Fatal("wrong invitation routing")
		}
	}
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	busy, err := st.FreeBusyForUser(ctx, room.UserID, start, start.Add(time.Hour))
	if err != nil || len(busy) != 1 {
		t.Fatal("room not booked", busy, err)
	}
	// Echoing the same participant identities, even without display names, must
	// retain resource acceptance and avoid another round of invitations.
	updates := standardMeetingField("SetItemField", "RequiredAttendees", standardAttendee(peer.Email, "")) + standardMeetingField("SetItemField", "OptionalAttendees", standardAttendee("outside@external.example", "")) + standardMeetingField("SetItemField", "Resources", standardAttendee(room.Email, ""))
	echo := calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, key, "SendOnlyToAll", updates))
	if echo.find("ResponseCode").Text != "NoError" {
		t.Fatal(echo.render())
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("echo resent invitations", len(jobs))
	}
	after, _ := st.GetCalendarObjectByID(ctx, id)
	parsed, _ := calutil.ParseICS(after.Data)
	if parsed.Sequence != event.Sequence {
		t.Fatal("echo advanced organizer version")
	}
}
func TestEWSStandardAttendeeSetAppendDeleteAndAuthorization(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	peer := standardMeetingPeer(t, f.ctx, f.st, f.owner, "new-peer")
	source := f.original
	update := func(user *storage.User, updates string) *protocolNode {
		return calendarSOAP(t, h, f.ctx, user, standardMeetingUpdate(source.ID, ewsCalendarChangeKey(source, nil), "SendOnlyToAll", updates))
	}
	response := update(f.owner, standardMeetingField("AppendToItemField", "OptionalAttendees", standardAttendee(peer.Email, "New peer")))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	source, _ = f.st.GetCalendarObjectByID(f.ctx, source.ID)
	event, _ := calutil.ParseICS(source.Data)
	if len(event.Attendees) != 2 {
		t.Fatal(source.Data)
	}
	invites, _ := f.st.ListCalendarInvitesForUser(f.ctx, peer.ID, false)
	if len(invites) != 1 {
		t.Fatal("new peer not invited")
	}
	newCopyID := invites[0].CalendarObjectID
	drainCalendarJobs(t, f.ctx, f.st)
	// Replacing only the required group must keep the optional participant.
	response = update(f.owner, standardMeetingField("SetItemField", "RequiredAttendees", standardAttendee("external@outside.example", "External")))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	source, _ = f.st.GetCalendarObjectByID(f.ctx, source.ID)
	event, _ = calutil.ParseICS(source.Data)
	if len(event.Attendees) != 2 {
		t.Fatal(source.Data)
	}
	oldCopy, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	oldEvent, _ := calutil.ParseICS(oldCopy.Data)
	if oldEvent.Status != "CANCELLED" {
		t.Fatal("removed participant not cancelled", oldCopy.Data)
	}
	jobs := drainCalendarJobs(t, f.ctx, f.st)
	cancelled := false
	for _, job := range jobs {
		if job.Recipient == f.guest.Email && strings.Contains(string(job.Data), "METHOD:CANCEL") {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatal("missing removal cancellation")
	}
	// A recipient may not change the organizer's participant list using its copy.
	recipientCopy, _ := f.st.GetCalendarObjectByID(f.ctx, newCopyID)
	denied := calendarSOAP(t, h, f.ctx, peer, standardMeetingUpdate(recipientCopy.ID, ewsCalendarChangeKey(recipientCopy, nil), "SendOnlyToAll", standardMeetingField("DeleteItemField", "RequiredAttendees", "")))
	if denied.find("ResponseCode").Text != "ErrorAccessDenied" {
		t.Fatal(denied.render())
	}
	response = update(f.owner, standardMeetingField("DeleteItemField", "OptionalAttendees", ""))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	copy, _ := f.st.GetCalendarObjectByID(f.ctx, newCopyID)
	parsed, _ := calutil.ParseICS(copy.Data)
	if parsed.Status != "CANCELLED" {
		t.Fatal("deleted group retained its copy", copy.Data)
	}
}
func TestEWSStandardCreateInvalidFieldsAndRollback(t *testing.T) {
	for _, kind := range []string{"bad-email", "duplicate", "organizer", "recurrence", "body", "batch", "invalid-mode", "queue-failure"} {
		t.Run(kind, func(t *testing.T) {
			ctx, st, owner, eas, _, _ := syncFixture(t)
			h := &ewsHandler{store: st, ms: eas.ms}
			peer := standardMeetingPeer(t, ctx, st, owner, "rollback-peer")
			calendar, _ := st.GetCalendarByName(ctx, owner.ID, "default")
			fields := `<t:RequiredAttendees>` + standardAttendee(peer.Email, "Peer") + `</t:RequiredAttendees>`
			expected := "ErrorInvalidPropertySet"
			switch kind {
			case "bad-email":
				fields = `<t:RequiredAttendees>` + standardAttendee("bad\r\nBcc: other@example.com", "") + `</t:RequiredAttendees>`
			case "duplicate":
				fields += `<t:OptionalAttendees>` + standardAttendee(peer.Email, "") + `</t:OptionalAttendees>`
			case "organizer":
				fields += `<t:Organizer><t:Mailbox><t:EmailAddress>other@example.com</t:EmailAddress></t:Mailbox></t:Organizer>`
				expected = "ErrorAccessDenied"
			case "recurrence":
				fields += `<t:Recurrence><t:DailyRecurrence><t:Interval>1</t:Interval></t:DailyRecurrence></t:Recurrence>`
			case "body":
				fields += `<t:Body BodyType="RTF">unsupported</t:Body>`
			case "queue-failure":
				_, err := st.(*storage.Store).DB().Exec(`CREATE TRIGGER fail_standard_ews_queue BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`)
				if err != nil {
					t.Fatal(err)
				}
				expected = "ErrorInternalServerError"
			}
			body := standardMeetingCreate(calendar.ID, "SendOnlyToAll", fields)
			if kind == "batch" {
				body = strings.Replace(body, `</m:Items>`, `<t:CalendarItem><t:Subject>Must not be silently dropped</t:Subject></t:CalendarItem></m:Items>`, 1)
			}
			if kind == "invalid-mode" {
				body = standardMeetingCreate(calendar.ID, "SendToChangedAndSaveCopy", fields)
				expected = "ErrorInvalidRequest"
			}
			response := calendarSOAP(t, h, ctx, owner, body)
			if response.find("ResponseCode").Text != expected {
				t.Fatal(expected, response.render())
			}
			objects, err := st.ListCalendarObjects(ctx, calendar.ID)
			if err != nil || len(objects) != 0 {
				t.Fatal("failed create leaked source", objects, err)
			}
			invites, _ := st.ListCalendarInvitesForUser(ctx, peer.ID, false)
			if len(invites) != 0 {
				t.Fatal("failed create leaked participant", invites)
			}
			if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
				t.Fatal("failed create leaked mail", len(jobs))
			}
		})
	}
}
func TestEWSStandardUpdateRejectsMismatchedPathsAndKeepsUnspecifiedFields(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	malformed := `<t:SetItemField><t:FieldURI FieldURI="calendar:RequiredAttendees"/><t:CalendarItem><t:Subject>Wrong target</t:Subject></t:CalendarItem></t:SetItemField>`
	response := calendarSOAP(t, h, f.ctx, f.owner, standardMeetingUpdate(f.original.ID, ewsCalendarChangeKey(f.original, nil), "SendOnlyToAll", malformed))
	if response.find("ResponseCode").Text != "ErrorInvalidPropertySet" {
		t.Fatal(response.render())
	}
	after, _ := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if after.ETag != f.original.ETag {
		t.Fatal("invalid field path changed calendar")
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("invalid field path queued mail")
	}
	// Missing attendees are a partial update, not a request to remove them.
	response = calendarSOAP(t, h, f.ctx, f.owner, standardMeetingUpdate(after.ID, ewsCalendarChangeKey(after, nil), "SendToNone", standardMeetingField("SetItemField", "Body", "New description")))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	after, _ = f.st.GetCalendarObjectByID(f.ctx, after.ID)
	event, _ := calutil.ParseICS(after.Data)
	if len(event.Attendees) != 1 || event.Description != "New description" {
		t.Fatal(after.Data)
	}
}

func TestEWSStandardCreateSendToNoneAndRoomConflict(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	peer := standardMeetingPeer(t, ctx, st, owner, "silent-peer")
	room, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: owner.DomainID, LocalPart: "silent-room", DisplayName: "Room", AutoAccept: true})
	if err != nil {
		t.Fatal(err)
	}
	calendar, _ := st.GetCalendarByName(ctx, owner.ID, "default")
	fields := `<t:RequiredAttendees>` + standardAttendee(peer.Email, "Peer") + `</t:RequiredAttendees><t:Resources>` + standardAttendee(room.Email, "Room") + `</t:Resources>`
	for i, mode := range []string{"SendToNone", "SendOnlyToAll"} {
		response := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(calendar.ID, mode, fields))
		want := "NoError"
		if i == 1 {
			want = "ErrorIrresolvableConflict"
		}
		if response.find("ResponseCode").Text != want {
			t.Fatal(response.render())
		}
		objects, err := st.ListCalendarObjects(ctx, calendar.ID)
		if err != nil || len(objects) != 1 {
			t.Fatal("unexpected organizer events", objects, err)
		}
		invites, err := st.ListCalendarInvitesForUser(ctx, peer.ID, false)
		if err != nil || len(invites) != 1 {
			t.Fatal("unexpected participant copies", invites, err)
		}
		if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
			t.Fatal("silent create or failed booking queued mail", len(jobs))
		}
	}
}

func TestEWSStandardAttendeeUpdateRollbackAndStaleKey(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	peer := standardMeetingPeer(t, f.ctx, f.st, f.owner, "atomic-peer")
	updates := standardMeetingField("SetItemField", "RequiredAttendees", standardAttendee(peer.Email, "New participant"))
	request := standardMeetingUpdate(f.original.ID, ewsCalendarChangeKey(f.original, nil), "SendOnlyToAll", updates)
	db := f.st.(*storage.Store).DB()
	if _, err := db.Exec(`CREATE TRIGGER fail_standard_ews_update BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	response := calendarSOAP(t, h, f.ctx, f.owner, request)
	if response.find("ResponseCode").Text != "ErrorInternalServerError" {
		t.Fatal(response.render())
	}
	for _, before := range []*storage.CalendarObject{f.original, f.received} {
		after, err := f.st.GetCalendarObjectByID(f.ctx, before.ID)
		if err != nil || before.Data != after.Data || before.ETag != after.ETag {
			t.Fatal("participant change partially committed", err)
		}
	}
	invites, err := f.st.ListCalendarInvitesForUser(f.ctx, peer.ID, false)
	if err != nil || len(invites) != 0 {
		t.Fatal("failed update leaked new invite", invites, err)
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("failed update leaked mail", len(jobs))
	}
	if _, err := db.Exec(`DROP TRIGGER fail_standard_ews_update`); err != nil {
		t.Fatal(err)
	}
	response = calendarSOAP(t, h, f.ctx, f.owner, request)
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	drainCalendarJobs(t, f.ctx, f.st)
	response = calendarSOAP(t, h, f.ctx, f.owner, request)
	if response.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal("stale participant update accepted", response.render())
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("stale update queued mail", len(jobs))
	}
}
