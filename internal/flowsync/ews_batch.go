// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"errors"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func (h *ewsHandler) batchItems(ctx context.Context, u *storage.User, op, body string, fn func(context.Context, *storage.User, string) (string, error)) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	group := "ItemIds"
	if op == "UpdateItem" {
		group = "ItemChanges"
	}
	ids := doc.find(group)
	if ids == nil || len(ids.Children) == 0 {
		return ewsOperationError(op, "ErrorInvalidRequest", "missing items"), nil
	}
	limit := 512
	if op == "MoveItem" {
		limit = maxEWSDeleteItems
	}
	if len(ids.Children) > limit {
		return ewsOperationError(op, "ErrorBatchProcessingStopped", "too many items"), nil
	}
	children := ids.Children
	moved := map[string]bool{}
	if op == "MoveItem" {
		if dest := doc.find("ToFolderId"); dest != nil && len(dest.Children) == 1 {
			target, e := h.resolveEWSFolder(ctx, u, dest.render())
			if e == nil && h.ensureMailboxOwned(ctx, u.ID, target) == nil {
				itemIDs := make([]string, 0, len(children))
				for _, item := range children {
					if item.Name.Local == "ItemId" {
						itemIDs = append(itemIDs, nodeAttr(item, "Id"))
					}
				}
				messages, e := h.store.DisposeMessages(ctx, u.ID, itemIDs, target)
				if e != nil {
					return "", e
				}
				for _, message := range messages {
					moved[message.ID] = true
				}
			}
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, `<m:%sResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages>`, op)
	for _, id := range children {
		ids.Children = []*protocolNode{id}
		var result string
		var err error
		if op == "MoveItem" && id.Name.Local == "ItemId" && moved[nodeAttr(id, "Id")] {
			result = ewsNotificationSuccess("MoveItem", fmt.Sprintf(`<m:Items><t:Message><t:ItemId Id="%s"/></t:Message></m:Items>`, xmlEscape(nodeAttr(id, "Id"))))
		} else {
			result, err = fn(ctx, u, doc.render())
		}
		if err != nil {
			result = ewsOperationError(op, "ErrorInternalServerError", "operation failed")
			if errors.Is(err, errExchangeRecurrence) {
				result = ewsOperationError(op, "ErrorCalendarInvalidRecurrence", "Recurrence requires the full iCalendar contract")
			}
		}
		if strings.Contains(result, "<m:FaultResponse") {
			result = ewsOperationError(op, extractTag(result, "ResponseCode"), extractTag(result, "MessageText"))
		}
		parsed, err := parseProtocolXML([]byte(result))
		if err != nil {
			return "", err
		}
		if messages := parsed.find("ResponseMessages"); messages != nil {
			for _, message := range messages.Children {
				out.WriteString(message.render())
			}
		}
	}
	fmt.Fprintf(&out, `</m:ResponseMessages></m:%sResponse>`, op)
	return out.String(), nil
}
