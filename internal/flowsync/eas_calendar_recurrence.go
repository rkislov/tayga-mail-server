// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func easRecurrence(data string, n *protocolNode) (string, error) {
	if n == nil {
		return data, nil
	}
	if len(n.Children) == 0 {
		return calutil.SetClientRecurrence(data, "")
	}
	p := calendarPattern{Interval: 1, WeekStart: "MO"}
	values := map[string]string{}
	for _, c := range n.Children {
		if _, ok := values[c.Name.Local]; ok || len(c.Children) > 0 {
			return "", fmt.Errorf("invalid recurrence field")
		}
		values[c.Name.Local] = c.Text
	}
	typ, err := patternInt(values["Type"], 0, 6)
	if err != nil || typ == 4 {
		return "", fmt.Errorf("unsupported recurrence type")
	}
	p.Frequency = map[int]string{0: "DAILY", 1: "WEEKLY", 2: "MONTHLY", 3: "MONTHLY", 5: "YEARLY", 6: "YEARLY"}[typ]
	allowed := map[string]bool{"Type": true, "Interval": true, "Occurrences": true, "Until": true, "CalendarType": true, "IsLeapMonth": true}
	if typ == 1 {
		allowed["DayOfWeek"] = true
		allowed["FirstDayOfWeek"] = true
	}
	if typ == 2 || typ == 5 {
		allowed["DayOfMonth"] = true
	}
	if typ == 3 || typ == 6 {
		allowed["DayOfWeek"] = true
		allowed["WeekOfMonth"] = true
	}
	if typ == 5 || typ == 6 {
		allowed["MonthOfYear"] = true
	}
	for key, v := range values {
		if !allowed[key] {
			return "", fmt.Errorf("unsupported recurrence selector %s", key)
		}
		switch key {
		case "Interval":
			p.Interval, err = patternInt(v, 1, 99)
		case "Occurrences":
			p.Count, err = patternInt(v, 1, 999)
		case "Until":
			p.Until = parseASTime(v)
			if p.Until == nil {
				err = fmt.Errorf("invalid recurrence end")
			}
		case "DayOfMonth":
			p.Day, err = patternInt(v, 1, 31)
			if p.Day > 28 {
				p.ClampDay = p.Day
				p.Day = 0
			}
		case "MonthOfYear":
			p.Month, err = patternInt(v, 1, 12)
		case "WeekOfMonth":
			p.Ordinal, err = patternInt(v, 1, 5)
			if p.Ordinal == 5 {
				p.Ordinal = -1
			}
		case "DayOfWeek":
			var mask int
			mask, err = patternInt(v, 1, 127)
			for i, d := range calendarDays {
				if mask&(1<<i) != 0 {
					p.Days = append(p.Days, d)
				}
			}
		case "FirstDayOfWeek":
			var day int
			day, err = patternInt(v, 0, 6)
			if err == nil {
				p.WeekStart = calendarDays[day]
			}
		case "CalendarType":
			if v != "0" && v != "1" {
				err = fmt.Errorf("only Gregorian calendars supported")
			}
		case "IsLeapMonth":
			if v != "0" {
				err = fmt.Errorf("leap months unsupported")
			}
		}
		if err != nil {
			return "", err
		}
	}
	if (typ == 1 || typ == 3 || typ == 6) && len(p.Days) == 0 || (typ == 3 || typ == 6) && p.Ordinal == 0 || (typ == 2 || typ == 5) && p.Day == 0 && p.ClampDay == 0 || (typ == 5 || typ == 6) && p.Month == 0 {
		return "", fmt.Errorf("incomplete recurrence pattern")
	}
	parsed, _ := calutil.ParseICS(data)
	raw, err := p.raw(parsed.AllDay)
	if err != nil {
		return "", err
	}
	return calutil.SetClientRecurrence(data, raw)
}

func calendarRecurrenceXML(ctx context.Context, data, viewer string) (string, error) {
	p, err := readCalendarPattern(data)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", nil
	}
	if p.Count > 999 {
		return "", fmt.Errorf("EAS supports at most 999 occurrences")
	}
	if err := validateExchangeDate(p); err != nil {
		return "", err
	}

	_, _, start, err := calendarComponent(data)
	if err != nil {
		return "", err
	}
	zone := calutil.ReadRecurrence(data).Timezone
	if zone == "" || zone == "floating" {
		zone = "UTC"
	}
	tzi, err := calendarTZI(zone, start.Year())
	if err != nil {
		return "", err
	}
	var b strings.Builder
	field := func(k, v string) {
		fmt.Fprintf(&b, `<Calendar:%s xmlns:Calendar="Calendar:">%s</Calendar:%s>`, k, xmlEscape(v), k)
	}
	field("Timezone", base64.StdEncoding.EncodeToString(tzi))
	b.WriteString(`<Calendar:Recurrence xmlns:Calendar="Calendar:">`)
	typ := map[string]int{"DAILY": 0, "WEEKLY": 1, "MONTHLY": 2, "YEARLY": 5}[p.Frequency]
	days := p.Days
	ordinal := p.Ordinal
	if p.Day == -1 {
		ordinal = -1
		days = calendarDays
	}
	if ordinal != 0 {
		typ++
	}
	field("Type", strconv.Itoa(typ))
	field("Interval", strconv.Itoa(p.Interval))
	if p.Count > 0 {
		field("Occurrences", strconv.Itoa(p.Count))
	}
	if p.Until != nil {
		// Legacy imports can bound RRULE before DTSTART. DTSTART still defines
		// the first iCalendar instance; do not send an inverted EAS range.
		until := *p.Until
		if until.Before(start) {
			until = start
		}
		field("Until", formatCalendarTime(&until))
	}
	if len(days) > 0 {
		mask := 0
		for _, d := range days {
			mask |= 1 << dayIndex(d)
		}
		field("DayOfWeek", strconv.Itoa(mask))
	}
	if p.Frequency == "WEEKLY" {
		field("FirstDayOfWeek", strconv.Itoa(dayIndex(p.WeekStart)))
	}
	if ordinal != 0 {
		if ordinal == -1 {
			ordinal = 5
		}
		field("WeekOfMonth", strconv.Itoa(ordinal))
	} else {
		day := p.Day
		if p.ClampDay > 0 {
			day = p.ClampDay
		}
		if day > 0 {
			field("DayOfMonth", strconv.Itoa(day))
		}
	}
	if p.Month > 0 {
		field("MonthOfYear", strconv.Itoa(p.Month))
	}
	b.WriteString(`</Calendar:Recurrence>`)
	exceptions, err := calutil.RecurrenceExceptions(data)
	if err != nil {
		return "", err
	}
	if len(exceptions) > 256 {
		return "", fmt.Errorf("too many EAS exceptions")
	}
	b.WriteString(`<Calendar:Exceptions xmlns:Calendar="Calendar:">`)
	version, _ := ctx.Value(easVersionKey{}).(string)
	for _, e := range exceptions {
		b.WriteString(`<Calendar:Exception>`)
		if version == "16.0" || version == "16.1" {
			fmt.Fprintf(&b, `<InstanceId xmlns="AirSyncBase:">%s</InstanceId>`, e.Original.UTC().Format(time.RFC3339))
		} else {
			field("ExceptionStartTime", formatCalendarTime(&e.Original))
		}
		if e.Cancelled {
			field("Deleted", "1")
		} else {
			field("Deleted", "0")
			field("Subject", e.Occurrence.Summary)
			field("StartTime", formatCalendarTime(e.Occurrence.Start))
			field("EndTime", formatCalendarTime(e.Occurrence.End))
			day := "0"
			if e.Occurrence.AllDay {
				day = "1"
			}
			field("AllDayEvent", day)
			wire, err := calutil.OccurrenceMessage(e.Occurrence, false)
			if err != nil {
				return "", err
			}
			if version == "16.0" || version == "16.1" {
				b.WriteString(calendarLocationXML(ctx, wire))
			} else {
				field("Location", e.Occurrence.Location)
			}
			b.WriteString(calendarExceptionMeetingXML(wire, viewer))
			b.WriteString(`<Body xmlns="AirSyncBase:"><Type>1</Type><Data>` + xmlEscape(e.Occurrence.Description) + `</Data></Body>`)
		}
		b.WriteString(`</Calendar:Exception>`)
	}
	b.WriteString(`</Calendar:Exceptions>`)
	return b.String(), nil
}

func easInstanceRID(data, value string) (string, error) {
	p, err := calutil.ParseICS(data)
	if err != nil {
		return "", err
	}
	t := parseASTime(value)
	if t == nil {
		return "", fmt.Errorf("invalid instance id")
	}
	if p.AllDay {
		return t.Format("2006-01-02"), nil
	}
	return t.UTC().Format(time.RFC3339), nil
}

func easExceptionEdits(data string, u *storage.User, n *protocolNode) ([]storage.CalendarOccurrenceEdit, string, error) {
	if n == nil {
		return nil, data, nil
	}
	if len(n.Children) > 256 {
		return nil, "", fmt.Errorf("too many exceptions")
	}
	var edits []storage.CalendarOccurrenceEdit
	candidate := data
	seen := map[string]bool{}
	for _, e := range n.Children {
		if e.Name.Local != "Exception" {
			return nil, "", fmt.Errorf("invalid exception")
		}
		rid, err := easInstanceRID(data, e.value("ExceptionStartTime"))
		if err != nil || seen[rid] {
			return nil, "", fmt.Errorf("invalid or duplicate exception id")
		}
		seen[rid] = true
		current, err := calutil.GetOccurrence(candidate, rid)
		if err != nil {
			return nil, "", err
		}
		change := calutil.OccurrenceChange{}
		switch e.value("Deleted") {
		case "1":
			change.Cancel = true
		case "", "0":
			wire, err := calutil.OccurrenceMessage(current, false)
			if err != nil {
				return nil, "", err
			}
			// Exceptions cannot alter the series UID, organizer, recurrence or roster.
			for _, c := range e.Children {
				switch c.Name.Local {
				case "ExceptionStartTime", "Deleted", "Subject", "StartTime", "EndTime", "AllDayEvent", "Location", "Body", "DTStamp", "BusyStatus", "MeetingStatus", "ResponseType", "Attendees":
				default:
					return nil, "", fmt.Errorf("unsupported exception field %s", c.Name.Local)
				}
			}
			edited, err := editEASCalendar(wire, u, clientOp{Kind: "change", ApplicationData: e})
			if err != nil {
				return nil, "", err
			}
			after, err := calutil.ParseICS(edited)
			if err != nil {
				return nil, "", err
			}
			if err = validateEASInstanceEdit(wire, edited, e); err != nil {
				return nil, "", err
			}
			change = occurrenceDelta(current.ParseResult, after)
			if !change.ResetResponses {
				continue
			}
		default:
			return nil, "", fmt.Errorf("invalid deleted flag")
		}
		if change.Cancel && strings.EqualFold(current.Status, "CANCELLED") {
			continue
		}
		candidate, _, _, err = calutil.ChangeOccurrence(candidate, rid, change)
		if err != nil {
			return nil, "", err
		}
		edits = append(edits, storage.CalendarOccurrenceEdit{RecurrenceID: rid, Change: change})
	}
	return edits, candidate, nil
}

func finishEASRecurrence(data, old string, op clientOp) (string, error) {
	zone := calutil.ReadRecurrence(old).Timezone
	if zone == "" || zone == "floating" {
		zone = "UTC"
	}
	_, _, start, err := calendarComponent(data)
	if err != nil {
		return "", err
	}
	raw := op.ApplicationData.child("Timezone")
	if raw != nil {
		zone, err = decodeCalendarTimezone(raw.Text, zone, start.Year())
		if err != nil {
			return "", err
		}
	}
	data, err = calutil.SetClientTimezone(data, zone)
	if err != nil {
		return "", err
	}
	before := data
	data, err = easRecurrence(data, op.ApplicationData.child("Recurrence"))
	if err != nil {
		return "", err
	}
	data = preserveEquivalentRecurrence(before, data)
	if raw := davTextProperty(data, "RRULE"); raw != "" {
		return calutil.SetClientRecurrence(data, raw)
	}
	return data, nil
}

// Properties outside OccurrenceChange must never be acknowledged and discarded.
func validateEASInstanceEdit(before, after string, app *protocolNode) error {
	a, err := calutil.ParseICS(before)
	if err != nil {
		return err
	}
	b, err := calutil.ParseICS(after)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(a.Attendees, b.Attendees) || !reflect.DeepEqual(a.Organizer, b.Organizer) {
		return fmt.Errorf("instance roster changes are not supported")
	}
	if app == nil {
		return nil
	}
	for _, n := range app.Children {
		switch n.Name.Local {
		case "Subject", "StartTime", "EndTime", "AllDayEvent", "Location", "Body", "ExceptionStartTime", "Deleted", "DTStamp", "MeetingStatus", "ResponseType", "Attendees", "OrganizerEmail", "OrganizerName":
		case "BusyStatus":
			if davTextProperty(before, "TRANSP") != davTextProperty(after, "TRANSP") || davTextProperty(before, "X-MICROSOFT-CDO-BUSYSTATUS") != davTextProperty(after, "X-MICROSOFT-CDO-BUSYSTATUS") {
				return fmt.Errorf("instance busy-status changes are not supported")
			}
		default:
			return fmt.Errorf("unsupported instance property %s", n.Name.Local)
		}
	}
	return nil
}

func calendarExceptionMeetingXML(data, viewer string) string {
	doc, err := parseProtocolXML([]byte("<Exception>" + calendarMeetingXML(data, viewer) + "</Exception>"))
	if err != nil {
		return ""
	}
	var out strings.Builder
	for _, n := range doc.child("Exception").Children {
		switch n.Name.Local {
		case "OrganizerName", "OrganizerEmail":
			continue
		}
		out.WriteString(n.render())
	}
	return out.String()
}
