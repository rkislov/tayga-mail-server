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
