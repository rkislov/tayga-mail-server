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
	} // Preserve legacy callers that omit DeleteType.
	destination := ""
	switch mode {
	case "HardDelete":
	case "MoveToDeletedItems":
		if err = h.ensureStandardMailFolders(ctx, u); err != nil {
			return "", err
		}
		mb, e := h.store.GetMailbox(ctx, u.ID, "Trash")
		if e != nil {
			return "", e
		}
		destination = mb.ID
	case "SoftDelete":
		mb, e := h.recoveryMailbox(ctx, u)
		if e != nil {
			return "", e
		}
		destination = mb.ID
	default:
		return ewsOperationError("DeleteItem", "ErrorInvalidRequest", "Invalid DeleteType"), nil
	}
	children := ids.Children
	itemIDs := make([]string, 0, len(children))
	for _, item := range children {
		if item.Name.Local == "ItemId" {
			itemIDs = append(itemIDs, nodeAttr(item, "Id"))
		}
	}
	messages, err := h.store.DisposeMessages(ctx, u.ID, itemIDs, destination)
	if err != nil {
		return "", err
	}
	disposed := map[string]bool{}
	for _, msg := range messages {
		disposed[msg.ID] = true
		if destination == "" && h.ms != nil {
			if err = h.ms.Delete(msg.FilePath); err != nil {
				return "", err
			}
		}
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
