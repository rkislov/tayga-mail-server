package calutil

import (
	"strings"
	"testing"
	"time"
)

func ruleFixture(t *testing.T, start string, allDay bool) string {
	t.Helper()
	s, err := time.Parse(time.RFC3339, start)
	if err != nil {
		t.Fatal(err)
	}
	e := s.Add(time.Hour)
	if allDay {
		e = s.AddDate(0, 0, 1)
	}
	data, err := BuildSimpleVEVENT(Event{UID: "rule-test", Summary: "Planning", Start: &s, End: &e, AllDay: allDay})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRuleEditorDSTAndUntil(t *testing.T) {
	data := ruleFixture(t, "2026-10-19T08:00:00Z", false)
	data, err := SetRecurrenceTimezone(data, "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	data, err = WithRecurrence(data, RecurrenceRule{Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"MO"}, Until: "2026-11-02"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "UNTIL=20261102T225959Z") {
		t.Fatal(data)
	}
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 2, 0)
	items, err := ExpandOccurrences(data, from, to)
	if err != nil || len(items) != 3 {
		t.Fatalf("%+v %v", items, err)
	}
	for i, item := range items {
		if item.Start.In(time.FixedZone("comparison", 0)).Hour() != []int{8, 9, 9}[i] {
			t.Fatalf("DST lost: %v", item.Start)
		}
	}
	info := ReadRecurrence(data)
	if !info.Editable || info.Timezone != "Europe/Berlin" || info.Rule.Until != "2026-11-02" || info.Rule.Count != 0 {
		t.Fatalf("%+v", info)
	}
}

func TestRuleEditorDateMonthlyAndYearly(t *testing.T) {
	for _, tc := range []struct {
		start, freq, until string
		count              int
		want               []string
	}{
		{"2026-10-19T00:00:00Z", "DAILY", "2026-10-21", 0, []string{"2026-10-19", "2026-10-20", "2026-10-21"}},
		{"2026-01-31T00:00:00Z", "MONTHLY", "", 3, []string{"2026-01-31", "2026-03-31", "2026-05-31"}},
		{"2024-02-29T00:00:00Z", "YEARLY", "", 2, []string{"2024-02-29", "2028-02-29"}},
	} {
		t.Run(tc.freq, func(t *testing.T) {
			data, err := WithRecurrence(ruleFixture(t, tc.start, true), RecurrenceRule{Frequency: tc.freq, Interval: 1, Count: tc.count, Until: tc.until}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if tc.until != "" && !strings.Contains(data, "UNTIL=20261021") {
				t.Fatal(data)
			}
			from, _ := time.Parse(time.RFC3339, tc.start)
			items, err := ExpandOccurrences(data, from, from.AddDate(5, 0, 0))
			if err != nil || len(items) != len(tc.want) {
				t.Fatalf("%+v %v", items, err)
			}
			for i, item := range items {
				if item.Start.Format("2006-01-02") != tc.want[i] {
					t.Fatalf("%v", item.Start)
				}
			}
		})
	}
}

func TestRuleEditorPreservesExceptionsAndAlarms(t *testing.T) {
	rule := RecurrenceRule{Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"MO"}, Count: 8}
	data, err := WithRecurrence(recurringFixture, rule, 8)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"BEGIN:VALARM", "BEGIN:VTIMEZONE", "SUMMARY:Moved instance", "EXDATE", "RDATE", "https://example.com/agenda.pdf", "X-CUSTOM:value", "X-TEST=retained", "RECURRENCE-ID;TZID=Europe/Berlin:20261026T100000"} {
		if !strings.Contains(data, want) {
			t.Fatalf("lost %s", want)
		}
	}
	if strings.Count(data, "SEQUENCE:8") != 2 {
		t.Fatal("versions not updated")
	}
	for _, rule := range []RecurrenceRule{{Interval: 1}, {Frequency: "WEEKLY", Interval: 2, Count: 5}, {Frequency: "WEEKLY", Interval: 1, Count: 1}} {
		if _, err := WithRecurrence(recurringFixture, rule, 8); err == nil {
			t.Fatalf("allowed orphaning with %+v", rule)
		}
	}
}

func TestRuleEditorRejectsInvalidAndComplex(t *testing.T) {
	data := ruleFixture(t, "2026-10-19T10:00:00Z", false)
	for _, rule := range []RecurrenceRule{
		{Frequency: "HOURLY", Interval: 1}, {Frequency: "DAILY", Interval: -1}, {Frequency: "DAILY", Interval: 100},
		{Frequency: "DAILY", Interval: 1, Count: -1}, {Frequency: "DAILY", Interval: 1, Count: 1001},
		{Frequency: "DAILY", Interval: 1, Count: 2, Until: "2026-12-01"}, {Frequency: "DAILY", Interval: 1, Until: "2026-10-18"},
		{Frequency: "DAILY", Interval: 1, Until: "2026-02-30"}, {Frequency: "DAILY", Interval: 1, Weekdays: []string{"MO"}},
		{Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"TU"}}, {Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"MO", "MO"}},
		{Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"MO", "MO\r\nSUMMARY:attack"}},
	} {
		if _, err := WithRecurrence(data, rule, 1); err == nil {
			t.Fatalf("accepted %+v", rule)
		}
	}
	for _, raw := range []string{"FREQ=MONTHLY;BYDAY=MO,TU;BYSETPOS=1", "FREQ=WEEKLY;WKST=SU", "FREQ=DAILY;BYHOUR=10", "FREQ=DAILY;COUNT=0", "COUNT=2", "FREQ=DAILY;FREQ=WEEKLY"} {
		complex := strings.Replace(data, "END:VEVENT", "RRULE:"+raw+"\r\nEND:VEVENT", 1)
		if ReadRecurrence(complex).Editable {
			t.Fatalf("approximated %s", raw)
		}
		if _, err := WithRecurrence(complex, RecurrenceRule{Frequency: "DAILY", Interval: 1}, 1); err == nil {
			t.Fatal("overwrote complex rule")
		}
	}
	if _, err := SetRecurrenceTimezone(data, "Invalid/Timezone"); err == nil {
		t.Fatal("accepted invalid timezone")
	}
}

func TestRuleEditorOrdinalCalendarPatterns(t *testing.T) {
	for _, tc := range []struct {
		name, start, zone string
		rule              RecurrenceRule
		want              []string
	}{
		{"third Tuesday DST", "2026-09-15T08:00:00Z", "Europe/Berlin", RecurrenceRule{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: 3, Count: 3}, []string{"2026-09-15 10:00", "2026-10-20 10:00", "2026-11-17 10:00"}},
		{"last Friday", "2026-10-30T10:00:00Z", "UTC", RecurrenceRule{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"FR"}, WeekOrdinal: -1, Count: 3}, []string{"2026-10-30 10:00", "2026-11-27 10:00", "2026-12-25 10:00"}},
		{"fifth Monday skips missing", "2026-03-30T10:00:00Z", "UTC", RecurrenceRule{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"MO"}, WeekOrdinal: 5, Count: 3}, []string{"2026-03-30 10:00", "2026-06-29 10:00", "2026-08-31 10:00"}},
		{"yearly ordinal stays in month", "2026-10-20T10:00:00Z", "UTC", RecurrenceRule{Frequency: "YEARLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: 3, Count: 3}, []string{"2026-10-20 10:00", "2027-10-19 10:00", "2028-10-17 10:00"}},
		{"last day leap February", "2028-01-31T10:00:00Z", "UTC", RecurrenceRule{Frequency: "MONTHLY", Interval: 1, MonthDay: -1, Count: 3}, []string{"2028-01-31 10:00", "2028-02-29 10:00", "2028-03-31 10:00"}},
		{"yearly last day", "2028-02-29T10:00:00Z", "UTC", RecurrenceRule{Frequency: "YEARLY", Interval: 1, MonthDay: -1, Count: 3}, []string{"2028-02-29 10:00", "2029-02-28 10:00", "2030-02-28 10:00"}},
		{"two month interval", "2026-10-20T10:00:00Z", "UTC", RecurrenceRule{Frequency: "MONTHLY", Interval: 2, Weekdays: []string{"TU"}, WeekOrdinal: 3, Until: "2027-02-16"}, []string{"2026-10-20 10:00", "2026-12-15 10:00", "2027-02-16 10:00"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := SetRecurrenceTimezone(ruleFixture(t, tc.start, false), tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			data, err = WithRecurrence(data, tc.rule, 1)
			if err != nil {
				t.Fatal(err)
			}
			info := ReadRecurrence(data)
			if !info.Editable || info.Rule.WeekOrdinal != tc.rule.WeekOrdinal || info.Rule.MonthDay != tc.rule.MonthDay {
				t.Fatalf("%+v", info)
			}
			// Reopening and saving must retain the exact set of dates.
			roundtrip, err := WithRecurrence(data, info.Rule, 2)
			if err != nil {
				t.Fatal(err)
			}
			from, _ := time.Parse(time.RFC3339, tc.start)
			loc, _ := time.LoadLocation(tc.zone)
			for _, source := range []string{data, roundtrip} {
				items, err := ExpandOccurrences(source, from, from.AddDate(5, 0, 0))
				if err != nil || len(items) != len(tc.want) {
					t.Fatalf("count %d: %v", len(items), err)
				}
				for i, item := range items {
					if got := item.Start.In(loc).Format("2006-01-02 15:04"); got != tc.want[i] {
						t.Fatalf("got %s, want %s", got, tc.want[i])
					}
				}
			}
		})
	}
}

func TestRuleEditorImportedOrdinalAndUnsafePatterns(t *testing.T) {
	data := ruleFixture(t, "2026-10-20T00:00:00Z", true)
	for _, raw := range []string{
		"FREQ=MONTHLY;BYDAY=3TU;COUNT=3",
		"FREQ=MONTHLY;BYDAY=TU;BYSETPOS=3;COUNT=3",
		"FREQ=YEARLY;BYMONTH=10;BYDAY=3TU;COUNT=3",
		"FREQ=YEARLY;BYMONTH=10;BYDAY=TU;BYSETPOS=3;COUNT=3",
		"FREQ=MONTHLY;BYMONTHDAY=20;COUNT=3",
	} {
		imported := strings.Replace(data, "END:VEVENT", "RRULE:"+raw+"\r\nEND:VEVENT", 1)
		info := ReadRecurrence(imported)
		if !info.Editable {
			t.Fatalf("uneditable %s: %+v", raw, info)
		}
		updated, err := WithRecurrence(imported, info.Rule, 1)
		if err != nil {
			t.Fatal(err)
		}
		from := recurrenceTime("2026-10-01T00:00:00Z")
		before, err := ExpandOccurrences(imported, from, from.AddDate(4, 0, 0))
		if err != nil {
			t.Fatal(err)
		}
		after, err := ExpandOccurrences(updated, from, from.AddDate(4, 0, 0))
		if err != nil || len(before) != len(after) {
			t.Fatalf("%s: changed count", raw)
		}
		for i := range before {
			if !before[i].Start.Equal(*after[i].Start) {
				t.Fatalf("%s: changed dates", raw)
			}
		}
	}
	for _, raw := range []string{
		"FREQ=YEARLY;BYDAY=3TU", "FREQ=YEARLY;BYMONTHDAY=20", "FREQ=YEARLY;BYMONTH=11;BYDAY=3TU",
		"FREQ=MONTHLY;BYDAY=MO,TU;BYSETPOS=3", "FREQ=MONTHLY;BYDAY=3TU;BYSETPOS=1",
		"FREQ=MONTHLY;BYSETPOS=3", "FREQ=MONTHLY;BYDAY=0TU", "FREQ=MONTHLY;BYDAY=6TU",
		"FREQ=MONTHLY;BYMONTHDAY=0", "FREQ=MONTHLY;BYMONTHDAY=20,21", "FREQ=MONTHLY;BYMONTH=10;BYDAY=3TU",
	} {
		imported := strings.Replace(data, "END:VEVENT", "RRULE:"+raw+"\r\nEND:VEVENT", 1)
		if ReadRecurrence(imported).Editable {
			t.Fatalf("approximated %s", raw)
		}
	}
	for _, rule := range []RecurrenceRule{
		{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"MO"}, WeekOrdinal: 3},
		{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: 2},
		{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: -1},
		{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"TU", "MO"}, WeekOrdinal: 3},
		{Frequency: "MONTHLY", Interval: 1, WeekOrdinal: 3},
		{Frequency: "MONTHLY", Interval: 1, MonthDay: -1},
		{Frequency: "MONTHLY", Interval: 1, MonthDay: 32},
		{Frequency: "MONTHLY", Interval: 1, MonthDay: -2},
		{Frequency: "MONTHLY", Interval: 1, MonthDay: 20, Weekdays: []string{"TU"}, WeekOrdinal: 3},
		{Frequency: "DAILY", Interval: 1, MonthDay: 20},
		{Frequency: "WEEKLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: 3},
		{Interval: 1, MonthDay: -1},
	} {
		if _, err := WithRecurrence(data, rule, 1); err == nil {
			t.Fatalf("accepted %+v", rule)
		}
	}
}

func TestOrdinalRuleRetainsExceptionsAndRejectsOrphans(t *testing.T) {
	data := ruleFixture(t, "2026-10-20T10:00:00Z", false)
	data = strings.Replace(data, "END:VEVENT", "ATTACH:https://example.com/agenda.pdf\r\nX-CUSTOM:retained\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nTRIGGER:-PT5M\r\nEND:VALARM\r\nEND:VEVENT", 1)
	rule := RecurrenceRule{Frequency: "MONTHLY", Interval: 1, Weekdays: []string{"TU"}, WeekOrdinal: 3, Count: 3}
	data, err := WithRecurrence(data, rule, 1)
	if err != nil {
		t.Fatal(err)
	}
	start, end := recurrenceTime("2026-11-18T10:00:00Z"), recurrenceTime("2026-11-18T11:00:00Z")
	data, _, _, err = ChangeOccurrence(data, "2026-11-17T10:00:00Z", OccurrenceChange{Start: &start, End: &end})
	if err != nil {
		t.Fatal(err)
	}
	rule.Count = 4
	updated, err := WithRecurrence(data, rule, NextSequence(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"BEGIN:VALARM", "ATTACH:https://example.com/agenda.pdf", "X-CUSTOM:retained", "RECURRENCE-ID:20261117T100000Z"} {
		if !strings.Contains(updated, want) {
			t.Fatalf("lost %s", want)
		}
	}
	instance, err := GetOccurrence(updated, "2026-11-17T10:00:00Z")
	if err != nil || !instance.Start.Equal(start) {
		t.Fatalf("lost exception: %+v %v", instance, err)
	}
	rule.Interval = 2
	if _, err := WithRecurrence(data, rule, NextSequence(data)); err == nil {
		t.Fatal("removed edited occurrence")
	}
}

func TestRuleEditorImportedCutoffAndSeparateDates(t *testing.T) {
	data := ruleFixture(t, "2026-10-19T10:00:00Z", false)
	data = strings.Replace(data, "END:VEVENT", "RRULE:FREQ=DAILY;UNTIL=20261020T090000Z\r\nEND:VEVENT", 1)
	info := ReadRecurrence(data)
	if !info.Editable || info.Rule.Until != "2026-10-19" {
		t.Fatalf("cutoff broadened: %+v", info)
	}
	updated, err := WithRecurrence(data, info.Rule, 1)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	for _, source := range []string{data, updated} {
		items, err := ExpandOccurrences(source, start, start.AddDate(0, 0, 3))
		if err != nil || len(items) != 1 {
			t.Fatalf("changed cutoff: %d %v", len(items), err)
		}
	}
	for _, name := range []string{"RDATE", "EXDATE"} {
		data := strings.Replace(ruleFixture(t, "2026-10-19T10:00:00Z", false), "END:VEVENT", name+":20261020T100000Z\r\nEND:VEVENT", 1)
		if ReadRecurrence(data).Editable {
			t.Fatal("misrepresented date-only recurrence set")
		}
	}
}
