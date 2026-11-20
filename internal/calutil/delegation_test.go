package calutil

import (
	"github.com/emersion/go-ical"
	"strings"
	"testing"
	"time"
)

func TestSchedulingSecretaryPreservesPrincipal(t *testing.T) {
	start := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data, err := BuildRequestICS(Event{UID: "delegated", Summary: "Meeting", Start: &start, End: &end, Organizer: Attendee{Email: "owner@example.com"}, Attendees: []Attendee{{Email: "guest@example.com", PartStat: "NEEDS-ACTION"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{"owner@example.com", "guest@example.com"} {
		wire, err := WithSchedulingActor(data, from, "secretary@example.com")
		if err != nil {
			t.Fatal(err)
		}
		cal, err := ical.NewDecoder(strings.NewReader(wire)).Decode()
		if err != nil {
			t.Fatal(err)
		}
		event := MasterEvent(cal)
		count := 0
		for _, key := range []string{"ORGANIZER", "ATTENDEE"} {
			for _, p := range event.Props[key] {
				if strings.EqualFold(p.Value, "mailto:"+from) {
					if p.Params.Get("SENT-BY") != "mailto:secretary@example.com" {
						t.Fatal(p)
					}
					count++
				} else if p.Params.Get("SENT-BY") != "" {
					t.Fatal("attributed another participant", p)
				}
			}
		}
		if count != 1 {
			t.Fatal(wire)
		}
		parsed, err := ParseICS(wire)
		if err != nil || parsed.Organizer.Email != "owner@example.com" || len(parsed.Attendees) != 1 || parsed.Attendees[0].Email != "guest@example.com" {
			t.Fatal(wire, err)
		}
	}
}
