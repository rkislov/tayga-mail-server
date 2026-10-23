package flowsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

// Attachment metadata is stored separately from the VEVENT. Including it in
// the change key also wakes streaming subscribers when only a file changes.
func ewsCalendarChangeKey(o *storage.CalendarObject, files []*storage.CalendarAttachment) string {
	type fileVersion struct {
		ID, Name, ContentType string
		Size                  int64
	}
	versions := make([]fileVersion, 0, len(files))
	for _, f := range files {
		versions = append(versions, fileVersion{f.ID, f.Filename, f.ContentType, f.Size})
	}
	data, _ := json.Marshal(versions)
	return digest(o.ETag + ":" + string(data))
}

func ewsParticipantResponse(partStat string) string {
	switch strings.ToUpper(partStat) {
	case "ACCEPTED":
		return "Accept"
	case "DECLINED":
		return "Decline"
	case "TENTATIVE":
		return "Tentative"
	case "", "NEEDS-ACTION":
		return "NoResponseReceived"
	default:
		return "Unknown"
	}
}

func ewsCalendarMailbox(a calutil.Attendee) string {
	return fmt.Sprintf(`<t:Mailbox><t:Name>%s</t:Name><t:EmailAddress>%s</t:EmailAddress><t:RoutingType>SMTP</t:RoutingType></t:Mailbox>`, xmlEscape(a.Name), xmlEscape(a.Email))
}

// Used by GetItem, FindItem and consequently the durable SyncFolderItems diff.
// Serialize the same meeting data on all paths, with EWS ItemType fields first.
func (h *ewsHandler) calendarItemXML(ctx context.Context, u *storage.User, o *storage.CalendarObject, includeBody bool) (string, error) {
	files, err := h.store.ListCalendarEventAttachments(ctx, o.ID)
	if err != nil {
		return "", err
	}
	ev, err := calutil.ParseICS(o.Data)
	if err != nil {
		return "", err
	}
	subject := ev.Summary
	if subject == "" {
		subject = o.UID
	}
	start, end := ev.Start, ev.End
	if start == nil {
		start = o.DTStart
	}
	if end == nil {
		end = o.DTEnd
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/><t:ParentFolderId Id="%s"/><t:ItemClass>IPM.Appointment</t:ItemClass><t:Subject>%s</t:Subject>`, xmlEscape(o.ID), xmlEscape(ewsCalendarChangeKey(o, files)), xmlEscape(o.CalendarID), xmlEscape(subject))
	if includeBody {
		fmt.Fprintf(&b, `<t:Body BodyType="Text">%s</t:Body>`, xmlEscape(ev.Description))
	}
	// Empty collections explicitly clear cached participants/files after removal.
	b.WriteString(`<t:Attachments>`)
	for _, f := range files {
		fmt.Fprintf(&b, `<t:FileAttachment><t:AttachmentId Id="%s"/><t:Name>%s</t:Name><t:ContentType>%s</t:ContentType><t:Size>%d</t:Size><t:IsInline>false</t:IsInline></t:FileAttachment>`, xmlEscape("calendar:"+o.ID+":"+f.ID), xmlEscape(f.Filename), xmlEscape(f.ContentType), f.Size)
	}
	fmt.Fprintf(&b, `</t:Attachments><t:HasAttachments>%t</t:HasAttachments><t:UID>%s</t:UID><t:Start>%s</t:Start><t:End>%s</t:End><t:IsAllDayEvent>%t</t:IsAllDayEvent>`, len(files) > 0, xmlEscape(o.UID), ewsDate(start), ewsDate(end), ev.AllDay)
	busy := "Busy"
	if strings.EqualFold(davTextProperty(o.Data, "TRANSP"), "TRANSPARENT") || strings.EqualFold(ev.Status, "CANCELLED") {
		busy = "Free"
	} else if strings.EqualFold(ev.Status, "TENTATIVE") {
		busy = "Tentative"
	}
	fmt.Fprintf(&b, `<t:LegacyFreeBusyStatus>%s</t:LegacyFreeBusyStatus><t:Location>%s</t:Location><t:IsMeeting>%t</t:IsMeeting><t:IsCancelled>%t</t:IsCancelled>`, busy, xmlEscape(ev.Location), len(ev.Attendees) > 0, strings.EqualFold(ev.Status, "CANCELLED"))
	if davTextProperty(o.Data, "RRULE") == "" {
		b.WriteString(`<t:CalendarItemType>Single</t:CalendarItemType>`)
	}
	response := "Unknown"
	for _, a := range ev.Attendees {
		if strings.EqualFold(a.Email, u.Email) {
			response = ewsParticipantResponse(a.PartStat)
		}
	}
	if ev.Organizer.Email != "" && strings.EqualFold(ev.Organizer.Email, u.Email) {
		response = "Organizer"
	}
	fmt.Fprintf(&b, `<t:MyResponseType>%s</t:MyResponseType>`, response)
	if ev.Organizer.Email != "" {
		b.WriteString(`<t:Organizer>` + ewsCalendarMailbox(ev.Organizer) + `</t:Organizer>`)
	}
	groups := map[string][]calutil.Attendee{}
	for _, a := range ev.Attendees {
		group := "RequiredAttendees"
		if strings.EqualFold(a.CUType, "ROOM") || strings.EqualFold(a.CUType, "RESOURCE") {
			group = "Resources"
		} else if strings.EqualFold(a.Role, "OPT-PARTICIPANT") {
			group = "OptionalAttendees"
		}
		groups[group] = append(groups[group], a)
	}
	for _, group := range []string{"RequiredAttendees", "OptionalAttendees", "Resources"} {
		fmt.Fprintf(&b, `<t:%s>`, group)
		for _, a := range groups[group] {
			fmt.Fprintf(&b, `<t:Attendee>%s<t:ResponseType>%s</t:ResponseType></t:Attendee>`, ewsCalendarMailbox(a), ewsParticipantResponse(a.PartStat))
		}
		fmt.Fprintf(&b, `</t:%s>`, group)
	}
	fmt.Fprintf(&b, `<t:AppointmentSequenceNumber>%d</t:AppointmentSequenceNumber></t:CalendarItem>`, ev.Sequence)
	return b.String(), nil
}
