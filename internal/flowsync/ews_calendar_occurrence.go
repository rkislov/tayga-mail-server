// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func occurrenceDelta(before, after calutil.ParseResult) calutil.OccurrenceChange {
	c := calutil.OccurrenceChange{}
	if before.Start != nil && after.Start != nil && !before.Start.Equal(*after.Start) {
		c.Start = after.Start
	}
	if before.End != nil && after.End != nil && !before.End.Equal(*after.End) {
		c.End = after.End
	}
	if before.Summary != after.Summary {
		c.Summary = &after.Summary
	}
	if before.Location != after.Location {
		c.Location = &after.Location
	}
	if before.Description != after.Description {
		c.Description = &after.Description
	}
	if before.Geo != after.Geo {
		c.Geo = &after.Geo
	}
	if before.AllDay != after.AllDay {
		c.AllDay = &after.AllDay
	}
	c.ResetResponses = c.Start != nil || c.End != nil || c.Summary != nil || c.Location != nil || c.Description != nil || c.Geo != nil || c.AllDay != nil
	return c
}

func (h *ewsHandler) occurrenceXML(ctx context.Context, u *storage.User, master *storage.CalendarObject, rid string, body bool) (string, error) {
	instance, err := calutil.GetOccurrence(master.Data, rid)
	if err != nil {
		return "", err
	}
	wire, err := calutil.OccurrenceMessage(instance, false)
	if err != nil {
		return "", err
	}
	copy := *master
	copy.Data = wire
	item, err := h.calendarItemXML(ctx, u, &copy, body)
	if err != nil {
		return "", err
	}
	item = strings.Replace(item, `Id="`+xmlEscape(master.ID)+`"`, `Id="`+xmlEscape(occurrenceItemID(master.ID, rid))+`"`, 1)
	kind := "Occurrence"
	exceptions, _ := calutil.RecurrenceExceptions(master.Data)
	for _, e := range exceptions {
		if e.ID == rid {
			kind = "Exception"
		}
	}
	item = strings.Replace(item, `<t:CalendarItemType>Single</t:CalendarItemType>`, `<t:CalendarItemType>`+kind+`</t:CalendarItemType>`, 1)
	original := rid
	if len(rid) == 10 {
		original += "T00:00:00Z"
	}
	item = strings.Replace(item, `</t:CalendarItem>`, `<t:OriginalStart>`+xmlEscape(original)+`</t:OriginalStart><t:RecurringMasterItemId Id="`+xmlEscape(master.ID)+`"/></t:CalendarItem>`, 1)
	return item, nil
}

func (h *ewsHandler) ewsOccurrence(ctx context.Context, u *storage.User, body, op string) (string, bool, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", false, nil
	}
	n := doc.find("OccurrenceItemId")
	id := extractAttr(body, "ItemId", "Id")
	if n == nil && !strings.HasPrefix(id, "ews-occ:") {
		return "", false, nil
	}
	fail := func(code, msg string) (string, bool, error) { return ewsOperationError(op, code, msg), true, nil }
	master, rid, key := "", "", ""
	index := 0
	if n != nil {
		master, index, err = occurrenceIndex(n)
		key = nodeAttr(n, "ChangeKey")
	} else {
		master, rid, err = parseOccurrenceID(id)
		key = extractAttr(body, "ItemId", "ChangeKey")
	}
	if err != nil {
		return fail("ErrorInvalidId", err.Error())
	}
	o, err := h.store.GetCalendarObjectByID(ctx, master)
	if err != nil {
		return fail("ErrorItemNotFound", "Series unavailable")
	}
	if err = h.ensureCalendarOwned(ctx, u.ID, o.CalendarID); err != nil {
		return fail("ErrorAccessDenied", "Calendar unavailable")
	}
	files, err := h.store.ListCalendarEventAttachments(ctx, o.ID)
	if err != nil {
		return "", true, err
	}
	if op != "GetItem" && key != "" && key != ewsCalendarChangeKey(o, files) {
		return fail("ErrorIrresolvableConflict", "Series changed")
	}
	if index > 0 {
		rid, err = calutil.RecurrenceIDAtIndex(o.Data, index)
		if err != nil {
			return fail("ErrorCalendarOccurrenceIndexIsOutOfRecurrenceRange", err.Error())
		}
	}
	instance, err := calutil.GetOccurrence(o.Data, rid)
	if err != nil || strings.EqualFold(instance.Status, "CANCELLED") {
		return fail("ErrorCalendarOccurrenceIsDeletedFromRecurrence", "Occurrence unavailable")
	}
	if op == "GetItem" {
		item, err := h.occurrenceXML(ctx, u, o, rid, true)
		return ewsNotificationSuccess(op, "<m:Items>"+item+"</m:Items>"), true, err
	}
	change := calutil.OccurrenceChange{Cancel: op == "DeleteItem"}
	if op == "UpdateItem" {
		edits, err := ewsCalendarEdits(doc, false)
		if err != nil {
			return fail("ErrorInvalidPropertySet", err.Error())
		}
		for _, e := range edits {
			switch e.name {
			case "Subject", "Start", "End", "Location", "Body", "IsAllDayEvent":
			default:
				return fail("ErrorInvalidPropertySet", "Property requires a series edit")
			}
		}
		wire, wireErr := calutil.OccurrenceMessage(instance, false)
		if wireErr != nil {
			return "", true, wireErr
		}
		if _, err = ewsCalendarZone(edits, wire); err != nil {
			return fail("ErrorInvalidPropertySet", err.Error())
		}
		event, err := applyEWSCalendarEdits(instance.Event, edits, u, false)
		if err != nil {
			return fail("ErrorInvalidPropertySet", err.Error())
		}
		after := instance.ParseResult
		after.Event = event
		change = occurrenceDelta(instance.ParseResult, after)
	}
	if op == "UpdateItem" && !change.ResetResponses {
		return ewsNotificationSuccess(op, fmt.Sprintf(`<m:Items><t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/></t:CalendarItem></m:Items>`, xmlEscape(occurrenceItemID(o.ID, rid)), xmlEscape(ewsCalendarChangeKey(o, files)))), true, nil
	}
	writeCtx, err := ewsCalendarDeliveryContext(ctx, body, op)
	if err != nil {
		return fail("ErrorInvalidRequest", err.Error())
	}
	saved, err := h.store.ChangeCalendarOccurrence(writeCtx, u.ID, o.ID, o.ETag, rid, change)
	if err != nil {
		out, e := calendarWriteError(op, err)
		return out, true, e
	}
	if op == "DeleteItem" {
		return ewsNotificationSuccess(op, ""), true, nil
	}
	return ewsNotificationSuccess(op, fmt.Sprintf(`<m:Items><t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/></t:CalendarItem></m:Items>`, xmlEscape(occurrenceItemID(o.ID, rid)), xmlEscape(ewsCalendarChangeKey(saved, files)))), true, nil
}
