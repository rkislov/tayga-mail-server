// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

// Standard EWS calendar commands honor delivery selection. The Tayga named
// iCalendar contract owns scheduling server-side and is handled before this path.
func ewsCalendarDeliveryContext(ctx context.Context, body, op string) (context.Context, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return nil, err
	}
	attr := "SendMeetingInvitationsOrCancellations"
	if op == "CreateItem" {
		attr = "SendMeetingInvitations"
	}
	if op == "DeleteItem" {
		attr = "SendMeetingCancellations"
	}
	switch value := nodeAttr(doc.find(op), attr); value {
	case "", "SendToNone":
		return storage.WithoutCalendarMail(ctx), nil
	case "SendOnlyToAll":
		return ctx, nil
	case "SendToAllAndSaveCopy":
		return storage.WithCalendarMailOptions(ctx, true, false), nil
	case "SendOnlyToChanged", "SendToChangedAndSaveCopy":
		if op != "UpdateItem" {
			return nil, fmt.Errorf("changed-recipient delivery requires UpdateItem")
		}
		return storage.WithCalendarMailOptions(ctx, value == "SendToChangedAndSaveCopy", true), nil
	default:
		return nil, fmt.Errorf("unsupported calendar delivery mode: %s", value)
	}
}

var ewsCalendarFields = map[string]string{
	"item:Subject": "Subject", "item:Body": "Body", "calendar:Location": "Location",
	"calendar:Start": "Start", "calendar:End": "End", "calendar:IsAllDayEvent": "IsAllDayEvent",
	"calendar:Recurrence": "Recurrence", "calendar:StartTimeZone": "StartTimeZone", "calendar:EndTimeZone": "EndTimeZone",
	"calendar:RequiredAttendees": "RequiredAttendees", "calendar:OptionalAttendees": "OptionalAttendees", "calendar:Resources": "Resources",
}

type ewsCalendarEdit struct {
	kind, name string
	node       *protocolNode
}

func ewsCalendarEdits(doc *protocolNode, create bool) ([]ewsCalendarEdit, error) {
	var out []ewsCalendarEdit
	addItem := func(item *protocolNode, kind, path string) error {
		if item == nil || len(item.Children) == 0 {
			return fmt.Errorf("missing calendar properties")
		}
		seen := map[string]bool{}
		for _, field := range item.Children {
			if seen[field.Name.Local] {
				return fmt.Errorf("duplicate calendar property")
			}
			seen[field.Name.Local] = true
			if path != "" && (len(item.Children) != 1 || ewsCalendarFields[path] != field.Name.Local) {
				return fmt.Errorf("calendar field path does not match value")
			}
			out = append(out, ewsCalendarEdit{kind, field.Name.Local, field})
		}
		return nil
	}
	if create {
		items := doc.find("Items")
		if items == nil || len(items.Children) != 1 || items.Children[0].Name.Local != "CalendarItem" {
			return nil, fmt.Errorf("one calendar item per CreateItem is supported")
		}
		err := addItem(items.Children[0], "SetItemField", "")
		return out, err
	}
	updates := doc.find("Updates")
	if updates == nil {
		return nil, fmt.Errorf("missing calendar updates")
	}
	for _, update := range updates.Children {
		seen := map[string]bool{}
		for _, child := range update.Children {
			if (child.Name.Local != "FieldURI" && child.Name.Local != "CalendarItem") || seen[child.Name.Local] {
				return nil, fmt.Errorf("unsupported or duplicate calendar update element")
			}
			seen[child.Name.Local] = true
		}
		if update.Name.Local == "DeleteItemField" && update.child("CalendarItem") != nil {
			return nil, fmt.Errorf("delete requires only a field path")
		}
		field := update.child("FieldURI")
		path := nodeAttr(field, "FieldURI")
		if field != nil && ewsCalendarFields[path] == "" {
			return nil, fmt.Errorf("unsupported calendar field")
		}
		switch update.Name.Local {
		case "SetItemField", "AppendToItemField":
			// Keep compatibility with earlier FlowSync callers without FieldURI, but
			// never accept an indexed/extended path as an unrelated calendar field.
			if update.child("IndexedFieldURI") != nil || update.child("ExtendedFieldURI") != nil {
				return nil, fmt.Errorf("unsupported calendar field path")
			}
			if err := addItem(update.child("CalendarItem"), update.Name.Local, path); err != nil {
				return nil, err
			}
		case "DeleteItemField":
			if path == "" {
				return nil, fmt.Errorf("missing calendar field path")
			}
			out = append(out, ewsCalendarEdit{update.Name.Local, ewsCalendarFields[path], nil})
		default:
			return nil, fmt.Errorf("unsupported calendar update action")
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("missing calendar updates")
	}
	return out, nil
}
func ewsAttendeeGroup(a calutil.Attendee) string {
	if strings.EqualFold(a.CUType, "RESOURCE") || strings.EqualFold(a.CUType, "ROOM") {
		return "Resources"
	}
	if strings.EqualFold(a.Role, "OPT-PARTICIPANT") {
		return "OptionalAttendees"
	}
	return "RequiredAttendees"
}
func parseEWSAttendees(node *protocolNode, group string) ([]calutil.Attendee, error) {
	var out []calutil.Attendee
	if node == nil {
		return out, nil
	}
	for _, entry := range node.Children {
		if entry.Name.Local != "Attendee" {
			return nil, fmt.Errorf("invalid calendar attendee")
		}
		mailbox := entry.child("Mailbox")
		if mailbox == nil || mailbox.child("EmailAddress") == nil {
			return nil, fmt.Errorf("attendee email required")
		}
		if routing := mailbox.value("RoutingType"); routing != "" && !strings.EqualFold(routing, "SMTP") {
			return nil, fmt.Errorf("only SMTP attendees are supported")
		}
		email, err := calendarEmail(mailbox.value("EmailAddress"))
		if err != nil {
			return nil, err
		}
		name := mailbox.value("Name")
		if strings.ContainsAny(name, "\r\n\x00") || len(name) > 1000 {
			return nil, fmt.Errorf("invalid attendee name")
		}
		attendee := calutil.Attendee{Email: email, Name: name, Role: "REQ-PARTICIPANT", CUType: "INDIVIDUAL", PartStat: "NEEDS-ACTION", RSVP: true}
		if group == "OptionalAttendees" {
			attendee.Role = "OPT-PARTICIPANT"
		}
		if group == "Resources" {
			attendee.CUType = "RESOURCE"
		}
		out = append(out, attendee)
		if len(out) > 1000 {
			return nil, fmt.Errorf("too many attendees")
		}
	}
	return out, nil
}
func applyEWSCalendarEdits(event calutil.Event, edits []ewsCalendarEdit, u *storage.User, create bool) (calutil.Event, error) {
	for _, edit := range edits {
		if edit.name == "Recurrence" || edit.name == "StartTimeZone" || edit.name == "EndTimeZone" {
			continue
		}
		group := edit.name == "RequiredAttendees" || edit.name == "OptionalAttendees" || edit.name == "Resources"
		if edit.kind == "AppendToItemField" && !group {
			return event, fmt.Errorf("append is supported only for attendee collections")
		}
		text := ""
		if edit.node != nil {
			text = edit.node.Text
		}
		if !group && edit.node != nil && len(edit.node.Children) > 0 && edit.name != "Organizer" {
			return event, fmt.Errorf("unsupported structured calendar property")
		}
		if group {
			if event.Organizer.Email != "" && !strings.EqualFold(event.Organizer.Email, u.Email) {
				return event, storage.ErrUnauthorized
			}
			attendees, err := parseEWSAttendees(edit.node, edit.name)
			if err != nil {
				return event, err
			}
			before := event.Attendees
			for i := range attendees {
				if attendees[i].Name == "" {
					for _, prior := range before {
						if strings.EqualFold(prior.Email, attendees[i].Email) {
							attendees[i].Name = prior.Name
							break
						}
					}
				}
			}
			var updated []calutil.Attendee
			for _, prior := range before {
				if edit.kind == "AppendToItemField" || ewsAttendeeGroup(prior) != edit.name {
					updated = append(updated, prior)
				}
			}
			event.Attendees = append(updated, attendees...)
			if event.Organizer.Email == "" {
				event.Organizer = calutil.Attendee{Email: u.Email, Name: u.DisplayName}
			}
			continue
		}
		switch edit.name {
		case "Subject":
			event.Summary = text
		case "Location":
			event.Location = text
		case "Body":
			if edit.node != nil && len(edit.node.Children) > 0 {
				return event, fmt.Errorf("invalid calendar body")
			}
			format := strings.ToUpper(nodeAttr(edit.node, "BodyType"))
			if format == "HTML" {
				text = mailsearch.ExtractHTMLText(text)
			} else if format != "" && format != "TEXT" {
				return event, fmt.Errorf("unsupported calendar body format")
			}
			event.Description = text
		case "Start", "End":
			if edit.kind == "DeleteItemField" {
				return event, fmt.Errorf("calendar dates cannot be deleted")
			}
			value := parseASTime(text)
			if value == nil {
				return event, fmt.Errorf("invalid calendar date")
			}
			if edit.name == "Start" {
				event.Start = value
			} else {
				event.End = value
			}
		case "IsAllDayEvent":
			if edit.kind == "DeleteItemField" {
				return event, fmt.Errorf("all-day state cannot be deleted")
			}
			switch strings.TrimSpace(text) {
			case "true", "1":
				event.AllDay = true
			case "false", "0":
				event.AllDay = false
			default:
				return event, fmt.Errorf("invalid all-day state")
			}
		case "Organizer":
			if !create {
				return event, fmt.Errorf("organizer cannot be changed")
			}
			mailbox := edit.node.child("Mailbox")
			if mailbox == nil {
				return event, fmt.Errorf("organizer email required")
			}
			email, err := calendarEmail(mailbox.value("EmailAddress"))
			if err != nil || !strings.EqualFold(email, u.Email) {
				return event, storage.ErrUnauthorized
			}
		case "ItemClass":
			if !create || text != "IPM.Appointment" {
				return event, fmt.Errorf("unsupported calendar item class")
			}
		default:
			return event, fmt.Errorf("unsupported calendar property: %s", edit.name)
		}
	}
	seen := map[string]bool{}
	if len(event.Attendees) > 1000 {
		return event, fmt.Errorf("too many attendees")
	}
	for _, attendee := range event.Attendees {
		key := strings.ToLower(attendee.Email)
		if seen[key] {
			return event, fmt.Errorf("duplicate calendar attendee")
		}
		seen[key] = true
	}
	if event.Start == nil || event.End == nil || !event.End.After(*event.Start) {
		return event, fmt.Errorf("end must be after start")
	}
	return event, nil
}
func (h *ewsHandler) createCalendarItem(ctx context.Context, u *storage.User, body, folder string) (string, error) {
	writeCtx, err := ewsCalendarDeliveryContext(ctx, body, "CreateItem")
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidRequest", err.Error()), nil
	}
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	edits, err := ewsCalendarEdits(doc, true)
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidPropertySet", err.Error()), nil
	}
	zone, err := ewsCalendarZone(edits, "")
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidPropertySet", err.Error()), nil
	}
	event, err := applyEWSCalendarEdits(calutil.Event{UID: storage.NewID(), Organizer: calutil.Attendee{Email: u.Email, Name: u.DisplayName}}, edits, u, true)
	if errors.Is(err, storage.ErrUnauthorized) {
		return ewsOperationError("CreateItem", "ErrorAccessDenied", "Calendar organizer must be the authenticated user"), nil
	}
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidPropertySet", err.Error()), nil
	}
	data, err := calutil.BuildSimpleVEVENT(event)
	if err != nil {
		return "", err
	}
	data, err = finishEWSRecurrence(data, zone, edits)
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidPropertySet", err.Error()), nil
	}
	saved, err := h.store.SaveCalendarClientObject(writeCtx, u.ID, &storage.CalendarObject{CalendarID: folder, UID: event.UID, HrefName: storage.NewID() + ".ics", Component: "VEVENT", Data: data, DTStart: event.Start, DTEnd: event.End, CreateOnly: true})
	if err != nil {
		return calendarWriteError("CreateItem", err)
	}
	return ewsNotificationSuccess("CreateItem", fmt.Sprintf(`<m:Items><t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/></t:CalendarItem></m:Items>`, xmlEscape(saved.ID), xmlEscape(ewsCalendarChangeKey(saved, nil)))), nil
}
func calendarWriteError(op string, err error) (string, error) {
	var booking *storage.CalendarBookingError
	if errors.Is(err, storage.ErrCalendarConflict) || errors.As(err, &booking) {
		return ewsOperationError(op, "ErrorIrresolvableConflict", "Calendar changed or resource unavailable"), nil
	}
	if errors.Is(err, storage.ErrUnauthorized) {
		return ewsOperationError(op, "ErrorAccessDenied", "Calendar write denied"), nil
	}
	return "", err
}
