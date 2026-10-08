package flowsync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/noteutil"
	"github.com/tayga/tms/internal/storage"
)

const folderTypeNotes = 10

type noteAdd struct {
	ServerID  string
	Subject   string
	Body      string
	BodyType  string // 1=plain 2=HTML
	LastMod   string
	Created   string
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
		body := n.BodyHTML
		bt := "2"
		if body == "" {
			body = n.BodyText
			bt = "1"
		}
		subject := n.Title
		if subject == "" {
			subject = firstLine(n.BodyText)
		}
		adds = append(adds, noteAdd{
			ServerID: n.ID,
			Subject:  subject,
			Body:     body,
			BodyType: bt,
			LastMod:  n.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
			Created:  n.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
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
		fmt.Fprintf(&b, `<Notes:Body xmlns:Notes="Notes:">%s</Notes:Body>`, xmlEscape(a.Body))
		fmt.Fprintf(&b, `<Notes:BodyType xmlns:Notes="Notes:">%s</Notes:BodyType>`, xmlEscape(a.BodyType))
		if a.LastMod != "" {
			fmt.Fprintf(&b, `<Notes:LastModifiedDate xmlns:Notes="Notes:">%s</Notes:LastModifiedDate>`, xmlEscape(a.LastMod))
		}
		b.WriteString(`</ApplicationData></Add>`)
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String()
}

func (h *easHandler) wbNotes(ctx context.Context, u *storage.User, folderID string, op clientOp) (string, error) {
	switch op.Kind {
	case "add":
		if rights, err := h.store.NoteFolderRightsForUser(ctx, folderID, u.ID); err != nil || rights != "write" {
			return "", fmt.Errorf("forbidden")
		}
		subject := op.Fields["Subject"]
		body := op.Fields["Body"]
		doc := noteutil.DocumentFromHTMLOrText(body)
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
		return n.ID, nil
	case "change":
		rights, err := h.store.NoteRightsForUser(ctx, op.ServerID, u.ID)
		if err != nil || rights != "write" {
			return op.ServerID, fmt.Errorf("forbidden")
		}
		n, err := h.store.GetNoteItemByID(ctx, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
		if v := op.Fields["Subject"]; v != "" {
			n.Title = v
		}
		if v := op.Fields["Body"]; v != "" {
			doc := noteutil.DocumentFromHTMLOrText(v)
			n.DocumentJSON = noteutil.MarshalDocumentJSON(doc)
			n.BodyHTML, n.BodyText = noteutil.DeriveBodies(n.DocumentJSON)
		}
		if err := h.store.UpdateNoteItem(ctx, n); err != nil {
			return op.ServerID, err
		}
		return n.ID, nil
	case "delete":
		rights, err := h.store.NoteRightsForUser(ctx, op.ServerID, u.ID)
		if err != nil || rights != "write" {
			return op.ServerID, fmt.Errorf("forbidden")
		}
		if err := h.store.DeleteNoteItemByID(ctx, op.ServerID); err != nil {
			return op.ServerID, err
		}
		return op.ServerID, nil
	default:
		return op.ServerID, fmt.Errorf("unsupported op")
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
	if len(s) > 80 {
		s = s[:80]
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

func (h *ewsHandler) createStickyNote(ctx context.Context, u *storage.User, folderID, subject, body string) (string, error) {
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
	doc := noteutil.DocumentFromHTMLOrText(body)
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
	return ewsCreateResponse("Message", n.ID), nil
}
