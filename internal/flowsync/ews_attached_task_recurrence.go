// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"fmt"
	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/calutil"
	"github.com/teambition/rrule-go"
	"strings"
	"time"
)

func validateAttachedTaskRecurrence(n *protocolNode) error {
	if len(n.Attr) != 0 || len(n.Children) != 2 || strings.TrimSpace(n.Text) != "" {
		return fmt.Errorf("task recurrence requires a pattern and range")
	}
	for _, c := range n.Children {
		if strings.HasSuffix(c.Name.Local, "Regeneration") {
			if !strings.Contains(" DailyRegeneration WeeklyRegeneration MonthlyRegeneration YearlyRegeneration ", " "+c.Name.Local+" ") || len(c.Attr) != 0 || len(c.Children) != 1 || c.Children[0].Name.Local != "Interval" || len(c.Children[0].Attr) != 0 || len(c.Children[0].Children) != 0 {
				return fmt.Errorf("invalid task regeneration")
			}
			if _, err := patternInt(c.Children[0].Text, 1, 99); err != nil {
				return err
			}
		} else if err := validateAttachedRecurrence(c); err != nil {
			return err
		}
	}
	*n = *metadataNode(n)
	return nil
}

// Validation and fixed-pattern conversion use the calendar recurrence engine
// on a detached VEVENT. Nothing is inserted into calendars or delivery queues.
func exportAttachedTaskRecurrence(n *protocolNode, todo *ical.Component) error {
	rec := n.child("Recurrence")
	if rec == nil {
		if n.value("IsRecurring") == "true" || n.value("IsRecurring") == "1" {
			return fmt.Errorf("IsRecurring requires Recurrence")
		}
		return nil
	}
	if n.value("IsRecurring") == "false" || n.value("IsRecurring") == "0" {
		return fmt.Errorf("Recurrence conflicts with IsRecurring")
	}
	if err := validateAttachedTaskRecurrence(rec); err != nil {
		return err
	}
	copy := metadataNode(rec)
	var rangeNode *protocolNode
	var regeneration string
	for _, c := range copy.Children {
		if strings.HasSuffix(c.Name.Local, "Regeneration") {
			if regeneration != "" {
				return fmt.Errorf("multiple task regeneration patterns")
			}
			regeneration = c.Name.Local
			// A daily proxy validates the interval/range, not regeneration semantics.
			c.Name.Local = "DailyRecurrence"
		}
		switch c.Name.Local {
		case "NoEndRecurrence", "EndDateRecurrence", "NumberedRecurrence":
			rangeNode = c
		}
	}
	if rangeNode == nil {
		return fmt.Errorf("task recurrence range required")
	}
	start, err := time.Parse("2006-01-02", rangeNode.value("StartDate"))
	if err != nil {
		return fmt.Errorf("invalid task recurrence start date")
	}
	if f := n.child("StartDate"); f != nil && regeneration == "" {
		value, _ := time.Parse(time.RFC3339, f.Text)
		if value.Format("2006-01-02") != start.Format("2006-01-02") {
			return fmt.Errorf("task recurrence start must match StartDate")
		}
		start = value
	}
	if value := n.value("DueDate"); value != "" && regeneration == "" {
		due, _ := time.Parse(time.RFC3339, value)
		if due.Before(start) {
			return fmt.Errorf("task due date precedes recurrence start")
		}
	}
	end := start.Add(time.Hour)
	data, err := calutil.BuildSimpleVEVENT(calutil.Event{UID: "task-recurrence-validation", Start: &start, End: &end})
	if err != nil {
		return err
	}
	data, err = applyEWSRecurrenceForItem(data, []ewsCalendarEdit{{kind: "SetItemField", name: "Recurrence", node: copy}}, true)
	if err != nil {
		return err
	}
	if regeneration != "" {
		// RFC 5545 RRULE has no completion-relative regeneration semantics. Retain
		// exact EWS metadata, rather than silently turning this into a fixed series.
		p := ical.NewProp("X-TAYGA-EWS-TASK-REGENERATION")
		p.SetText(rec.render())
		todo.Props.Set(p)
		return nil
	}
	if todo.Props.Get("DTSTART") == nil {
		todo.Props.SetDateTime("DTSTART", start)
	}
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return err
	}
	rule := cal.Children[0].Props.Get("RRULE")
	if rule == nil {
		return fmt.Errorf("task recurrence did not produce a rule")
	}
	options, err := rrule.StrToROption(rule.Value)
	if err != nil {
		return err
	}
	options.Dtstart = start
	series, err := rrule.NewRRule(*options)
	if err != nil {
		return err
	}
	first := series.After(start, true)
	if first.IsZero() {
		return fmt.Errorf("task recurrence contains no occurrences")
	}
	todo.Props.SetDateTime("DTSTART", first)
	if due := todo.Props.Get("DUE"); due != nil {
		date, e := due.DateTime(time.UTC)
		if e != nil {
			return e
		}
		todo.Props.SetDateTime("DUE", date.Add(first.Sub(start)))
	}
	for key, props := range cal.Children[0].Props {
		if key == "RRULE" || strings.HasPrefix(key, "X-TAYGA-") {
			todo.Props[key] = props
		}
	}
	return nil
}
