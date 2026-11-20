// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

const maxEWSDeleteItems = 10000
const ewsRecoveryFolder = "Recoverable Items/Deletions"

func (h *ewsHandler) recoveryMailbox(ctx context.Context, u *storage.User) (*storage.Mailbox, error) {
	for _, name := range []string{"Recoverable Items", ewsRecoveryFolder} {
		path := ""
		if h.ms != nil {
			var err error
			path, err = h.ms.EnsureFolder(u.Email, name)
			if err != nil {
				return nil, err
			}
		}
		mb, err := h.store.EnsureMailbox(ctx, u.ID, name, path)
		if err != nil {
			return nil, err
		}
		if name == ewsRecoveryFolder {
			return mb, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (h *ewsHandler) deleteEWSItems(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	root := doc.find("DeleteItem")
	ids := doc.find("ItemIds")
	if root == nil || ids == nil || len(ids.Children) == 0 {
		return ewsOperationError("DeleteItem", "ErrorInvalidRequest", "Missing items"), nil
	}
	if len(ids.Children) > maxEWSDeleteItems {
		return ewsOperationError("DeleteItem", "ErrorBatchProcessingStopped", "Too many items"), nil
	}
	mode := nodeAttr(root, "DeleteType")
	if mode == "" {
		mode = "HardDelete"
	}
	if mode != "HardDelete" && mode != "SoftDelete" && mode != "MoveToDeletedItems" {
		return ewsOperationError("DeleteItem", "ErrorInvalidRequest", "Invalid DeleteType"), nil
	}
	children := ids.Children
	groups := map[string][]string{}
	for _, item := range children {
		if item.Name.Local != "ItemId" {
			continue
		}
		msg, e := h.store.GetMessageByID(ctx, nodeAttr(item, "Id"))
		if e != nil {
			continue
		}
		if e = h.mailboxRight(ctx, u.ID, msg.MailboxID, "t"); e != nil {
			continue
		}
		mb, e := h.store.GetMailboxByID(ctx, msg.MailboxID)
		if e != nil {
			return "", e
		}
		groups[mb.UserID] = append(groups[mb.UserID], msg.ID)
	}
	var messages []*storage.Message
	for ownerID, itemIDs := range groups {
		owner, e := h.store.GetUserByID(ctx, ownerID)
		if e != nil {
			return "", e
		}
		destination := ""
		if mode == "MoveToDeletedItems" {
			if e = h.ensureStandardMailFolders(ctx, owner); e != nil {
				return "", e
			}
			mb, e := h.store.GetMailbox(ctx, ownerID, "Trash")
			if e != nil {
				return "", e
			}
			destination = mb.ID
		}
		if mode == "SoftDelete" {
			mb, e := h.recoveryMailbox(ctx, owner)
			if e != nil {
				return "", e
			}
			destination = mb.ID
		}
		disposed, e := h.store.DisposeMessages(ctx, u.ID, itemIDs, destination, ownerID)
		if e != nil {
			return "", e
		}
		if destination == "" && h.ms != nil {
			for _, msg := range disposed {
				if e = h.ms.Delete(msg.FilePath); e != nil {
					return "", e
				}
			}
		}
		messages = append(messages, disposed...)
	}
	disposed := map[string]bool{}
	for _, msg := range messages {
		disposed[msg.ID] = true

	}
	var out strings.Builder
	for _, item := range children {
		if item.Name.Local != "OccurrenceItemId" && (item.Name.Local != "ItemId" || nodeAttr(item, "Id") == "") {
			out.WriteString(folderWriteMessage("DeleteItem", "ErrorInvalidId", "Invalid ItemId", ""))
			continue
		}
		if disposed[nodeAttr(item, "Id")] {
			out.WriteString(folderWriteMessage("DeleteItem", "NoError", "", ""))
			continue
		}
		// Calendar, contacts, notes and tasks retain their existing ownership rules.
		ids.Children = []*protocolNode{item}
		result, e := h.deleteItem(ctx, u, doc.render())
		if e != nil {
			result = ewsOperationError("DeleteItem", "ErrorInternalServerError", "Operation failed")
		}
		if strings.Contains(result, "<m:FaultResponse") {
			result = ewsOperationError("DeleteItem", extractTag(result, "ResponseCode"), extractTag(result, "MessageText"))
		}
		parsed, e := parseProtocolXML([]byte(result))
		if e != nil {
			return "", e
		}
		if responses := parsed.find("ResponseMessages"); responses != nil {
			for _, response := range responses.Children {
				out.WriteString(response.render())
			}
		}
	}
	return fmt.Sprintf(`<m:DeleteItemResponse%s><m:ResponseMessages>%s</m:ResponseMessages></m:DeleteItemResponse>`, ewsNamespaces, out.String()), nil
}
