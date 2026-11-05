// SPDX-License-Identifier: Apache-2.0
package calutil

import (
	"fmt"
	"github.com/emersion/go-ical"
	"strings"
)

type AttendeeResponseChange struct{ Email, RecurrenceID, Status string }

func meetingEvents(cal *ical.Calendar) (map[string]*ical.Component, error) {
	out := map[string]*ical.Component{}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		key, err := meetingComponentKey(c)
		if err != nil || out[key] != nil {
			return nil, fmt.Errorf("invalid/duplicate recurrence identity")
		}
		out[key] = c
	}
	return out, nil
}

// Merge stale foreign RSVP data when a Schedule-Tag still matches. Other
// scheduling fields remain the client's responsibility and are checked below.
func MergeScheduleResponses(input, current string, owners ...string) (string, error) {
	cal, _, err := recurrenceCalendar(input)
	if err != nil {
		return "", err
	}
	old, _, err := recurrenceCalendar(current)
	if err != nil {
		return "", err
	}
	events, err := meetingEvents(cal)
	if err != nil {
		return "", err
	}
	prior, err := meetingEvents(old)
	if err != nil {
		return "", err
	}
	for key, c := range events {
		before := prior[key]
		if before == nil {
			continue
		}
		for i, a := range c.Props["ATTENDEE"] {
			owned := false
			for _, owner := range owners {
				if strings.EqualFold(strings.TrimPrefix(a.Value, "mailto:"), owner) {
					owned = true
					break
				}
			}
			if owned {
				continue
			}
			for _, p := range before.Props.Values("ATTENDEE") {
				if strings.EqualFold(p.Value, a.Value) {
					for _, name := range []string{"PARTSTAT", "RSVP", "SCHEDULE-STATUS"} {
						delete(a.Params, name)
						if v := p.Params[name]; v != nil {
							a.Params[name] = append([]string{}, v...)
						}
					}
				}
			}
			c.Props["ATTENDEE"][i] = a
		}
	}
	// A reply can materialize a detached VEVENT. Preserve only response-only
	// overrides absent from the stale client's snapshot.
	for key, c := range prior {
		if key == "" || events[key] != nil {
			continue
		}
		baseline, err := cloneEvent(old, MasterEvent(old))
		if err != nil {
			return "", err
		}
		probe := componentCalendar(old, baseline)
		raw, err := encode(probe)
		if err != nil {
			return "", err
		}
		instance, err := GetOccurrence(raw, key)
		if err != nil {
			continue
		}
		wire, err := OccurrenceMessage(instance, false)
		if err != nil {
			return "", err
		}
		single, err := ical.NewDecoder(strings.NewReader(wire)).Decode()
		if err != nil {
			return "", err
		}
		if schedulingFingerprint(c) == schedulingFingerprint(MasterEvent(single)) {
			cal.Children = append(cal.Children, c)
		}
	}
	return encode(cal)
}

func attendeeComparable(c *ical.Component) string {
	copy := ical.NewComponent(c.Name)
	for k, v := range c.Props {
		switch k {
		case "COMMENT", "TRANSP", "CATEGORIES", "COLOR", "X-APPLE-DEFAULT-ALARM", "X-MOZ-LASTACK", "X-MOZ-SNOOZE-TIME":
			continue
		}
		copy.Props[k] = v
	}
	copy.Children = c.Children
	return schedulingFingerprint(copy)
}

// CalDAV attendee writes may change their own response and personal metadata,
// never organizer time, recurrence, roster, sequence or another user's reply.
func CalDAVAttendeeResponses(input, current string, emails []string) ([]AttendeeResponseChange, error) {
	cal, master, err := recurrenceCalendar(input)
	if err != nil {
		return nil, err
	}
	old, oldMaster, err := recurrenceCalendar(current)
	if err != nil {
		return nil, err
	}
	uid, _ := master.Props.Text("UID")
	oldUID, _ := oldMaster.Props.Text("UID")
	if uid != oldUID {
		return nil, fmt.Errorf("meeting UID cannot change")
	}
	actual, err := meetingEvents(cal)
	if err != nil {
		return nil, err
	}
	prior, err := meetingEvents(old)
	if err != nil {
		return nil, err
	}
	if actual[""] == nil {
		return nil, fmt.Errorf("master required")
	}
	zones := func(c *ical.Calendar) string {
		p := ical.NewComponent("VCALENDAR")
		for _, child := range c.Children {
			if child.Name != "VEVENT" {
				p.Children = append(p.Children, child)
			}
		}
		return schedulingFingerprint(p)
	}
	if zones(cal) != zones(old) {
		return nil, fmt.Errorf("timezone definition cannot change")
	}
	keys := []string{""}
	for key := range actual {
		if key != "" {
			keys = append(keys, key)
		}
	}
	var changes []AttendeeResponseChange
	for _, key := range keys {
		c := actual[key]
		before := prior[key]
		if before == nil {
			instance, e := GetOccurrence(current, key)
			if e != nil {
				return nil, e
			}
			wire, e := OccurrenceMessage(instance, false)
			if e != nil {
				return nil, e
			}
			one, e := ical.NewDecoder(strings.NewReader(wire)).Decode()
			if e != nil {
				return nil, e
			}
			before = MasterEvent(one)
		}
		if attendeeComparable(c) != attendeeComparable(before) {
			return nil, fmt.Errorf("attendee cannot change organizer properties")
		}
		sequence := func(c *ical.Component) (int, error) {
			if prop := c.Props.Get("SEQUENCE"); prop != nil {
				return prop.Int()
			}
			return 0, nil
		}
		oldSeq, e1 := sequence(before)
		seq, e2 := sequence(c)
		if e1 != nil || e2 != nil || seq != oldSeq {
			return nil, fmt.Errorf("attendee cannot change sequence")
		}

		for _, email := range emails {
			status, previous := "", ""
			for _, a := range before.Props.Values("ATTENDEE") {
				if strings.EqualFold(a.Value, "mailto:"+email) {
					previous = a.Params.Get("PARTSTAT")
					if previous == "" {
						previous = "NEEDS-ACTION"
					}
				}
			}
			for _, a := range c.Props.Values("ATTENDEE") {
				if strings.EqualFold(a.Value, "mailto:"+email) {
					status = a.Params.Get("PARTSTAT")
					if status == "" {
						status = "NEEDS-ACTION"
					}
				}
			}
			if status == "" || previous == "" {
				return nil, fmt.Errorf("responding attendee required")
			}
			if status != previous || key != "" {
				if status != "ACCEPTED" && status != "TENTATIVE" && status != "DECLINED" {
					if status == previous {
						continue
					}
					return nil, fmt.Errorf("invalid response status")
				}
				changes = append(changes, AttendeeResponseChange{email, key, status})
			}
		}
	}
	// Missing nontrivial overrides cannot be removed by an attendee.
	for key := range prior {
		if actual[key] == nil {
			return nil, fmt.Errorf("attendee cannot remove an exception")
		}
	}
	return changes, nil
}

func MergeCalDAVPersonal(source, input string) (string, error) {
	data, err := MergeMeetingCopy(source, input)
	if err != nil {
		return "", err
	}
	cal, _, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	client, _, err := recurrenceCalendar(input)
	if err != nil {
		return "", err
	}
	events, err := meetingEvents(client)
	if err != nil {
		return "", err
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		key, err := meetingComponentKey(c)
		if err != nil {
			return "", err
		}
		old := events[key]
		if old == nil {
			continue
		}
		for _, name := range []string{"COMMENT", "TRANSP"} {
			delete(c.Props, name)
			for _, p := range old.Props.Values(name) {
				c.Props.Add(&p)
			}
		}
	}
	return encode(cal)
}
