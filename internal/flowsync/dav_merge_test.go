// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"strings"
	"testing"
)

func TestDAVUpdatePreservesUneditedData(t *testing.T) {
	original := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:one\r\nSUMMARY:Old\r\nRRULE:FREQ=WEEKLY\r\nDESCRIPTION:Keep\r\nBEGIN:VALARM\r\nSUMMARY:Alarm\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	result := mergeDAVProperties(original, "BEGIN:VEVENT\r\nSUMMARY:New\r\nEND:VEVENT\r\n", "VEVENT", map[string]bool{"SUMMARY": true})
	for _, want := range []string{"SUMMARY:New", "RRULE:FREQ=WEEKLY", "SUMMARY:Alarm", "TRIGGER:-PT15M", "DESCRIPTION:Keep"} {
		if !strings.Contains(result, want) {
			t.Fatalf("lost %s: %s", want, result)
		}
	}
	contact := "BEGIN:VCARD\r\nFN:Old\r\nEMAIL:first@example.com\r\nEMAIL:second@example.com\r\nPHOTO:keep\r\nEND:VCARD\r\n"
	result = mergeDAVProperties(contact, "BEGIN:VCARD\r\nFN:New\r\nEND:VCARD\r\n", "VCARD", map[string]bool{"FN": true})
	for _, want := range []string{"FN:New", "EMAIL:first@example.com", "EMAIL:second@example.com", "PHOTO:keep"} {
		if !strings.Contains(result, want) {
			t.Fatalf("lost %s", want)
		}
	}
}

func TestCalendarMergeSelectsMasterAfterException(t *testing.T) {
	original := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:series\r\nRECURRENCE-ID:20261026T100000Z\r\nDTSTART:20261026T120000Z\r\nSUMMARY:Exception\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:series\r\nDTSTART;TZID=Europe/Berlin:20261019T100000\r\nDTEND;TZID=Europe/Berlin:20261019T110000\r\nSUMMARY:Master\r\nRRULE:FREQ=WEEKLY\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	merged := mergeDAVProperties(original, "BEGIN:VEVENT\r\nSUMMARY:Renamed\r\nEND:VEVENT\r\n", "VEVENT", map[string]bool{"SUMMARY": true})
	if !strings.Contains(merged, "SUMMARY:Exception") || strings.Contains(merged, "SUMMARY:Master") || !strings.Contains(merged, "DTSTART;TZID=Europe/Berlin:20261019T100000") {
		t.Fatal(merged)
	}
	ev := parseICalEvent(merged)
	if ev.Summary != "Renamed" || ev.Start != "20261019T080000Z" || ev.End != "20261019T090000Z" {
		t.Fatalf("wrong master: %+v", ev)
	}
}
