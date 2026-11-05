package flowsync

import (
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func meetingEWSUpdate(id, key, mode, fields string) string {
	return `<m:UpdateItem SendMeetingInvitationsOrCancellations="` + mode + `"><m:ItemChanges><t:ItemChange><t:ItemId Id="` + id + `" ChangeKey="` + xmlEscape(key) + `"/><t:Updates><t:SetItemField><t:CalendarItem>` + fields + `</t:CalendarItem></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
}
func meetingEWSDelete(id, key, mode string) string {
	return `<m:DeleteItem DeleteType="HardDelete" SendMeetingCancellations="` + mode + `"><m:ItemIds><t:ItemId Id="` + id + `" ChangeKey="` + xmlEscape(key) + `"/></m:ItemIds></m:DeleteItem>`
}
func TestEWSOrganizerMoveMobileReplyAndCancel(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	oldKey := ewsCalendarChangeKey(f.original, nil)
	fields := `<t:Subject>Перенос из EWS</t:Subject><t:Location></t:Location><t:Start>2026-10-23T17:00:00Z</t:Start><t:End>2026-10-23T18:00:00Z</t:End>`
	response := calendarSOAP(t, h, f.ctx, f.owner, meetingEWSUpdate(f.original.ID, oldKey, "SendOnlyToAll", fields))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	current, _ := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	copy, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	for _, o := range []*storage.CalendarObject{current, copy} {
		event, _ := calutil.ParseICS(o.Data)
		if event.Summary != "Перенос из EWS" || event.Start.UTC().Format("2006-01-02T15:04:05Z") != "2026-10-23T17:00:00Z" || event.Geo != "56.8;60.6" || len(event.Attendees) != 1 {
			t.Fatal(o.Data)
		}
	}
	key := extractAttr(response.render(), "ItemId", "ChangeKey")
	if key == oldKey || key != ewsCalendarChangeKey(current, nil) {
		t.Fatal("wrong calendar change key")
	}
	jobs := drainCalendarJobs(t, f.ctx, f.st)
	if len(jobs) != 1 || jobs[0].Recipient != f.guest.Email || !strings.Contains(string(jobs[0].Data), "METHOD:REQUEST") {
		t.Fatal("missing meeting update", len(jobs))
	}
	if result := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", `<SendResponse/>`); result.value("Status") != "1" {
		t.Fatal(result.render())
	}
	current, _ = f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	parsed, _ := calutil.ParseICS(current.Data)
	if parsed.Attendees[0].PartStat != "ACCEPTED" {
		t.Fatal(current.Data)
	}
	drainCalendarJobs(t, f.ctx, f.st)
	// A stale organizer must not overwrite a phone response or cancel a newer meeting.
	for _, request := range []string{meetingEWSUpdate(current.ID, key, "SendOnlyToAll", fields), meetingEWSDelete(current.ID, key, "SendOnlyToAll")} {
		result := calendarSOAP(t, h, f.ctx, f.owner, request)
		if result.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
			t.Fatal(result.render())
		}
	}
	key = ewsCalendarChangeKey(current, nil)
	result := calendarSOAP(t, h, f.ctx, f.owner, meetingEWSDelete(current.ID, key, "SendOnlyToAll"))
	if result.find("ResponseCode").Text != "NoError" {
		t.Fatal(result.render())
	}
	if _, err := f.st.GetCalendarObjectByID(f.ctx, current.ID); err != storage.ErrNotFound {
		t.Fatal(err)
	}
	copy, _ = f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	parsed, _ = calutil.ParseICS(copy.Data)
	if parsed.Status != "CANCELLED" {
		t.Fatal("participant copy not cancelled", copy.Data)
	}
	jobs = drainCalendarJobs(t, f.ctx, f.st)
	if len(jobs) != 1 || !strings.Contains(string(jobs[0].Data), "METHOD:CANCEL") {
		t.Fatal("missing cancellation", len(jobs))
	}
	if result := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, "1", "16.1", ""); result.value("Status") == "1" {
		t.Fatal("cancelled meeting accepted")
	}
}

func TestEWSSendToNoneUpdatesAndCancelsCopiesWithoutMail(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	result := calendarSOAP(t, h, f.ctx, f.owner, meetingEWSUpdate(f.original.ID, ewsCalendarChangeKey(f.original, nil), "SendToNone", `<t:Subject>Silent sync</t:Subject>`))
	if result.find("ResponseCode").Text != "NoError" {
		t.Fatal(result.render())
	}
	copy, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	parsed, _ := calutil.ParseICS(copy.Data)
	if parsed.Summary != "Silent sync" {
		t.Fatal(copy.Data)
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("SendToNone queued mail", len(jobs))
	}
	result = calendarSOAP(t, h, f.ctx, f.owner, meetingEWSDelete(f.original.ID, extractAttr(result.render(), "ItemId", "ChangeKey"), "SendToNone"))
	if result.find("ResponseCode").Text != "NoError" {
		t.Fatal(result.render())
	}
	copy, _ = f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	parsed, _ = calutil.ParseICS(copy.Data)
	if parsed.Status != "CANCELLED" {
		t.Fatal(copy.Data)
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("SendToNone queued cancellation", len(jobs))
	}
}

func TestEWSMeetingWritesRollbackWithOutboxFailure(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		t.Run(map[bool]string{false: "update", true: "delete"}[deleting], func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			if _, err := f.st.(*storage.Store).DB().Exec(`CREATE TRIGGER fail_ews_calendar_mail BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
				t.Fatal(err)
			}
			key := ewsCalendarChangeKey(f.original, nil)
			var err error
			if deleting {
				_, err = h.deleteItem(f.ctx, f.owner, meetingEWSDelete(f.original.ID, key, "SendOnlyToAll"))
			} else {
				_, err = h.updateItem(f.ctx, f.owner, meetingEWSUpdate(f.original.ID, key, "SendOnlyToAll", `<t:Subject>Must roll back</t:Subject>`))
			}
			if err == nil {
				t.Fatal("outbox failure not returned")
			}
			for _, before := range []*storage.CalendarObject{f.original, f.received} {
				after, err := f.st.GetCalendarObjectByID(f.ctx, before.ID)
				if err != nil || before.Data != after.Data || before.ETag != after.ETag {
					t.Fatal("partial calendar write", err)
				}
			}
		})
	}
}

func TestEWSAppointmentReturnsUsableChangeKeys(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	created := calendarSOAP(t, h, ctx, user, `<m:CreateItem SendMeetingInvitations="SendToNone"><m:Items><t:CalendarItem><t:Subject>Appointment</t:Subject><t:Start>2026-11-06T09:00:00Z</t:Start><t:End>2026-11-06T10:00:00Z</t:End></t:CalendarItem></m:Items></m:CreateItem>`)
	if created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	id, key := extractAttr(created.render(), "ItemId", "Id"), extractAttr(created.render(), "ItemId", "ChangeKey")
	object, _ := st.GetCalendarObjectByID(ctx, id)
	if key != ewsCalendarChangeKey(object, nil) {
		t.Fatal("CreateItem returned a placeholder change key")
	}
	updated := calendarSOAP(t, h, ctx, user, meetingEWSUpdate(id, key, "SendToNone", `<t:Subject>Updated appointment</t:Subject>`))
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	object, _ = st.GetCalendarObjectByID(ctx, id)
	if key == extractAttr(updated.render(), "ItemId", "ChangeKey") || extractAttr(updated.render(), "ItemId", "ChangeKey") != ewsCalendarChangeKey(object, nil) {
		t.Fatal("UpdateItem returned a placeholder change key")
	}
}

func TestEWSMeetingEditPreservesTimezoneRecurrenceAndPrivateReminder(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	edit := *f.original
	edit.ExpectedETag = f.original.ETag
	edit.Data = strings.Replace(edit.Data, "DTSTART:20261022T170000Z", "DTSTART;TZID=Europe/Moscow:20261022T200000", 1)
	edit.Data = strings.Replace(edit.Data, "DTEND:20261022T180000Z", "DTEND;TZID=Europe/Moscow:20261022T210000", 1)
	edit.Data = strings.Replace(edit.Data, "END:VEVENT", "RRULE:FREQ=DAILY;COUNT=2\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Private organizer reminder\r\nEND:VALARM\r\nEND:VEVENT", 1)
	saved, err := f.st.SaveCalendarClientObject(f.ctx, f.owner.ID, &edit)
	if err != nil {
		t.Fatal(err)
	}
	drainCalendarJobs(t, f.ctx, f.st)
	response := calendarSOAP(t, h, f.ctx, f.owner, meetingEWSUpdate(saved.ID, ewsCalendarChangeKey(saved, nil), "SendOnlyToAll", `<t:Start>2026-10-22T18:00:00Z</t:Start><t:End>2026-10-22T19:00:00Z</t:End>`))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	saved, _ = f.st.GetCalendarObjectByID(f.ctx, saved.ID)
	for _, text := range []string{"DTSTART;TZID=Europe/Moscow:20261022T210000", "RRULE:FREQ=DAILY;COUNT=2", "Private organizer reminder"} {
		if !strings.Contains(saved.Data, text) {
			t.Fatal("calendar detail lost", saved.Data)
		}
	}
	copy, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	if strings.Contains(copy.Data, "Private organizer reminder") || !strings.Contains(copy.Data, "RRULE:") {
		t.Fatal("participant copy damaged or private reminder leaked", copy.Data)
	}
}
