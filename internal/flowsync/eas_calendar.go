// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

// Calendar edits use the same atomic scheduling/outbox path as CalDAV. In
// particular an attendee-only edit must not be acknowledged and discarded.
func (h *easHandler) wbCalendar(ctx context.Context, u *storage.User, calendarID string, op clientOp) (string, error) {
	var obj *storage.CalendarObject
	if op.Kind == "add" {
		uid := op.Fields["UID"]
		device, _ := ctx.Value(taskDeviceKey{}).(string)
		href := storage.NewID() + ".ics"
		if device != "" && op.ClientID != "" {
			href = "eas-" + digest(device+"\x00"+op.ClientID) + ".ics"
			old, err := h.store.GetCalendarObject(ctx, calendarID, href)
			if err == nil {
				if uid != "" && old.UID != uid {
					return "", storage.ErrCalendarConflict
				}
				return old.ID, nil // retry after an accepted Add, including a restart
			}
			if err != storage.ErrNotFound {
				return "", err
			}
		}
		if uid == "" {
			uid = storage.NewID()
		}
		obj = &storage.CalendarObject{CalendarID: calendarID, UID: uid, HrefName: href, Component: "VEVENT", CreateOnly: true}
		obj.Data = buildVEVENT(uid, "", "", nil, nil, false)
	} else {
		var err error
		obj, err = h.store.GetCalendarObjectByID(ctx, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
		if obj.CalendarID != calendarID {
			return op.ServerID, storage.ErrUnauthorized
		}
		obj.ExpectedETag = obj.ETag
	}
	if op.Kind == "delete" {
		return obj.ID, h.store.DeleteCalendarMeeting(ctx, u.ID, obj)
	}
	if op.Kind != "add" && op.Kind != "change" {
		return op.ServerID, fmt.Errorf("invalid calendar operation")
	}
	data, err := editEASCalendar(obj.Data, u, op)
	if err != nil {
		return op.ServerID, err
	}
	p, err := calutil.ParseICS(data)
	if err != nil || p.Start == nil || p.End == nil || !p.End.After(*p.Start) {
		return op.ServerID, fmt.Errorf("invalid calendar dates")
	}
	obj.Data, obj.DTStart, obj.DTEnd = data, p.Start, p.End
	saved, err := h.store.SaveCalendarClientObject(ctx, u.ID, obj)
	if err != nil {
		return op.ServerID, err
	}
	return saved.ID, nil
}

func calendarEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	a, err := mail.ParseAddress(raw)
	if err != nil || !strings.EqualFold(a.Address, raw) || strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("invalid calendar email")
	}
	return strings.ToLower(a.Address), nil
}

func editEASCalendar(data string, u *storage.User, op clientOp) (string, error) {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return "", err
	}
	ev := calutil.MasterEvent(cal)
	if ev == nil {
		return "", fmt.Errorf("missing event")
	}
	if cal.Props.Get("PRODID") == nil {
		cal.Props.SetText("PRODID", calutil.ProdID)
	}
	if cal.Props.Get("VERSION") == nil {
		cal.Props.SetText("VERSION", "2.0")
	}
	if ev.Props.Get("DTSTAMP") == nil {
		ev.Props.SetDateTime("DTSTAMP", time.Now().UTC())
	}
	fields := make(map[string]string, len(op.Fields))
	for k, v := range op.Fields {
		fields[k] = v
	}
	// Direct children preserve empty values and avoid mistaking an attendee's
	// nested fields for event fields.
	if op.ApplicationData != nil {
		for _, n := range op.ApplicationData.Children {
			if len(n.Children) == 0 {
				fields[n.Name.Local] = n.Text
			}
		}
	}
	if location := op.ApplicationData.child("Location"); location != nil && location.Name.Space == "AirSyncBase:" {
		fields["Location"] = location.value("DisplayName")
	}
	storedUID, _ := ev.Props.Text("UID")
	if uid, ok := fields["UID"]; ok && uid != storedUID {
		return "", storage.ErrCalendarConflict
	}
	for field, prop := range map[string]string{"Subject": "SUMMARY", "Location": "LOCATION"} {
		if v, ok := fields[field]; ok {
			ev.Props.SetText(prop, v)
		}
	}
	parsed, err := calutil.ParseICS(data)
	if err != nil && op.Kind != "add" {
		return "", err
	}
	allDay := parsed.AllDay
	if v, ok := fields["AllDayEvent"]; ok {
		if v != "0" && v != "1" {
			return "", fmt.Errorf("invalid all-day flag")
		}
		allDay = v == "1"
	}
	for field, prop := range map[string]string{"StartTime": "DTSTART", "EndTime": "DTEND"} {
		raw, changed := fields[field]
		_, dayChanged := fields["AllDayEvent"]
		if !changed && (!dayChanged || allDay == parsed.AllDay) {
			continue
		}
		t := parseASTime(raw)
		if !changed {
			if field == "StartTime" {
				t = parsed.Start
			} else {
				t = parsed.End
			}
		}
		if t == nil {
			return "", fmt.Errorf("invalid calendar time")
		}
		if allDay {
			ev.Props.SetDate(prop, *t)
		} else {
			ev.Props.SetDateTime(prop, *t)
		}
	}
	if v, ok := fields["BusyStatus"]; ok {
		status := map[string]string{"0": "FREE", "1": "TENTATIVE", "2": "BUSY", "3": "OOF", "4": "WORKINGELSEWHERE"}[v]
		if status == "" {
			return "", fmt.Errorf("invalid busy status")
		}
		ev.Props.SetText("X-MICROSOFT-CDO-BUSYSTATUS", status)
		transp := "OPAQUE"
		if v == "0" {
			transp = "TRANSPARENT"
		}
		ev.Props.SetText("TRANSP", transp)
	}
	attendees := op.ApplicationData.child("Attendees")
	organizer := parsed.Organizer.Email
	if raw, ok := fields["OrganizerEmail"]; ok && raw != "" {
		requested, e := calendarEmail(raw)
		if e != nil {
			return "", e
		}
		if organizer != "" && !strings.EqualFold(requested, organizer) || organizer == "" && !strings.EqualFold(requested, u.Email) {
			return "", storage.ErrUnauthorized
		}
		organizer = requested
	}
	if attendees != nil && len(attendees.Children) > 0 && organizer == "" {
		organizer = u.Email
	}
	if organizer != "" && parsed.Organizer.Email == "" {
		for _, component := range cal.Children {
			if component.Name != "VEVENT" {
				continue
			}
			p := ical.NewProp("ORGANIZER")
			p.Value = "mailto:" + organizer
			p.Params.Set("CN", fieldOr(fields["OrganizerName"], u.DisplayName))
			component.Props.Set(p)
		}
	}
	if attendees != nil {
		if len(attendees.Children) > 1000 {
			return "", fmt.Errorf("too many attendees")
		}
		var list []ical.Prop
		seen := map[string]bool{}
		for _, n := range attendees.Children {
			if n.Name.Local != "Attendee" {
				return "", fmt.Errorf("invalid attendee")
			}
			email, e := calendarEmail(n.value("Email"))
			if e != nil {
				return "", e
			}
			if seen[email] {
				continue
			}
			seen[email] = true
			p := ical.NewProp("ATTENDEE")
			p.Value = "mailto:" + email
			p.Params.Set("CN", n.value("Name"))
			p.Params.Set("ROLE", "REQ-PARTICIPANT")
			switch n.value("AttendeeType") {
			case "", "1":
			case "2":
				p.Params.Set("ROLE", "OPT-PARTICIPANT")
			case "3":
				p.Params.Set("CUTYPE", "RESOURCE")
			default:
				return "", fmt.Errorf("invalid attendee type")
			}
			// Organizer edits cannot forge replies; retain existing participant
			// state until PrepareOrganizerWrite decides whether a new RSVP is due.
			p.Params.Set("PARTSTAT", "NEEDS-ACTION")
			p.Params.Set("RSVP", "TRUE")
			for _, old := range ev.Props.Values("ATTENDEE") {
				if strings.EqualFold(strings.TrimPrefix(old.Value, "mailto:"), email) {
					for _, k := range []string{"PARTSTAT", "RSVP", "SCHEDULE-AGENT"} {
						if v := old.Params.Get(k); v != "" {
							p.Params.Set(k, v)
						}
					}
				}
			}
			list = append(list, *p)
		}
		// Received meetings may echo their attendee list, but cannot change
		// it or schedule mail on behalf of the organizer.
		if organizer != "" && !strings.EqualFold(organizer, u.Email) {
			if len(seen) != len(parsed.Attendees) {
				return "", storage.ErrUnauthorized
			}
			for _, a := range parsed.Attendees {
				if !seen[strings.ToLower(a.Email)] {
					return "", storage.ErrUnauthorized
				}
			}
		} else {
			ev.Props["ATTENDEE"] = list
		}
	}
	var out strings.Builder
	if err := ical.NewEncoder(&out).Encode(cal); err != nil {
		return "", err
	}
	return out.String(), nil
}

func calendarMeetingXML(data, viewer string) string {
	p, err := calutil.ParseICS(data)
	if err != nil {
		return ""
	}
	var out strings.Builder
	field := func(k, v string) {
		fmt.Fprintf(&out, `<Calendar:%s xmlns:Calendar="Calendar:">%s</Calendar:%s>`, k, xmlEscape(v), k)
	}
	busy := "2"
	if strings.EqualFold(davTextProperty(data, "TRANSP"), "TRANSPARENT") {
		busy = "0"
	} else {
		switch strings.ToUpper(davTextProperty(data, "X-MICROSOFT-CDO-BUSYSTATUS")) {
		case "TENTATIVE":
			busy = "1"
		case "OOF":
			busy = "3"
		case "WORKINGELSEWHERE":
			busy = "4"
		}
	}
	if stamp := davTextProperty(data, "DTSTAMP"); stamp != "" {
		field("DTStamp", stamp)
	}
	field("BusyStatus", busy)
	status := 0
	if len(p.Attendees) > 0 {
		status = 1
		if !strings.EqualFold(p.Organizer.Email, viewer) {
			status |= 2
		}
	}
	if p.Status == "CANCELLED" {
		status |= 4
	}
	field("MeetingStatus", fmt.Sprint(status))
	if p.Organizer.Email != "" {
		field("OrganizerEmail", p.Organizer.Email)
		field("OrganizerName", fieldOr(p.Organizer.Name, p.Organizer.Email))
	}
	if len(p.Attendees) > 0 {
		out.WriteString(`<Calendar:Attendees xmlns:Calendar="Calendar:">`)
		for _, a := range p.Attendees {
			out.WriteString(`<Calendar:Attendee>`)
			field("Email", a.Email)
			field("Name", fieldOr(a.Name, a.Email))
			typ := "1"
			if a.Role == "OPT-PARTICIPANT" {
				typ = "2"
			}
			if a.CUType == "RESOURCE" || a.CUType == "ROOM" {
				typ = "3"
			}
			field("AttendeeType", typ)
			state := map[string]string{"ACCEPTED": "3", "TENTATIVE": "2", "DECLINED": "4"}[a.PartStat]
			if state == "" {
				state = "5"
			}
			field("AttendeeStatus", state)
			out.WriteString(`</Calendar:Attendee>`)
		}
		out.WriteString(`</Calendar:Attendees>`)
	}
	return out.String()
}
