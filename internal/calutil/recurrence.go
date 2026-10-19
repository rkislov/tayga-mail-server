package calutil

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
)

// Occurrence keeps the original start as its identity, including after a move.
// RecurrenceID is empty for a non-recurring event, a date for VALUE=DATE, or UTC.
type Occurrence struct {
	ParseResult
	RecurrenceID string
	Transparent  bool
}

type OccurrenceChange struct {
	Start, End                          *time.Time
	Summary, Location, Description, Geo *string
	Cancel                              bool
}

// NextSequence also accounts for detached instances that have newer versions
// than the master, so a later whole-series update/cancellation is not stale.
func NextSequence(data string) int {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return parseSimple(data).Sequence + 1
	}
	max := 0
	for _, c := range cal.Children {
		if c.Name == "VEVENT" {
			if p := c.Props.Get("SEQUENCE"); p != nil {
				n, _ := p.Int()
				if n > max {
					max = n
				}
			}
		}
	}
	return max + 1
}

func recurrenceKey(t time.Time, date bool) string {
	if date {
		return t.Format("2006-01-02")
	}
	return t.UTC().Format(time.RFC3339)
}

func dateProp(p *ical.Prop) bool {
	return p != nil && (p.Params.Get("VALUE") == "DATE" || len(p.Value) == 8)
}

func componentCalendar(cal *ical.Calendar, event *ical.Component) *ical.Calendar {
	out := ical.NewCalendar()
	out.Props.SetText("VERSION", "2.0")
	out.Props.SetText("PRODID", ProdID)
	for _, c := range cal.Children {
		if c.Name == "VTIMEZONE" {
			out.Children = append(out.Children, c)
		}
	}
	out.Children = append(out.Children, event)
	return out
}

func cloneEvent(cal *ical.Calendar, event *ical.Component) (*ical.Component, error) {
	data, err := encode(componentCalendar(cal, event))
	if err != nil {
		return nil, err
	}
	copy, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return nil, err
	}
	return MasterEvent(copy), nil
}

func recurrenceCalendar(data string) (*ical.Calendar, *ical.Component, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return nil, nil, err
	}
	master := MasterEvent(cal)
	if master == nil {
		return nil, nil, fmt.Errorf("VEVENT required")
	}
	uid, _ := master.Props.Text("UID")
	if uid == "" {
		return nil, nil, fmt.Errorf("event UID required")
	}
	if cal.Props.Get("VERSION") == nil {
		cal.Props.SetText("VERSION", "2.0")
	}
	if cal.Props.Get("PRODID") == nil {
		cal.Props.SetText("PRODID", ProdID)
	}
	masters := 0
	seen := map[string]bool{}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		// Older imports can lack DTSTAMP. Use a stable value for their projected
		// representation; merely listing a calendar never rewrites stored data.
		if c.Props.Get("DTSTAMP") == nil {
			c.Props.SetDateTime("DTSTAMP", time.Unix(0, 0).UTC())
		}
		u, _ := c.Props.Text("UID")
		if u != uid {
			return nil, nil, fmt.Errorf("mixed event UIDs")
		}
		if p := c.Props.Get("RECURRENCE-ID"); p != nil {
			if p.Params.Get("RANGE") != "" {
				return nil, nil, fmt.Errorf("RANGE recurrence requires a calendar client")
			}
			t, err := p.DateTime(time.UTC)
			if err != nil {
				return nil, nil, err
			}
			key := recurrenceKey(t, dateProp(p))
			if seen[key] {
				return nil, nil, fmt.Errorf("duplicate recurrence exception")
			}
			seen[key] = true
			if dateProp(p) != dateProp(master.Props.Get("DTSTART")) {
				return nil, nil, fmt.Errorf("recurrence value type mismatch")
			}
		} else {
			masters++
		}
	}
	if masters > 1 {
		return nil, nil, fmt.Errorf("multiple recurrence masters")
	}
	return cal, master, nil
}

// go-ical's RecurrenceSet doesn't handle comma-separated dates or RDATE-only
// series. Build a set explicitly, preserving the master's local timezone.
func recurrenceSet(c *ical.Component, start time.Time) (*rrule.Set, error) {
	set := &rrule.Set{}
	set.DTStart(start)
	set.RDate(start)
	if p := c.Props.Get("RRULE"); p != nil {
		upper := strings.ToUpper(p.Value)
		for _, dense := range []string{"FREQ=SECONDLY", "FREQ=MINUTELY", "FREQ=HOURLY", "BYSECOND=", "BYMINUTE=", "BYHOUR="} {
			if strings.Contains(upper, dense) {
				return nil, fmt.Errorf("unsupported dense recurrence")
			}
		}
		if len(c.Props.Values("RRULE")) != 1 {
			return nil, fmt.Errorf("multiple recurrence rules")
		}
		opt, err := c.Props.RecurrenceRule()
		if err != nil {
			return nil, err
		}
		opt.Dtstart = start
		rule, err := rrule.NewRRule(*opt)
		if err != nil {
			return nil, err
		}
		set.RRule(rule)
	}
	for _, name := range []string{"RDATE", "EXDATE"} {
		for _, p := range c.Props.Values(name) {
			for _, value := range strings.Split(p.Value, ",") {
				prop := p
				prop.Value = value
				t, err := prop.DateTime(start.Location())
				if err != nil {
					return nil, err
				}
				if name == "RDATE" {
					set.RDate(t)
				} else {
					set.ExDate(t)
				}
			}
		}
	}
	return set, nil
}

func setTimeLike(c *ical.Component, name string, value time.Time, template *ical.Prop) error {
	p := ical.NewProp(name)
	switch {
	case dateProp(template):
		p.SetDate(value)
	case template != nil && template.Params.Get("TZID") != "":
		tz := template.Params.Get("TZID")
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("unsupported timezone %q", tz)
		}
		p.Params.Set("TZID", tz)
		p.Value = value.In(loc).Format("20060102T150405")
	case template != nil && !strings.HasSuffix(template.Value, "Z"):
		p.Value = value.UTC().Format("20060102T150405")
	default:
		p.SetDateTime(value.UTC())
	}
	c.Props.Set(p)
	return nil
}

func occurrenceFrom(cal *ical.Calendar, c *ical.Component, rid string) (Occurrence, error) {
	data, err := encode(componentCalendar(cal, c))
	if err != nil {
		return Occurrence{}, err
	}
	p, err := ParseICS(data)
	transp, _ := c.Props.Text("TRANSP")
	return Occurrence{ParseResult: p, RecurrenceID: rid, Transparent: strings.EqualFold(transp, "TRANSPARENT")}, err
}

func generatedOccurrence(cal *ical.Calendar, master *ical.Component, start, end time.Time) (*ical.Component, error) {
	c, err := cloneEvent(cal, master)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"RRULE", "RDATE", "EXDATE", "DURATION"} {
		delete(c.Props, name)
	}
	template := master.Props.Get("DTSTART")
	if err := setTimeLike(c, "RECURRENCE-ID", start, template); err != nil {
		return nil, err
	}
	if err := setTimeLike(c, "DTSTART", start, template); err != nil {
		return nil, err
	}
	if err := setTimeLike(c, "DTEND", end, template); err != nil {
		return nil, err
	}
	return c, nil
}

// ExpandOccurrences is bounded both by output size and scanned recurrences.
// Detached exceptions are included by their actual interval, even if their
// original start lies outside the requested window. Cancelled instances vanish.
func ExpandOccurrences(data string, from, to time.Time) ([]Occurrence, error) {
	if !to.After(from) {
		return nil, fmt.Errorf("invalid recurrence window")
	}
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return nil, err
	}
	out := []Occurrence{}
	status, _ := master.Props.Text("STATUS")
	if master.Props.Get("RECURRENCE-ID") == nil && strings.EqualFold(status, "CANCELLED") {
		return out, nil
	}
	overrides := map[string]bool{}
	add := func(c *ical.Component, rid string) error {
		o, err := occurrenceFrom(cal, c, rid)
		if err != nil {
			return err
		}
		if strings.EqualFold(o.Status, "CANCELLED") {
			return nil
		}
		if o.Start == nil || o.End == nil {
			return fmt.Errorf("invalid occurrence interval")
		}
		if o.Start.Before(to) && (o.End.After(from) || o.End.Equal(*o.Start) && !o.Start.Before(from)) {
			out = append(out, o)
			if len(out) > 10000 {
				return fmt.Errorf("too many occurrences")
			}
		}
		return nil
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		if p := c.Props.Get("RECURRENCE-ID"); p != nil {
			t, err := p.DateTime(time.UTC)
			if err != nil {
				return nil, err
			}
			rid := recurrenceKey(t, dateProp(p))
			if overrides[rid] {
				return nil, fmt.Errorf("duplicate recurrence exception")
			}
			overrides[rid] = true
			if err := add(c, rid); err != nil {
				return nil, err
			}
		}
	}
	if master.Props.Get("RECURRENCE-ID") == nil {
		recurring := master.Props.Get("RRULE") != nil || master.Props.Get("RDATE") != nil
		if !recurring {
			if err := add(master, ""); err != nil {
				return nil, err
			}
		} else {
			e := ical.Event{Component: master}
			start, err := e.DateTimeStart(time.UTC)
			if err != nil {
				return nil, err
			}
			end, err := e.DateTimeEnd(time.UTC)
			if err != nil || end.Before(start) || end.Sub(start) > 31*24*time.Hour {
				return nil, fmt.Errorf("invalid recurrence duration")
			}
			set, err := recurrenceSet(master, start)
			if err != nil {
				return nil, err
			}
			next := set.Iterator()
			for count := 0; ; count++ {
				date, ok := next()
				if !ok || !date.Before(to) {
					break
				}
				if count >= 100000 {
					return nil, fmt.Errorf("recurrence scan limit exceeded")
				}
				rid := recurrenceKey(date, dateProp(master.Props.Get("DTSTART")))
				if overrides[rid] || date.Add(end.Sub(start)).Before(from) {
					continue
				}
				c, err := generatedOccurrence(cal, master, date, date.Add(end.Sub(start)))
				if err != nil {
					return nil, err
				}
				if err := add(c, rid); err != nil {
					return nil, err
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(*out[j].Start) })
	return out, nil
}

// GetOccurrence resolves an original recurrence identity, never a moved start.
func GetOccurrence(data, rid string) (Occurrence, error) {
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return Occurrence{}, err
	}
	for _, c := range cal.Children {
		if c.Name != "VEVENT" {
			continue
		}
		if p := c.Props.Get("RECURRENCE-ID"); p != nil {
			t, err := p.DateTime(time.UTC)
			if err != nil {
				return Occurrence{}, err
			}
			if recurrenceKey(t, dateProp(p)) == rid {
				return occurrenceFrom(cal, c, rid)
			}
		}
	}
	if master.Props.Get("RRULE") == nil && master.Props.Get("RDATE") == nil {
		return Occurrence{}, fmt.Errorf("not a recurring event")
	}
	layout := time.RFC3339
	if dateProp(master.Props.Get("DTSTART")) {
		layout = "2006-01-02"
	}
	t, err := time.Parse(layout, rid)
	if err != nil {
		return Occurrence{}, fmt.Errorf("invalid recurrence identity")
	}
	items, err := ExpandOccurrences(data, t, t.Add(time.Second))
	if err != nil {
		return Occurrence{}, err
	}
	for _, o := range items {
		if o.RecurrenceID == rid {
			return o, nil
		}
	}
	return Occurrence{}, fmt.Errorf("recurrence instance not found")
}

// OccurrenceMessage emits only the selected instance and timezone definitions.
// It never sends the series master or private reminder alarms.
func OccurrenceMessage(instance Occurrence, cancel bool) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(instance.source)).Decode()
	if err != nil {
		return "", err
	}
	c := MasterEvent(cal)
	if c == nil || c.Props.Get("RECURRENCE-ID") == nil {
		return "", fmt.Errorf("recurrence identity required")
	}
	c.Children = nil
	method := "REQUEST"
	if cancel {
		method = "CANCEL"
		c.Props.SetText("STATUS", "CANCELLED")
	}
	cal.Props.SetText("METHOD", method)
	return encode(cal)
}

// ChangeOccurrence returns the full stored series and a single-instance iMIP
// payload. The master, recurrence rules and other exceptions remain untouched.
func ChangeOccurrence(data, rid string, change OccurrenceChange) (string, string, Occurrence, error) {
	current, err := GetOccurrence(data, rid)
	if err != nil {
		return "", "", Occurrence{}, err
	}
	if strings.EqualFold(current.Status, "CANCELLED") {
		return "", "", Occurrence{}, fmt.Errorf("occurrence already cancelled")
	}
	ev := current.Event
	if !change.Cancel {
		if change.Start != nil {
			ev.Start = change.Start
		}
		if change.End != nil {
			ev.End = change.End
		}
		if ev.Start == nil || ev.End == nil || !ev.End.After(*ev.Start) {
			return "", "", Occurrence{}, fmt.Errorf("end must be after start")
		}
		if ev.AllDay && (ev.Start.UTC().Format("150405") != "000000" || ev.End.UTC().Format("150405") != "000000") {
			return "", "", Occurrence{}, fmt.Errorf("all-day occurrence requires dates")
		}
		if change.Summary != nil {
			ev.Summary = *change.Summary
		}
		if change.Location != nil {
			ev.Location = *change.Location
		}
		if change.Description != nil {
			ev.Description = *change.Description
		}
		if change.Geo != nil {
			ev.Geo = *change.Geo
		}
	} else {
		ev.Status = "CANCELLED"
	}
	master, _ := ParseICS(data)
	if ev.Organizer.Email == "" {
		ev.Organizer = master.Organizer
	}
	if len(ev.Attendees) == 0 {
		ev.Attendees = master.Attendees
	}
	ev.Sequence = NextSequence(data)
	instance, err := BuildSimpleVEVENT(ev)
	if err != nil {
		return "", "", Occurrence{}, err
	}
	full, err := MergeOccurrence(data, instance)
	if err != nil {
		return "", "", Occurrence{}, err
	}
	message, _, err := recurrenceCalendar(instance)
	if err != nil {
		return "", "", Occurrence{}, err
	}
	method := "REQUEST"
	if change.Cancel {
		method = "CANCEL"
	}
	message.Props.SetText("METHOD", method)
	MasterEvent(message).Children = nil // Reminders belong to each recipient.
	wire, err := encode(message)
	if err != nil {
		return "", "", Occurrence{}, err
	}
	out, err := GetOccurrence(full, rid)
	return full, wire, out, err
}

// MergeOccurrence updates one exception in a recipient copy, retaining its
// own master (including participation, reminders and unrelated exceptions).
func MergeOccurrence(data, instance string) (string, error) {
	cal, master, err := recurrenceCalendar(data)
	if err != nil {
		return "", err
	}
	inc, target, err := recurrenceCalendar(instance)
	if err != nil {
		return "", err
	}
	p := target.Props.Get("RECURRENCE-ID")
	if p == nil {
		return "", fmt.Errorf("recurrence identity required")
	}
	t, err := p.DateTime(time.UTC)
	if err != nil {
		return "", err
	}
	rid := recurrenceKey(t, dateProp(p))
	uid, _ := master.Props.Text("UID")
	other, _ := target.Props.Text("UID")
	if uid != other {
		return "", fmt.Errorf("recurrence UID mismatch")
	}
	children := []*ical.Component{}
	reminders := master.Children
	for _, c := range cal.Children {
		if c.Name == "VEVENT" {
			if old := c.Props.Get("RECURRENCE-ID"); old != nil {
				v, err := old.DateTime(time.UTC)
				if err != nil {
					return "", err
				}
				if recurrenceKey(v, dateProp(old)) == rid {
					reminders = c.Children
					continue
				}
			}
		}
		children = append(children, c)
	}
	// An organizer update must not replace the recipient's reminder settings.
	target.Children = reminders
	// Keep any timezone referenced only by the new exception.
	for _, zone := range inc.Children {
		if zone.Name == "VTIMEZONE" {
			id, _ := zone.Props.Text("TZID")
			found := false
			for _, old := range children {
				v, _ := old.Props.Text("TZID")
				if old.Name == "VTIMEZONE" && v == id {
					found = true
				}
			}
			if !found {
				children = append(children, zone)
			}
		}
	}
	cal.Children = append(children, target)
	delete(cal.Props, "METHOD")
	return encode(cal)
}
