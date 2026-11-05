// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/storage"
)

// moveItem checks both mailboxes before changing the message. The stable item ID
// preserves flags, task links and metadata; folder snapshots detect the move.
func (h *ewsHandler) moveItem(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	dest := doc.find("ToFolderId")
	if dest == nil || len(dest.Children) != 1 {
		return ewsFault("ErrorInvalidRequest", "missing destination"), nil
	}
	if dest.Children[0].Name.Local != "FolderId" && dest.Children[0].Name.Local != "DistinguishedFolderId" {
		return ewsFault("ErrorInvalidFolderId", "invalid destination"), nil
	}
	dst, err := h.resolveEWSFolder(ctx, u, dest.render())
	if err != nil {
		return ewsFault("ErrorFolderNotFound", "destination unavailable"), nil
	}
	ids := doc.find("ItemIds")
	if ids == nil || len(ids.Children) != 1 || ids.Children[0].Name.Local != "ItemId" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}
	if obj, e := h.store.GetCalendarObjectByID(ctx, nodeAttr(ids.Children[0], "Id")); e == nil {
		if _, e = h.store.GetCalendarByID(ctx, u.ID, obj.CalendarID); e != nil {
			return ewsFault("ErrorAccessDenied", "event unavailable"), nil
		}
		if _, e = h.store.GetCalendarByID(ctx, u.ID, dst); e != nil {
			return ewsFault("ErrorAccessDenied", "destination unavailable"), nil
		}
		moved, e := h.store.MoveCalendarObject(ctx, u.ID, obj.ID, obj.CalendarID, dst)
		if e == storage.ErrCalendarConflict {
			return ewsFault("ErrorItemSave", "event already exists in destination"), nil
		}
		if e != nil {
			return "", e
		}
		files, e := h.store.ListCalendarEventAttachments(ctx, moved.ID)
		if e != nil {
			return "", e
		}
		return ewsNotificationSuccess("MoveItem", fmt.Sprintf(`<m:Items><t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/></t:CalendarItem></m:Items>`, xmlEscape(moved.ID), xmlEscape(ewsCalendarChangeKey(moved, files)))), nil
	}
	if err = h.ensureMailboxOwned(ctx, u.ID, dst); err != nil {
		return ewsFault("ErrorAccessDenied", "destination unavailable"), nil
	}
	msg, err := h.store.GetMessageByID(ctx, nodeAttr(ids.Children[0], "Id"))
	if err != nil {
		return ewsFault("ErrorItemNotFound", "message unavailable"), nil
	}
	if err = h.ensureMailboxOwned(ctx, u.ID, msg.MailboxID); err != nil {
		return ewsFault("ErrorAccessDenied", "message unavailable"), nil
	}
	moved, err := h.store.MoveMessage(ctx, msg.ID, dst)
	if err != nil {
		return "", err
	}
	return ewsNotificationSuccess("MoveItem", fmt.Sprintf(`<m:Items><t:Message><t:ItemId Id="%s"/></t:Message></m:Items>`, xmlEscape(moved.ID))), nil
}
