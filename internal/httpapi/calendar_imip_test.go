package httpapi

import (
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

func TestBuildIMIPMIMEContainsMethodRequest(t *testing.T) {
	start := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	ics, err := calutil.BuildRequestICS(calutil.Event{
		UID: "uid-1", Summary: "Demo", Start: &start, End: &end,
		Organizer: calutil.Attendee{Email: "org@ex.com"},
		Attendees: []calutil.Attendee{{Email: "a@ex.com", PartStat: "NEEDS-ACTION"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(buildIMIPMIME("org@ex.com", "Org", []string{"a@ex.com"}, "Invitation: Demo", "Please join", ics, "REQUEST"))
	if !strings.Contains(raw, "Content-Type: text/calendar") {
		t.Fatalf("missing calendar part:\n%s", raw)
	}
	if !strings.Contains(raw, "method=REQUEST") {
		t.Fatalf("missing method=REQUEST:\n%s", raw)
	}
	if !strings.Contains(raw, "METHOD:REQUEST") {
		t.Fatalf("missing METHOD:REQUEST body:\n%s", raw)
	}
}
