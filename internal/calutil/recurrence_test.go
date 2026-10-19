package calutil

import (
	"strings"
	"testing"
	"time"
)

func recurrenceTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

const instanceSeries = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:weekly
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261019T100000
DTEND;TZID=Europe/Berlin:20261019T110000
SUMMARY:Planning
SEQUENCE:4
RRULE:FREQ=WEEKLY;COUNT=5
EXDATE;TZID=Europe/Berlin:20261102T100000,20261109T100000
RDATE;TZID=Europe/Berlin:20261117T100000,20261118T100000
ORGANIZER:mailto:owner@example.com
ATTENDEE;PARTSTAT=ACCEPTED:mailto:reader@example.com
X-PRIVATE:keep
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Reminder
TRIGGER:-PT15M
END:VALARM
END:VEVENT
END:VCALENDAR
`

func TestExpandAndChangeOccurrenceAcrossDST(t *testing.T) {
	from, to := recurrenceTime("2026-10-01T00:00:00Z"), recurrenceTime("2026-12-01T00:00:00Z")
	rows, err := ExpandOccurrences(instanceSeries, from, to)
	if err != nil || len(rows) != 5 {
		t.Fatalf("%d %v", len(rows), err)
	}
	if rows[0].Start.Hour() != 10 || rows[0].Start.UTC().Hour() != 8 || rows[1].Start.UTC().Hour() != 9 {
		t.Fatalf("DST: %+v", rows)
	}
	rid := "2026-10-26T09:00:00Z"
	start, end := recurrenceTime("2026-12-08T12:00:00Z"), recurrenceTime("2026-12-08T13:30:00Z")
	summary := "Moved planning"
	full, wire, instance, err := ChangeOccurrence(instanceSeries, rid, OccurrenceChange{Start: &start, End: &end, Summary: &summary})
	if err != nil {
		t.Fatal(err)
	}
	if instance.RecurrenceID != rid || instance.Sequence != 5 || !instance.Start.Equal(start) {
		t.Fatalf("%+v", instance)
	}
	master, _ := ParseICS(full)
	if master.Sequence != 4 || master.Summary != "Planning" || master.Start.UTC().Hour() != 8 {
		t.Fatalf("master changed %+v", master)
	}
	for _, needle := range []string{"RRULE:FREQ=WEEKLY;COUNT=5", "EXDATE;TZID=Europe/Berlin:20261102T100000,20261109T100000", "RECURRENCE-ID;TZID=Europe/Berlin:20261026T100000", "X-PRIVATE:keep"} {
		if !strings.Contains(full, needle) {
			t.Fatalf("lost %s in %s", needle, full)
		}
	}
	if strings.Count(wire, "BEGIN:VEVENT") != 1 || strings.Contains(wire, "RRULE:") || strings.Contains(wire, "VALARM") || !strings.Contains(wire, "METHOD:REQUEST") {
		t.Fatal(wire)
	}
	rows, err = ExpandOccurrences(full, from, to)
	if err != nil || len(rows) != 4 {
		t.Fatalf("old window %d %v", len(rows), err)
	}
	rows, err = ExpandOccurrences(full, start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil || len(rows) != 1 || rows[0].RecurrenceID != rid {
		t.Fatalf("moved into window %+v %v", rows, err)
	}
	if _, err := GetOccurrence(full, start.Format(time.RFC3339)); err == nil {
		t.Fatal("moved start became identity")
	}
	full, wire, instance, err = ChangeOccurrence(full, rid, OccurrenceChange{Cancel: true})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Sequence != 6 || !strings.Contains(wire, "METHOD:CANCEL") || strings.Count(wire, "BEGIN:VEVENT") != 1 {
		t.Fatal(wire)
	}
	rows, err = ExpandOccurrences(full, start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil || len(rows) != 0 {
		t.Fatalf("cancelled still visible %v %v", rows, err)
	}
	rows, err = ExpandOccurrences(full, from, to)
	if err != nil || len(rows) != 4 {
		t.Fatalf("other instances changed %v %v", rows, err)
	}
	if _, _, _, err := ChangeOccurrence(full, rid, OccurrenceChange{Cancel: true}); err == nil {
		t.Fatal("cancel replay accepted")
	}
	if _, _, _, err := ChangeOccurrence(instanceSeries, "2026-11-02T09:00:00Z", OccurrenceChange{Cancel: true}); err == nil {
		t.Fatal("excluded date editable")
	}
}

func TestAllDayRDateOnlyAndRecipientMerge(t *testing.T) {
	data := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VEVENT
UID:dates
DTSTAMP:20261001T000000Z
DTSTART;VALUE=DATE:20261019
DTEND;VALUE=DATE:20261020
RDATE;VALUE=DATE:20261020,20261021
EXDATE;VALUE=DATE:20261020
SUMMARY:Day
END:VEVENT
END:VCALENDAR
`
	from, to := recurrenceTime("2026-10-01T00:00:00Z"), recurrenceTime("2026-11-01T00:00:00Z")
	rows, err := ExpandOccurrences(data, from, to)
	if err != nil || len(rows) != 2 || rows[1].RecurrenceID != "2026-10-21" {
		t.Fatalf("%+v %v", rows, err)
	}
	start, end := recurrenceTime("2026-10-23T00:00:00Z"), recurrenceTime("2026-10-25T00:00:00Z")
	full, wire, _, err := ChangeOccurrence(data, "2026-10-21", OccurrenceChange{Start: &start, End: &end})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wire, "RECURRENCE-ID;VALUE=DATE:20261021") || !strings.Contains(wire, "DTEND;VALUE=DATE:20261025") {
		t.Fatal(wire)
	}
	copy, err := MergeOccurrence(strings.ReplaceAll(data, "SUMMARY:Day", "SUMMARY:Personal title"), wire)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := ParseICS(copy)
	if parsed.Summary != "Personal title" {
		t.Fatal("recipient master overwritten")
	}
	rows, err = ExpandOccurrences(full, from, to)
	if err != nil || len(rows) != 2 || rows[1].End.Sub(*rows[1].Start) != 48*time.Hour {
		t.Fatalf("%+v %v", rows, err)
	}
	start = start.Add(time.Hour)
	if _, _, _, err := ChangeOccurrence(data, "2026-10-21", OccurrenceChange{Start: &start, End: &end}); err == nil {
		t.Fatal("silently discarded all-day hours")
	}
}

func TestRecurrenceLimitsAndDetachedRange(t *testing.T) {
	from, to := recurrenceTime("2026-10-01T00:00:00Z"), recurrenceTime("2026-12-01T00:00:00Z")
	if _, err := ExpandOccurrences(strings.Replace(instanceSeries, "FREQ=WEEKLY;COUNT=5", "FREQ=SECONDLY;COUNT=2000000", 1), from, to); err == nil {
		t.Fatal("dense rule accepted")
	}
	ranged := strings.Replace(instanceSeries, "RRULE:FREQ=WEEKLY;COUNT=5", "RECURRENCE-ID;RANGE=THISANDFUTURE;TZID=Europe/Berlin:20261019T100000", 1)
	if _, err := ExpandOccurrences(ranged, from, to); err == nil {
		t.Fatal("range silently treated as one exception")
	}
}

func TestSeriesRevisionFollowsNewerExceptions(t *testing.T) {
	full, _, _, err := ChangeOccurrence(instanceSeries, "2026-10-26T09:00:00Z", OccurrenceChange{Cancel: true})
	if err != nil {
		t.Fatal(err)
	}
	full, _, second, err := ChangeOccurrence(full, "2026-11-16T09:00:00Z", OccurrenceChange{Cancel: true})
	if err != nil || second.Sequence != 6 || NextSequence(full) != 7 {
		t.Fatalf("version %d %v", second.Sequence, err)
	}
	master, _ := ParseICS(full)
	if master.Sequence != 4 {
		t.Fatal("master version rewritten")
	}
	// Cancelling the whole series must also suppress detached non-cancelled events.
	start, end := recurrenceTime("2026-12-08T12:00:00Z"), recurrenceTime("2026-12-08T13:00:00Z")
	full, _, _, err = ChangeOccurrence(instanceSeries, "2026-10-26T09:00:00Z", OccurrenceChange{Start: &start, End: &end})
	if err != nil {
		t.Fatal(err)
	}
	full = strings.Replace(full, "SUMMARY:Planning", "STATUS:CANCELLED\r\nSUMMARY:Planning", 1)
	rows, err := ExpandOccurrences(full, start.Add(-time.Hour), end.Add(time.Hour))
	if err != nil || len(rows) != 0 {
		t.Fatalf("cancelled series projected: %+v %v", rows, err)
	}
}
