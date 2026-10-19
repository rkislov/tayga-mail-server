package calutil

import (
	"strings"
	"testing"

	"github.com/emersion/go-ical"
)

func TestMeetingCopyKeepsRecipientRemindersAndSchedulingData(t *testing.T) {
	previous := strings.Replace(recurringFixture, "-PT15M", "-PT5M", 1)
	previous = strings.Replace(previous, "X-CUSTOM:value", "X-CUSTOM:old\nCATEGORIES:Personal", 1)
	p, err := ParseICS(recurringFixture)
	if err != nil {
		t.Fatal(err)
	}
	p.Summary, p.Sequence = "Updated", 8
	p.Attendees = []Attendee{{Email: "new@example.com", PartStat: "NEEDS-ACTION"}}
	source, err := BuildRequestICS(p.Event)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := MergeMeetingCopy(source, previous)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TRIGGER:-PT5M", "CATEGORIES:Personal", "SUMMARY:Updated", "SUMMARY:Moved instance", "SEQUENCE:8", "RRULE:", "EXDATE", "RDATE", "BEGIN:VTIMEZONE", "X-CUSTOM:value", "agenda.pdf"} {
		if !strings.Contains(copy, want) {
			t.Fatalf("lost %s: %s", want, copy)
		}
	}
	if strings.Contains(copy, "-PT15M") || strings.Contains(copy, "reader@example.com") || strings.Contains(copy, "METHOD:") {
		t.Fatal(copy)
	}
	cal, _ := ical.NewDecoder(strings.NewReader(copy)).Decode()
	for _, event := range cal.Events() {
		if event.Props.Get("ATTENDEE").Value != "mailto:new@example.com" || event.Props.Get("SEQUENCE").Value != "8" {
			t.Fatal("stale detached instance")
		}
	}
	newCopy, err := MergeMeetingCopy(source, "")
	if err != nil || strings.Contains(newCopy, "VALARM") {
		t.Fatal("organizer alarms copied", err)
	}
	// An explicitly alarm-free recipient must remain alarm-free on updates.
	noAlarm, err := MergeMeetingCopy(source, newCopy)
	if err != nil || strings.Contains(noAlarm, "VALARM") {
		t.Fatal("alarm restored", err)
	}
}
