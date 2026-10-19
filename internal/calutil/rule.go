package calutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// RecurrenceRule is the deliberately small, lossless subset exposed by the web
// editor. Until is an inclusive date in the DTSTART timezone, not browser UTC.
type RecurrenceRule struct {
	Frequency string   `json:"frequency"`
	Interval  int      `json:"interval"`
	Weekdays  []string `json:"weekdays,omitempty"`
	Count     int      `json:"count,omitempty"`
	Until     string   `json:"until,omitempty"`
}

type RecurrenceInfo struct {
	Rule       RecurrenceRule `json:"rule"`
	Editable   bool           `json:"editable"`
	Reason     string         `json:"reason,omitempty"`
	Raw        string         `json:"raw,omitempty"`
	Timezone   string         `json:"timezone"`
	Exceptions int            `json:"exceptions"`
}

var weekdays = []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

func ruleStart(master *ical.Component) (time.Time, error) {
	p := master.Props.Get("DTSTART")
	if p == nil {
		return time.Time{}, fmt.Errorf("DTSTART required")
	}
	if tz := p.Params.Get("TZID"); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return time.Time{}, fmt.Errorf("unknown timezone: %s", tz)
		}
	}
	return p.DateTime(time.UTC)
}

func buildRule(rule RecurrenceRule, master *ical.Component) (string, error) {
	start, err := ruleStart(master)
	if err != nil {
		return "", err
	}
	if rule.Frequency == "" {
		if rule.Count != 0 || rule.Until != "" || len(rule.Weekdays) != 0 || rule.Interval > 1 || rule.Interval < 0 {
			return "", fmt.Errorf("non-recurring event cannot have recurrence options")
		}
		return "", nil
	}
	switch rule.Frequency {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	default:
		return "", fmt.Errorf("unsupported recurrence frequency")
	}
	if rule.Interval < 1 || rule.Interval > 99 {
		return "", fmt.Errorf("recurrence interval must be 1–99")
	}
	if rule.Count < 0 || rule.Count > 1000 || rule.Count > 0 && rule.Until != "" {
		return "", fmt.Errorf("choose either a count of 1–1000 or an end date")
	}
	if len(rule.Weekdays) > 0 && rule.Frequency != "WEEKLY" {
		return "", fmt.Errorf("weekdays require a weekly recurrence")
	}
	parts := []string{"FREQ=" + rule.Frequency, "INTERVAL=" + strconv.Itoa(rule.Interval)}
	if rule.Frequency == "WEEKLY" {
		if len(rule.Weekdays) == 0 {
			rule.Weekdays = []string{weekdays[start.Weekday()]}
		}
		seen := map[string]bool{}
		for _, day := range rule.Weekdays {
			if !strings.Contains(",SU,MO,TU,WE,TH,FR,SA,", ","+day+",") || len(day) != 2 || seen[day] {
				return "", fmt.Errorf("invalid or repeated weekday")
			}
			seen[day] = true
		}
		if !seen[weekdays[start.Weekday()]] {
			return "", fmt.Errorf("weekdays must include the first event's day")
		}
		ordered := []string{}
		for _, day := range []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"} {
			if seen[day] {
				ordered = append(ordered, day)
			}
		}
		parts = append(parts, "BYDAY="+strings.Join(ordered, ","), "WKST=MO")
	}
	if rule.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(rule.Count))
	}
	if rule.Until != "" {
		date, e := time.ParseInLocation("2006-01-02", rule.Until, start.Location())
		if e != nil || rule.Until < start.Format("2006-01-02") {
			return "", fmt.Errorf("recurrence end date must be on or after the first event")
		}
		p := master.Props.Get("DTSTART")
		until := date.Format("20060102")
		if !dateProp(p) {
			end := date.AddDate(0, 0, 1).Add(-time.Second)
			if p.Params.Get("TZID") != "" || strings.HasSuffix(p.Value, "Z") {
				until = end.UTC().Format("20060102T150405Z")
			} else {
				until = end.Format("20060102T150405")
			}
		}
		parts = append(parts, "UNTIL="+until)
	}
	return strings.Join(parts, ";"), nil
}

// ReadRecurrence refuses to approximate complex imported rules. They continue
// to round-trip unchanged through ordinary event edits.
func ReadRecurrence(data string) RecurrenceInfo {
	info := RecurrenceInfo{Rule: RecurrenceRule{Interval: 1}, Editable: false}
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		info.Reason = err.Error()
		return info
	}
	start, err := ruleStart(master)
	if err != nil {
		info.Reason = err.Error()
		return info
	}
	info.Timezone = master.Props.Get("DTSTART").Params.Get("TZID")
	if info.Timezone == "" {
		info.Timezone = "UTC"
		if !dateProp(master.Props.Get("DTSTART")) && !strings.HasSuffix(master.Props.Get("DTSTART").Value, "Z") {
			info.Timezone = "floating"
		}
	}
	for _, c := range cal.Children {
		if c.Name == "VEVENT" && c.Props.Get("RECURRENCE-ID") != nil {
			info.Exceptions++
		}
	}
	if master.Props.Get("RECURRENCE-ID") != nil {
		info.Reason = "detached occurrence has no series master"
		return info
	}
	p := master.Props.Get("RRULE")
	if p == nil {
		if master.Props.Get("RDATE") != nil || master.Props.Get("EXDATE") != nil {
			info.Reason = "separate recurrence dates require the original calendar client"
			return info
		}
		info.Editable = true
		return info
	}
	info.Raw = p.Value
	info.Reason = "complex recurrence must be edited in the original calendar client"
	if len(master.Props.Values("RRULE")) != 1 {
		return info
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(p.Value, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 || seen[kv[0]] {
			return info
		}
		seen[kv[0]] = true
		switch kv[0] {
		case "FREQ":
			info.Rule.Frequency = kv[1]
		case "INTERVAL":
			info.Rule.Interval, err = strconv.Atoi(kv[1])
		case "BYDAY":
			info.Rule.Weekdays = strings.Split(kv[1], ",")
		case "WKST":
			if kv[1] != "MO" {
				return info
			}
		case "COUNT":
			info.Rule.Count, err = strconv.Atoi(kv[1])
			if info.Rule.Count < 1 {
				return info
			}
		case "UNTIL":
			u := ical.NewProp("DTSTART")
			u.Value = kv[1]
			if dateProp(u) != dateProp(master.Props.Get("DTSTART")) {
				return info
			}
			if dateProp(master.Props.Get("DTSTART")) {
				u.Params.Set("VALUE", "DATE")
			}
			var end time.Time
			end, err = u.DateTime(start.Location())
			if err == nil {
				local := end.In(start.Location())
				// A cutoff before the event's wall-clock time excludes that day.
				if !dateProp(u) && local.Format("150405") < start.Format("150405") {
					local = local.AddDate(0, 0, -1)
				}
				info.Rule.Until = local.Format("2006-01-02")
			}
		default:
			return info
		}
		if err != nil {
			return info
		}
	}
	if !seen["FREQ"] || seen["COUNT"] && seen["UNTIL"] {
		return info
	}
	if _, err = buildRule(info.Rule, master); err != nil {
		return info
	}
	info.Editable = true
	info.Reason = ""
	return info
}

// WithRecurrence changes only recurrence and scheduling versions. It retains
// timezone definitions, reminders, attachments and every detached instance.
// A rule that would orphan an edited/cancelled occurrence is rejected.
func WithRecurrence(data string, rule RecurrenceRule, sequence int) (string, error) {
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	info := ReadRecurrence(data)
	if !info.Editable {
		return "", fmt.Errorf("%s", info.Reason)
	}
	raw, err := buildRule(rule, master)
	if err != nil {
		return "", err
	}
	if raw == "" && (info.Exceptions > 0 || master.Props.Get("RDATE") != nil || master.Props.Get("EXDATE") != nil) {
		return "", fmt.Errorf("cannot remove recurrence while separate dates or exceptions exist")
	}
	delete(master.Props, "RRULE")
	if raw != "" {
		master.Props.Set(&ical.Prop{Name: "RRULE", Value: raw})
	}
	// Test identities against the new generation rule, not detached components
	// (GetOccurrence deliberately returns detached components even outside it).
	probe, err := cloneEvent(cal, master)
	if err != nil {
		return "", err
	}
	delete(probe.Props, "EXDATE")
	probeData, err := encode(componentCalendar(cal, probe))
	if err != nil {
		return "", err
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		if rid := c.Props.Get("RECURRENCE-ID"); rid != nil {
			value, e := rid.DateTime(time.UTC)
			if e != nil {
				return "", e
			}
			if _, e = GetOccurrence(probeData, recurrenceKey(value, dateProp(rid))); e != nil {
				return "", fmt.Errorf("new rule would remove an edited or cancelled occurrence: %s", recurrenceKey(value, dateProp(rid)))
			}
		}
		c.Props.Set(&ical.Prop{Name: "SEQUENCE", Value: strconv.Itoa(sequence)})
		c.Props.SetDateTime("DTSTAMP", time.Now().UTC())
		for _, a := range c.Props.Values("ATTENDEE") {
			status, _ := c.Props.Text("STATUS")
			if !strings.EqualFold(status, "CANCELLED") {
				a.Params.Set("PARTSTAT", "NEEDS-ACTION")
				a.Params.Set("RSVP", "TRUE")
			}
		}
	}
	return encode(cal)
}

// SetRecurrenceTimezone is used only for newly created web series. Existing
// DTSTART representation is retained by WithRecurrence.
func SetRecurrenceTimezone(data, timezone string) (string, error) {
	if timezone == "" || timezone == "UTC" {
		return data, nil
	}
	if timezone == "Local" {
		return "", fmt.Errorf("an IANA timezone is required")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("unknown timezone")
	}
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	for _, key := range []string{"DTSTART", "DTEND"} {
		p := master.Props.Get(key)
		if p == nil || dateProp(p) {
			continue
		}
		v, e := p.DateTime(time.UTC)
		if e != nil {
			return "", e
		}
		p.Value = v.In(loc).Format("20060102T150405")
		p.Params.Set("TZID", timezone)
	}
	return encode(cal)
}
