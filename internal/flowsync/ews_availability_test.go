// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const availabilityMSK = `<TimeZone><Bias>-180</Bias><StandardTime><Bias>0</Bias><Time>00:00:00</Time><DayOrder>1</DayOrder><Month>0</Month><DayOfWeek>Sunday</DayOfWeek></StandardTime><DaylightTime><Bias>0</Bias><Time>00:00:00</Time><DayOrder>1</DayOrder><Month>0</Month><DayOfWeek>Sunday</DayOfWeek></DaylightTime></TimeZone>`

func availabilityRequest(addresses ...string) string {
	var mailboxes strings.Builder
	for _, address := range addresses {
		mailboxes.WriteString(`<MailboxData><Email><Address>` + address + `</Address></Email><AttendeeType>Required</AttendeeType><ExcludeConflicts>false</ExcludeConflicts></MailboxData>`)
	}
	return `<GetUserAvailabilityRequest>` + availabilityMSK + `<MailboxDataArray>` + mailboxes.String() + `</MailboxDataArray><FreeBusyViewOptions><TimeWindow><StartTime>2026-10-17T09:00:00</StartTime><EndTime>2026-10-17T11:00:00</EndTime></TimeWindow><MergedFreeBusyIntervalInMinutes>30</MergedFreeBusyIntervalInMinutes><RequestedView>DetailedMerged</RequestedView></FreeBusyViewOptions></GetUserAvailabilityRequest>`
}

func TestAvailabilityDispatchPrivacyAndIsolation(t *testing.T) {
	ctx, st, u, _, _, _ := syncFixture(t)
	h := &ewsHandler{store: st}
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	room, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: u.DomainID, LocalPart: "room201", DisplayName: "Комната 201", Kind: "room"})
	if err != nil {
		t.Fatal(err)
	}
	cal, err := st.GetCalendarByName(ctx, room.UserID, "default")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "private", HrefName: "private.ics", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:private\r\nSUMMARY:Secret appointment\r\nLOCATION:Secret location\r\nDTSTART:20261017T063000Z\r\nDTEND:20261017T070000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string) string {
		t.Helper()
		r := httptest.NewRequest("POST", "/EWS/Exchange.asmx", strings.NewReader(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>`+body+`</s:Body></s:Envelope>`)).WithContext(withUser(ctx, u))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		out := w.Body.String()
		requireXML(t, out)
		if strings.Contains(out, "GetUserAvailabilityRequestResponse") || !strings.Contains(out, "GetUserAvailabilityResponse") {
			t.Fatal(out)
		}
		return out
	}
	out := call(availabilityRequest(room.Email, u.Email, "other@other.example"))
	for _, want := range []string{"<t:MergedFreeBusy>0200</t:MergedFreeBusy>", "<t:MergedFreeBusy>0000</t:MergedFreeBusy>", "2026-10-17T09:30:00+03:00", "ErrorMailRecipientNotFound", "<t:FreeBusyViewType>FreeBusyMerged</t:FreeBusyViewType>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s: %s", want, out)
		}
	}
	if strings.Contains(out, "Secret") || strings.Contains(out, "CalendarEventDetails") {
		t.Fatal("private details disclosed", out)
	}
	if err = st.PutSetting(ctx, "domain.gal."+u.DomainID, "false"); err != nil {
		t.Fatal(err)
	}
	out = call(availabilityRequest(room.Email, u.Email))
	if strings.Count(out, "ErrorMailRecipientNotFound") != 1 || !strings.Contains(out, "<t:MergedFreeBusy>0000</t:MergedFreeBusy>") {
		t.Fatal(out)
	}
	if err = st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateUserEnabled(ctx, room.UserID, false); err != nil {
		t.Fatal(err)
	}
	if out = call(availabilityRequest(room.Email)); !strings.Contains(out, "ErrorMailRecipientNotFound") {
		t.Fatal(out)
	}
	for _, body := range []string{
		strings.Replace(availabilityRequest(u.Email), "2026-10-17T11:00:00", "2026-10-17T08:00:00", 1),
		strings.Replace(availabilityRequest(u.Email), "2026-10-17T11:00:00", "2027-10-17T11:00:00", 1),
		strings.Replace(availabilityRequest(u.Email), ">30<", ">0<", 1),
		strings.Replace(availabilityRequest(u.Email), availabilityMSK, "", 1),
	} {
		if out = call(body); !strings.Contains(out, `ResponseClass="Error"`) {
			t.Fatal(out)
		}
	}
}

func TestAvailabilityUnsupportedRecurrenceIsNotFree(t *testing.T) {
	ctx, st, u, _, _, _ := syncFixture(t)
	cal, err := st.GetCalendarByName(ctx, u.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, UID: "dense", HrefName: "dense.ics", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:dense\r\nDTSTART:20261017T063000Z\r\nDTEND:20261017T070000Z\r\nRRULE:FREQ=MINUTELY\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := (&ewsHandler{store: st}).availabilityEWS(ctx, u, availabilityRequest(u.Email))
	if err != nil || !strings.Contains(out, "ErrorFreeBusyGenerationFailed") || strings.Contains(out, "<t:MergedFreeBusy>") {
		t.Fatal(out, err)
	}
}

func TestAvailabilityDaylightTransitions(t *testing.T) {
	doc, err := parseProtocolXML([]byte(`<TimeZone><Bias>300</Bias><StandardTime><Bias>0</Bias><Time>02:00:00</Time><DayOrder>1</DayOrder><Month>11</Month><DayOfWeek>Sunday</DayOfWeek></StandardTime><DaylightTime><Bias>-60</Bias><Time>02:00:00</Time><DayOrder>2</DayOrder><Month>3</Month><DayOfWeek>Sunday</DayOfWeek></DaylightTime></TimeZone>`))
	if err != nil {
		t.Fatal(err)
	}
	z, err := parseAvailabilityZone(doc.find("TimeZone"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ wall, utc string }{
		{"2026-01-17T09:00:00", "2026-01-17T14:00:00Z"},
		{"2026-10-17T09:00:00", "2026-10-17T13:00:00Z"},
		{"2026-03-08T01:59:59", "2026-03-08T06:59:59Z"},
		{"2026-03-08T03:00:00", "2026-03-08T07:00:00Z"},
		{"2026-11-01T02:00:00", "2026-11-01T07:00:00Z"},
	} {
		got, err := z.parse(tc.wall)
		if err != nil || got.Format(time.RFC3339) != tc.utc {
			t.Fatal(tc, got, err)
		}
		if !strings.HasPrefix(z.format(got), tc.wall) {
			t.Fatal("timezone roundtrip", tc)
		}
	}
	if _, err := z.parse("2026-03-08T02:30:00"); err == nil {
		t.Fatal("daylight gap accepted")
	}
}
