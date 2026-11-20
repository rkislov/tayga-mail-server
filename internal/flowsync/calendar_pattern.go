// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/calutil"
)

var calendarDays = []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
var ewsDays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
var ewsMonths = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}

type calendarPattern struct {
	Frequency                                      string
	Interval, Count, Month, Day, Ordinal, ClampDay int
	Days                                           []string
	WeekStart                                      string
	Until                                          *time.Time
}

func calendarComponent(data string) (*ical.Calendar, *ical.Component, time.Time, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	m := calutil.MasterEvent(cal)
	if m == nil {
		return nil, nil, time.Time{}, fmt.Errorf("VEVENT required")
	}
	event := ical.Event{Component: m}
	start, err := event.DateTimeStart(time.UTC)
	return cal, m, start, err
}
func patternInt(raw string, min, max int) (int, error) {
	n, e := strconv.Atoi(raw)
	if e != nil || n < min || n > max {
		return 0, fmt.Errorf("invalid recurrence integer")
	}
	return n, nil
}
func dayIndex(day string) int {
	for i, d := range calendarDays {
		if day == d {
			return i
		}
	}
	return -1
}

func readCalendarPattern(data string) (*calendarPattern, error) {
	_, m, start, err := calendarComponent(data)
	if err != nil {
		return nil, err
	}
	if m.Props.Get("RDATE") != nil {
		return nil, fmt.Errorf("RDATE-only/additional dates have no Exchange recurrence pattern")
	}
	prop := m.Props.Get("RRULE")
	if prop == nil {
		return nil, nil
	}
	if len(m.Props.Values("RRULE")) != 1 {
		return nil, fmt.Errorf("multiple rules")
	}
	p := &calendarPattern{Interval: 1, WeekStart: "MO"}
	parts := map[string]string{}
	for _, part := range strings.Split(prop.Value, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 || parts[kv[0]] != "" {
			return nil, fmt.Errorf("invalid recurrence")
		}
		parts[kv[0]] = kv[1]
	}
	for key, value := range parts {
		switch key {
		case "FREQ":
			p.Frequency = value
		case "INTERVAL":
			p.Interval, err = patternInt(value, 1, 99)
		case "COUNT":
			p.Count, err = patternInt(value, 1, 65535)
		case "UNTIL":
			u := ical.NewProp("DTSTART")
			u.Value = value
			var v time.Time
			v, err = u.DateTime(start.Location())
			p.Until = &v
		case "BYMONTH":
			p.Month, err = patternInt(value, 1, 12)
		case "BYMONTHDAY":
			if value == "-1" {
				p.Day = -1
			} else if strings.Contains(value, ",") {
				values := strings.Split(value, ",")
				if len(values) < 2 || len(values) > 4 {
					return nil, fmt.Errorf("unsupported month days")
				}
				for i, v := range values {
					if v != strconv.Itoa(28+i) {
						return nil, fmt.Errorf("unsupported month days")
					}
				}
				p.ClampDay = 27 + len(values)
			} else {
				p.Day, err = patternInt(value, 1, 31)
			}
		case "BYDAY":
			p.Days = strings.Split(value, ",")
			if len(p.Days) == 1 && len(value) > 2 {
				p.Ordinal, err = strconv.Atoi(value[:len(value)-2])
				p.Days[0] = value[len(value)-2:]
			}
		case "BYSETPOS": // evaluated after BYDAY (RRULE order is irrelevant)
		case "WKST":
			if dayIndex(value) < 0 {
				return nil, fmt.Errorf("invalid week start")
			}
			p.WeekStart = value
		default:
			return nil, fmt.Errorf("unsupported recurrence field %s", key)
		}
		if err != nil {
			return nil, err
		}
	}
	if value := parts["BYSETPOS"]; value != "" {
		if p.Ordinal != 0 {
			return nil, fmt.Errorf("combined ordinal selectors")
		}
		p.Ordinal, err = strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
	}
	if p.ClampDay > 0 {
		if p.Ordinal != -1 || len(p.Days) > 0 {
			return nil, fmt.Errorf("unsupported clamped date")
		}
		p.Ordinal = 0
	}
	if p.Count > 0 && p.Until != nil {
		return nil, fmt.Errorf("count and until")
	}
	seen := map[string]bool{}
	for _, day := range p.Days {
		if dayIndex(day) < 0 || seen[day] {
			return nil, fmt.Errorf("invalid weekday")
		}
		seen[day] = true
	}
	switch p.Frequency {
	case "DAILY":
		if len(p.Days) > 0 || p.Day != 0 || p.Month != 0 || p.Ordinal != 0 || p.ClampDay != 0 {
			return nil, fmt.Errorf("unsupported daily selectors")
		}
	case "WEEKLY":
		if p.Day != 0 || p.Ordinal != 0 || p.Month != 0 || p.ClampDay != 0 {
			return nil, fmt.Errorf("unsupported weekly selectors")
		}
		if len(p.Days) == 0 {
			p.Days = []string{calendarDays[start.Weekday()]}
		}
	case "MONTHLY", "YEARLY":
		if p.Frequency == "YEARLY" {
			if p.Month == 0 {
				if p.Ordinal != 0 || p.Day != 0 || p.ClampDay != 0 {
					return nil, fmt.Errorf("year-wide selectors")
				}
				p.Month = int(start.Month())
			}
		} else if p.Month != 0 {
			return nil, fmt.Errorf("monthly month filter")
		}
		if p.Ordinal != 0 {
			if p.Ordinal < -1 || p.Ordinal > 4 || len(p.Days) == 0 || p.Day != 0 {
				return nil, fmt.Errorf("unrepresentable ordinal")
			}
		}
		if p.Ordinal == 0 && len(p.Days) > 0 {
			return nil, fmt.Errorf("missing ordinal")
		}
		if p.Ordinal == 0 && p.Day == 0 && p.ClampDay == 0 {
			p.Day = start.Day()
		}
	default:
		return nil, fmt.Errorf("unsupported recurrence frequency")
	}
	return p, nil
}

func (p calendarPattern) raw(allDay bool) (string, error) {
	if p.Interval < 1 || p.Interval > 99 || p.Count < 0 || p.Count > 65535 || p.Count > 0 && p.Until != nil {
		return "", fmt.Errorf("invalid recurrence range")
	}
	parts := []string{"FREQ=" + p.Frequency, "INTERVAL=" + strconv.Itoa(p.Interval)}
	if p.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(p.Count))
	}
	if p.Until != nil {
		value := p.Until.UTC().Format("20060102T150405Z")
		if allDay {
			value = p.Until.Format("20060102")
		}
		parts = append(parts, "UNTIL="+value)
	}
	if p.Month > 0 {
		parts = append(parts, "BYMONTH="+strconv.Itoa(p.Month))
	}
	if p.ClampDay > 28 {
		var days []string
		for d := 28; d <= p.ClampDay; d++ {
			days = append(days, strconv.Itoa(d))
		}
		parts = append(parts, "BYMONTHDAY="+strings.Join(days, ","), "BYSETPOS=-1")
	} else if p.Day != 0 {
		parts = append(parts, "BYMONTHDAY="+strconv.Itoa(p.Day))
	}
	if len(p.Days) > 0 {
		for _, d := range p.Days {
			if dayIndex(d) < 0 {
				return "", fmt.Errorf("invalid weekday")
			}
		}
		parts = append(parts, "BYDAY="+strings.Join(p.Days, ","))
	}
	if p.Ordinal != 0 {
		if p.Ordinal < -1 || p.Ordinal > 4 || len(p.Days) == 0 {
			return "", fmt.Errorf("invalid ordinal")
		}
		parts = append(parts, "BYSETPOS="+strconv.Itoa(p.Ordinal))
	}
	if p.Frequency == "WEEKLY" {
		if dayIndex(p.WeekStart) < 0 {
			return "", fmt.Errorf("invalid week start")
		}
		parts = append(parts, "WKST="+p.WeekStart)
	}
	return strings.Join(parts, ";"), nil
}

func isRecurringCalendar(data string) bool {
	return davTextProperty(data, "RRULE") != "" || davTextProperty(data, "RDATE") != ""
}

func validateExchangeDate(p *calendarPattern) error {
	if p.Day > 28 && p.ClampDay == 0 && !(p.Frequency == "YEARLY" && p.Month != 2 && p.Day <= time.Date(2001, time.Month(p.Month)+1, 0, 0, 0, 0, 0, time.UTC).Day()) {
		return fmt.Errorf("skipped month dates cannot be expressed as clamped Exchange dates")
	}
	return nil
}

// Keep the stored RRULE when a client echoes an equivalent protocol pattern.
// Cosmetic normalization must not turn an instance patch into a series update.
func preserveEquivalentRecurrence(before, after string) string {
	a, ea := readCalendarPattern(before)
	b, eb := readCalendarPattern(after)
	if ea != nil || eb != nil || a == nil || b == nil {
		return after
	}
	_, _, sa, ea := calendarComponent(before)
	_, _, sb, eb := calendarComponent(after)
	if ea != nil || eb != nil || sa.Location().String() != sb.Location().String() || sa.Format("150405") != sb.Format("150405") {
		return after
	}
	normalize := func(p *calendarPattern, start time.Time) string {
		end := ""
		if p.Until != nil {
			v := p.Until.In(start.Location())
			if v.Format("150405") < start.Format("150405") {
				v = v.AddDate(0, 0, -1)
			}
			end = v.Format("2006-01-02")
			p.Until = nil
		}
		if p.ClampDay == 31 {
			p.ClampDay = 0
			p.Day = -1
		}
		if p.Day == -1 {
			p.Day = 0
			p.Ordinal = -1
			p.Days = append([]string{}, calendarDays...)
		}
		sort.Slice(p.Days, func(i, j int) bool { return dayIndex(p.Days[i]) < dayIndex(p.Days[j]) })
		return end
	}
	enda, endb := normalize(a, sa), normalize(b, sb)
	if enda != endb || !reflect.DeepEqual(a, b) {
		return after
	}
	restored, err := calutil.SetClientRecurrence(after, davTextProperty(before, "RRULE"))
	if err != nil {
		return after
	}
	return restored
}
