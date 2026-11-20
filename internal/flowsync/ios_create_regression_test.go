package flowsync

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

func TestIOSCreateShape(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	tz, e := calendarTZI("Europe/Moscow", 2026)
	if e != nil {
		t.Fatal(e)
	}
	fields := `<Timezone>` + base64.StdEncoding.EncodeToString(tz) + `</Timezone><AllDayEvent>0</AllDayEvent><BusyStatus>2</BusyStatus><EndTime>20261109T100000Z</EndTime><Location xmlns="AirSyncBase:"><DisplayName>Место</DisplayName><Annotation>Россия</Annotation><Latitude>55.75</Latitude><Longitude>37.61</Longitude></Location><Reminder/><Sensitivity>0</Sensitivity><Subject>Проверка Exchange</Subject><StartTime>20261109T090000Z</StartTime><ClientUid>ios-shape-test</ClientUid><MeetingStatus>1</MeetingStatus><Attendees><Attendee><Name>Bob</Name><Email>bob@example.com</Email><AttendeeType>1</AttendeeType></Attendee></Attendees><Recurrence/>`
	_, e = h.wbCalendar(ctx, u, cal.ID, calendarOp(t, "add", "", fields))
	if e != nil {
		t.Fatal(e)
	}
}

func TestIOSCalendarTimezoneWithoutDST(t *testing.T) {
	tzi, err := calendarTZI("Europe/Moscow", 2026)
	if err != nil {
		t.Fatal(err)
	}
	// iOS may retain an unused daylight bias in a zone with no transitions.
	daylightBias := int32(-60)
	binary.LittleEndian.PutUint32(tzi[168:], uint32(daylightBias))
	zone, err := decodeCalendarTimezone(base64.StdEncoding.EncodeToString(tzi), "Europe/Moscow", 2026)
	if err != nil || zone != "Europe/Moscow" {
		t.Fatal(zone, err)
	}
	ctx, st, u, h, _, _ := syncFixture(t)
	cal, _ := st.GetCalendarByName(ctx, u.ID, "default")
	op := calendarOp(t, "add", "", `<Timezone>`+base64.StdEncoding.EncodeToString(tzi)+`</Timezone><StartTime>20261109T090000Z</StartTime><EndTime>20261109T100000Z</EndTime><Subject>iOS fixed-offset</Subject><ClientUid>ios-nodst</ClientUid><Attendees><Attendee><Email>bob@example.com</Email></Attendee></Attendees><Recurrence/>`)
	id, err := h.wbCalendar(ctx, u, cal.ID, op)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil || obj.DTStart.UTC().Hour() != 9 {
		t.Fatal(obj, err)
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 1 {
		t.Fatal("missing invitation", len(jobs))
	}
}
