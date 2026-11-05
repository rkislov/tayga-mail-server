// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

const maxEWSCalendarAttachmentBytes = 32 << 20 // combined decoded files per request
const maxEWSAttachmentRequestBytes = 48 << 20  // includes base64 and XML

type ewsCalendarUpload struct {
	file *storage.CalendarAttachment
	data []byte
}

func (h *ewsHandler) createCalendarAttachment(ctx context.Context, u *storage.User, body string) (string, error) {
	reject := func(code, message string) (string, error) {
		return ewsOperationError("CreateAttachment", code, message), nil
	}
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	op := doc.find("CreateAttachment")
	if op == nil {
		return reject("ErrorInvalidRequest", "Missing CreateAttachment")
	}
	seen := map[string]bool{}
	for _, node := range op.Children {
		name := node.Name.Local
		if (name != "ParentItemId" && name != "Attachments") || seen[name] {
			return reject("ErrorInvalidRequest", "Unsupported or duplicate request element")
		}
		seen[name] = true
	}
	parent := op.child("ParentItemId")
	id := nodeAttr(parent, "Id")
	if id == "" {
		return reject("ErrorInvalidId", "Parent item required")
	}
	object, err := h.store.GetCalendarObjectByID(ctx, id)
	if err != nil {
		return reject("ErrorItemNotFound", "Calendar item not found")
	}
	if err = h.ensureCalendarOwned(ctx, u.ID, object.CalendarID); err != nil {
		return reject("ErrorAccessDenied", "Calendar owner required")
	}
	files, err := h.store.ListCalendarEventAttachments(ctx, id)
	if err != nil {
		return "", err
	}
	if key := nodeAttr(parent, "ChangeKey"); key != "" && key != ewsCalendarChangeKey(object, files) {
		return reject("ErrorIrresolvableConflict", "Calendar changed")
	}
	nodes := op.child("Attachments")
	if nodes == nil || len(nodes.Children) == 0 || len(nodes.Children) > 100 {
		return reject("ErrorInvalidRequest", "Between 1 and 100 file attachments required")
	}
	var uploads []ewsCalendarUpload
	total := 0
	for _, node := range nodes.Children {
		if node.Name.Local != "FileAttachment" {
			return reject("ErrorInvalidAttachmentId", "Only file attachments on calendar items are supported")
		}
		seen := map[string]bool{}
		for _, field := range node.Children {
			name := field.Name.Local
			if (name != "Name" && name != "ContentType" && name != "Content" && name != "IsInline") || seen[name] || len(field.Children) != 0 {
				return reject("ErrorInvalidRequest", "Unsupported or duplicate attachment property")
			}
			seen[name] = true
		}
		name := strings.TrimSpace(node.value("Name"))
		if name == "" {
			return reject("ErrorRequiredPropertyMissing", "Attachment name required")
		}
		if len(name) > 255 || name == "." || name == ".." || strings.ContainsAny(name, "/\\\r\n\x00") {
			return reject("ErrorInvalidRequest", "Invalid attachment name")
		}
		// Inline metadata cannot be represented by the calendar file model.
		if inline := strings.TrimSpace(node.value("IsInline")); inline != "" && inline != "false" && inline != "0" {
			return reject("ErrorInvalidRequest", "Inline calendar attachments are unsupported")
		}
		ct := strings.TrimSpace(node.value("ContentType"))
		if ct == "" {
			ct = "application/octet-stream"
		}
		if len(ct) > 255 || strings.ContainsAny(ct, "\r\n\x00") {
			return reject("ErrorInvalidRequest", "Invalid attachment content type")
		}
		if _, _, err = mime.ParseMediaType(ct); err != nil {
			return reject("ErrorInvalidRequest", "Invalid attachment content type")
		}
		if node.child("Content") == nil {
			return reject("ErrorRequiredPropertyMissing", "Attachment content required")
		}
		data, decodeErr := base64.StdEncoding.Strict().DecodeString(strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
				return -1
			}
			return r
		}, node.value("Content")))
		if decodeErr != nil {
			return reject("ErrorInvalidRequest", "Invalid base64 attachment content")
		}
		total += len(data)
		if total > maxEWSCalendarAttachmentBytes {
			return reject("ErrorAttachmentSizeLimitExceeded", "Calendar attachments exceed 32 MiB per request")
		}
		fileID := storage.NewID()
		// File names are metadata only; generated identifiers form the disk path.
		file := &storage.CalendarAttachment{ID: fileID, CalendarObjectID: id, UserID: u.ID, Filename: name, ContentType: ct, Size: int64(len(data)), StoragePath: filepath.ToSlash(filepath.Join(u.ID, id, fileID))}
		uploads = append(uploads, ewsCalendarUpload{file, data})
	}
	parsed, err := calutil.ParseICS(object.Data)
	if err != nil {
		return "", err
	}
	event := parsed.Event
	var records []*storage.CalendarAttachment
	for _, upload := range uploads {
		records = append(records, upload.file)
		uri := strings.TrimRight(h.publicURL, "/") + "/api/v1/calendar/attachments/" + url.PathEscape(upload.file.ID)
		event.Attachments = append(event.Attachments, calutil.Attachment{URI: uri, Filename: upload.file.Filename, ContentType: upload.file.ContentType})
	}
	data, err := calutil.BuildSimpleVEVENT(event)
	if err != nil {
		return "", err
	}
	if h.ms == nil {
		return "", fmt.Errorf("calendar file storage unavailable")
	}
	rootPath := filepath.Join(h.ms.Root, "calendar-attachments")
	if err = os.MkdirAll(rootPath, 0750); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", err
	}
	defer root.Close()
	committed := false
	var staged []string
	defer func() {
		if !committed {
			for _, name := range staged {
				_ = root.Remove(name)
			}
		}
	}()
	for _, upload := range uploads {
		path := filepath.FromSlash(upload.file.StoragePath)
		if err = root.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return "", err
		}
		f, e := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if e != nil {
			return "", e
		}
		staged = append(staged, path)
		_, err = f.Write(upload.data)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	edit := *object
	edit.ExpectedETag, edit.Data = object.ETag, data
	saved, err := h.store.SaveCalendarClientAttachments(storage.WithoutCalendarMail(ctx), u.ID, &edit, records)
	if err != nil {
		return calendarWriteError("CreateAttachment", err)
	}
	committed = true
	// Shared files from an organizer remain part of a recipient's change key.
	files, err = h.store.ListCalendarEventAttachments(ctx, saved.ID)
	if err != nil {
		return "", err
	}
	key := ewsCalendarChangeKey(saved, files)
	var response strings.Builder
	response.WriteString(`<m:CreateAttachmentResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages>`)
	for _, upload := range uploads {
		fmt.Fprintf(&response, `<m:CreateAttachmentResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Attachments><t:FileAttachment><t:AttachmentId Id="%s" RootItemId="%s" RootItemChangeKey="%s"/></t:FileAttachment></m:Attachments></m:CreateAttachmentResponseMessage>`, xmlEscape("calendar:"+id+":"+upload.file.ID), xmlEscape(id), xmlEscape(key))
	}
	response.WriteString(`</m:ResponseMessages></m:CreateAttachmentResponse>`)
	return response.String(), nil
}
