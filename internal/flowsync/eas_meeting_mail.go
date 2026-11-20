package flowsync

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/mail"
	"strings"

	"github.com/tayga/tms/internal/calutil"
)

// A MIME calendar attachment alone is not an ActiveSync meeting request.
// MS-ASEMAIL 2.2.2.48 requires the Email/MeetingRequest envelope for the native inbox.
func easMeetingMail(ctx context.Context, content messageContent, viewer string) string {
	for _, attachment := range content.Attachments {
		if !strings.EqualFold(attachment.ContentType, "text/calendar") {
			continue
		}
		data := string(attachment.Data)
		p, err := calutil.ParseICS(data)
		if err != nil || p.UID == "" || p.Start == nil || p.End == nil {
			continue
		}
		if strings.EqualFold(p.Organizer.Email, viewer) {
			return ""
		}
		attendee := false
		response := false
		for _, a := range p.Attendees {
			if strings.EqualFold(a.Email, viewer) {
				attendee = true
				response = a.RSVP || strings.EqualFold(a.PartStat, "NEEDS-ACTION")
			}
		}
		if !attendee {
			continue
		}
		method := p.Method
		class, typ := "IPM.Schedule.Meeting.Request", "1"
		if method == "CANCEL" {
			class, typ = "IPM.Schedule.Meeting.Canceled", "3"
			response = false
		} else if method != "REQUEST" {
			continue
		}
		var b strings.Builder
		field := func(name, value string) {
			fmt.Fprintf(&b, `<Email:%s xmlns:Email="Email:">%s</Email:%s>`, name, xmlEscape(value), name)
		}
		field("MessageClass", class)
		field("ContentClass", "urn:content-classes:calendarmessage")
		b.WriteString(`<Email:MeetingRequest xmlns:Email="Email:">`)
		field("AllDayEvent", fmt.Sprint(bool01(p.AllDay)))
		stamp := p.Start.UTC().Format("2006-01-02T15:04:05.000Z")
		if at := parseASTime(davTextProperty(data, "DTSTAMP")); at != nil {
			stamp = at.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		field("DtStamp", stamp)
		field("StartTime", p.Start.UTC().Format("2006-01-02T15:04:05.000Z"))
		field("EndTime", p.End.UTC().Format("2006-01-02T15:04:05.000Z"))
		zone := calutil.ReadRecurrence(data).Timezone
		if zone == "" || zone == "floating" {
			zone = "UTC"
		}
		tzi, err := calendarTZI(zone, p.Start.Year())
		if err != nil {
			tzi, _ = calendarTZI("UTC", p.Start.Year())
		}
		field("TimeZone", base64.StdEncoding.EncodeToString(tzi))
		version, _ := ctx.Value(easVersionKey{}).(string)
		if version == "16.0" || version == "16.1" {
			fmt.Fprintf(&b, `<Calendar:UID xmlns:Calendar="Calendar:">%s</Calendar:UID>`, xmlEscape(p.UID))
			b.WriteString(calendarLocationXML(ctx, data))
		} else {
			field("GlobalObjId", easGlobalObjectID(p.UID))
			field("Location", p.Location)
		}
		field("Organizer", (&mail.Address{Name: p.Organizer.Name, Address: p.Organizer.Email}).String())
		field("ResponseRequested", fmt.Sprint(bool01(response)))
		field("Sensitivity", "0")
		field("BusyStatus", "1")
		instance := "0"
		if recurrence, err := calendarRecurrenceXML(ctx, data, viewer); err == nil && recurrence != "" {
			if doc, err := parseProtocolXML([]byte(`<Event>` + recurrence + `</Event>`)); err == nil {
				if n := doc.find("Recurrence"); n != nil {
					instance = "1"
					b.WriteString(`<Email:Recurrences><Email:Recurrence>`)
					for _, c := range n.Children {
						switch c.Name.Local {
						case "Type", "Until", "Occurrences", "Interval", "DayOfWeek", "DayOfMonth", "WeekOfMonth", "MonthOfYear":
							field(c.Name.Local, c.Text)
						}
					}
					b.WriteString(`</Email:Recurrence></Email:Recurrences>`)
				}
			}
		}
		field("InstanceType", instance)
		if version != "14.0" {
			fmt.Fprintf(&b, `<Email2:MeetingMessageType xmlns:Email2="Email2:">%s</Email2:MeetingMessageType>`, typ)
		}
		b.WriteString(`</Email:MeetingRequest>`)
		return b.String()
	}
	return ""
}

// MS-ASEMAIL 2.2.2.37: a stable vCal UID wrapper, with no instance date.
func easGlobalObjectID(uid string) string {
	data := append([]byte("vCal-Uid\x01\x00\x00\x00"), []byte(uid)...)
	data = append(data, 0)
	out := make([]byte, 40)
	copy(out, []byte{4, 0, 0, 0, 0x82, 0, 0xe0, 0, 0x74, 0xc5, 0xb7, 0x10, 0x1a, 0x82, 0xe0, 8})
	binary.LittleEndian.PutUint32(out[36:], uint32(len(data)))
	return base64.StdEncoding.EncodeToString(append(out, data...))
}
