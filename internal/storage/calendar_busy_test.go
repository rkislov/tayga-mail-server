package storage

import (
	"testing"
	"time"
)

func TestBusyRecurringWithException(t *testing.T) {
	data := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:one
DTSTART:20261002T100000Z
DTEND:20261002T110000Z
RRULE:FREQ=WEEKLY;COUNT=4
EXDATE:20261016T100000Z
END:VEVENT
END:VCALENDAR
`
	from := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	busy, err := objectBusyIntervals(data, from, from.Add(14*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(busy) != 1 || busy[0].Start.Day() != 9 || busy[0].End.Hour() != 11 {
		t.Fatalf("%+v", busy)
	}
}
