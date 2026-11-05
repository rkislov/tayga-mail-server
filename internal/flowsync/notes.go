// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/noteutil"
	"github.com/tayga/tms/internal/storage"
)

const folderTypeNotes = 10

type noteAdd struct {
	ServerID   string
	Subject    string
	Body       string
	BodyType   string // 1=plain 2=HTML
	LastMod    string
	Created    string
	Categories string
}

func (h *easHandler) syncNotes(ctx context.Context, u *storage.User, collectionID, next string, wbxml bool, wb []writebackResult) (string, []byte, error) {
	_ = h.store.EnsureNoteDefaults(ctx, u.ID)
	if _, err := h.store.NoteFolderRightsForUser(ctx, collectionID, u.ID); err != nil {
		return "", nil, err
	}
	folder, err := h.store.GetNoteFolder(ctx, collectionID)
	if err != nil {
		return "", nil, err
	}
	items, err := h.store.ListNoteItemsInFolder(ctx, folder.ID, false)
	if err != nil {
		return "", nil, err
	}
	adds := make([]noteAdd, 0, len(items))
	for _, n := range items {
		adds = append(adds, noteSyncAdd(n))
	}
	if wbxml {
		// Fall back to XML for Notes class in v1 (encoder parity later).
		_ = wbxml
	}
	return renderNotesSyncXML(next, collectionID, adds, wb), nil, nil
}

func renderNotesSyncXML(syncKey, collectionID string, adds []noteAdd, wb []writebackResult) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Sync xmlns="AirSync:"><Collections><Collection>`)
	fmt.Fprintf(&b, `<Class>Notes</Class><SyncKey>%s</SyncKey>`, xmlEscape(syncKey))
	fmt.Fprintf(&b, `<CollectionId>%s</CollectionId>`, xmlEscape(collectionID))
	b.WriteString(`<Status>1</Status>`)
	b.WriteString(renderWritebackResponses(wb))
	b.WriteString(`<Commands>`)
	for _, a := range adds {
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(a.ServerID))
		b.WriteString(`<ApplicationData>`)
		fmt.Fprintf(&b, `<Notes:Subject xmlns:Notes="Notes:">%s</Notes:Subject>`, xmlEscape(a.Subject))
		fmt.Fprintf(&b, `<Body xmlns="AirSyncBase:"><Type>%s</Type><EstimatedDataSize>%d</EstimatedDataSize><Truncated>0</Truncated><Data>%s</Data></Body>`, xmlEscape(a.BodyType), len([]byte(a.Body)), xmlEscape(a.Body))
		if a.LastMod != "" {
			fmt.Fprintf(&b, `<Notes:LastModifiedDate xmlns:Notes="Notes:">%s</Notes:LastModifiedDate>`, xmlEscape(a.LastMod))
		}
		b.WriteString(`</ApplicationData></Add>`)
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String()
}

type noteStateKey struct{}
type noteWriteState struct {
	Snapshot collectionState
	Written  map[string]storage.NoteItem
}

func rememberNoteWrite(ctx context.Context, n *storage.NoteItem) {
	if state, ok := ctx.Value(noteStateKey{}).(*noteWriteState); ok {
		state.Written[n.ID] = *n
	}
}

var errInvalidNote = errors.New("invalid note conversion")

func noteSyncAdd(n *storage.NoteItem) noteAdd {
	body, bt := n.BodyHTML, "2"
	if body == "" {
		body, bt = n.BodyText, "1"
	}
	subject := n.Title
	if subject == "" {
		subject = firstLine(n.BodyText)
	}
	return noteAdd{ServerID: n.ID, Subject: subject, Body: body, BodyType: bt, LastMod: n.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z")}
}

func noteClientDocument(op clientOp) (noteutil.Node, bool, error) {
	if op.ApplicationData != nil {
		body := op.ApplicationData.child("Body")
		if body == nil {
			return noteutil.Node{}, false, nil
		}
		if body.value("Truncated") == "1" {
			return noteutil.Node{}, true, errInvalidNote
		}
		switch body.value("Type") {
		case "1":
			return noteutil.DocumentFromText(body.value("Data")), true, nil
		case "2":
			doc, err := noteutil.DocumentFromSyncHTML(body.value("Data"))
			if err != nil {
				return doc, true, fmt.Errorf("%w: %s", errInvalidNote, err)
			}
			return doc, true, nil
		default:
			return noteutil.Node{}, true, errInvalidNote
		}
	}
	body, ok := op.Fields["Body"]
	return noteutil.DocumentFromHTMLOrText(body), ok, nil
}

func (h *easHandler) wbNotes(ctx context.Context, u *storage.User, folderID string, op clientOp) (string, error) {
	rights, err := h.store.NoteFolderRightsForUser(ctx, folderID, u.ID)
	if err != nil || rights != "write" {
		return op.ServerID, storage.ErrUnauthorized
	}
	doc, hasBody, err := noteClientDocument(op)
	if err != nil {
		return op.ServerID, err
	}
	if op.Kind == "add" {
		if err := noteutil.ValidateSyncReplacement(noteutil.EmptyDoc(), doc); err != nil {
			return "", fmt.Errorf("%w: %s", errInvalidNote, err)
		}
		raw := noteutil.MarshalDocumentJSON(doc)
		html, text := noteutil.DeriveBodies(raw)
		subject := op.Fields["Subject"]
		if subject == "" {
			subject = firstLine(text)
		}
		n, err := h.store.CreateNoteItem(ctx, &storage.NoteItem{FolderID: folderID, UserID: u.ID, Title: subject, DocumentJSON: raw, BodyHTML: html, BodyText: text})
		if err != nil {
			return "", err
		}
		rememberNoteWrite(ctx, n)
		return n.ID, nil
	}
	n, err := h.store.GetNoteItemByID(ctx, op.ServerID)
	if err != nil {
		return op.ServerID, err
	}
	if n.FolderID != folderID {
		return op.ServerID, storage.ErrUnauthorized
	}
	if rights, err = h.store.NoteRightsForUser(ctx, n.ID, u.ID); err != nil || rights != "write" {
		return op.ServerID, storage.ErrUnauthorized
	}
	if state, ok := ctx.Value(noteStateKey{}).(*noteWriteState); ok {
		if written, exists := state.Written[n.ID]; exists {
			if written.ETag != n.ETag {
				return n.ID, storage.ErrNoteConflict
			}
		} else {
			hash, err := noteSnapshotHash(n, state.Snapshot.itemPreferences())
			if err != nil {
				return n.ID, err
			}
			if state.Snapshot.Items[n.ID] == "" || state.Snapshot.Items[n.ID] != hash {
				return n.ID, storage.ErrNoteConflict
			}
		}
	}
	switch op.Kind {
	case "change":
		if v, ok := op.Fields["Subject"]; ok {
			n.Title = v
		}
		if op.ApplicationData != nil {
			if subject := op.ApplicationData.child("Subject"); subject != nil {
				n.Title = subject.Text
			}
		}
		if hasBody {
			if err := noteutil.ValidateSyncReplacement(noteutil.ParseDocumentJSON(n.DocumentJSON), doc); err != nil {
				return n.ID, fmt.Errorf("%w: %s", errInvalidNote, err)
			}
			n.DocumentJSON = noteutil.MarshalDocumentJSON(doc)
			n.BodyHTML, n.BodyText = noteutil.DeriveBodies(n.DocumentJSON)
		}
		if err := h.store.UpdateNoteItem(ctx, n); err != nil {
			return n.ID, err
		}
		rememberNoteWrite(ctx, n)
		return n.ID, nil
	case "delete":
		return n.ID, h.store.DeleteNoteItemIfMatch(ctx, n.ID, n.ETag)
	default:
		return n.ID, errInvalidNote
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Note"
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if runes := []rune(s); len(runes) > 80 {
		s = string(runes[:80])
	}
	return s
}

func (h *ewsHandler) findNoteItems(ctx context.Context, u *storage.User, folderID string) (string, error) {
	_ = u
	items, err := h.store.ListNoteItemsInFolder(ctx, folderID, false)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<m:FindItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder><t:Items>`)
	for _, n := range items {
		subject := n.Title
		if subject == "" {
			subject = firstLine(n.BodyText)
		}
		fmt.Fprintf(&b, `<t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:ItemClass>IPM.StickyNote</t:ItemClass><t:DateTimeCreated>%s</t:DateTimeCreated><t:LastModifiedTime>%s</t:LastModifiedTime></t:Message>`,
			xmlEscape(n.ID), xmlEscape(n.ETag), xmlEscape(subject),
			n.CreatedAt.UTC().Format(time.RFC3339), n.UpdatedAt.UTC().Format(time.RFC3339))
	}
	b.WriteString(`</t:Items></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) getNoteItem(ctx context.Context, u *storage.User, id string) (string, error) {
	if _, err := h.store.NoteRightsForUser(ctx, id, u.ID); err != nil {
		return "", err
	}
	n, err := h.store.GetNoteItemByID(ctx, id)
	if err != nil {
		return "", err
	}
	body := n.BodyHTML
	if body == "" {
		body = n.BodyText
	}
	subject := n.Title
	if subject == "" {
		subject = firstLine(n.BodyText)
	}
	return fmt.Sprintf(`<m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:GetItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items><t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:ItemClass>IPM.StickyNote</t:ItemClass><t:Body BodyType="HTML">%s</t:Body></t:Message></m:Items></m:GetItemResponseMessage></m:ResponseMessages></m:GetItemResponse>`,
		xmlEscape(n.ID), xmlEscape(n.ETag), xmlEscape(subject), xmlEscape(body)), nil
}

func (h *ewsHandler) createStickyNote(ctx context.Context, u *storage.User, folderID, subject string, doc noteutil.Node) (string, error) {
	_ = h.store.EnsureNoteDefaults(ctx, u.ID)
	if folderID == "" {
		f, err := h.store.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
		if err != nil {
			return "", err
		}
		folderID = f.ID
	} else if rights, err := h.store.NoteFolderRightsForUser(ctx, folderID, u.ID); err != nil || rights != "write" {
		return ewsFault("ErrorAccessDenied", "forbidden"), nil
	}
	docJSON := noteutil.MarshalDocumentJSON(doc)
	htmlOut, textOut := noteutil.DeriveBodies(docJSON)
	if subject == "" {
		subject = firstLine(textOut)
	}
	n, err := h.store.CreateNoteItem(ctx, &storage.NoteItem{
		FolderID: folderID, UserID: u.ID, Title: subject,
		DocumentJSON: docJSON, BodyHTML: htmlOut, BodyText: textOut,
	})
	if err != nil {
		return "", err
	}
	return noteEWSResponse("CreateItem", n), nil
}

func noteSnapshotHash(n *storage.NoteItem, preferences syncPreferences) (string, error) {
	raw := renderNotesSyncXML("unused", n.FolderID, []noteAdd{noteSyncAdd(n)}, nil)
	tree, err := parseProtocolXML([]byte(raw))
	if err != nil {
		return "", err
	}
	hashes, _ := snapshotItemHashes(kindNotes, tree.find("Commands"), preferences, nil)
	return hashes[n.ID], nil
}
