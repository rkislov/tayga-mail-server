package calutil

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// PrepareOrganizerWrite separates meaningful organizer edits from client echoes
// and personal reminder changes. It retains the full iCalendar document,
// including timezones and detached recurrence instances.
func PrepareOrganizerWrite(data, previous, actorEmail string) (string, bool, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", false, err
	}
	master := MasterEvent(cal)
	if master == nil {
		return data, false, nil
	}
	p, err := ParseICS(data)
	if err != nil {
		return "", false, err
	}
	seen := map[string]bool{}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		uid, _ := c.Props.Text("UID")
		org := c.Props.Get("ORGANIZER")
		value := ""
		if org != nil {
			value = strings.TrimPrefix(strings.ToLower(org.Value), "mailto:")
		}
		key, e := meetingComponentKey(c)
		if e != nil || seen[key] || uid != p.UID || !strings.EqualFold(value, p.Organizer.Email) {
			return "", false, fmt.Errorf("inconsistent meeting identity")
		}
		seen[key] = true
		for _, a := range c.Props.Values("ATTENDEE") {
			agent := strings.ToUpper(a.Params.Get("SCHEDULE-AGENT"))
			if agent != "" && agent != "SERVER" && agent != "CLIENT" && agent != "NONE" {
				return "", false, fmt.Errorf("invalid schedule agent")
			}
		}
	}
	var old *ical.Calendar
	if previous != "" {
		before, e := ParseICS(previous)
		if e != nil || before.UID != p.UID || before.Organizer.Email != "" && !strings.EqualFold(before.Organizer.Email, p.Organizer.Email) {
			return "", false, fmt.Errorf("meeting identity cannot be changed")
		}
		old, err = ical.NewDecoder(strings.NewReader(previous)).Decode()
		if err != nil {
			return "", false, err
		}
	}
	if p.Organizer.Email == "" || !strings.EqualFold(p.Organizer.Email, actorEmail) {
		// Received/imported copies never initiate an organizer fan-out.
		return data, false, nil
	}
	changed := old == nil || schedulingFingerprint(cal.Component) != schedulingFingerprint(old.Component)
	version := NextSequence(data) - 1
	if previous != "" && changed && version < NextSequence(previous) {
		version = NextSequence(previous)
	}
	oldEvents := map[string]*ical.Component{}
	if old != nil {
		for _, c := range old.Children {
			if c.Name == "VEVENT" {
				key, _ := meetingComponentKey(c)
				oldEvents[key] = c
			}
		}
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		key, _ := meetingComponentKey(c)
		before := oldEvents[key]
		if changed {
			c.Props.Set(&ical.Prop{Name: "SEQUENCE", Value: fmt.Sprint(version)})
			c.Props.SetDateTime("DTSTAMP", time.Now().UTC())
		} else {
			for _, name := range []string{"SEQUENCE", "DTSTAMP"} {
				delete(c.Props, name)
				for _, prop := range before.Props.Values(name) {
					c.Props.Add(&prop)
				}
			}
		}
		for _, a := range c.Props.Values("ATTENDEE") {
			if agent := a.Params.Get("SCHEDULE-AGENT"); agent != "" && !strings.EqualFold(agent, "SERVER") {
				continue
			}
			if strings.EqualFold(a.Value, "mailto:"+actorEmail) {
				continue
			}
			if changed {
				a.Params.Set("PARTSTAT", "NEEDS-ACTION")
				a.Params.Set("RSVP", "TRUE")
				delete(a.Params, "SCHEDULE-STATUS")
			} else {
				for _, prior := range before.Props.Values("ATTENDEE") {
					if !strings.EqualFold(a.Value, prior.Value) {
						continue
					}
					for _, name := range []string{"PARTSTAT", "RSVP", "SCHEDULE-STATUS"} {
						delete(a.Params, name)
						if values, ok := prior.Params[name]; ok {
							a.Params[name] = append([]string(nil), values...)
						}
					}
				}
			}
		}
	}
	delete(cal.Props, "METHOD")
	out, err := encode(cal)
	return out, changed, err
}

// Canonical comparison ignores wire ordering, client bookkeeping, personal
// alarms and participant responses. Changes to the actual meeting still send
// REQUEST, even when a client forgot to advance SEQUENCE.
func schedulingFingerprint(c *ical.Component) string {
	var fields []string
	for name, values := range c.Props {
		if c.Name == "VCALENDAR" {
			continue
		}
		if c.Name == "VEVENT" {
			switch name {
			case "DTSTAMP", "LAST-MODIFIED", "CREATED", "SEQUENCE", "CATEGORIES", "COLOR", "X-APPLE-DEFAULT-ALARM", "X-MOZ-LASTACK", "X-MOZ-SNOOZE-TIME":
				continue
			}
		}
		for _, p := range values {
			params := ical.Params{}
			for k, v := range p.Params {
				params[k] = append([]string(nil), v...)
			}
			if name == "ATTENDEE" {
				for _, k := range []string{"PARTSTAT", "RSVP", "SCHEDULE-STATUS", "VALUE"} {
					delete(params, k)
				}
				for k, def := range map[string]string{"ROLE": "REQ-PARTICIPANT", "CUTYPE": "INDIVIDUAL", "SCHEDULE-AGENT": "SERVER"} {
					if params.Get(k) == "" {
						params.Set(k, def)
					}
				}
			}
			if name == "ORGANIZER" {
				delete(params, "VALUE")
				delete(params, "SCHEDULE-STATUS")
				if params.Get("SCHEDULE-AGENT") == "" {
					params.Set("SCHEDULE-AGENT", "SERVER")
				}
			}
			value := p.Value
			if name == "ATTENDEE" || name == "ORGANIZER" {
				value = strings.ToLower(value)
			}
			b, _ := json.Marshal([]any{name, value, params})
			fields = append(fields, string(b))
		}
	}
	for _, child := range c.Children {
		if child.Name != "VALARM" {
			fields = append(fields, schedulingFingerprint(child))
		}
	}
	sort.Strings(fields)
	b, _ := json.Marshal([]any{c.Name, fields})
	return string(b)
}
