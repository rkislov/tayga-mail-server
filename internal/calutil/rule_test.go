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
	for _, raw := range []string{"FREQ=MONTHLY;BYDAY=1MO", "FREQ=WEEKLY;WKST=SU", "FREQ=DAILY;BYHOUR=10", "FREQ=DAILY;COUNT=0", "COUNT=2", "FREQ=DAILY;FREQ=WEEKLY"} {
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
