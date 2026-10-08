// Package calutil builds and parses meeting-invite iCalendar payloads (iMIP).
package calutil

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

const ProdID = "-//Tayga//Calendar//EN"

// Attendee is a meeting participant.
type Attendee struct {
	Email    string
	Name     string
	Role     string // REQ-PARTICIPANT, OPT-PARTICIPANT, CHAIR
	PartStat string // NEEDS-ACTION, ACCEPTED, DECLINED, TENTATIVE
	RSVP     bool
	CUType   string // INDIVIDUAL
}

// Attachment is an ATTACH property (typically a download URI).
type Attachment struct {
	URI         string
	Filename    string
	ContentType string
}

// Event holds fields for REQUEST / REPLY / COUNTER VEVENT payloads.
type Event struct {
	UID         string
	Summary     string
	Location    string
	Description string
	Start       *time.Time
	End         *time.Time
	AllDay      bool
	Sequence    int
	Status      string // CONFIRMED, CANCELLED, TENTATIVE
	Organizer   Attendee
	Attendees   []Attendee
	Attachments []Attachment
	Comment     string
}

// ParseResult is a decoded calendar event with scheduling metadata.
type ParseResult struct {
	Event
	Method string
}

func mailtoURL(email string) *url.URL {
	email = strings.TrimSpace(strings.ToLower(email))
	return &url.URL{Scheme: "mailto", Opaque: email}
}

func emailFromURI(u *url.URL) string {
	if u == nil {
		return ""
	}
	if u.Scheme == "mailto" {
		if u.Opaque != "" {
			return strings.ToLower(strings.TrimSpace(u.Opaque))
		}
		return strings.ToLower(strings.TrimSpace(u.String()[len("mailto:"):]))
	}
	s := u.String()
	s = strings.TrimPrefix(strings.ToLower(s), "mailto:")
	return strings.TrimSpace(s)
}

func setAttendeeProp(props ical.Props, a Attendee, isOrganizer bool) {
	name := ical.PropAttendee
	if isOrganizer {
		name = ical.PropOrganizer
	}
	prop := ical.NewProp(name)
	prop.SetURI(mailtoURL(a.Email))
	if a.Name != "" {
		prop.Params.Set(ical.ParamCommonName, a.Name)
	}
	if !isOrganizer {
		role := a.Role
		if role == "" {
			role = "REQ-PARTICIPANT"
		}
		prop.Params.Set(ical.ParamRole, role)
		ps := a.PartStat
		if ps == "" {
			ps = "NEEDS-ACTION"
		}
		prop.Params.Set(ical.ParamParticipationStatus, ps)
		if a.RSVP {
			prop.Params.Set(ical.ParamRSVP, "TRUE")
		}
		cutype := a.CUType
		if cutype == "" {
			cutype = "INDIVIDUAL"
		}
		prop.Params.Set("CUTYPE", cutype)
		props.Add(prop)
		return
	}
	props.Set(prop)
}

func buildCalendar(method string, ev Event) (*ical.Calendar, error) {
	if ev.UID == "" {
		return nil, fmt.Errorf("uid required")
	}
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, ProdID)
	if method != "" {
		cal.Props.SetText(ical.PropMethod, method)
	}
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, ev.UID)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	if ev.Summary != "" {
		event.Props.SetText(ical.PropSummary, ev.Summary)
	}
	if ev.Location != "" {
		event.Props.SetText(ical.PropLocation, ev.Location)
	}
	if ev.Description != "" {
		event.Props.SetText(ical.PropDescription, ev.Description)
	}
	if ev.Comment != "" {
		event.Props.SetText(ical.PropComment, ev.Comment)
	}
	if ev.Status != "" {
		event.Props.SetText(ical.PropStatus, ev.Status)
	}
	seqProp := ical.NewProp(ical.PropSequence)
	seqProp.Value = fmt.Sprintf("%d", ev.Sequence)
	event.Props.Set(seqProp)
	if ev.Start != nil {
		if ev.AllDay {
			event.Props.SetDate(ical.PropDateTimeStart, ev.Start.UTC())
		} else {
			event.Props.SetDateTime(ical.PropDateTimeStart, ev.Start.UTC())
		}
	}
	if ev.End != nil {
		if ev.AllDay {
			event.Props.SetDate(ical.PropDateTimeEnd, ev.End.UTC())
		} else {
			event.Props.SetDateTime(ical.PropDateTimeEnd, ev.End.UTC())
		}
	}
	if ev.Organizer.Email != "" {
		setAttendeeProp(event.Props, ev.Organizer, true)
	}
	for _, a := range ev.Attendees {
		if strings.TrimSpace(a.Email) == "" {
			continue
		}
		a.RSVP = a.RSVP || a.PartStat == "" || strings.EqualFold(a.PartStat, "NEEDS-ACTION")
		setAttendeeProp(event.Props, a, false)
	}
	for _, att := range ev.Attachments {
		uri := strings.TrimSpace(att.URI)
		if uri == "" {
			continue
		}
		prop := ical.NewProp(ical.PropAttach)
		if u, err := url.Parse(uri); err == nil {
			prop.SetURI(u)
		} else {
			prop.Value = uri
		}
		if att.ContentType != "" {
			prop.Params.Set(ical.ParamFormatType, att.ContentType)
		}
		if att.Filename != "" {
			prop.Params.Set("FILENAME", att.Filename)
		}
		event.Props.Add(prop)
	}
	cal.Children = append(cal.Children, event.Component)
	return cal, nil
}

func encode(cal *ical.Calendar) (string, error) {
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// BuildRequestICS builds a METHOD:REQUEST invite.
func BuildRequestICS(ev Event) (string, error) {
	if ev.Status == "" {
		ev.Status = "CONFIRMED"
	}
	for i := range ev.Attendees {
		if ev.Attendees[i].PartStat == "" {
			ev.Attendees[i].PartStat = "NEEDS-ACTION"
		}
		ev.Attendees[i].RSVP = true
	}
	cal, err := buildCalendar("REQUEST", ev)
	if err != nil {
		return "", err
	}
	return encode(cal)
}

// BuildReplyICS builds a METHOD:REPLY from one attendee.
func BuildReplyICS(ev Event, responder Attendee) (string, error) {
	ev.Attendees = []Attendee{responder}
	cal, err := buildCalendar("REPLY", ev)
	if err != nil {
		return "", err
	}
	return encode(cal)
}

// BuildCounterICS builds a METHOD:COUNTER with proposed times.
func BuildCounterICS(ev Event, responder Attendee) (string, error) {
	ev.Attendees = []Attendee{responder}
	cal, err := buildCalendar("COUNTER", ev)
	if err != nil {
		return "", err
	}
	return encode(cal)
}

// BuildSimpleVEVENT builds a plain VEVENT (no METHOD) for local storage.
func BuildSimpleVEVENT(ev Event) (string, error) {
	cal, err := buildCalendar("", ev)
	if err != nil {
		return "", err
	}
	return encode(cal)
}

// ParseICS decodes METHOD, ORGANIZER, ATTENDEE and core event fields.
func ParseICS(data string) (ParseResult, error) {
	var out ParseResult
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		// Fall back to line parser for minimal / hand-built ICS.
		return parseSimple(data), nil
	}
	if m, err := cal.Props.Text(ical.PropMethod); err == nil {
		out.Method = strings.ToUpper(m)
	}
	for _, event := range cal.Events() {
		out.UID, _ = event.Props.Text(ical.PropUID)
		out.Summary, _ = event.Props.Text(ical.PropSummary)
		out.Location, _ = event.Props.Text(ical.PropLocation)
		out.Description, _ = event.Props.Text(ical.PropDescription)
		out.Comment, _ = event.Props.Text(ical.PropComment)
		out.Status, _ = event.Props.Text(ical.PropStatus)
		if prop := event.Props.Get(ical.PropSequence); prop != nil {
			if n, err := prop.Int(); err == nil {
				out.Sequence = n
			} else {
				fmt.Sscanf(prop.Value, "%d", &out.Sequence)
			}
		}
		if t, err := event.Props.DateTime(ical.PropDateTimeStart, time.UTC); err == nil {
			out.Start = &t
		}
		if t, err := event.Props.DateTime(ical.PropDateTimeEnd, time.UTC); err == nil {
			out.End = &t
		}
		if org := event.Props.Get(ical.PropOrganizer); org != nil {
			if u, err := org.URI(); err == nil {
				out.Organizer.Email = emailFromURI(u)
			}
			out.Organizer.Name = org.Params.Get(ical.ParamCommonName)
		}
		for _, prop := range event.Props.Values(ical.PropAttendee) {
			a := Attendee{
				Name:     prop.Params.Get(ical.ParamCommonName),
				Role:     prop.Params.Get(ical.ParamRole),
				PartStat: prop.Params.Get(ical.ParamParticipationStatus),
				CUType:   prop.Params.Get("CUTYPE"),
				RSVP:     strings.EqualFold(prop.Params.Get(ical.ParamRSVP), "TRUE"),
			}
			if u, err := prop.URI(); err == nil {
				a.Email = emailFromURI(u)
			}
			if a.Email != "" {
				out.Attendees = append(out.Attendees, a)
			}
		}
		for _, prop := range event.Props.Values(ical.PropAttach) {
			att := Attachment{
				Filename:    prop.Params.Get("FILENAME"),
				ContentType: prop.Params.Get(ical.ParamFormatType),
			}
			if u, err := prop.URI(); err == nil && u != nil {
				att.URI = u.String()
			} else {
				att.URI = prop.Value
			}
			if att.URI != "" {
				out.Attachments = append(out.Attachments, att)
			}
		}
		break
	}
	return out, nil
}

// ParseAttendees extracts ATTENDEE lines from ICS text.
func ParseAttendees(data string) []Attendee {
	p, _ := ParseICS(data)
	return p.Attendees
}

func parseSimple(data string) ParseResult {
	var out ParseResult
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "METHOD:"):
			out.Method = strings.TrimSpace(line[len("METHOD:"):])
		case strings.HasPrefix(upper, "UID:"):
			out.UID = strings.TrimSpace(line[len("UID:"):])
		case strings.HasPrefix(upper, "SUMMARY:"):
			out.Summary = unescape(line[len("SUMMARY:"):])
		case strings.HasPrefix(upper, "LOCATION:"):
			out.Location = unescape(line[len("LOCATION:"):])
		case strings.HasPrefix(upper, "DESCRIPTION:"):
			out.Description = unescape(line[len("DESCRIPTION:"):])
		case strings.HasPrefix(upper, "SEQUENCE:"):
			fmt.Sscanf(strings.TrimSpace(line[len("SEQUENCE:"):]), "%d", &out.Sequence)
		case strings.HasPrefix(upper, "STATUS:"):
			out.Status = strings.TrimSpace(line[len("STATUS:"):])
		case strings.HasPrefix(upper, "DTSTART"):
			if i := strings.IndexByte(line, ':'); i >= 0 {
				if t := parseICSTime(line[i+1:]); t != nil {
					out.Start = t
				}
			}
		case strings.HasPrefix(upper, "DTEND"):
			if i := strings.IndexByte(line, ':'); i >= 0 {
				if t := parseICSTime(line[i+1:]); t != nil {
					out.End = t
				}
			}
		case strings.HasPrefix(upper, "ORGANIZER"):
			out.Organizer = parseAttendeeLine(line)
		case strings.HasPrefix(upper, "ATTENDEE"):
			out.Attendees = append(out.Attendees, parseAttendeeLine(line))
		}
	}
	return out
}

func parseAttendeeLine(line string) Attendee {
	a := Attendee{}
	if i := strings.IndexByte(line, ':'); i >= 0 {
		params := line[:i]
		val := line[i+1:]
		a.Email = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(val)), "mailto:")
		for _, part := range strings.Split(params, ";") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch strings.ToUpper(kv[0]) {
			case "CN":
				a.Name = unescape(kv[1])
			case "ROLE":
				a.Role = kv[1]
			case "PARTSTAT":
				a.PartStat = kv[1]
			case "RSVP":
				a.RSVP = strings.EqualFold(kv[1], "TRUE")
			case "CUTYPE":
				a.CUType = kv[1]
			}
		}
	}
	return a
}

func parseICSTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		"20060102T150405Z", "20060102T150405", "20060102",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return strings.TrimSpace(s)
}

// FormatWhen returns a short human-readable interval.
func FormatWhen(start, end *time.Time) string {
	if start == nil {
		return ""
	}
	s := start.Local().Format("2006-01-02 15:04")
	if end != nil {
		s += " – " + end.Local().Format("15:04")
	}
	return s
}
