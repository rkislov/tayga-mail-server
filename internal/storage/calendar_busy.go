package storage

import (
	"fmt"
	"github.com/emersion/go-ical"
	"strings"
	"time"
)

func objectBusyIntervals(data string, from, to time.Time) ([]BusyInterval, error) {
	calendar, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return nil, err
	}
	overrides := map[string]bool{}
	for _, component := range calendar.Children {
		if component.Name == "VEVENT" {
			if prop := component.Props.Get("RECURRENCE-ID"); prop != nil {
				value, err := prop.DateTime(time.UTC)
				if err != nil {
					return nil, err
				}
				overrides[value.UTC().Format(time.RFC3339)] = true
			}
		}
	}
	result := []BusyInterval{}
	for _, component := range calendar.Children {
		if component.Name != "VEVENT" {
			continue
		}
		status, _ := component.Props.Text("STATUS")
		transp, _ := component.Props.Text("TRANSP")
		if status == "CANCELLED" || transp == "TRANSPARENT" {
			continue
		}
		event := ical.Event{Component: component}
		start, err := event.DateTimeStart(time.UTC)
		if err != nil {
			return nil, err
		}
		end, err := event.DateTimeEnd(time.UTC)
		if err != nil || !end.After(start) {
			end = start.Add(time.Hour)
		}
		duration := end.Sub(start)
		if component.Props.Get("RRULE") == nil && component.Props.Get("RDATE") != nil {
			return nil, fmt.Errorf("RDATE-only recurrence requires manual availability confirmation")
		}
		if rule := component.Props.Get("RRULE"); rule != nil {
			// Do not turn extremely dense recurrences into misleading availability or unbounded work.
			upper := strings.ToUpper(rule.Value)
			if strings.Contains(upper, "FREQ=SECONDLY") || strings.Contains(upper, "FREQ=MINUTELY") || strings.Contains(upper, "FREQ=HOURLY") || strings.Contains(upper, "BYSECOND=") || strings.Contains(upper, "BYMINUTE=") || strings.Contains(upper, "BYHOUR=") || duration > 31*24*time.Hour {
				return nil, fmt.Errorf("unsupported dense recurrence")
			}
			set, err := component.RecurrenceSet(start.Location())
			if err != nil {
				return nil, err
			}
			if set != nil {
				for _, date := range set.Between(from.Add(-duration), to, true) {
					if overrides[date.UTC().Format(time.RFC3339)] {
						continue
					}
					if date.Before(to) && date.Add(duration).After(from) {
						result = append(result, BusyInterval{Start: date.UTC(), End: date.Add(duration).UTC()})
					}
				}
				continue
			}
		}
		if start.Before(to) && end.After(from) {
			result = append(result, BusyInterval{Start: start.UTC(), End: end.UTC()})
		}
	}
	return result, nil
}
