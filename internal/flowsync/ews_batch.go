// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
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
	if len(ids.Children) > 512 {
		return ewsOperationError(op, "ErrorBatchProcessingStopped", "too many items"), nil
	}
	children := ids.Children
	var out strings.Builder
	fmt.Fprintf(&out, `<m:%sResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages>`, op)
	for _, id := range children {
		ids.Children = []*protocolNode{id}
		result, err := fn(ctx, u, doc.render())
		if err != nil {
			result = ewsOperationError(op, "ErrorInternalServerError", "operation failed")
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
