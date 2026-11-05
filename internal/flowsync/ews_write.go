// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/noteutil"
	"github.com/tayga/tms/internal/storage"
)

func (h *ewsHandler) createItem(ctx context.Context, u *storage.User, body string) (string, error) {
	if response, handled, err := h.taygaWrite(ctx, u, body, true); handled {
		return response, err
	}
	if strings.HasPrefix(extractAttr(body, "FolderId", "Id"), "ews-gal:") || strings.HasPrefix(extractAttr(body, "ParentFolderId", "Id"), "ews-gal:") {
		return ewsFault("ErrorAccessDenied", "Directory is read-only"), nil
	}
	if doc, e := parseProtocolXML([]byte(body)); e == nil && doc.find("Task") != nil {
		return h.writeTask(ctx, u, body, true)
	}
	doc, parseErr := parseProtocolXML([]byte(body))
	if parseErr != nil {
		return "", parseErr
	}
	if items := doc.find("Items"); items != nil && len(items.Children) > 0 && items.Children[0].Name.Local == "Message" && !strings.Contains(body, "IPM.StickyNote") {
		return h.createMailItem(ctx, u, body)
	}
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	folderID := extractAttr(body, "ParentFolderId", "Id")
	if folderID == "" {
		folderID = extractAttr(body, "FolderId", "Id")
	}

	if doc.find("CalendarItem") != nil {
		if folderID == "" {
			cal, err := h.store.GetCalendarByName(ctx, u.ID, "default")
			if err != nil {
				return "", err
			}
			folderID = cal.ID
		}
		if _, err := h.store.GetCalendarByID(ctx, u.ID, folderID); err != nil {
			return ewsFault("ErrorInvalidFolderId", err.Error()), nil
		}
		return h.createCalendarItem(ctx, u, body, folderID)
	}

	if strings.Contains(body, "Contact") || strings.Contains(body, "t:Contact") {
		if folderID == "" {
			ab, err := h.store.GetAddressBookByName(ctx, u.ID, "default")
			if err != nil {
				return "", err
			}
			folderID = ab.ID
		}
		if _, err := h.store.GetAddressBookByID(ctx, u.ID, folderID); err != nil {
			return ewsFault("ErrorInvalidFolderId", err.Error()), nil
		}
		uid := storage.NewID()
		fields := map[string]string{
			"FileAs":            extractTag(body, "DisplayName"),
			"FirstName":         extractTag(body, "GivenName"),
			"LastName":          extractTag(body, "Surname"),
			"Email1Address":     firstEmailFromEWS(body),
			"MobilePhoneNumber": extractTag(body, "PhoneNumber"),
		}
		if fields["FileAs"] == "" {
			fields["FileAs"] = strings.TrimSpace(fields["FirstName"] + " " + fields["LastName"])
		}
		o, err := h.store.UpsertAddressObject(ctx, &storage.AddressObject{
			AddressBookID: folderID,
			UID:           uid,
			HrefName:      uid + ".vcf",
			Data:          buildVCARD(uid, fields),
		})
		if err != nil {
			return "", err
		}
		return ewsCreateResponse("Contact", o.ID), nil
	}

	isSticky := strings.Contains(body, "IPM.StickyNote") || strings.Contains(body, "StickyNote")
	if !isSticky && folderID != "" {
		if _, err := h.store.NoteFolderRightsForUser(ctx, folderID, u.ID); err == nil {
			isSticky = true
		}
	}
	if isSticky {
		message := doc.find("Message")
		if message == nil {
			return ewsFault("ErrorInvalidRequest", "missing note message"), nil
		}
		content := noteutil.EmptyDoc()
		if value := message.child("Body"); value != nil {
			format := ""
			for _, attr := range value.Attr {
				if attr.Name.Local == "BodyType" {
					format = attr.Value
				}
			}
			switch format {
			case "Text":
				content = noteutil.DocumentFromText(value.Text)
			case "HTML":
				var err error
				content, err = noteutil.DocumentFromSyncHTML(value.Text)
				if err != nil {
					return ewsFault("ErrorInvalidPropertySet", err.Error()), nil
				}
			default:
				return ewsFault("ErrorInvalidPropertySet", "unsupported body type"), nil
			}
		}
		if err := noteutil.ValidateSyncReplacement(noteutil.EmptyDoc(), content); err != nil {
			return ewsFault("ErrorInvalidPropertySet", err.Error()), nil
		}
		return h.createStickyNote(ctx, u, folderID, message.value("Subject"), content)
	}

	return ewsFault("ErrorInvalidRequest", "FlowSync: CreateItem supports CalendarItem, Contact, StickyNote"), nil
}

func (h *ewsHandler) updateItem(ctx context.Context, u *storage.User, body string) (string, error) {
	if out, handled, err := h.ewsOccurrence(ctx, u, body, "UpdateItem"); handled {
		return out, err
	}
	if response, handled, err := h.taygaWrite(ctx, u, body, false); handled {
		return response, err
	}
	id := extractAttr(body, "ItemId", "Id")
	if id == "" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}

	if strings.HasPrefix(id, "ews-gal-item:") {
		return ewsFault("ErrorAccessDenied", "Directory entries are read-only"), nil
	}
	if taskID(id) {
		return h.writeTask(ctx, u, body, false)
	}

	if o, err := h.store.GetCalendarObjectByID(ctx, id); err == nil {
		if err := h.ensureCalendarOwned(ctx, u.ID, o.CalendarID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		files, err := h.store.ListCalendarEventAttachments(ctx, o.ID)
		if err != nil {
			return "", err
		}
		if key := extractAttr(body, "ItemId", "ChangeKey"); key != "" && key != ewsCalendarChangeKey(o, files) {
			return ewsOperationError("UpdateItem", "ErrorIrresolvableConflict", "Calendar changed"), nil
		}
		writeCtx, err := ewsCalendarDeliveryContext(ctx, body, "UpdateItem")
		if err != nil {
			return ewsOperationError("UpdateItem", "ErrorInvalidRequest", err.Error()), nil
		}
		parsed, err := calutil.ParseICS(o.Data)
		if err != nil {
			return "", err
		}
		ev := parsed.Event
		doc, err := parseProtocolXML([]byte(body))
		if err != nil {
			return "", err
		}
		edits, err := ewsCalendarEdits(doc, false)
		if err != nil {
			return ewsOperationError("UpdateItem", "ErrorInvalidPropertySet", err.Error()), nil
		}
		zone, err := ewsCalendarZone(edits, o.Data)
		if err != nil {
			return ewsOperationError("UpdateItem", "ErrorInvalidPropertySet", err.Error()), nil
		}
		ev, err = applyEWSCalendarEdits(ev, edits, u, false)
		if errors.Is(err, storage.ErrUnauthorized) {
			return ewsOperationError("UpdateItem", "ErrorAccessDenied", "Only the organizer may change attendees"), nil
		}
		if err != nil {
			return ewsOperationError("UpdateItem", "ErrorInvalidPropertySet", err.Error()), nil
		}
		data, err := calutil.BuildSimpleVEVENT(ev)
		if err != nil {
			return ewsOperationError("UpdateItem", "ErrorInvalidPropertySet", "Invalid calendar edit"), nil
		}
		data, err = finishEWSRecurrence(data, zone, edits)
		if err != nil {
			return ewsOperationError("UpdateItem", "ErrorInvalidPropertySet", err.Error()), nil
		}
		edit := *o
		edit.ExpectedETag, edit.Data, edit.DTStart, edit.DTEnd = o.ETag, data, ev.Start, ev.End
		saved, err := h.store.SaveCalendarClientObject(writeCtx, u.ID, &edit)
		if err != nil {
			return calendarWriteError("UpdateItem", err)
		}
		files, err = h.store.ListCalendarEventAttachments(ctx, saved.ID)
		if err != nil {
			return "", err
		}
		return ewsNotificationSuccess("UpdateItem", fmt.Sprintf(`<m:Items><t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/></t:CalendarItem></m:Items>`, xmlEscape(saved.ID), xmlEscape(ewsCalendarChangeKey(saved, files)))), nil
	}

	if o, err := h.store.GetAddressObjectByID(ctx, id); err == nil {
		if err := h.ensureABOwned(ctx, u.ID, o.AddressBookID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		c := parseVCard(o.Data)
		fields := map[string]string{
			"FileAs":            fieldOr(extractTag(body, "DisplayName"), c.FN),
			"FirstName":         fieldOr(extractTag(body, "GivenName"), c.FirstName),
			"LastName":          fieldOr(extractTag(body, "Surname"), c.LastName),
			"Email1Address":     fieldOr(firstEmailFromEWS(body), c.Email),
			"MobilePhoneNumber": fieldOr(extractTag(body, "PhoneNumber"), c.Tel),
		}
		_, err = h.store.UpsertAddressObject(ctx, &storage.AddressObject{
			ID: o.ID, AddressBookID: o.AddressBookID, UID: o.UID, HrefName: o.HrefName,
			Data: mergeDAVProperties(o.Data, buildVCARD(o.UID, fields), "VCARD", changedDAVProperties(body, map[string]string{"DisplayName": "FN", "GivenName": "N", "Surname": "N", "EmailAddress": "EMAIL", "PhoneNumber": "TEL"})),
		})
		if err != nil {
			return "", err
		}
		return ewsUpdateResponse("Contact", o.ID), nil
	}

	if msg, err := h.store.GetMessageByID(ctx, id); err == nil {
		if err := h.ensureMailboxOwned(ctx, u.ID, msg.MailboxID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		return h.updateMailMetadata(ctx, u, msg, body)
	}

	if n, err := h.store.GetNoteItemByID(ctx, id); err == nil {
		rights, err := h.store.NoteRightsForUser(ctx, n.ID, u.ID)
		if err != nil || rights != "write" {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		return h.updateStickyNote(ctx, n, body)
	}

	return ewsFault("ErrorItemNotFound", "not found"), nil
}

func (h *ewsHandler) deleteItem(ctx context.Context, u *storage.User, body string) (string, error) {
	if out, handled, err := h.ewsOccurrence(ctx, u, body, "DeleteItem"); handled {
		return out, err
	}
	id := extractAttr(body, "ItemId", "Id")
	if id == "" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}
	if strings.HasPrefix(id, "ews-gal-item:") {
		return ewsFault("ErrorAccessDenied", "Directory entries are read-only"), nil
	}
	if taskID(id) {
		if err := h.store.DeleteTask(ctx, u.ID, id); err != nil {
			if err == storage.ErrUnauthorized {
				return ewsFault("ErrorAccessDenied", "Only the author can delete the task"), nil
			}
			return ewsFault("ErrorItemNotFound", "task unavailable"), nil
		}
		return ewsDeleteResponse(), nil
	}

	if o, err := h.store.GetCalendarObjectByID(ctx, id); err == nil {
		if err := h.ensureCalendarOwned(ctx, u.ID, o.CalendarID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		files, err := h.store.ListCalendarEventAttachments(ctx, o.ID)
		if err != nil {
			return "", err
		}
		if key := extractAttr(body, "ItemId", "ChangeKey"); key != "" && key != ewsCalendarChangeKey(o, files) {
			return ewsOperationError("DeleteItem", "ErrorIrresolvableConflict", "Calendar changed"), nil
		}
		writeCtx, err := ewsCalendarDeliveryContext(ctx, body, "DeleteItem")
		if err != nil {
			return ewsOperationError("DeleteItem", "ErrorInvalidRequest", err.Error()), nil
		}
		if err = h.store.DeleteCalendarMeeting(writeCtx, u.ID, o); err != nil {
			if errors.Is(err, storage.ErrCalendarConflict) {
				return ewsOperationError("DeleteItem", "ErrorIrresolvableConflict", "Calendar changed"), nil
			}
			if errors.Is(err, storage.ErrUnauthorized) {
				return ewsOperationError("DeleteItem", "ErrorAccessDenied", "Calendar write denied"), nil
			}
			return "", err
		}
		return ewsDeleteResponse(), nil
	}
	if o, err := h.store.GetAddressObjectByID(ctx, id); err == nil {
		if err := h.ensureABOwned(ctx, u.ID, o.AddressBookID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		if err := h.store.DeleteAddressObject(ctx, o.AddressBookID, o.HrefName); err != nil {
			return "", err
		}
		return ewsDeleteResponse(), nil
	}
	if msg, err := h.store.GetMessageByID(ctx, id); err == nil {
		if err := h.ensureMailboxOwned(ctx, u.ID, msg.MailboxID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		if err := h.store.DeleteMessage(ctx, msg.ID); err != nil {
			return "", err
		}
		return ewsDeleteResponse(), nil
	}
	if n, err := h.store.GetNoteItemByID(ctx, id); err == nil {
		rights, err := h.store.NoteRightsForUser(ctx, n.ID, u.ID)
		if err != nil || rights != "write" {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		if err := h.store.DeleteNoteItemByID(ctx, n.ID); err != nil {
			return "", err
		}
		return ewsDeleteResponse(), nil
	}
	return ewsFault("ErrorItemNotFound", "not found"), nil
}

func (h *ewsHandler) ensureCalendarOwned(ctx context.Context, userID, calendarID string) error {
	_, err := h.store.GetCalendarByID(ctx, userID, calendarID)
	return err
}

func (h *ewsHandler) ensureABOwned(ctx context.Context, userID, abID string) error {
	_, err := h.store.GetAddressBookByID(ctx, userID, abID)
	return err
}

func (h *ewsHandler) ensureMailboxOwned(ctx context.Context, userID, mailboxID string) error {
	mbs, err := h.store.ListMailboxes(ctx, userID)
	if err != nil {
		return err
	}
	for _, mb := range mbs {
		if mb.ID == mailboxID {
			return nil
		}
	}
	return storage.ErrUnauthorized
}

func firstEmailFromEWS(body string) string {
	if v := extractTag(body, "EmailAddress"); v != "" {
		return v
	}
	return extractTag(body, "Entry")
}

func ewsCreateResponse(itemType, id string) string {
	return fmt.Sprintf(`<m:CreateItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:CreateItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items><t:%s><t:ItemId Id="%s" ChangeKey="%s"/></t:%s></m:Items></m:CreateItemResponseMessage></m:ResponseMessages></m:CreateItemResponse>`,
		itemType, xmlEscape(id), xmlEscape(id), itemType)
}

func ewsUpdateResponse(itemType, id string) string {
	return fmt.Sprintf(`<m:UpdateItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:UpdateItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items><t:%s><t:ItemId Id="%s" ChangeKey="%s"/></t:%s></m:Items></m:UpdateItemResponseMessage></m:ResponseMessages></m:UpdateItemResponse>`,
		itemType, xmlEscape(id), xmlEscape(id), itemType)
}

func ewsDeleteResponse() string {
	return `<m:DeleteItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"><m:ResponseMessages><m:DeleteItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode></m:DeleteItemResponseMessage></m:ResponseMessages></m:DeleteItemResponse>`
}
