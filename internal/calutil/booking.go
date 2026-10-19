package calutil

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// BookingOccurrences enumerates the complete booking, never a rolling window.
// Limits apply only to resource reservations, not ordinary personal calendars.
func BookingOccurrences(data string) ([]Occurrence, error) {
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return nil, err
	}
	status, _ := master.Props.Text("STATUS")
	if strings.EqualFold(status, "CANCELLED") {
		return nil, nil
	}
	event := ical.Event{Component: master}
	start, err := event.DateTimeStart(time.UTC)
	if err != nil {
		return nil, err
	}
	end, err := event.DateTimeEnd(time.UTC)
	if err != nil || !end.After(start) || end.Sub(start) > 31*24*time.Hour {
		return nil, fmt.Errorf("resource booking requires a positive duration")
	}
	from, to := start, end
	limit := start.AddDate(5, 0, 0)
	if master.Props.Get("RRULE") != nil {
		opt, err := master.Props.RecurrenceRule()
		if err != nil {
			return nil, err
		}
		if opt.Count == 0 && opt.Until.IsZero() {
			return nil, fmt.Errorf("resource booking requires a recurrence end date or count")
		}
		if opt.Count > 1000 || !opt.Until.IsZero() && opt.Until.After(limit) {
			return nil, fmt.Errorf("resource booking is limited to 1000 occurrences within 5 years")
		}
	}
	set, err := recurrenceSet(master, start)
	if err != nil {
		return nil, err
	}
	next := set.Iterator()
	for n := 0; ; n++ {
		date, ok := next()
		if !ok {
			break
		}
		if n >= 1000 || date.After(limit) {
			return nil, fmt.Errorf("resource booking is limited to 1000 occurrences within 5 years")
		}
		if date.Before(from) {
			from = date
		}
		if last := date.Add(end.Sub(start)); last.After(to) {
			to = last
		}
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" || c.Props.Get("RECURRENCE-ID") == nil {
			continue
		}
		status, _ := c.Props.Text("STATUS")
		if strings.EqualFold(status, "CANCELLED") {
			continue
		}
		e := ical.Event{Component: c}
		a, e1 := e.DateTimeStart(time.UTC)
		b, e2 := e.DateTimeEnd(time.UTC)
		if e1 != nil || e2 != nil || !b.After(a) || b.Sub(a) > 31*24*time.Hour {
			return nil, fmt.Errorf("invalid resource occurrence interval")
		}
		if a.Before(from) {
			from = a
		}
		if b.After(to) {
			to = b
		}
	}
	if to.After(from.AddDate(5, 0, 0).Add(31 * 24 * time.Hour)) {
		return nil, fmt.Errorf("resource booking is limited to 1000 occurrences within 5 years")
	}
	out, err := ExpandOccurrences(data, from, to)
	if err != nil {
		return nil, err
	}
	if len(out) > 1000 {
		return nil, fmt.Errorf("resource booking is limited to 1000 occurrences within 5 years")
	}
	for _, o := range out {
		if o.Start != nil && len(out) > 0 && o.Start.After(out[0].Start.AddDate(5, 0, 0)) {
			return nil, fmt.Errorf("resource booking is limited to 1000 occurrences within 5 years")
		}
		if o.Start == nil || o.End == nil || !o.End.After(*o.Start) {
			return nil, fmt.Errorf("invalid resource occurrence interval")
		}
	}
	return out, nil
}

// WithResourcePartStats changes only resource response parameters, preserving
// recurrence exceptions, timezones, alarms and other opaque event properties.
func WithResourcePartStats(data string, statuses map[string]string) (string, error) {
	if len(statuses) == 0 {
		return data, nil
	}
	cal, _, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		props := c.Props.Values("ATTENDEE")
		for i := range props {
			uri, err := url.Parse(props[i].Value)
			if err != nil {
				return "", err
			}
			email := strings.ToLower(emailFromURI(uri))
			if status, ok := statuses[email]; ok {
				props[i].Params.Set("PARTSTAT", status)
				props[i].Params.Set("CUTYPE", "RESOURCE")
				props[i].Params.Set("RSVP", "FALSE")
			}
		}
		c.Props["ATTENDEE"] = props
	}
	return encode(cal)
}

// EventAttendees includes detached instances, so resource discovery cannot miss
// a room added in an imported exception.
func EventAttendees(data string) ([]Attendee, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return nil, err
	}
	var out []Attendee
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		wire, err := encode(componentCalendar(cal, c))
		if err != nil {
			return nil, err
		}
		p, err := ParseICS(wire)
		if err != nil {
			return nil, err
		}
		out = append(out, p.Attendees...)
	}
	return out, nil
}
