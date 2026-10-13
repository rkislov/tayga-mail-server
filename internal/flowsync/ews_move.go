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
	if err = h.ensureMailboxOwned(ctx, u.ID, dst); err != nil {
		return ewsFault("ErrorAccessDenied", "destination unavailable"), nil
	}
	ids := doc.find("ItemIds")
	if ids == nil || len(ids.Children) != 1 || ids.Children[0].Name.Local != "ItemId" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
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
