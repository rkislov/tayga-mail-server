package httpapi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func TestIPhoneExchangeCreateVisibleInWebAndRecipientInbox(t *testing.T) {
	for _, version := range []string{"14.1", "16.1"} {
		t.Run(version, func(t *testing.T) {
			f := newFileFixture(t)
			f.s.cfg.FlowSync.Enabled = true
			ctx := context.Background()
			calendar, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
			if err != nil {
				t.Fatal(err)
			}
			sync := func(key, commands string) string {
				t.Helper()
				body := `<Sync xmlns="AirSync:"><Collections><Collection><Class>Calendar</Class><SyncKey>` + key + `</SyncKey><CollectionId>` + calendar.ID + `</CollectionId><GetChanges>0</GetChanges>` + commands + `</Collection></Collections></Sync>`
				return f.call(0, "POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=iphone-inbox&DeviceType=iPhone", body, 200, "Content-Type", "application/xml", "MS-ASProtocolVersion", version).Body.String()
			}
			value := func(wire, name string) string {
				decoder := xml.NewDecoder(strings.NewReader(wire))
				for {
					token, err := decoder.Token()
					if err != nil {
						return ""
					}
					if start, ok := token.(xml.StartElement); ok && start.Name.Local == name {
						var result string
						if decoder.DecodeElement(&result, &start) == nil {
							return result
						}
						return ""
					}
				}
			}
			key := value(sync("0", ""), "SyncKey")
			if key == "" {
				t.Fatal("missing initial sync key")
			}
			response := sync(key, `<Commands><Add><ClientId>ios-create-inbox</ClientId><ApplicationData xmlns:c="Calendar:"><c:Subject>iPhone invitation</c:Subject><c:StartTime>20261110T090000Z</c:StartTime><c:EndTime>20261110T100000Z</c:EndTime><c:Attendees><c:Attendee><c:Email>reader@example.com</c:Email><c:Name>Reader</c:Name><c:AttendeeType>1</c:AttendeeType></c:Attendee></c:Attendees></ApplicationData></Add></Commands>`)
			id := value(response, "ServerId")
			if id == "" {
				t.Fatal(response)
			}
			web := f.call(0, "GET", "/api/v1/calendar/events/"+id, "", 200)
			var event struct {
				Summary   string `json:"summary"`
				Attendees []struct {
					Email string `json:"email"`
				} `json:"attendees"`
			}
			if err := json.Unmarshal(web.Body.Bytes(), &event); err != nil || event.Summary != "iPhone invitation" || len(event.Attendees) != 1 || event.Attendees[0].Email != "reader@example.com" {
				t.Fatal(web.Body.String(), err)
			}
			inbox := f.call(1, "GET", "/api/v1/calendar/invites?all=1", "", 200)
			var data struct {
				Invites []struct {
					Summary  string `json:"summary"`
					CanReply bool   `json:"can_reply"`
					ObjectID string `json:"calendar_object_id"`
				} `json:"invites"`
			}
			if err := json.Unmarshal(inbox.Body.Bytes(), &data); err != nil || len(data.Invites) != 1 || !data.Invites[0].CanReply || data.Invites[0].Summary != event.Summary || data.Invites[0].ObjectID == "" {
				t.Fatal(inbox.Body.String(), err)
			}
			f.call(1, "GET", "/api/v1/calendar/events/"+data.Invites[0].ObjectID, "", 200)
		})
	}
}
