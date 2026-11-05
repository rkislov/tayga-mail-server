// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

// EWS attachment deletion reports each identifier separately, in request order.
// A successful item is committed even if a later attachment cannot be deleted.
func (h *ewsHandler) deleteCalendarAttachments(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	op := doc.find("DeleteAttachment")
	if op == nil || len(op.Children) != 1 || op.Children[0].Name.Local != "AttachmentIds" {
		return ewsOperationError("DeleteAttachment", "ErrorInvalidRequest", "AttachmentIds required"), nil
	}
	ids := op.child("AttachmentIds")
	if len(ids.Children) == 0 || len(ids.Children) > 100 {
		return ewsOperationError("DeleteAttachment", "ErrorInvalidRequest", "Between 1 and 100 attachment identifiers required"), nil
	}
	var out strings.Builder
	out.WriteString(`<m:DeleteAttachmentResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages>`)
	for _, node := range ids.Children {
		code, message := "ErrorInvalidAttachmentId", "Invalid calendar attachment identifier"
		valid := node.Name.Local == "AttachmentId" && len(node.Children) == 0 && strings.TrimSpace(node.Text) == "" && len(node.Attr) == 1 && node.Attr[0].Name.Local == "Id"
		if valid {
			response, err := h.deleteCalendarAttachment(ctx, u, nodeAttr(node, "Id"))
			if err != nil {
				code, message = "ErrorInternalServerError", "Attachment deletion failed"
			} else {
				out.WriteString(response)
				continue
			}
		}
		fmt.Fprintf(&out, `<m:DeleteAttachmentResponseMessage ResponseClass="Error"><m:MessageText>%s</m:MessageText><m:ResponseCode>%s</m:ResponseCode></m:DeleteAttachmentResponseMessage>`, xmlEscape(message), code)
	}
	out.WriteString(`</m:ResponseMessages></m:DeleteAttachmentResponse>`)
	return out.String(), nil
}

func (h *ewsHandler) deleteCalendarAttachment(ctx context.Context, u *storage.User, reference string) (string, error) {
	reject := func(code, message string) (string, error) {
		return fmt.Sprintf(`<m:DeleteAttachmentResponseMessage ResponseClass="Error"><m:MessageText>%s</m:MessageText><m:ResponseCode>%s</m:ResponseCode></m:DeleteAttachmentResponseMessage>`, xmlEscape(message), code), nil
	}
	parts := strings.Split(reference, ":")
	if len(parts) != 3 || parts[0] != "calendar" || parts[1] == "" || parts[2] == "" {
		return reject("ErrorInvalidAttachmentId", "Invalid calendar attachment identifier")
	}
	object, err := h.store.GetCalendarObjectByID(ctx, parts[1])
	if err != nil {
		return reject("ErrorInvalidAttachmentId", "Calendar attachment not found")
	}
	if err = h.ensureCalendarOwned(ctx, u.ID, object.CalendarID); err != nil {
		return reject("ErrorAccessDenied", "Calendar owner required")
	}
	file, err := h.store.GetCalendarAttachment(ctx, parts[2])
	if err != nil {
		return reject("ErrorInvalidAttachmentId", "Calendar attachment not found")
	}
	// Read access through an invitation never grants deletion of the source file.
	if file.CalendarObjectID != object.ID || file.UserID != u.ID {
		return reject("ErrorAccessDenied", "Only the file owner may delete this attachment")
	}
	data, err := calutil.WithoutFileAttachment(object.Data, file.ID)
	if err != nil {
		return "", err
	}
	edit := *object
	edit.ExpectedETag, edit.Data = object.ETag, data
	saved, err := h.store.DeleteCalendarClientAttachments(storage.WithoutCalendarMail(ctx), u.ID, &edit, []*storage.CalendarAttachment{file})
	if err != nil {
		if response, e := calendarWriteError("DeleteAttachment", err); e == nil {
			parsed, e := parseProtocolXML([]byte(response))
			if e != nil {
				return "", e
			}
			return parsed.find("DeleteAttachmentResponseMessage").render(), nil
		}
		return "", err
	}
	// Revocation is already committed. A missing blob or cleanup failure must not
	// turn success into a retryable deletion of a record which no longer exists.
	if h.ms != nil && filepath.IsLocal(file.StoragePath) {
		if root, e := os.OpenRoot(filepath.Join(h.ms.Root, "calendar-attachments")); e == nil {
			_ = root.Remove(filepath.FromSlash(file.StoragePath))
			_ = root.Close()
		}
	}
	files, err := h.store.ListCalendarEventAttachments(ctx, saved.ID)
	if err != nil {
		return "", err
	}
	key := ewsCalendarChangeKey(saved, files)
	return fmt.Sprintf(`<m:DeleteAttachmentResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootItemId RootItemId="%s" RootItemChangeKey="%s"/></m:DeleteAttachmentResponseMessage>`, xmlEscape(saved.ID), xmlEscape(key)), nil
}
