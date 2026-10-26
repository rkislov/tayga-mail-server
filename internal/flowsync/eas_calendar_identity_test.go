package flowsync

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
)

func TestCalendar16ClientUIDAndAddAcknowledgement(t *testing.T) {
	for _, version := range []string{"16.0", "16.1"} {
		t.Run(version, func(t *testing.T) {
			ctx, st, user, handler, _, _ := syncFixture(t)
			calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
			call := func(device, key, commands string) *protocolNode {
				t.Helper()
				wire, err := encodeWire(`<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + calendar.ID + `</CollectionId><GetChanges>0</GetChanges>` + commands + `</Collection></Collections></Sync>`)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId="+device+"&DeviceType=iPhone", bytes.NewReader(wire)).WithContext(withUser(ctx, user))
				req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
				req.Header.Set("MS-ASProtocolVersion", version)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				if response.Code != 200 {
					t.Fatal(response.Code, response.Body.String())
				}
				xml, err := decodeWire(response.Body.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				doc, err := parseProtocolXML(xml)
				if err != nil {
					t.Fatal(err)
				}
				return doc.find("Collection")
			}
			fields := `<c:Subject>Новое событие тест</c:Subject><c:StartTime>20261023T130000Z</c:StartTime><c:EndTime>20261023T140000Z</c:EndTime><c:ClientUid>iphone-event-1</c:ClientUid><Location xmlns="AirSyncBase:"><DisplayName>Test place</DisplayName><Latitude>56.8</Latitude><Longitude>60.6</Longitude></Location>`
			add := func(clientID, data string) string {
				return `<Commands><Add><ClientId>` + clientID + `</ClientId><ApplicationData xmlns:c="Calendar:">` + data + `</ApplicationData></Add></Commands>`
			}
			key := call("iphone", "0", "").value("SyncKey")
			response := call("iphone", key, add("first-request", fields))
			ack := response.child("Responses").child("Add")
			id, uid := ack.value("ServerId"), ack.child("ApplicationData").value("UID")
			if id == "" || uid == "" || ack.value("Status") != "1" || ack.value("ClientId") != "first-request" || ack.child("ApplicationData").child("UID").Name.Space != "Calendar:" {
				t.Fatal(response.render())
			}
			if response.child("Commands") != nil {
				t.Fatal("GetChanges=0 produced commands")
			}
			if replay := call("iphone", key, add("first-request", fields)); replay.render() != response.render() {
				t.Fatal("cached retry changed the acknowledgement")
			}
			// A new handler and device/request IDs must still find the same event.
			handler = &easHandler{store: st, ms: handler.ms}
			key = call("iphone-reconnected", "0", "").value("SyncKey")
			response = call("iphone-reconnected", key, add("second-request", fields))
			ack = response.child("Responses").child("Add")
			if ack.value("ServerId") != id || ack.child("ApplicationData").value("UID") != uid {
				t.Fatal(response.render())
			}
			objects, _ := st.ListCalendarObjects(ctx, calendar.ID)
			if len(objects) != 1 {
				t.Fatalf("repeated Add created %d events", len(objects))
			}
			if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
				t.Fatal("personal event sent invitations")
			}
			// Edit the acknowledged object: one guest, the same ID and coordinates.
			change := `<Commands><Change><ServerId>` + id + `</ServerId><ApplicationData xmlns:c="Calendar:"><c:Attendees><c:Attendee><c:Email>guest@example.net</c:Email><c:Name>Guest</c:Name><c:AttendeeType>1</c:AttendeeType></c:Attendee></c:Attendees></ApplicationData></Change></Commands>`
			response = call("iphone-reconnected", response.value("SyncKey"), change)
			if response.child("Responses").child("Change").value("Status") != "1" {
				t.Fatal(response.render())
			}
			objects, _ = st.ListCalendarObjects(ctx, calendar.ID)
			if len(objects) != 1 || objects[0].ID != id || objects[0].UID != uid {
				t.Fatal("edit replaced the event", objects)
			}
			parsed, _ := calutil.ParseICS(objects[0].Data)
			if len(parsed.Attendees) != 1 || parsed.Organizer.Email != user.Email || parsed.Geo != "56.8;60.6" {
				t.Fatal(objects[0].Data)
			}
			if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 1 {
				t.Fatal("expected one invitation", len(jobs))
			}
			etag := objects[0].ETag
			response = call("iphone-reconnected", response.value("SyncKey"), add("third-request", fields))
			objects, _ = st.ListCalendarObjects(ctx, calendar.ID)
			if len(objects) != 1 || objects[0].ETag != etag {
				t.Fatal("stale Add overwrote participant edit")
			}
			if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
				t.Fatal("replayed Add resent invitation")
			}
			// Identical title/time does not identify an event: distinct ClientUid does.
			call("iphone-reconnected", response.value("SyncKey"), add("fourth-request", strings.ReplaceAll(fields, "iphone-event-1", "iphone-event-2")))
			objects, _ = st.ListCalendarObjects(ctx, calendar.ID)
			if len(objects) != 2 {
				t.Fatal("distinct events were merged")
			}
		})
	}
}

func TestCalendarLegacyAddHasNoApplicationData(t *testing.T) {
	ctx, st, user, handler, _, _ := syncFixture(t)
	calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
	op := calendarOp(t, "add", "", `<UID>legacy-uid</UID><StartTime>20261023T130000Z</StartTime><EndTime>20261023T140000Z</EndTime>`)
	results := handler.applyWritebacks(ctx, user, kindCalendar, calendar.ID, []clientOp{op})
	if len(results) != 1 || results[0].Status != 1 || results[0].ApplicationData != "" {
		t.Fatal(results)
	}
	object, _ := st.GetCalendarObjectByID(ctx, results[0].ServerID)
	if object.UID != "legacy-uid" {
		t.Fatal(object.UID)
	}
}
