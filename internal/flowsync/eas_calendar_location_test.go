package flowsync

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
)

func TestCalendarLocationWBXMLRoundTrip(t *testing.T) {
	ctx, st, user, handler, _, _ := syncFixture(t)
	calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
	call := func(key, commands string) string {
		t.Helper()
		wire, err := encodeWire(`<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + calendar.ID + `</CollectionId>` + commands + `</Collection></Collections></Sync>`)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=test&DeviceType=test", bytes.NewReader(wire)).WithContext(withUser(ctx, user))
		req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		req.Header.Set("MS-ASProtocolVersion", "16.1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		xml, err := decodeWire(response.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		return string(xml)
	}
	key := extractTag(call("0", ""), "SyncKey")
	fields := `<c:Subject xmlns:c="Calendar:">Meeting with a pin</c:Subject><c:StartTime xmlns:c="Calendar:">20261023T161000Z</c:StartTime><c:EndTime xmlns:c="Calendar:">20261023T171000Z</c:EndTime><Location xmlns="AirSyncBase:"><DisplayName>Airport &amp; terminal</DisplayName><Latitude>55.9726</Latitude><Longitude>37.4146</Longitude></Location>`
	response := call(key, `<Commands><Add><ClientId>iphone-pin</ClientId><ApplicationData>`+fields+`</ApplicationData></Add></Commands>`)
	objects, err := st.ListCalendarObjects(ctx, calendar.ID)
	if err != nil || len(objects) != 1 {
		t.Fatal(objects, err, response)
	}
	parsed, _ := calutil.ParseICS(objects[0].Data)
	if parsed.Geo != "55.9726;37.4146" || parsed.Location != "Airport & terminal" {
		t.Fatal(objects[0].Data)
	}
	// Read it back through the real 16.1 handler, not just the serializer.
	key = extractTag(call("0", ""), "SyncKey")
	response = call(key, "")
	doc, _ := parseProtocolXML([]byte(response))
	location := doc.find("Location")
	if location == nil || location.Name.Space != "AirSyncBase:" || location.value("Latitude") != "55.9726" || location.value("Longitude") != "37.4146" || location.value("DisplayName") != parsed.Location {
		t.Fatal(response)
	}
	// A body-only update retains the point. Empty Location explicitly removes it.
	for _, change := range []struct{ fields, geo, place string }{
		{`<Subject>Updated meeting</Subject>`, "55.9726;37.4146", parsed.Location},
		{`<Location xmlns="AirSyncBase:"/>`, "", ""},
	} {
		if _, err = handler.wbCalendar(ctx, user, calendar.ID, calendarOp(t, "change", objects[0].ID, change.fields)); err != nil {
			t.Fatal(err)
		}
		object, _ := st.GetCalendarObjectByID(ctx, objects[0].ID)
		p, _ := calutil.ParseICS(object.Data)
		if p.Geo != change.geo || p.Location != change.place {
			t.Fatal(object.Data)
		}
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("personal event sent invitations")
	}
}

func TestCalendarLocationValidationAndLegacy(t *testing.T) {
	ctx, st, user, handler, _, _ := syncFixture(t)
	calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
	for _, coordinates := range []string{
		`<Latitude>55</Latitude>`, `<Latitude>NaN</Latitude><Longitude>37</Longitude>`,
		`<Latitude>91</Latitude><Longitude>37</Longitude>`, `<Latitude>0</Latitude><Longitude>Inf</Longitude>`,
	} {
		_, err := handler.wbCalendar(ctx, user, calendar.ID, calendarOp(t, "add", "", `<Subject>Invalid</Subject><StartTime>20261023T161000Z</StartTime><EndTime>20261023T171000Z</EndTime><Location xmlns="AirSyncBase:">`+coordinates+`</Location>`))
		if err == nil {
			t.Fatal("accepted invalid coordinates", coordinates)
		}
	}
	objects, _ := st.ListCalendarObjects(ctx, calendar.ID)
	if len(objects) != 0 {
		t.Fatal("partial write", objects)
	}
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:point\r\nDTSTART:20261023T161000Z\r\nDTEND:20261023T171000Z\r\nSUMMARY:Point\r\nLOCATION:Old place\r\nGEO:0;0\r\nX-APPLE-STRUCTURED-LOCATION:geo:0,0\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	for _, change := range []struct{ fields, geo, place string }{
		{`<Location>Old place</Location>`, "0;0", "Old place"},
		{`<Location>New place</Location>`, "", "New place"},
		{`<Location xmlns="AirSyncBase:"><City>Moscow</City><Street>Street 1</Street><Latitude>0</Latitude><Longitude>0</Longitude></Location>`, "0;0", "Street 1, Moscow"},
	} {
		updated, err := editEASCalendar(data, user, calendarOp(t, "change", "", change.fields))
		if err != nil {
			t.Fatal(err)
		}
		p, _ := calutil.ParseICS(updated)
		if p.Geo != change.geo || p.Location != change.place {
			t.Fatal(updated)
		}
		if change.place != "Old place" && strings.Contains(updated, "X-APPLE-STRUCTURED-LOCATION") {
			t.Fatal("old Apple pin retained")
		}
	}
	for _, version := range []string{"14.1", "16.0", "16.1"} {
		location := calendarLocationXML(context.WithValue(ctx, easVersionKey{}, version), data)
		if version == "14.1" && location != "" {
			t.Fatal("16.x property on legacy client")
		}
		if version != "14.1" && (!strings.Contains(location, "<Latitude>0</Latitude>") || !strings.Contains(location, "<Longitude>0</Longitude>")) {
			t.Fatal(location)
		}
	}
}
