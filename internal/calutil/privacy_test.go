package calutil

import (
	"strings"
	"testing"
)

func TestBusyOnlyRemovesPersonalData(t *testing.T) {
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:secret\r\nBEGIN:VEVENT\r\nUID:private-id\r\nDTSTAMP:20261009T060000Z\r\nDTSTART:20261009T070000Z\r\nDTEND:20261009T080000Z\r\nRRULE:FREQ=WEEKLY;COUNT=3\r\nSUMMARY:secret title\r\nLOCATION:secret place\r\nDESCRIPTION:secret notes\r\nGEO:55.7;37.6\r\nATTENDEE:mailto:secret@example.com\r\nATTACH:https://secret.example/file\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:secret alarm\r\nTRIGGER:-PT10M\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	result, err := BusyOnlyICS(data, "opaque")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"secret", "private-id", "ATTENDEE", "ATTACH", "GEO:", "VALARM"} {
		if strings.Contains(result, value) {
			t.Fatal("leaked " + value)
		}
	}
	if !strings.Contains(result, "RRULE:FREQ=WEEKLY;COUNT=3") || !strings.Contains(result, "SUMMARY:Busy") {
		t.Fatal(result)
	}
}
