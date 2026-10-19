package calutil

import (
	"strings"
	"testing"
)

func TestOrganizerClientEdits(t *testing.T) {
	old, err := WithAttendeeStatus(recurringFixture, "reader@example.com", "ACCEPTED")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, from, to string
		changed        bool
	}{
		{"echo", "nothing", "nothing", false},
		{"alarm", "-PT15M", "-PT30M", false},
		{"client", "-//Test//EN", "-//Phone//EN", false},
		{"sequence only", "SEQUENCE:7", "SEQUENCE:8", false},
		{"stale response", "PARTSTAT=ACCEPTED", "PARTSTAT=NEEDS-ACTION", false},
		{"default organizer type", "ORGANIZER:", "ORGANIZER;VALUE=CAL-ADDRESS;SCHEDULE-AGENT=SERVER:", false},
		{"location", "SUMMARY:Weekly meeting", "SUMMARY:Weekly meeting\r\nLOCATION:New room", true},
		{"instance", "SUMMARY:Moved instance", "SUMMARY:Changed instance", true},
		{"recurrence", "COUNT=5", "COUNT=6", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.ReplaceAll(old, tc.from, tc.to)
			out, changed, err := PrepareOrganizerWrite(input, old, "owner@example.com")
			if err != nil || changed != tc.changed {
				t.Fatalf("changed=%v err=%v", changed, err)
			}
			p, err := ParseICS(out)
			if err != nil {
				t.Fatal(err)
			}
			seq, status := 7, "ACCEPTED"
			if changed {
				seq, status = 8, "NEEDS-ACTION"
			}
			if p.Sequence != seq || p.Attendees[0].PartStat != status {
				t.Fatalf("sequence/status: %+v", p.Event)
			}
			for _, required := range []string{"BEGIN:VTIMEZONE", "RECURRENCE-ID", "X-CUSTOM:value", "X-TEST=retained", "BEGIN:VALARM"} {
				if !strings.Contains(out, required) {
					t.Fatal("lost", required)
				}
			}
		})
	}
}

func TestSchedulingMailExcludesPersonalData(t *testing.T) {
	data := strings.Replace(recurringFixture, "ATTENDEE;", "ATTENDEE;SCHEDULE-AGENT=SERVER;SCHEDULE-STATUS=1.2;", 1)
	p, _ := ParseICS(data)
	data, _ = BuildRequestICS(p.Event)
	wire, err := SchedulingMailICS(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"VALARM", "SCHEDULE-AGENT", "SCHEDULE-STATUS"} {
		if strings.Contains(wire, forbidden) {
			t.Fatal("private/DAV data in mail", forbidden)
		}
	}
	for _, want := range []string{"METHOD:REQUEST", "RECURRENCE-ID", "VTIMEZONE", "RRULE:", "ATTACH;"} {
		if !strings.Contains(wire, want) {
			t.Fatal("lost", want)
		}
	}
}

func TestOrganizerClientIdentityAndScheduleAgent(t *testing.T) {
	personal := strings.ReplaceAll(recurringFixture, "ORGANIZER:mailto:owner@example.com\n", "")
	if _, changed, err := PrepareOrganizerWrite(recurringFixture, personal, "owner@example.com"); err != nil || !changed {
		t.Fatal("cannot invite from an existing personal event", err)
	}
	for _, input := range []string{
		strings.ReplaceAll(recurringFixture, "UID:series", "UID:other"),
		strings.ReplaceAll(recurringFixture, "owner@example.com", "other@example.com"),
		strings.Replace(recurringFixture, "owner@example.com", "other@example.com", 1),
		strings.Replace(recurringFixture, "ATTENDEE;", "ATTENDEE;SCHEDULE-AGENT=BAD;", 1),
	} {
		if _, _, err := PrepareOrganizerWrite(input, recurringFixture, "owner@example.com"); err == nil {
			t.Fatal("bad identity accepted")
		}
	}
	if _, changed, err := PrepareOrganizerWrite(recurringFixture, "", "reader@example.com"); err != nil || changed {
		t.Fatal("recipient fan-out", changed, err)
	}
	for _, agent := range []string{"CLIENT", "NONE", "SERVER"} {
		input := strings.ReplaceAll(recurringFixture, "ATTENDEE;", "ATTENDEE;SCHEDULE-AGENT="+agent+";")
		out, changed, err := PrepareOrganizerWrite(input, "", "owner@example.com")
		if err != nil || !changed {
			t.Fatal(err)
		}
		p, _ := ParseICS(out)
		wire, err := BuildRequestICS(p.Event)
		if err != nil || !strings.Contains(wire, "SCHEDULE-AGENT="+agent) {
			t.Fatal("agent lost", err)
		}
		a, err := SchedulingAttendees(wire)
		if err != nil || (len(a) > 0) != (agent == "SERVER") {
			t.Fatal("wrong recipients", a, err)
		}
	}
}
