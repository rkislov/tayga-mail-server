package calutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// MergeMeetingCopy takes scheduling data from the organizer but retains the
// recipient's reminders and personal categories. Never copy organizer alarms
// to a new recipient, or reintroduce an alarm the recipient removed.
func MergeMeetingCopy(source, previous string) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(source)).Decode()
	if err != nil || MasterEvent(cal) == nil {
		return "", fmt.Errorf("invalid meeting source")
	}
	personal := map[string]*ical.Component{}
	if previous != "" {
		old, err := ical.NewDecoder(strings.NewReader(previous)).Decode()
		if err != nil || MasterEvent(old) == nil {
			return "", fmt.Errorf("invalid participant copy")
		}
		for _, c := range old.Children {
			if c.Name == "VEVENT" {
				key, err := meetingComponentKey(c)
				if err != nil {
					return "", err
				}
				personal[key] = c
			}
		}
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		key, err := meetingComponentKey(c)
		if err != nil {
			return "", err
		}
		old := personal[key]
		if old == nil && key != "" {
			old = personal[""]
		}
		children := c.Children[:0]
		for _, child := range c.Children {
			if child.Name != "VALARM" {
				children = append(children, child)
			}
		}
		c.Children = children
		if old != nil {
			for _, child := range old.Children {
				if child.Name == "VALARM" {
					c.Children = append(c.Children, child)
				}
			}
		}
		for _, name := range []string{"CATEGORIES", "COLOR", "X-APPLE-DEFAULT-ALARM", "X-MOZ-LASTACK", "X-MOZ-SNOOZE-TIME"} {
			delete(c.Props, name)
			if old != nil {
				for _, p := range old.Props.Values(name) {
					c.Props.Add(&p)
				}
			}
		}
	}
	delete(cal.Props, "METHOD")
	return encode(cal)
}

func meetingComponentKey(c *ical.Component) (string, error) {
	p := c.Props.Get("RECURRENCE-ID")
	if p == nil {
		return "", nil
	}
	v, err := p.DateTime(time.UTC)
	if err != nil {
		return "", err
	}
	return recurrenceKey(v, dateProp(p)), nil
}
