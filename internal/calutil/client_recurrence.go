// SPDX-License-Identifier: Apache-2.0
package calutil

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// SetClientRecurrence preserves detached instances, alarms and timezone data.
// Protocol adapters validate their supported RRULE vocabulary before calling.
func SetClientRecurrence(data, raw string) (string, error) {
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	if raw == "" {
		for _, c := range cal.Children {
			if c.Name == "VEVENT" && c.Props.Get("RECURRENCE-ID") != nil {
				return "", fmt.Errorf("cannot remove recurrence with exceptions")
			}
		}
		if master.Props.Get("EXDATE") != nil || master.Props.Get("RDATE") != nil {
			return "", fmt.Errorf("cannot remove recurrence with separate dates")
		}
		delete(master.Props, "RRULE")
		return encode(cal)
	}
	master.Props.Set(&ical.Prop{Name: "RRULE", Value: raw})
	start, err := ruleStart(master)
	if err != nil {
		return "", err
	}
	probe, err := cloneEvent(cal, master)
	if err != nil {
		return "", err
	}
	delete(probe.Props, "EXDATE")
	set, err := recurrenceSet(probe, start)
	if err != nil {
		return "", err
	}
	// Reject undefined DTSTART/rule combinations (RFC 5545).
	if r := set.GetRRule(); r != nil && !r.After(start.Add(-time.Second), true).Equal(start) {
		return "", fmt.Errorf("recurrence must include first event")
	}
	probeData, err := encode(componentCalendar(cal, probe))
	if err != nil {
		return "", err
	}
	existing, err := RecurrenceExceptions(data)
	if err != nil {
		return "", err
	}
	for _, exception := range existing {
		if _, e := GetOccurrence(probeData, exception.ID); e != nil {
			return "", fmt.Errorf("new rule removes an existing exception")
		}
	}
	return encode(cal)
}

// RecurrenceIDAtIndex counts original slots, including deleted/moved dates.
func RecurrenceIDAtIndex(data string, index int) (string, error) {
	if index < 1 || index > 10000 {
		return "", fmt.Errorf("occurrence index must be 1–10000")
	}
	_, master, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	if master.Props.Get("RRULE") == nil {
		return "", fmt.Errorf("not a recurring master")
	}
	start, err := ruleStart(master)
	if err != nil {
		return "", err
	}
	delete(master.Props, "EXDATE")
	set, err := recurrenceSet(master, start)
	if err != nil {
		return "", err
	}
	next := set.Iterator()
	for i := 1; i <= index; i++ {
		value, ok := next()
		if !ok {
			return "", fmt.Errorf("occurrence index outside series")
		}
		if i == index {
			return recurrenceKey(value, dateProp(master.Props.Get("DTSTART"))), nil
		}
	}
	return "", fmt.Errorf("occurrence not found")
}

type RecurrenceException struct {
	ID         string
	Original   time.Time
	Cancelled  bool
	Occurrence Occurrence
}

// Includes EXDATE cancellations, without expanding an unbounded series.
func RecurrenceExceptions(data string) ([]RecurrenceException, error) {
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return nil, err
	}
	start, err := ruleStart(master)
	if err != nil {
		return nil, err
	}
	entries := map[string]RecurrenceException{}
	for _, p := range master.Props.Values("EXDATE") {
		for _, value := range strings.Split(p.Value, ",") {
			p.Value = value
			t, e := p.DateTime(start.Location())
			if e != nil {
				return nil, e
			}
			id := recurrenceKey(t, dateProp(&p))
			entries[id] = RecurrenceException{ID: id, Original: t, Cancelled: true}
		}
	}
	for _, c := range cal.Children {
		p := c.Props.Get("RECURRENCE-ID")
		if c.Name != "VEVENT" || p == nil {
			continue
		}
		t, e := p.DateTime(start.Location())
		if e != nil {
			return nil, e
		}
		id := recurrenceKey(t, dateProp(p))
		instance, e := occurrenceFrom(cal, c, id)
		if e != nil {
			return nil, e
		}
		entries[id] = RecurrenceException{ID: id, Original: t, Cancelled: strings.EqualFold(instance.Status, "CANCELLED"), Occurrence: instance}
	}
	var out []RecurrenceException
	for _, e := range entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Original.Before(out[j].Original) })
	return out, nil
}

func CalendarMasterEqual(a, b string) bool {
	_, ma, ea := recurrenceCalendar(a)
	_, mb, eb := recurrenceCalendar(b)
	return ea == nil && eb == nil && schedulingFingerprint(ma) == schedulingFingerprint(mb)
}

// Set the master's wall-clock representation without changing the instants.
func SetClientTimezone(data, zone string) (string, error) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", err
	}
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	for _, name := range []string{"DTSTART", "DTEND"} {
		p := master.Props.Get(name)
		if p == nil || dateProp(p) {
			continue
		}
		t, e := p.DateTime(time.UTC)
		if e != nil {
			return "", e
		}
		fresh := ical.NewProp(name)
		if zone == "UTC" {
			fresh.SetDateTime(t.UTC())
		} else {
			fresh.Params.Set("TZID", zone)
			fresh.Value = t.In(loc).Format("20060102T150405")
		}
		master.Props.Set(fresh)
	}
	return encode(cal)
}
