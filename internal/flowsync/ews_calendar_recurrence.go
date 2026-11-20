// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
)

type rawCalendarContextKey struct{}

var errExchangeRecurrence = errors.New("recurrence cannot be represented by Exchange")

func ewsDayList(days []string) string {
	var out []string
	for _, d := range days {
		out = append(out, ewsDays[dayIndex(d)])
	}
	return strings.Join(out, " ")
}
func parseEWSDays(value string) ([]string, error) {
	var days []string
	seen := map[string]bool{}
	for _, name := range strings.Fields(value) {
		var list []string
		switch name {
		case "Day":
			list = calendarDays
		case "Weekday":
			list = calendarDays[1:6]
		case "WeekendDay":
			list = []string{"SU", "SA"}
		default:
			for i, d := range ewsDays {
				if d == name {
					list = []string{calendarDays[i]}
				}
			}
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("invalid EWS weekday")
		}
		for _, d := range list {
			if seen[d] {
				return nil, fmt.Errorf("duplicate weekday")
			}
			seen[d] = true
			days = append(days, d)
		}
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("weekday required")
	}
	return days, nil
}

func ewsRecurrenceXML(data string) (string, error) {
	p, err := readCalendarPattern(data)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", nil
	}
	_, _, start, err := calendarComponent(data)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<t:Recurrence>`)
	ordinal := p.Ordinal
	days := p.Days
	if p.Day == -1 {
		ordinal = -1
		days = calendarDays
	}
	index := map[int]string{1: "First", 2: "Second", 3: "Third", 4: "Fourth", -1: "Last"}[ordinal]
	day := p.Day
	if p.ClampDay > 0 {
		day = p.ClampDay
	}
	if err := validateExchangeDate(p); err != nil {
		return "", err
	}

	switch p.Frequency {
	case "DAILY":
		fmt.Fprintf(&b, `<t:DailyRecurrence><t:Interval>%d</t:Interval></t:DailyRecurrence>`, p.Interval)
	case "WEEKLY":
		fmt.Fprintf(&b, `<t:WeeklyRecurrence><t:Interval>%d</t:Interval><t:DaysOfWeek>%s</t:DaysOfWeek><t:FirstDayOfWeek>%s</t:FirstDayOfWeek></t:WeeklyRecurrence>`, p.Interval, ewsDayList(p.Days), ewsDays[dayIndex(p.WeekStart)])
	case "MONTHLY":
		if ordinal != 0 {
			fmt.Fprintf(&b, `<t:RelativeMonthlyRecurrence><t:Interval>%d</t:Interval><t:DaysOfWeek>%s</t:DaysOfWeek><t:DayOfWeekIndex>%s</t:DayOfWeekIndex></t:RelativeMonthlyRecurrence>`, p.Interval, ewsDayList(days), index)
		} else {
			fmt.Fprintf(&b, `<t:AbsoluteMonthlyRecurrence><t:Interval>%d</t:Interval><t:DayOfMonth>%d</t:DayOfMonth></t:AbsoluteMonthlyRecurrence>`, p.Interval, day)
		}
	case "YEARLY":
		if p.Interval != 1 {
			return "", fmt.Errorf("EWS yearly interval must be one")
		}
		if ordinal != 0 {
			fmt.Fprintf(&b, `<t:RelativeYearlyRecurrence><t:DaysOfWeek>%s</t:DaysOfWeek><t:DayOfWeekIndex>%s</t:DayOfWeekIndex><t:Month>%s</t:Month></t:RelativeYearlyRecurrence>`, ewsDayList(days), index, ewsMonths[p.Month-1])
		} else {
			fmt.Fprintf(&b, `<t:AbsoluteYearlyRecurrence><t:DayOfMonth>%d</t:DayOfMonth><t:Month>%s</t:Month></t:AbsoluteYearlyRecurrence>`, day, ewsMonths[p.Month-1])
		}
	}
	switch {
	case p.Count > 0:
		fmt.Fprintf(&b, `<t:NumberedRecurrence><t:StartDate>%s</t:StartDate><t:NumberOfOccurrences>%d</t:NumberOfOccurrences></t:NumberedRecurrence>`, start.Format("2006-01-02"), p.Count)
	case p.Until != nil:
		end := p.Until.In(start.Location())
		if end.Format("150405") < start.Format("150405") {
			end = end.AddDate(0, 0, -1)
		}
		fmt.Fprintf(&b, `<t:EndDateRecurrence><t:StartDate>%s</t:StartDate><t:EndDate>%s</t:EndDate></t:EndDateRecurrence>`, start.Format("2006-01-02"), end.Format("2006-01-02"))
	default:
		fmt.Fprintf(&b, `<t:NoEndRecurrence><t:StartDate>%s</t:StartDate></t:NoEndRecurrence>`, start.Format("2006-01-02"))
	}
	b.WriteString(`</t:Recurrence>`)
	return b.String(), nil
}

func applyEWSRecurrence(data string, edits []ewsCalendarEdit) (string, error) {
	for _, edit := range edits {
		if edit.name != "Recurrence" {
			continue
		}
		if edit.kind == "DeleteItemField" {
			return calutil.SetClientRecurrence(data, "")
		}
		if edit.kind != "SetItemField" || edit.node == nil || len(edit.node.Children) != 2 {
			return "", fmt.Errorf("recurrence requires one pattern and one range")
		}
		_, _, start, err := calendarComponent(data)
		if err != nil {
			return "", err
		}
		var pattern, span *protocolNode
		for _, n := range edit.node.Children {
			switch n.Name.Local {
			case "DailyRecurrence", "WeeklyRecurrence", "AbsoluteMonthlyRecurrence", "RelativeMonthlyRecurrence", "AbsoluteYearlyRecurrence", "RelativeYearlyRecurrence":
				if pattern != nil {
					return "", fmt.Errorf("duplicate pattern")
				}
				pattern = n
			case "NumberedRecurrence", "EndDateRecurrence", "NoEndRecurrence":
				if span != nil {
					return "", fmt.Errorf("duplicate range")
				}
				span = n
			default:
				return "", fmt.Errorf("unsupported recurrence element")
			}
		}
		if pattern == nil || span == nil {
			return "", fmt.Errorf("pattern and range required")
		}
		p := calendarPattern{Interval: 1, WeekStart: "MO"}
		allowed := map[string]bool{}
		switch pattern.Name.Local {
		case "DailyRecurrence":
			p.Frequency = "DAILY"
			allowed["Interval"] = true
		case "WeeklyRecurrence":
			p.Frequency = "WEEKLY"
			allowed["Interval"] = true
			allowed["DaysOfWeek"] = true
			allowed["FirstDayOfWeek"] = true
		case "AbsoluteMonthlyRecurrence":
			p.Frequency = "MONTHLY"
			allowed["Interval"] = true
			allowed["DayOfMonth"] = true
		case "RelativeMonthlyRecurrence":
			p.Frequency = "MONTHLY"
			allowed["Interval"] = true
			allowed["DaysOfWeek"] = true
			allowed["DayOfWeekIndex"] = true
		case "AbsoluteYearlyRecurrence":
			p.Frequency = "YEARLY"
			allowed["DayOfMonth"] = true
			allowed["Month"] = true
		case "RelativeYearlyRecurrence":
			p.Frequency = "YEARLY"
			allowed["DaysOfWeek"] = true
			allowed["DayOfWeekIndex"] = true
			allowed["Month"] = true
		}
		seen := map[string]bool{}
		for _, n := range pattern.Children {
			if !allowed[n.Name.Local] || seen[n.Name.Local] || len(n.Children) > 0 {
				return "", fmt.Errorf("invalid recurrence pattern field")
			}
			seen[n.Name.Local] = true
			switch n.Name.Local {
			case "Interval":
				p.Interval, err = patternInt(n.Text, 1, 99)
			case "DayOfMonth":
				p.Day, err = patternInt(n.Text, 1, 31)
			case "Month":
				for i, m := range ewsMonths {
					if n.Text == m {
						p.Month = i + 1
					}
				}
				if p.Month == 0 {
					err = fmt.Errorf("invalid month")
				}
			case "DaysOfWeek":
				p.Days, err = parseEWSDays(n.Text)
			case "FirstDayOfWeek":
				days, e := parseEWSDays(n.Text)
				err = e
				if len(days) != 1 {
					err = fmt.Errorf("one week start required")
				} else {
					p.WeekStart = days[0]
				}
			case "DayOfWeekIndex":
				p.Ordinal = map[string]int{"First": 1, "Second": 2, "Third": 3, "Fourth": 4, "Last": -1}[n.Text]
				if p.Ordinal == 0 {
					err = fmt.Errorf("invalid weekday index")
				}
			}
			if err != nil {
				return "", err
			}
		}
		for key := range allowed {
			if key != "FirstDayOfWeek" && !seen[key] {
				return "", fmt.Errorf("missing recurrence field %s", key)
			}
		}
		if p.Day > 28 {
			p.ClampDay = p.Day
			p.Day = 0
		}
		seen = map[string]bool{}
		for _, n := range span.Children {
			if seen[n.Name.Local] || len(n.Children) > 0 {
				return "", fmt.Errorf("duplicate range field")
			}
			seen[n.Name.Local] = true
			switch n.Name.Local {
			case "StartDate":
				if n.Text != start.Format("2006-01-02") {
					return "", fmt.Errorf("recurrence start date must equal event date")
				}
			case "NumberOfOccurrences":
				if span.Name.Local != "NumberedRecurrence" {
					return "", fmt.Errorf("invalid range")
				}
				p.Count, err = patternInt(n.Text, 1, 65535)
			case "EndDate":
				if span.Name.Local != "EndDateRecurrence" {
					return "", fmt.Errorf("invalid range")
				}
				var end time.Time
				end, err = time.ParseInLocation("2006-01-02", n.Text, start.Location())
				if err == nil {
					end = end.AddDate(0, 0, 1).Add(-time.Second)
					if end.Before(start) {
						err = fmt.Errorf("end before first event")
					}
					p.Until = &end
				}
			default:
				return "", fmt.Errorf("unsupported range field")
			}
			if err != nil {
				return "", err
			}
		}
		if !seen["StartDate"] || span.Name.Local == "NumberedRecurrence" && p.Count == 0 || span.Name.Local == "EndDateRecurrence" && p.Until == nil {
			return "", fmt.Errorf("incomplete recurrence range")
		}
		parsed, _ := calutil.ParseICS(data)
		raw, err := p.raw(parsed.AllDay)
		if err != nil {
			return "", err
		}
		return calutil.SetClientRecurrence(data, raw)
	}
	return data, nil
}

func occurrenceItemID(master, rid string) string {
	return "ews-occ:" + base64.RawURLEncoding.EncodeToString([]byte(master+"\x00"+rid))
}
func parseOccurrenceID(id string) (string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "ews-occ:"))
	parts := strings.Split(string(raw), "\x00")
	if err != nil || len(parts) != 2 || len(id) > 2048 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid occurrence id")
	}
	return parts[0], parts[1], nil
}

func ewsCalendarZone(edits []ewsCalendarEdit, existing string) (string, error) {
	zone := calutil.ReadRecurrence(existing).Timezone
	if zone == "" || zone == "floating" {
		zone = "UTC"
	}
	end := ""
	for _, e := range edits {
		if e.name == "StartTimeZone" || e.name == "EndTimeZone" {
			if e.kind != "SetItemField" || e.node == nil {
				return "", fmt.Errorf("timezone cannot be deleted")
			}
			z, err := calendarZone(nodeAttr(e.node, "Id"))
			if err != nil {
				return "", err
			}
			if e.name == "StartTimeZone" {
				zone = z
			} else {
				end = z
			}
		}
	}
	if end != "" && end != zone {
		return "", fmt.Errorf("different start/end timezones are not supported")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", err
	}

	allDay := false
	if stored, err := calutil.ParseICS(existing); err == nil {
		allDay = stored.AllDay
	}
	for _, e := range edits {
		if e.name == "IsAllDayEvent" && e.node != nil {
			allDay = e.node.Text == "true" || e.node.Text == "1"
		}
	}
	for _, e := range edits {
		if (e.name != "Start" && e.name != "End") || e.node == nil {
			continue
		}
		value, err := time.Parse(time.RFC3339, e.node.Text)
		if err != nil {
			value, err = time.ParseInLocation("2006-01-02T15:04:05", e.node.Text, loc)
			if err != nil {
				return "", err
			}
		}
		if allDay {
			local := value.In(loc)
			if local.Hour() != 0 || local.Minute() != 0 || local.Second() != 0 {
				return "", fmt.Errorf("all-day boundaries must be midnight in the event timezone")
			}
			value = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		}
		e.node.Text = value.UTC().Format(time.RFC3339)
	}

	return zone, nil
}

func occurrenceIndex(n *protocolNode) (string, int, error) {
	master := nodeAttr(n, "RecurringMasterId")
	index, err := strconv.Atoi(nodeAttr(n, "InstanceIndex"))
	if master == "" || err != nil || index < 1 || index > 10000 {
		return "", 0, fmt.Errorf("invalid occurrence index")
	}
	return master, index, nil
}

func finishEWSRecurrence(data, zone string, edits []ewsCalendarEdit) (string, error) {
	data, err := calutil.SetClientTimezone(data, zone)
	if err != nil {
		return "", err
	}
	before := data
	data, err = applyEWSRecurrence(data, edits)
	if err != nil {
		return "", err
	}
	data = preserveEquivalentRecurrence(before, data)
	if raw := davTextProperty(data, "RRULE"); raw != "" {
		return calutil.SetClientRecurrence(data, raw)
	}
	return data, nil
}
