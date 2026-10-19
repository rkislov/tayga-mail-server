package calutil

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

// Exception deliberately comes first, as permitted by CalDAV.
const recurringFixture = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//EN
BEGIN:VTIMEZONE
TZID:Europe/Berlin
BEGIN:STANDARD
DTSTART:20261025T030000
TZOFFSETFROM:+0200
TZOFFSETTO:+0100
END:STANDARD
END:VTIMEZONE
BEGIN:VEVENT
UID:series
DTSTAMP:20261018T000000Z
RECURRENCE-ID;TZID=Europe/Berlin:20261026T100000
DTSTART;TZID=Europe/Berlin:20261026T120000
DTEND;TZID=Europe/Berlin:20261026T130000
SUMMARY:Moved instance
ORGANIZER:mailto:owner@example.com
ATTENDEE;PARTSTAT=NEEDS-ACTION;X-TEST=retained:mailto:reader@example.com
SEQUENCE:7
END:VEVENT
BEGIN:VEVENT
UID:series
DTSTAMP:20261018T000000Z
DTSTART;TZID=Europe/Berlin:20261019T100000
DTEND;TZID=Europe/Berlin:20261019T110000
RRULE:FREQ=WEEKLY;COUNT=5
EXDATE;TZID=Europe/Berlin:20261102T100000
RDATE;TZID=Europe/Berlin:20261105T100000
SUMMARY:Weekly meeting
ORGANIZER:mailto:owner@example.com
ATTENDEE;PARTSTAT=NEEDS-ACTION;X-TEST=retained:mailto:reader@example.com
ATTACH;FMTTYPE=application/pdf:https://example.com/agenda.pdf
SEQUENCE:7
X-CUSTOM:value
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:Reminder
END:VALARM
END:VEVENT
END:VCALENDAR
`

func TestEditPreservesRecurrenceAndTimezone(t *testing.T) {
	p, err := ParseICS(recurringFixture)
	if err != nil || p.Summary != "Weekly meeting" || p.Start.Hour() != 10 || p.Start.Location().String() != "Europe/Berlin" || p.End.Hour() != 11 {
		t.Fatalf("master: %+v %v", p, err)
	}
	p.Summary = "Updated title"
	data, err := BuildSimpleVEVENT(p.Event)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"RRULE:FREQ=WEEKLY;COUNT=5", "EXDATE;TZID=Europe/Berlin:20261102T100000", "RDATE;TZID=Europe/Berlin:20261105T100000", "RECURRENCE-ID;TZID=Europe/Berlin:20261026T100000", "SUMMARY:Moved instance", "BEGIN:VTIMEZONE", "BEGIN:VALARM", "X-CUSTOM:value", "X-TEST=retained", "https://example.com/agenda.pdf", "DTSTART;TZID=Europe/Berlin:20261019T100000"} {
		if !strings.Contains(data, want) {
			t.Fatalf("lost %s:\n%s", want, data)
		}
	}
	// RFC3339 input is still written in the original local timezone.
	start := p.Start.Add(time.Hour).UTC()
	end := p.End.Add(time.Hour).UTC()
	p.Start, p.End = &start, &end
	data, err = BuildRequestICS(p.Event)
	if err != nil || !strings.Contains(data, "DTSTART;TZID=Europe/Berlin:20261019T110000") {
		t.Fatalf("%s %v", data, err)
	}
}

func TestRSVPPreservesSeriesAndSequence(t *testing.T) {
	data, err := WithAttendeeStatus(recurringFixture, "READER@example.com", "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(data, "PARTSTAT=ACCEPTED") != 2 || strings.Count(data, "SEQUENCE:7") != 2 || !strings.Contains(data, "X-TEST=retained") || !strings.Contains(data, "BEGIN:VALARM") {
		t.Fatal(data)
	}
	p, _ := ParseICS(data)
	reply, err := BuildReplyICS(p.Event, Attendee{Email: "reader@example.com", PartStat: "ACCEPTED"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(reply, "BEGIN:VEVENT") != 1 || strings.Contains(reply, "BEGIN:VALARM") || !strings.Contains(reply, "SEQUENCE:7") || !strings.Contains(reply, "RRULE:") {
		t.Fatal(reply)
	}
	if _, err := WithAttendeeStatus(data, "stranger@example.com", "ACCEPTED"); err == nil {
		t.Fatal("uninvited attendee accepted")
	}
}

func TestAllDayAndDurationRoundtrip(t *testing.T) {
	for _, dates := range []string{"DTSTART;VALUE=DATE:20261019\nDTEND;VALUE=DATE:20261021", "DTSTART;VALUE=DATE:20261019\nDURATION:P2D"} {
		p, err := ParseICS("BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:days\n" + dates + "\nSUMMARY:Days\nEND:VEVENT\nEND:VCALENDAR\n")
		if err != nil || !p.AllDay || p.Start == nil || p.End == nil || p.End.Sub(*p.Start) != 48*time.Hour {
			t.Fatalf("%+v %v", p, err)
		}
		p.Summary = "Renamed"
		data, err := BuildSimpleVEVENT(p.Event)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(dates, "\n") {
			if !strings.Contains(data, line) {
				t.Fatalf("missing %s: %s", line, data)
			}
		}
	}
}

func TestCancellationIncludesExceptions(t *testing.T) {
	p, _ := ParseICS(recurringFixture)
	p.Sequence++
	data, err := BuildCancelICS(p.Event)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range cal.Events() {
		status, _ := e.Props.Text("STATUS")
		seq, _ := e.Props.Get("SEQUENCE").Int()
		if status != "CANCELLED" || seq != 8 {
			t.Fatal(data)
		}
	}
}
