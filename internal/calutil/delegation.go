package calutil

import (
	"github.com/emersion/go-ical"
	"strings"
)

// WithSchedulingActor preserves organizer/attendee identity and records the
// secretary using the RFC 5545 SENT-BY parameter on the scheduling sender.
func WithSchedulingActor(data, from, actor string) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", err
	}
	for _, event := range cal.Children {
		if event.Name != "VEVENT" {
			continue
		}
		for _, key := range []string{"ORGANIZER", "ATTENDEE"} {
			for i, p := range event.Props[key] {
				if !strings.EqualFold(strings.TrimPrefix(strings.ToLower(p.Value), "mailto:"), from) {
					continue
				}
				if p.Params == nil {
					p.Params = make(ical.Params)
				}
				p.Params["SENT-BY"] = []string{"mailto:" + actor}
				event.Props[key][i] = p
			}
		}
	}
	return encode(cal)
}
