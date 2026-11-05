// SPDX-License-Identifier: Apache-2.0
package calutil

import (
	"strings"
	"testing"
)

func TestWithoutFileAttachmentPreservesSeriesAndUnrelatedFiles(t *testing.T) {
	source := strings.ReplaceAll(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Tayga//Test//EN
BEGIN:VEVENT
UID:delete-file-series
DTSTAMP:20261105T090000Z
DTSTART;TZID=Europe/Moscow:20261110T120000
DTEND;TZID=Europe/Moscow:20261110T130000
RRULE:FREQ=DAILY;COUNT=3
ATTACH;FILENAME=remove.pdf:https://mail.example.com/tayga/api/v1/calendar/attachments/file-one
ATTACH;FILENAME=keep.pdf:https://outside.example/keep.pdf
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:Reminder
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:delete-file-series
DTSTAMP:20261105T090000Z
RECURRENCE-ID;TZID=Europe/Moscow:20261111T120000
DTSTART;TZID=Europe/Moscow:20261111T140000
DTEND;TZID=Europe/Moscow:20261111T150000
ATTACH:/api/v1/calendar/attachments/file-one
ATTACH:/api/v1/calendar/attachments/file-two
X-PRIVATE:preserve
END:VEVENT
END:VCALENDAR
`, "\n", "\r\n")
	result, err := WithoutFileAttachment(source, "file-one")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "file-one") {
		t.Fatal(result)
	}
	for _, property := range []string{"RRULE:FREQ=DAILY;COUNT=3", "DTSTART;TZID=Europe/Moscow:20261110T120000", "RECURRENCE-ID;TZID=Europe/Moscow:20261111T120000", "BEGIN:VALARM", "TRIGGER:-PT15M", "https://outside.example/keep.pdf", "/api/v1/calendar/attachments/file-two", "X-PRIVATE:preserve"} {
		if !strings.Contains(result, property) {
			t.Fatal("lost property", property, result)
		}
	}
	again, err := WithoutFileAttachment(result, "file-one")
	if err != nil || again != result {
		t.Fatal("repeat changed series", err)
	}
}
