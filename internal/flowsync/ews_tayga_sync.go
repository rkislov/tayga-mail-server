// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/google/uuid"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

// Named EWS properties preserve the complete DAV representation for Tayga
// clients, including recurrence exceptions and vCard fields EWS cannot express.
const taygaPropertySet = "f3db8cb0-04eb-49ca-bd89-720f85e98dd7"

func taygaProperty(name, value string) string {
	return `<t:ExtendedProperty><t:ExtendedFieldURI PropertySetId="` + taygaPropertySet + `" PropertyName="` + name + `" PropertyType="String"/><t:Value>` + xmlEscape(value) + `</t:Value></t:ExtendedProperty>`
}
func taygaProperties(body string) map[string]string {
	out := map[string]string{}
	doc, e := parseProtocolXML([]byte(body))
	if e != nil {
		return out
	}
	var visit func(*protocolNode)
	visit = func(n *protocolNode) {
		if n.Name.Local == "ExtendedProperty" {
			f := n.child("ExtendedFieldURI")
			if f != nil && strings.EqualFold(nodeAttr(f, "PropertySetId"), taygaPropertySet) && nodeAttr(f, "PropertyType") == "String" {
				out[nodeAttr(f, "PropertyName")] = n.value("Value")
			}
		}
		for _, c := range n.Children {
			visit(c)
		}
	}
	visit(doc)
	return out
}
func taygaRequested(body, name string) bool {
	doc, e := parseProtocolXML([]byte(body))
	if e != nil {
		return false
	}
	var visit func(*protocolNode) bool
	visit = func(n *protocolNode) bool {
		if n.Name.Local == "ExtendedFieldURI" && strings.EqualFold(nodeAttr(n, "PropertySetId"), taygaPropertySet) && nodeAttr(n, "PropertyName") == name {
			return true
		}
		for _, c := range n.Children {
			if visit(c) {
				return true
			}
		}
		return false
	}
	return visit(doc)
}
func taygaRawItem(item, body, kind, data string) string {
	if !taygaRequested(body, kind) {
		return item
	}
	anchor := "<t:UID>"
	if kind == "TaygaVCard" {
		anchor = "<t:FileAs>"
	}
	return strings.Replace(item, anchor, taygaProperty(kind, data)+anchor, 1)
}
func (h *ewsHandler) taygaWrite(ctx context.Context, u *storage.User, body string, create bool) (string, bool, error) {
	if response, handled, err := h.taygaTaskWrite(ctx, u, body, create); handled {
		return response, true, err
	}
	props := taygaProperties(body)
	raw, calendar := props["TaygaICalendar"]
	if !calendar {
		raw = props["TaygaVCard"]
	}
	remove := props["TaygaDelete"] == "true"
	if raw == "" && !remove {
		return "", false, nil
	}
	op := "UpdateItem"
	if create {
		op = "CreateItem"
	}
	fail := func(code string) (string, bool, error) {
		return ewsOperationError(op, code, "Tayga sync write rejected"), true, nil
	}
	if len(raw) > 2<<20 {
		return fail("ErrorDataSizeLimitExceeded")
	}
	id, key := extractAttr(body, "ItemId", "Id"), extractAttr(body, "ItemId", "ChangeKey")
	folder := extractAttr(body, "FolderId", "Id")
	if create {
		id = props["TaygaCreateId"]
		if _, e := uuid.Parse(id); e != nil {
			return fail("ErrorInvalidIdMalformed")
		}
	} else if id == "" || key == "" {
		return fail("ErrorChangeKeyRequired")
	}
	if !create {
		if o, e := h.store.GetCalendarObjectByID(ctx, id); e == nil {
			calendar = true
			folder = o.CalendarID
			if _, e = h.store.GetCalendarByID(ctx, u.ID, folder); e != nil {
				return fail("ErrorAccessDenied")
			}
			files, e := h.store.ListCalendarEventAttachments(ctx, id)
			if e != nil {
				return "", true, e
			}
			if key != ewsCalendarChangeKey(o, files) {
				return fail("ErrorIrresolvableConflict")
			}
			if remove {
				if e = h.store.DeleteCalendarMeeting(ctx, u.ID, o); e == storage.ErrCalendarConflict {
					return fail("ErrorIrresolvableConflict")
				} else if e != nil {
					return "", true, e
				}
				return ewsNotificationSuccess(op, ""), true, nil
			}
		} else if o, e := h.store.GetAddressObjectByID(ctx, id); e == nil {
			folder = o.AddressBookID
			if _, e = h.store.GetAddressBookByID(ctx, u.ID, folder); e != nil {
				return fail("ErrorAccessDenied")
			}
			if key != digest("contact-v2:"+o.ETag) {
				return fail("ErrorIrresolvableConflict")
			}
		} else {
			return fail("ErrorItemNotFound")
		}
	}
	if calendar {
		if _, e := h.store.GetCalendarByID(ctx, u.ID, folder); e != nil {
			return fail("ErrorAccessDenied")
		}
		cal, e := ical.NewDecoder(strings.NewReader(raw)).Decode()
		if e != nil || calutil.MasterEvent(cal) == nil {
			return fail("ErrorInvalidPropertySet")
		}
		masterCount := 0
		for _, component := range cal.Children {
			if component.Name != ical.CompEvent {
				continue
			}
			componentUID, _ := component.Props.Text(ical.PropUID)
			masterUID, _ := calutil.MasterEvent(cal).Props.Text(ical.PropUID)
			if componentUID != masterUID {
				return fail("ErrorInvalidPropertySet")
			}
			if component.Props.Get(ical.PropRecurrenceID) == nil {
				masterCount++
			}
		}
		if masterCount != 1 {
			return fail("ErrorInvalidPropertySet")
		}
		parsed, e := calutil.ParseICS(raw)
		if e != nil || parsed.UID == "" || parsed.Start == nil {
			return fail("ErrorInvalidPropertySet")
		}
		if create {
			if old, e := h.store.GetCalendarObjectByID(ctx, id); e == nil {
				if old.CalendarID != folder || old.UID != parsed.UID {
					return fail("ErrorIrresolvableConflict")
				}
				item, e := h.calendarItemXML(ctx, u, old, true)
				return ewsNotificationSuccess(op, "<m:Items>"+taygaRawItem(item, body, "TaygaICalendar", old.Data)+"</m:Items>"), true, e
			}
		}
		obj := &storage.CalendarObject{ID: id, CalendarID: folder, UID: parsed.UID, HrefName: storage.ContentETag(parsed.UID)[1:17] + ".ics", Component: "VEVENT", Data: raw, DTStart: parsed.Start, DTEnd: parsed.End, CreateOnly: create}
		if !create {
			old, e := h.store.GetCalendarObjectByID(ctx, id)
			if e != nil {
				return fail("ErrorItemNotFound")
			}
			if old.UID != parsed.UID {
				return fail("ErrorInvalidPropertySet")
			}
			obj.HrefName, obj.ExpectedETag = old.HrefName, old.ETag
			files, e := h.store.ListCalendarEventAttachments(ctx, id)
			if e != nil {
				return "", true, e
			}
			if key != ewsCalendarChangeKey(old, files) {
				return fail("ErrorIrresolvableConflict")
			}
		}
		saved, e := h.store.SaveCalendarClientObject(ctx, u.ID, obj)
		if e == storage.ErrCalendarConflict {
			return fail("ErrorIrresolvableConflict")
		}
		if e == storage.ErrUnauthorized {
			return fail("ErrorAccessDenied")
		}
		if e != nil {
			return "", true, e
		}
		item, e := h.calendarItemXML(ctx, u, saved, true)
		return ewsNotificationSuccess(op, "<m:Items>"+taygaRawItem(item, body, "TaygaICalendar", saved.Data)+"</m:Items>"), true, e
	}
	if _, e := h.store.GetAddressBookByID(ctx, u.ID, folder); e != nil {
		return fail("ErrorAccessDenied")
	}
	writer, ok := h.store.(interface {
		SaveEWSContact(context.Context, string, *storage.AddressObject, string, bool, bool) (*storage.AddressObject, error)
	})
	if !ok {
		return fail("ErrorInvalidRequest")
	}
	obj := &storage.AddressObject{ID: id, AddressBookID: folder, Data: raw}
	expected := ""
	if !create {
		old, e := h.store.GetAddressObjectByID(ctx, id)
		if e != nil {
			return fail("ErrorItemNotFound")
		}
		if key != digest("contact-v2:"+old.ETag) {
			return fail("ErrorIrresolvableConflict")
		}
		obj.UID, obj.HrefName, expected = old.UID, old.HrefName, old.ETag
	}
	if !remove {
		decoder := vcard.NewDecoder(strings.NewReader(raw))
		card, e := decoder.Decode()
		if e != nil || card.Value(vcard.FieldUID) == "" {
			return fail("ErrorInvalidPropertySet")
		}
		if _, trailing := decoder.Decode(); trailing != io.EOF {
			return fail("ErrorInvalidPropertySet")
		}
		if !create && obj.UID != card.Value(vcard.FieldUID) {
			return fail("ErrorInvalidPropertySet")
		}
		obj.UID = card.Value(vcard.FieldUID)
		if create {
			obj.HrefName = fmt.Sprintf("%s.vcf", id)
		}
	}
	saved, e := writer.SaveEWSContact(ctx, u.ID, obj, expected, create, remove)
	if e == storage.ErrCalendarConflict {
		return fail("ErrorIrresolvableConflict")
	}
	if e == storage.ErrUnauthorized {
		return fail("ErrorAccessDenied")
	}
	if e != nil {
		return "", true, e
	}
	if remove {
		return ewsNotificationSuccess(op, ""), true, nil
	}
	return ewsNotificationSuccess(op, "<m:Items>"+taygaRawItem(ewsContactXML(saved), body, "TaygaVCard", saved.Data)+"</m:Items>"), true, nil
}
