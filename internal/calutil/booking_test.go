package calutil

import (
	"strings"
	"testing"
	"time"
)

func TestBookingOccurrencesCompleteSeries(t *testing.T) {
	base := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:room\r\nDTSTAMP:20261019T000000Z\r\nDTSTART;TZID=Europe/Berlin:20261019T100000\r\nDTEND;TZID=Europe/Berlin:20261019T110000\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:room@example.com\r\nRRULE:FREQ=WEEKLY;COUNT=3\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	items, err := BookingOccurrences(base)
	if err != nil || len(items) != 3 {
		t.Fatalf("%v %v", items, err)
	}
	if items[0].Start.UTC().Hour() != 8 || items[1].Start.UTC().Hour() != 9 {
		t.Fatal("DST shifted wall time")
	}
	moved := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	end := moved.Add(time.Hour)
	data, _, _, err := ChangeOccurrence(base, "2026-10-26T09:00:00Z", OccurrenceChange{Start: &moved, End: &end})
	if err != nil {
		t.Fatal(err)
	}
	data, _, _, err = ChangeOccurrence(data, "2026-11-02T09:00:00Z", OccurrenceChange{Cancel: true})
	if err != nil {
		t.Fatal(err)
	}
	items, err = BookingOccurrences(data)
	if err != nil || len(items) != 2 || !items[1].Start.Equal(moved) {
		t.Fatalf("exceptions %v %v", items, err)
	}
	data, err = WithResourcePartStats(data, map[string]string{"room@example.com": "ACCEPTED"})
	if err != nil {
		t.Fatal(err)
	}
	items, err = BookingOccurrences(data)
	if err != nil || len(items) != 2 || items[1].Attendees[0].PartStat != "ACCEPTED" {
		t.Fatalf("status %v %v", items, err)
	}
	for _, rule := range []string{"FREQ=WEEKLY", "FREQ=DAILY;COUNT=1001", "FREQ=YEARLY;COUNT=10", "FREQ=MINUTELY;COUNT=10", "FREQ=DAILY;UNTIL=20991231T090000Z"} {
		if _, err := BookingOccurrences(strings.Replace(base, "FREQ=WEEKLY;COUNT=3", rule, 1)); err == nil {
			t.Fatalf("accepted %s", rule)
		}
	}
}
