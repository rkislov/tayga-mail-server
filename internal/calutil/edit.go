package calutil

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// MasterEvent selects the series master, independently of component order.
// A standalone detached instance is also a valid calendar object.
func MasterEvent(cal *ical.Calendar) *ical.Component {
	var first *ical.Component
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		if first == nil {
			first = c
		}
		if c.Props.Get("RECURRENCE-ID") == nil {
			return c
		}
	}
	return first
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func mergeStoredEvent(ev Event, generated *ical.Calendar, method string) (*ical.Calendar, error) {
	cal, err := ical.NewDecoder(strings.NewReader(ev.source)).Decode()
	if err != nil {
		return nil, err
	}
	if cal.Props.Get("PRODID") == nil {
		cal.Props.SetText("PRODID", ProdID)
	}
	if cal.Props.Get("VERSION") == nil {
		cal.Props.SetText("VERSION", "2.0")
	}
	target, fresh := MasterEvent(cal), MasterEvent(generated)
	if target == nil || fresh == nil {
		return nil, fmt.Errorf("VEVENT required")
	}
	before, err := ParseICS(ev.source)
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{
		"UID": ev.UID != before.UID, "SUMMARY": ev.Summary != before.Summary,
		"LOCATION": ev.Location != before.Location, "GEO": ev.Geo != before.Geo,
		"DESCRIPTION": ev.Description != before.Description, "COMMENT": ev.Comment != before.Comment,
		"STATUS": ev.Status != before.Status, "SEQUENCE": ev.Sequence != before.Sequence,
		"DTSTART":   !sameTime(ev.Start, before.Start) || ev.AllDay != before.AllDay,
		"DTEND":     !sameTime(ev.End, before.End) || ev.AllDay != before.AllDay,
		"ORGANIZER": ev.Organizer != before.Organizer,
		"ATTENDEE":  !reflect.DeepEqual(ev.Attendees, before.Attendees),
		"ATTACH":    !reflect.DeepEqual(ev.Attachments, before.Attachments),
	}
	for name, replace := range changed {
		if !replace {
			continue
		}
		// Preserve local recurrence time across DST when the master is moved.
		if (name == "DTSTART" || name == "DTEND") && !ev.AllDay {
			if old := target.Props.Get(name); old != nil && old.Params.Get("TZID") != "" {
				loc, err := time.LoadLocation(old.Params.Get("TZID"))
				if err != nil {
					return nil, fmt.Errorf("cannot edit time in timezone %q", old.Params.Get("TZID"))
				}
				value := ev.Start
				if name == "DTEND" {
					value = ev.End
				}
				if value != nil {
					p := ical.NewProp(name)
					p.Value = value.In(loc).Format("20060102T150405")
					p.Params.Set("TZID", old.Params.Get("TZID"))
					fresh.Props.Set(p)
				}
			}
		}
		delete(target.Props, name)
		for _, p := range fresh.Props.Values(name) {
			target.Props.Add(&p)
		}
	}
	if changed["DTEND"] {
		delete(target.Props, "DURATION")
	}
	// Editing the whole meeting's participants also updates detached instances.
	// Otherwise a removed participant remains invited by an old exception.
	if method == "REQUEST" || method == "" {
		for _, child := range cal.Children {
			if child.Name != "VEVENT" || child == target {
				continue
			}
			if changed["ATTENDEE"] {
				delete(child.Props, "ATTENDEE")
				for _, a := range ev.Attendees {
					setAttendeeProp(child.Props, a, false)
				}
			}
			if changed["SEQUENCE"] {
				child.Props.Set(&ical.Prop{Name: "SEQUENCE", Value: fmt.Sprint(ev.Sequence)})
			}
		}
	}
	target.Props.SetDateTime("DTSTAMP", time.Now().UTC())
	delete(cal.Props, "METHOD")
	if method != "" {
		cal.Props.SetText("METHOD", method)
	}
	if method == "REPLY" || method == "COUNTER" || method == "DECLINECOUNTER" {
		// A series response carries the master and its timezone definitions,
		// never unrelated exception attendees or private reminder alarms.
		children := []*ical.Component{}
		for _, child := range cal.Children {
			if child.Name == "VTIMEZONE" || child == target {
				children = append(children, child)
			}
		}
		cal.Children = children
		target.Children = nil
	}
	if method == "CANCEL" {
		for _, child := range cal.Children {
			if child.Name == "VEVENT" {
				child.Props.SetText("STATUS", "CANCELLED")
				child.Props.Set(&ical.Prop{Name: "SEQUENCE", Value: fmt.Sprint(ev.Sequence)})
				delete(child.Props, "ATTENDEE")
				for _, a := range ev.Attendees {
					setAttendeeProp(child.Props, a, false)
				}
				child.Children = nil
			}
		}
	}
	return cal, nil
}

// WithAttendeeStatus changes only the responder's participation parameters.
// The organizer's version, times, exceptions and alarms remain unchanged.
func WithAttendeeStatus(data, email, status string) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", err
	}
	found := false
	for _, event := range cal.Events() {
		for _, p := range event.Props.Values("ATTENDEE") {
			u, err := url.Parse(p.Value)
			if err == nil && strings.EqualFold(emailFromURI(u), email) {
				p.Params.Set("PARTSTAT", status)
				p.Params.Set("RSVP", "FALSE")
				found = true
			}
		}
	}
	if !found {
		return "", fmt.Errorf("attendee not in meeting")
	}
	return encode(cal)
}
