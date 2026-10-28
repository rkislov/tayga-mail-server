// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"errors"
	"fmt"

	"github.com/tayga/tms/internal/noteutil"
	"github.com/tayga/tms/internal/storage"
)

func noteEWSResponse(operation string, n *storage.NoteItem) string {
	return ewsNotificationSuccess(operation, fmt.Sprintf(`<m:Items><t:Message><t:ItemId Id="%s" ChangeKey="%s"/></t:Message></m:Items>`, xmlEscape(n.ID), xmlEscape(n.ETag)))
}

func (h *ewsHandler) updateStickyNote(ctx context.Context, n *storage.NoteItem, source string) (string, error) {
	overwrite := extractAttr(source, "UpdateItem", "ConflictResolution") == "AlwaysOverwrite"
	key := extractAttr(source, "ItemId", "ChangeKey")
	if !overwrite && key == "" {
		return ewsFault("ErrorChangeKeyRequired", "Read the current note before updating it"), nil
	}
	if !overwrite && key != n.ETag {
		return ewsFault("ErrorIrresolvableConflict", "The note has changed; fetch the current version"), nil
	}
	tree, err := parseProtocolXML([]byte(source))
	if err != nil {
		return ewsFault("ErrorInvalidRequest", "invalid XML"), nil
	}
	updates := tree.find("Updates")
	if updates == nil || len(updates.Children) == 0 {
		return ewsFault("ErrorInvalidRequest", "missing updates"), nil
	}
	for _, update := range updates.Children {
		field := update.child("FieldURI")
		if field == nil {
			return ewsFault("ErrorInvalidPropertySet", "unsupported note property"), nil
		}
		name := ""
		for _, attr := range field.Attr {
			if attr.Name.Local == "FieldURI" {
				name = attr.Value
			}
		}
		if name != "item:Body" && name != "item:Subject" {
			return ewsFault("ErrorInvalidPropertySet", "unsupported note property"), nil
		}
		if update.Name.Local != "SetItemField" && update.Name.Local != "DeleteItemField" && update.Name.Local != "AppendToItemField" {
			return ewsFault("ErrorInvalidPropertySet", "unsupported note update"), nil
		}
		if name == "item:Subject" {
			if update.Name.Local == "AppendToItemField" {
				return ewsFault("ErrorInvalidPropertyAppend", "cannot append to subject"), nil
			}
			value := update.find("Subject")
			if value == nil && update.Name.Local != "DeleteItemField" {
				return ewsFault("ErrorInvalidPropertySet", "missing subject"), nil
			}
			n.Title = ""
			if value != nil {
				n.Title = value.Text
			}
			continue
		}
		next := noteutil.EmptyDoc()
		if update.Name.Local != "DeleteItemField" {
			value := update.find("Body")
			if value == nil {
				return ewsFault("ErrorInvalidPropertySet", "missing body"), nil
			}
			format := ""
			for _, attr := range value.Attr {
				if attr.Name.Local == "BodyType" {
					format = attr.Value
				}
			}
			switch format {
			case "Text":
				next = noteutil.DocumentFromText(value.Text)
			case "HTML":
				var err error
				next, err = noteutil.DocumentFromSyncHTML(value.Text)
				if err != nil {
					return ewsFault("ErrorInvalidPropertySet", err.Error()), nil
				}
			default:
				return ewsFault("ErrorInvalidPropertySet", "unsupported body type"), nil
			}
		}
		previous := noteutil.ParseDocumentJSON(n.DocumentJSON)
		if update.Name.Local == "AppendToItemField" {
			previous.Content = append(previous.Content, next.Content...)
			next = previous
		}
		if err := noteutil.ValidateSyncReplacement(noteutil.ParseDocumentJSON(n.DocumentJSON), next); err != nil {
			return ewsFault("ErrorInvalidPropertySet", err.Error()), nil
		}
		n.DocumentJSON = noteutil.MarshalDocumentJSON(next)
		n.BodyHTML, n.BodyText = noteutil.DeriveBodies(n.DocumentJSON)
	}
	if err := h.store.UpdateNoteItem(ctx, n); err != nil {
		if errors.Is(err, storage.ErrNoteConflict) {
			return ewsFault("ErrorIrresolvableConflict", "The note changed during the update"), nil
		}
		return "", err
	}
	return noteEWSResponse("UpdateItem", n), nil
}
