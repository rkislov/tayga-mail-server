package calutil

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRequestICSRoundtrip(t *testing.T) {
	start := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	ev := Event{
		UID: "meet-1", Summary: "Sync", Location: "Room A", Description: "Agenda",
		Start: &start, End: &end, Sequence: 0, Status: "CONFIRMED",
		Organizer: Attendee{Email: "org@example.com", Name: "Org"},
		Attendees: []Attendee{
			{Email: "a@example.com", Name: "Alice", PartStat: "NEEDS-ACTION", RSVP: true},
			{Email: "ext@other.com", PartStat: "NEEDS-ACTION", RSVP: true},
		},
	}
	ics, err := BuildRequestICS(ev)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ics, "METHOD:REQUEST") {
		t.Fatalf("missing METHOD:REQUEST:\n%s", ics)
	}
	if !strings.Contains(ics, "ORGANIZER") || !strings.Contains(strings.ToLower(ics), "mailto:org@example.com") {
		t.Fatalf("missing organizer:\n%s", ics)
	}
	if !strings.Contains(ics, "ATTENDEE") || !strings.Contains(ics, "PARTSTAT=NEEDS-ACTION") {
		t.Fatalf("missing attendee partstat:\n%s", ics)
	}
	if !strings.Contains(ics, "SEQUENCE:0") {
		t.Fatalf("missing sequence:\n%s", ics)
	}
	parsed, err := ParseICS(ics)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Method != "REQUEST" {
		t.Fatalf("method=%q", parsed.Method)
	}
	if parsed.UID != "meet-1" || parsed.Summary != "Sync" {
		t.Fatalf("uid/summary: %+v", parsed)
	}
	if parsed.Organizer.Email != "org@example.com" {
		t.Fatalf("organizer=%q", parsed.Organizer.Email)
	}
	atts := ParseAttendees(ics)
	if len(atts) != 2 {
		t.Fatalf("attendees=%d", len(atts))
	}
}

func TestBuildReplyAndCounter(t *testing.T) {
	start := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	ev := Event{
		UID: "meet-2", Summary: "Review", Start: &start, End: &end,
		Organizer: Attendee{Email: "org@example.com"},
	}
	reply, err := BuildReplyICS(ev, Attendee{Email: "a@example.com", PartStat: "ACCEPTED"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "METHOD:REPLY") || !strings.Contains(reply, "PARTSTAT=ACCEPTED") {
		t.Fatalf("bad reply:\n%s", reply)
	}
	ns := start.Add(time.Hour)
	ne := ns.Add(30 * time.Minute)
	ev.Start, ev.End = &ns, &ne
	counter, err := BuildCounterICS(ev, Attendee{Email: "a@example.com", PartStat: "TENTATIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(counter, "METHOD:COUNTER") {
		t.Fatalf("bad counter:\n%s", counter)
	}
	p, _ := ParseICS(counter)
	if p.Start == nil || !p.Start.UTC().Equal(ns) {
		t.Fatalf("counter start=%v want %v", p.Start, ns)
	}
}

func TestBuildIMIPHasMethodInCalendar(t *testing.T) {
	start := time.Now().UTC().Truncate(time.Minute)
	end := start.Add(time.Hour)
	ics, err := BuildRequestICS(Event{
		UID: "x", Summary: "S", Start: &start, End: &end,
		Organizer: Attendee{Email: "o@e.com"},
		Attendees: []Attendee{{Email: "a@e.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ics, "METHOD:REQUEST") {
		t.Fatal("expected METHOD:REQUEST")
	}
}

func TestBuildVEVENTWithAttachments(t *testing.T) {
	start := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	ics, err := BuildSimpleVEVENT(Event{
		UID: "att-1", Summary: "With files", Start: &start, End: &end,
		Attachments: []Attachment{
			{URI: "https://mail.example/api/v1/calendar/attachments/abc", Filename: "agenda.pdf", ContentType: "application/pdf"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ics, "ATTACH") || !strings.Contains(ics, "agenda.pdf") {
		t.Fatalf("missing ATTACH:\n%s", ics)
	}
	p, err := ParseICS(ics)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Attachments) != 1 || p.Attachments[0].Filename != "agenda.pdf" {
		t.Fatalf("%+v", p.Attachments)
	}
}
