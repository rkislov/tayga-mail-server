package httpapi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestCalendarThunderbirdWebAndActiveSyncLifecycle(t *testing.T) {
	f := newFileFixture(t)
	f.s.cfg.FlowSync.Enabled = true
	ctx := context.Background()
	if err := f.s.store.EnsureDAVDefaults(ctx, f.users[0]); err != nil {
		t.Fatal(err)
	}
	calendar, err := f.s.store.GetCalendarByName(ctx, f.users[0], "default")
	if err != nil {
		t.Fatal(err)
	}
	escape := func(value string) string { var b strings.Builder; xml.EscapeText(&b, []byte(value)); return b.String() }
	property := func(name, value string) string {
		return `<t:ExtendedProperty><t:ExtendedFieldURI PropertySetId="f3db8cb0-04eb-49ca-bd89-720f85e98dd7" PropertyName="` + name + `" PropertyType="String"/><t:Value>` + escape(value) + `</t:Value></t:ExtendedProperty>`
	}
	soap := func(user int, body string) string {
		t.Helper()
		wire := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><s:Body>` + body + `</s:Body></s:Envelope>`
		return f.call(user, "POST", "/EWS/Exchange.asmx", wire, 200, "Content-Type", "text/xml").Body.String()
	}
	value := func(wire, name string) string {
		decoder := xml.NewDecoder(strings.NewReader(wire))
		for {
			token, err := decoder.Token()
			if err != nil {
				return ""
			}
			if n, ok := token.(xml.StartElement); ok && n.Name.Local == name {
				var result string
				if decoder.DecodeElement(&result, &n) == nil {
					return result
				}
				return ""
			}
		}
	}
	hasTag := func(wire, name string) bool {
		decoder := xml.NewDecoder(strings.NewReader(wire))
		for {
			token, err := decoder.Token()
			if err != nil {
				return false
			}
			if n, ok := token.(xml.StartElement); ok && n.Name.Local == name {
				return true
			}
		}
	}
	changeKey := func(wire string) string {
		t.Helper()
		decoder := xml.NewDecoder(strings.NewReader(wire))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if n, ok := token.(xml.StartElement); ok && n.Name.Local == "ItemId" {
				for _, a := range n.Attr {
					if a.Name.Local == "ChangeKey" {
						return a.Value
					}
				}
			}
		}
		t.Fatal("missing calendar change key")
		return ""
	}
	get := func(user int, id string) string {
		return soap(user, `<m:GetItem><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape></m:ItemShape><m:ItemIds><t:ItemId Id="`+id+`"/></m:ItemIds></m:GetItem>`)
	}
	id := storage.NewID()
	raw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Cross-client test//EN\r\nBEGIN:VEVENT\r\nUID:cross-client-meeting\r\nDTSTART:20261105T090000Z\r\nDTEND:20261105T100000Z\r\nSUMMARY:Из Thunderbird\r\nORGANIZER:mailto:owner@example.com\r\nATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:reader@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	created := soap(0, `<m:CreateItem SendMeetingInvitations="SendToNone"><m:SavedItemFolderId><t:FolderId Id="`+calendar.ID+`"/></m:SavedItemFolderId><m:Items><t:CalendarItem>`+property("TaygaICalendar", raw)+property("TaygaCreateId", id)+`</t:CalendarItem></m:Items></m:CreateItem>`)
	if !strings.Contains(created, "<m:ResponseCode>NoError</m:ResponseCode>") {
		t.Fatal(created)
	}
	invites, err := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if err != nil || len(invites) != 1 {
		t.Fatal(invites, err)
	}
	copyID := invites[0].CalendarObjectID
	// A wire-level phone response must be visible through both the web API and EWS.
	reply := `<MeetingResponse xmlns="MeetingResponse:"><Request><UserResponse>1</UserResponse><CollectionId>`
	copy, _ := f.s.store.GetCalendarObjectByID(ctx, copyID)
	reply += copy.CalendarID + `</CollectionId><RequestId>` + copyID + `</RequestId><SendResponse/></Request></MeetingResponse>`
	responded := f.call(1, "POST", "/Microsoft-Server-ActiveSync?Cmd=MeetingResponse&DeviceId=cross-client&DeviceType=iPhone", reply, 200, "Content-Type", "application/xml", "MS-ASProtocolVersion", "16.1").Body.String()
	if !strings.Contains(responded, "<Status>1</Status>") {
		t.Fatal(responded)
	}
	if owner := get(0, id); value(owner, "ResponseType") != "Accept" {
		t.Fatal(owner)
	}
	web := f.call(0, "GET", "/api/v1/calendar/events/"+id, "", 200)
	var event struct {
		ETag string `json:"etag"`
		ICS  string `json:"ics"`
	}
	if err := json.Unmarshal(web.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	p, _ := calutil.ParseICS(event.ICS)
	if p.Attendees[0].PartStat != "ACCEPTED" {
		t.Fatal(event.ICS)
	}
	syncPhone := func(key string) string {
		t.Helper()
		changes := "<GetChanges>1</GetChanges>"
		if key == "0" {
			changes = ""
		}
		body := `<Sync xmlns="AirSync:"><Collections><Collection><Class>Calendar</Class><SyncKey>` + key + `</SyncKey><CollectionId>` + copy.CalendarID + `</CollectionId>` + changes + `</Collection></Collections></Sync>`
		page := f.call(1, "POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=cross-client&DeviceType=iPhone", body, 200, "Content-Type", "application/xml", "MS-ASProtocolVersion", "16.1").Body.String()
		if value(page, "Status") != "1" {
			t.Fatal(page)
		}
		return page
	}
	page := syncPhone("0")
	page = syncPhone(value(page, "SyncKey"))
	if !strings.Contains(page, copyID) || !hasTag(page, "Add") {
		t.Fatal("phone missing initial invitation", page)
	}
	staleKey := changeKey(get(0, id))
	payload, _ := json.Marshal(map[string]any{"id": id, "uid": "cross-client-meeting", "etag": event.ETag, "summary": "Перенос из веба", "start": "2026-11-06T11:00:00Z", "end": "2026-11-06T12:00:00Z", "attendees": []map[string]string{{"email": "reader@example.com"}}})
	f.call(0, "POST", "/api/v1/calendar/calendars/"+calendar.ID+"/events", string(payload), 200)
	current := get(0, id)
	if value(current, "Start") != "2026-11-06T11:00:00Z" {
		t.Fatal(current)
	}
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, copyID)
	p, _ = calutil.ParseICS(copy.Data)
	if p.Start.UTC().Format("2006-01-02T15:04:05Z") != "2026-11-06T11:00:00Z" || p.Attendees[0].PartStat != "NEEDS-ACTION" {
		t.Fatal(copy.Data)
	}
	page = syncPhone(value(page, "SyncKey"))
	if !hasTag(page, "Change") || value(page, "StartTime") != "20261106T110000Z" {
		t.Fatal("phone did not receive web move", page)
	}
	// An EWS edit after a web edit must refresh before cancelling the meeting.
	remove := func(key string) string {
		return soap(0, `<m:DeleteItem DeleteType="HardDelete" SendMeetingCancellations="SendOnlyToAll"><m:ItemIds><t:ItemId Id="`+id+`" ChangeKey="`+escape(key)+`"/></m:ItemIds></m:DeleteItem>`)
	}
	if stale := remove(staleKey); !strings.Contains(stale, "ErrorIrresolvableConflict") {
		t.Fatal(stale)
	}
	if result := remove(changeKey(current)); !strings.Contains(result, "NoError") {
		t.Fatal(result)
	}
	f.call(0, "GET", "/api/v1/calendar/events/"+id, "", 404)
	copy, _ = f.s.store.GetCalendarObjectByID(ctx, copyID)
	p, _ = calutil.ParseICS(copy.Data)
	if p.Status != "CANCELLED" {
		t.Fatal(copy.Data)
	}
	// The phone keeps the same item, explicitly cancelled and no longer busy.
	page = syncPhone(value(page, "SyncKey"))
	if !hasTag(page, "Change") || !strings.Contains(page, copyID) || value(page, "MeetingStatus") != "7" || value(page, "BusyStatus") != "0" {
		t.Fatal("phone did not receive cancellation", page)
	}
}
