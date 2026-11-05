// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func recurrenceData(start, rule string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:recurrence-test\r\nDTSTART:" + start + "\r\nDTEND:" + strings.Replace(start, "090000", "100000", 1) + "\r\nSUMMARY:Series\r\nRRULE:" + rule + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

func TestCalendarPatternProtocolRoundTrips(t *testing.T) {
	for _, tc := range []struct{ start, rule string }{
		{"20260101T090000Z", "FREQ=DAILY;COUNT=4"},
		{"20260105T090000Z", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,FR;WKST=SU;COUNT=8"},
		{"20260131T090000Z", "FREQ=MONTHLY;BYMONTHDAY=28,29,30,31;BYSETPOS=-1;COUNT=4"},
		{"20260130T090000Z", "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1;COUNT=4"},
		{"20260101T090000Z", "FREQ=YEARLY;BYMONTH=1;BYMONTHDAY=1;COUNT=4"},
		{"20260126T090000Z", "FREQ=YEARLY;BYMONTH=1;BYDAY=MO;BYSETPOS=-1;COUNT=4"},
	} {
		t.Run(tc.rule, func(t *testing.T) {
			original := recurrenceData(tc.start, tc.rule)
			from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			to := from.AddDate(5, 0, 0)
			want, err := calutil.ExpandOccurrences(original, from, to)
			if err != nil {
				t.Fatal(err)
			}
			ews, err := ewsRecurrenceXML(original)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parseProtocolXML([]byte(`<CalendarItem xmlns:t="types">` + ews + `</CalendarItem>`))
			if err != nil {
				t.Fatal(err)
			}
			got, err := applyEWSRecurrence(original, []ewsCalendarEdit{{kind: "SetItemField", name: "Recurrence", node: doc.find("Recurrence")}})
			if err != nil {
				t.Fatal(err)
			}
			eas, err := calendarRecurrenceXML(context.Background(), got, "")
			if err != nil {
				t.Fatal(err)
			}
			wire, err := encodeWire(`<ApplicationData xmlns="AirSync:">` + eas + `</ApplicationData>`)
			if err != nil {
				t.Fatal(err)
			}
			xml, err := decodeWire(wire)
			if err != nil {
				t.Fatal(err)
			}
			doc, err = parseProtocolXML(xml)
			if err != nil {
				t.Fatal(err)
			}
			got, err = easRecurrence(got, doc.find("Recurrence"))
			if err != nil {
				t.Fatal(err)
			}
			restored := preserveEquivalentRecurrence(original, got)
			if davTextProperty(restored, "RRULE") != tc.rule {
				t.Fatal("equivalent rule echo changed stored RRULE")
			}
			have, err := calutil.ExpandOccurrences(got, from, to)
			if err != nil || len(want) != len(have) {
				t.Fatal(err, len(want), len(have))
			}
			for i := range want {
				if want[i].RecurrenceID != have[i].RecurrenceID || !want[i].Start.Equal(*have[i].Start) {
					t.Fatal("changed occurrence", want[i], have[i])
				}
			}
			if strings.Contains(tc.rule, "28,29") {
				if have[1].Start.Day() != 28 || have[2].Start.Day() != 31 {
					t.Fatal("month end was skipped", have)
				}
			}
		})
	}
}

func TestCalendarTimezoneDSTAndRejectBadRules(t *testing.T) {
	data := recurrenceData("20260301T090000Z", "FREQ=WEEKLY;BYDAY=SU;COUNT=3")
	// Preserve 09:00 New York across the spring transition.
	data = strings.ReplaceAll(data, "20260301T090000Z", "20260301T140000Z")
	data = strings.ReplaceAll(data, "20260301T100000Z", "20260301T150000Z")
	data, err := calutil.SetClientTimezone(data, "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	tzi, err := calendarTZI("America/New_York", 2026)
	if err != nil {
		t.Fatal(err)
	}
	zone, err := decodeCalendarTimezone(base64.StdEncoding.EncodeToString(tzi), "America/New_York", 2026)
	if err != nil || zone != "America/New_York" {
		t.Fatal(zone, err)
	}
	got, err := calutil.ExpandOccurrences(data, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(got) != 3 {
		t.Fatal(err, got)
	}
	if got[0].Start.UTC().Hour() != 14 || got[1].Start.UTC().Hour() != 13 {
		t.Fatal("DST lost", got)
	}
	for _, rule := range []string{"FREQ=MONTHLY;BYDAY=5SU", "FREQ=MONTHLY;BYMONTHDAY=31", "FREQ=HOURLY;COUNT=4"} {
		if _, err := ewsRecurrenceXML(recurrenceData("20260301T090000Z", rule)); err == nil {
			t.Fatal("lossy rule accepted", rule)
		}
	}
	tzi[0] ^= 1
	if _, err := decodeCalendarTimezone(base64.StdEncoding.EncodeToString(tzi), "America/New_York", 2026); err == nil {
		t.Fatal("trusted mismatched timezone label")
	}
}

func TestEWSAndEASRecurrenceExceptions(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	updated := calendarSOAP(t, h, f.ctx, f.owner, `<m:UpdateItem SendMeetingInvitationsOrCancellations="SendOnlyToAll"><m:ItemChanges><t:ItemChange><t:ItemId Id="`+f.original.ID+`"/><t:Updates><t:SetItemField><t:FieldURI FieldURI="calendar:Recurrence"/><t:CalendarItem><t:Recurrence><t:DailyRecurrence><t:Interval>1</t:Interval></t:DailyRecurrence><t:NumberedRecurrence><t:StartDate>2026-10-22</t:StartDate><t:NumberOfOccurrences>4</t:NumberOfOccurrences></t:NumberedRecurrence></t:Recurrence></t:CalendarItem></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`)
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	drainCalendarJobs(t, f.ctx, f.st)
	master, _ := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	firstKey := ewsCalendarChangeKey(master, nil)
	moved := calendarSOAP(t, h, f.ctx, f.owner, `<m:UpdateItem SendMeetingInvitationsOrCancellations="SendOnlyToAll"><m:ItemChanges><t:ItemChange><t:OccurrenceItemId RecurringMasterId="`+master.ID+`" InstanceIndex="2" ChangeKey="`+firstKey+`"/><t:Updates><t:SetItemField><t:FieldURI FieldURI="calendar:Start"/><t:CalendarItem><t:Start>2026-10-23T19:00:00Z</t:Start></t:CalendarItem></t:SetItemField><t:SetItemField><t:FieldURI FieldURI="calendar:End"/><t:CalendarItem><t:End>2026-10-23T20:00:00Z</t:End></t:CalendarItem></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`)
	if moved.find("ResponseCode").Text != "NoError" {
		t.Fatal(moved.render())
	}
	stable := extractAttr(moved.render(), "ItemId", "Id")
	if stable != occurrenceItemID(master.ID, "2026-10-23T17:00:00Z") {
		t.Fatal(stable)
	}
	jobs := drainCalendarJobs(t, f.ctx, f.st)
	if len(jobs) != 1 || !strings.Contains(string(jobs[0].Data), "RECURRENCE-ID") {
		t.Fatal("instance delivery", jobs)
	}
	received, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	instance, err := calutil.GetOccurrence(received.Data, "2026-10-23T17:00:00Z")
	if err != nil || instance.Start.UTC().Hour() != 19 {
		t.Fatal("participant move", instance, err)
	}
	stale := calendarSOAP(t, h, f.ctx, f.owner, `<m:DeleteItem><m:ItemIds><t:ItemId Id="`+stable+`" ChangeKey="`+firstKey+`"/></m:ItemIds></m:DeleteItem>`)
	if stale.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal(stale.render())
	}
	// EAS 14.1 patches a second date; the omitted EWS move must survive.
	ctx14 := context.WithValue(f.ctx, easVersionKey{}, "14.1")
	if _, err = f.h.wbCalendar(ctx14, f.owner, master.CalendarID, calendarOp(t, "change", master.ID, `<Exceptions><Exception><ExceptionStartTime>20261024T170000Z</ExceptionStartTime><Deleted>1</Deleted></Exception></Exceptions>`)); err != nil {
		t.Fatal(err)
	}
	// EAS 16.x edits by master ServerId and sibling InstanceId, through WBXML.
	wire, err := encodeWire(`<Sync xmlns="AirSync:"><Collections><Collection><Commands><Change><ServerId>` + master.ID + `</ServerId><InstanceId xmlns="AirSyncBase:">2026-10-23T17:00:00Z</InstanceId><ApplicationData><Subject xmlns="Calendar:">Moved via phone</Subject></ApplicationData></Change></Commands></Collection></Collections></Sync>`)
	if err != nil {
		t.Fatal(err)
	}
	ops := parseSyncClientOps(wire)
	if len(ops) != 1 || ops[0].InstanceID == "" {
		t.Fatal(ops)
	}
	ctx16 := context.WithValue(f.ctx, easVersionKey{}, "16.1")
	if _, err = f.h.wbCalendar(ctx16, f.owner, master.CalendarID, ops[0]); err != nil {
		t.Fatal(err)
	}
	master, _ = f.st.GetCalendarObjectByID(f.ctx, master.ID)
	for _, ctx := range []context.Context{ctx14, ctx16} {
		out, bin, err := f.h.syncCalendar(ctx, f.owner, master.CalendarID, "1", true, nil)
		_ = out
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeWire(bin)
		if err != nil || !strings.Contains(string(decoded), "Moved via phone") {
			t.Fatal(err, string(decoded))
		}
	}
	got := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemIds><t:ItemId Id="`+master.ID+`"/></m:ItemIds></m:GetItem>`)
	if got.find("CalendarItemType").Text != "RecurringMaster" || got.find("ModifiedOccurrences").find("OriginalStart").Text != "2026-10-23T17:00:00Z" || got.find("DeletedOccurrences").find("Start").Text != "2026-10-24T17:00:00Z" {
		t.Fatal(got.render())
	}
	got = calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemIds><t:OccurrenceItemId RecurringMasterId="`+master.ID+`" InstanceIndex="4"/></m:ItemIds></m:GetItem>`)
	if got.find("Start").Text != "2026-10-25T17:00:00Z" {
		t.Fatal("cancelled date changed indexing", got.render())
	}
	view := calendarSOAP(t, h, f.ctx, f.owner, `<m:FindItem><m:CalendarView StartDate="2026-10-22T00:00:00Z" EndDate="2026-10-26T00:00:00Z"/><m:ParentFolderIds><t:FolderId Id="`+master.CalendarID+`"/></m:ParentFolderIds></m:FindItem>`)
	items := view.find("Items")
	if items == nil || len(items.Children) != 3 {
		t.Fatal(view.render())
	}
	// A bad second exception leaves the first edit and all delivery jobs untouched.
	before := master.ETag
	drainCalendarJobs(t, f.ctx, f.st)
	_, err = f.h.wbCalendar(ctx14, f.owner, master.CalendarID, calendarOp(t, "change", master.ID, `<Exceptions><Exception><ExceptionStartTime>20261025T170000Z</ExceptionStartTime><Subject>Must roll back</Subject></Exception><Exception><ExceptionStartTime>20300101T170000Z</ExceptionStartTime><Deleted>1</Deleted></Exception></Exceptions>`))
	if err == nil {
		t.Fatal("invalid exception accepted")
	}
	master, _ = f.st.GetCalendarObjectByID(f.ctx, master.ID)
	if master.ETag != before || len(drainCalendarJobs(t, f.ctx, f.st)) != 0 {
		t.Fatal("partial exception patch")
	}
	if _, err = f.h.wbCalendar(ctx16, f.guest, f.received.CalendarID, clientOp{Kind: "delete", ServerID: f.received.ID, InstanceID: "2026-10-25T17:00:00Z"}); err == nil {
		t.Fatal("attendee cancelled organizer occurrence")
	}
	deleted := calendarSOAP(t, h, f.ctx, f.owner, `<m:DeleteItem SendMeetingCancellations="SendOnlyToAll"><m:ItemIds><t:OccurrenceItemId RecurringMasterId="`+master.ID+`" InstanceIndex="4"/></m:ItemIds></m:DeleteItem>`)
	if deleted.find("ResponseCode").Text != "NoError" {
		t.Fatal(deleted.render())
	}
	master, _ = f.st.GetCalendarObjectByID(f.ctx, master.ID)
	cancelled, err := calutil.GetOccurrence(master.Data, "2026-10-25T17:00:00Z")
	if err != nil || cancelled.Status != "CANCELLED" {
		t.Fatal("indexed delete", err)
	}
	responses := f.h.applyWritebacks(ctx16, f.owner, kindCalendar, master.CalendarID, []clientOp{{Kind: "delete", ServerID: master.ID, InstanceID: "2026-10-22T17:00:00Z"}})
	if len(responses) != 1 || responses[0].Status != 1 || !strings.Contains(renderWritebackResponses(responses), "InstanceId") {
		t.Fatal(responses)
	}

}

func TestCreateRecurringCalendarsAndAllDayExceptions(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	created := calendarSOAP(t, h, ctx, u, `<m:CreateItem><m:SavedItemFolderId><t:FolderId Id="`+cal.ID+`"/></m:SavedItemFolderId><m:Items><t:CalendarItem><t:Subject>Local DST</t:Subject><t:Start>2026-03-01T09:00:00</t:Start><t:End>2026-03-01T10:00:00</t:End><t:StartTimeZone Id="Eastern Standard Time"/><t:EndTimeZone Id="Eastern Standard Time"/><t:Recurrence><t:WeeklyRecurrence><t:Interval>1</t:Interval><t:DaysOfWeek>Sunday</t:DaysOfWeek></t:WeeklyRecurrence><t:EndDateRecurrence><t:StartDate>2026-03-01</t:StartDate><t:EndDate>2026-03-15</t:EndDate></t:EndDateRecurrence></t:Recurrence></t:CalendarItem></m:Items></m:CreateItem>`)
	if created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	id := extractAttr(created.render(), "ItemId", "Id")
	obj, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	instances, err := calutil.ExpandOccurrences(obj.Data, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(instances) != 3 || instances[1].Start.UTC().Hour() != 13 {
		t.Fatal("created DST", err, instances)
	}
	ctx14 := context.WithValue(ctx, easVersionKey{}, "14.1")
	fields := `<Subject>All day month ends</Subject><StartTime>20260131T000000Z</StartTime><EndTime>20260201T000000Z</EndTime><AllDayEvent>1</AllDayEvent><Recurrence><Type>2</Type><DayOfMonth>31</DayOfMonth><Occurrences>4</Occurrences></Recurrence><Exceptions><Exception><ExceptionStartTime>20260228T000000Z</ExceptionStartTime><Subject>February only</Subject></Exception><Exception><ExceptionStartTime>20260331T000000Z</ExceptionStartTime><Deleted>1</Deleted></Exception></Exceptions>`
	id, err = eas.wbCalendar(ctx14, u, cal.ID, calendarOp(t, "add", "", fields))
	if err != nil {
		t.Fatal(err)
	}
	obj, _ = st.GetCalendarObjectByID(ctx, id)
	if !strings.Contains(obj.Data, "RECURRENCE-ID;VALUE=DATE:20260228") {
		t.Fatal(obj.Data)
	}
	master, _ := calutil.ParseICS(obj.Data)
	if master.Summary != "All day month ends" {
		t.Fatal("nested subject leaked into master", master.Summary)
	}
	item := calendarSOAP(t, h, ctx, u, `<m:GetItem><m:ItemIds><t:OccurrenceItemId RecurringMasterId="`+id+`" InstanceIndex="2"/></m:ItemIds></m:GetItem>`)
	if item.find("Subject").Text != "February only" || item.find("IsAllDayEvent").Text != "true" {
		t.Fatal(item.render())
	}
	// Echoing the exception does not bump the version or reset series responses.
	before := obj.ETag
	_, err = eas.wbCalendar(ctx14, u, cal.ID, calendarOp(t, "change", id, `<Exceptions><Exception><ExceptionStartTime>20260228T000000Z</ExceptionStartTime><Subject>February only</Subject></Exception></Exceptions>`))
	if err != nil {
		t.Fatal(err)
	}
	obj, _ = st.GetCalendarObjectByID(ctx, id)
	if obj.ETag != before {
		t.Fatal("echo rewrote the series")
	}
	// An actual exception patch changes only that date, keeping master and cancellation.
	_, err = eas.wbCalendar(ctx14, u, cal.ID, calendarOp(t, "change", id, `<Exceptions><Exception><ExceptionStartTime>20260228T000000Z</ExceptionStartTime><Subject>February changed</Subject></Exception></Exceptions>`))
	if err != nil {
		t.Fatal(err)
	}
	obj, _ = st.GetCalendarObjectByID(ctx, id)
	master, _ = calutil.ParseICS(obj.Data)
	exceptions, err := calutil.RecurrenceExceptions(obj.Data)
	if err != nil || master.Summary != "All day month ends" || len(exceptions) != 2 {
		t.Fatal(master, exceptions, err)
	}
}

func TestUnsupportedRecurrenceRetainsFullICalendarContract(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	data := recurrenceData("20260329T090000Z", "FREQ=MONTHLY;BYDAY=5SU;COUNT=3")
	obj, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, HrefName: "fifth.ics", UID: "recurrence-test", Component: "VEVENT", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	body := `<m:GetItem><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape><t:AdditionalProperties><t:ExtendedFieldURI PropertySetId="` + taygaPropertySet + `" PropertyName="TaygaICalendar" PropertyType="String"/></t:AdditionalProperties></m:ItemShape><m:ItemIds><t:ItemId Id="` + obj.ID + `"/></m:ItemIds></m:GetItem>`
	got := calendarSOAP(t, h, ctx, u, body)
	if got.find("ResponseCode").Text != "NoError" || taygaProperties(got.render())["TaygaICalendar"] != data {
		t.Fatal(got.render())
	}
	got = calendarSOAP(t, h, ctx, u, `<m:GetItem><m:ItemIds><t:ItemId Id="`+obj.ID+`"/></m:ItemIds></m:GetItem>`)
	if got.find("ResponseCode").Text != "ErrorCalendarInvalidRecurrence" {
		t.Fatal(got.render())
	}
	stored, _ := st.GetCalendarObjectByID(ctx, obj.ID)
	if stored.Data != data {
		t.Fatal("unsupported import changed")
	}
}

func TestEASRecurringInstanceHTTPReplayAndSyncKey(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	id, err := h.wbCalendar(ctx, u, cal.ID, calendarOp(t, "add", "", `<Subject>Series</Subject><StartTime>20261022T170000Z</StartTime><EndTime>20261022T180000Z</EndTime><Recurrence><Type>0</Type><Occurrences>4</Occurrences></Recurrence>`))
	if err != nil {
		t.Fatal(err)
	}
	call := func(key, commands string) (*protocolNode, string) {
		t.Helper()
		raw := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + cal.ID + `</CollectionId>` + commands + `</Collection></Collections></Sync>`
		wire, err := encodeWire(raw)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=test&DeviceType=test", bytes.NewReader(wire)).WithContext(withUser(ctx, u))
		req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		req.Header.Set("MS-ASProtocolVersion", "16.1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		decoded, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML(decoded)
		if err != nil {
			t.Fatal(err)
		}
		return doc, string(decoded)
	}
	init, _ := call("0", "")
	key := init.find("SyncKey").Text
	commands := `<Commands><Change><ServerId>` + id + `</ServerId><InstanceId xmlns="AirSyncBase:">2026-10-23T17:00:00Z</InstanceId><ApplicationData><Subject xmlns="Calendar:">Phone instance</Subject></ApplicationData></Change></Commands>`
	first, body := call(key, commands)
	if first.find("Status").Text != "1" {
		t.Fatal(body)
	}
	source, _ := st.GetCalendarObjectByID(ctx, id)
	instance, err := calutil.GetOccurrence(source.Data, "2026-10-23T17:00:00Z")
	if err != nil || instance.Summary != "Phone instance" {
		t.Fatal(instance, err)
	}
	_, retry := call(key, commands)
	if retry != body {
		t.Fatal("changed replay response")
	}
	after, _ := st.GetCalendarObjectByID(ctx, id)
	if after.ETag != source.ETag {
		t.Fatal("replay created another exception version")
	}
	stale, _ := call(key, strings.Replace(commands, "Phone instance", "Stale write", 1))
	if stale.find("Status").Text != "3" {
		t.Fatal(stale.render())
	}
	after, _ = st.GetCalendarObjectByID(ctx, id)
	if after.ETag != source.ETag {
		t.Fatal("stale SyncKey overwrote instance")
	}
}

func TestEWSAllDayRecurrenceUsesLocalDate(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	for _, start := range []string{"2026-01-31T00:00:00", "2026-01-30T21:00:00Z"} {
		end := "2026-02-01T00:00:00"
		if strings.HasSuffix(start, "Z") {
			end = "2026-01-31T21:00:00Z"
		}
		result := calendarSOAP(t, h, ctx, u, `<m:CreateItem><m:SavedItemFolderId><t:FolderId Id="`+cal.ID+`"/></m:SavedItemFolderId><m:Items><t:CalendarItem><t:Start>`+start+`</t:Start><t:End>`+end+`</t:End><t:IsAllDayEvent>true</t:IsAllDayEvent><t:StartTimeZone Id="Russian Standard Time"/><t:EndTimeZone Id="Russian Standard Time"/><t:Recurrence><t:DailyRecurrence><t:Interval>1</t:Interval></t:DailyRecurrence><t:NumberedRecurrence><t:StartDate>2026-01-31</t:StartDate><t:NumberOfOccurrences>2</t:NumberOfOccurrences></t:NumberedRecurrence></t:Recurrence></t:CalendarItem></m:Items></m:CreateItem>`)
		if result.find("ResponseCode").Text != "NoError" {
			t.Fatal(result.render())
		}
		obj, err := st.GetCalendarObjectByID(ctx, extractAttr(result.render(), "ItemId", "Id"))
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := calutil.ParseICS(obj.Data)
		if err != nil || !parsed.AllDay || parsed.Start.Format("2006-01-02") != "2026-01-31" {
			t.Fatal("all-day moved to UTC date", obj.Data, err)
		}
		for i := 0; i < 2; i++ {
			start, end := "2026-01-31T18:00:00Z", "2026-01-31T19:00:00Z"
			dayEdit := `<t:SetItemField><t:FieldURI FieldURI="calendar:IsAllDayEvent"/><t:CalendarItem><t:IsAllDayEvent>false</t:IsAllDayEvent></t:CalendarItem></t:SetItemField>`
			if i == 1 {
				start, end = "2026-01-31T20:00:00Z", "2026-01-31T21:00:00Z"
				dayEdit = ""
			}
			changed := calendarSOAP(t, h, ctx, u, `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:OccurrenceItemId RecurringMasterId="`+obj.ID+`" InstanceIndex="1"/><t:Updates>`+dayEdit+`<t:SetItemField><t:FieldURI FieldURI="calendar:Start"/><t:CalendarItem><t:Start>`+start+`</t:Start></t:CalendarItem></t:SetItemField><t:SetItemField><t:FieldURI FieldURI="calendar:End"/><t:CalendarItem><t:End>`+end+`</t:End></t:CalendarItem></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`)
			if changed.find("ResponseCode").Text != "NoError" {
				t.Fatal("timed exception in DATE series", changed.render())
			}
		}
		obj, _ = st.GetCalendarObjectByID(ctx, obj.ID)
		instance, err := calutil.GetOccurrence(obj.Data, "2026-01-31")
		if err != nil || instance.AllDay || instance.Start.UTC().Hour() != 20 {
			t.Fatal(instance, err)
		}

	}
}
