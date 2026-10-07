package flowsync

import (
	"context"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (h *ewsHandler) createItem(ctx context.Context, u *storage.User, body string) (string, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	folderID := extractAttr(body, "ParentFolderId", "Id")
	if folderID == "" {
		folderID = extractAttr(body, "FolderId", "Id")
	}

	if strings.Contains(body, "CalendarItem") || strings.Contains(body, "t:CalendarItem") {
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
		subject := extractTag(body, "Subject")
		location := extractTag(body, "Location")
		start := parseASTime(extractTag(body, "Start"))
		end := parseASTime(extractTag(body, "End"))
		uid := storage.NewID()
		o, err := h.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
			CalendarID: folderID,
			UID:        uid,
			HrefName:   uid + ".ics",
			Component:  "VEVENT",
			DTStart:    start,
			DTEnd:      end,
			Data:       buildVEVENT(uid, subject, location, start, end, false),
		})
		if err != nil {
			return "", err
		}
		return ewsCreateResponse("CalendarItem", o.ID), nil
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

	return ewsFault("ErrorInvalidRequest", "FlowSync: CreateItem supports CalendarItem and Contact"), nil
}

func (h *ewsHandler) updateItem(ctx context.Context, u *storage.User, body string) (string, error) {
	id := extractAttr(body, "ItemId", "Id")
	if id == "" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}

	if o, err := h.store.GetCalendarObjectByID(ctx, id); err == nil {
		if err := h.ensureCalendarOwned(ctx, u.ID, o.CalendarID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		ev := parseICalEvent(o.Data)
		subject := fieldOr(extractTag(body, "Subject"), ev.Summary)
		location := fieldOr(extractTag(body, "Location"), ev.Location)
		start, end := o.DTStart, o.DTEnd
		if t := parseASTime(extractTag(body, "Start")); t != nil {
			start = t
		}
		if t := parseASTime(extractTag(body, "End")); t != nil {
			end = t
		}
		_, err = h.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
			ID: o.ID, CalendarID: o.CalendarID, UID: o.UID, HrefName: o.HrefName,
			Component: "VEVENT", DTStart: start, DTEnd: end,
			Data: buildVEVENT(o.UID, subject, location, start, end, ev.AllDay),
		})
		if err != nil {
			return "", err
		}
		return ewsUpdateResponse("CalendarItem", o.ID), nil
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
			Data: buildVCARD(o.UID, fields),
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
		// IsRead via UpdateItem
		if strings.Contains(body, "IsRead") {
			flags := storage.ParseFlags(msg.Flags)
			val := extractTag(body, "IsRead")
			if strings.EqualFold(val, "true") || val == "1" {
				flags = addFlag(flags, `\Seen`)
			} else if val != "" {
				flags = removeFlag(flags, `\Seen`)
			}
			if err := h.store.UpdateMessageFlags(ctx, msg.ID, storage.NormalizeFlags(flags)); err != nil {
				return "", err
			}
		}
		return ewsUpdateResponse("Message", msg.ID), nil
	}

	return ewsFault("ErrorItemNotFound", "not found"), nil
}

func (h *ewsHandler) deleteItem(ctx context.Context, u *storage.User, body string) (string, error) {
	id := extractAttr(body, "ItemId", "Id")
	if id == "" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}
	if o, err := h.store.GetCalendarObjectByID(ctx, id); err == nil {
		if err := h.ensureCalendarOwned(ctx, u.ID, o.CalendarID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		if err := h.store.DeleteCalendarObject(ctx, o.CalendarID, o.HrefName); err != nil {
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
