// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// EWS-compatible SOAP endpoint handled by original FlowSync engine.
type ewsHandler struct {
	store     storage.Driver
	ms        *mailstore.Store
	publicURL string
}

func (h *ewsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-FlowSync", "Tayga-FlowSync")
	w.Header().Set("X-FlowSync-Engine", "FlowSync/1.0")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u, ok := userFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	op := detectEWSOp(string(body))
	if t := traceRequest(r); t != nil {
		t.Command = op
	}

	var soap string
	var err error
	switch op {
	case "FindItem":
		soap, err = h.findItem(r.Context(), u, string(body))
	case "GetItem":
		soap, err = h.getItem(r.Context(), u, string(body))
	case "CreateItem":
		soap, err = h.createItem(r.Context(), u, string(body))
	case "UpdateItem":
		soap, err = h.updateItem(r.Context(), u, string(body))
	case "DeleteItem":
		soap, err = h.deleteItem(r.Context(), u, string(body))
	case "SyncFolderItems":
		soap, err = h.syncFolderItems(r.Context(), u, string(body))
	case "GetFolder", "FindFolder":
		soap, err = h.findFolder(r.Context(), u)
	default:
		if t := traceRequest(r); t != nil {
			t.Result = "unsupported_operation"
		}
		soap = ewsFault("ErrorInvalidRequest", "FlowSync: unsupported operation "+op)
	}
	if err != nil {
		if t := traceRequest(r); t != nil {
			t.Result = "internal_error"
		}
		soap = ewsFault("ErrorInternalServerError", err.Error())
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(soapEnvelope(soap)))
}

func (h *ewsHandler) findFolder(ctx context.Context, u *storage.User) (string, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	mbs, err := h.store.ListMailboxes(ctx, u.ID)
	if err != nil {
		return "", err
	}
	cals, err := h.store.ListCalendars(ctx, u.ID)
	if err != nil {
		return "", err
	}
	abs, err := h.store.ListAddressBooks(ctx, u.ID)
	if err != nil {
		return "", err
	}
	_ = h.store.EnsureNoteDefaults(ctx, u.ID)
	nfs, err := h.store.ListNoteFoldersForUser(ctx, u.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<m:FindFolderResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder>`)
	for _, mb := range mbs {
		fmt.Fprintf(&b, `<t:Folders><t:Folder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.Note</t:FolderClass><t:TotalCount>0</t:TotalCount></t:Folder></t:Folders>`,
			xmlEscape(mb.ID), xmlEscape(mb.Name))
	}
	for _, cal := range cals {
		name := cal.DisplayName
		if name == "" {
			name = cal.Name
		}
		fmt.Fprintf(&b, `<t:Folders><t:CalendarFolder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.Appointment</t:FolderClass><t:TotalCount>0</t:TotalCount></t:CalendarFolder></t:Folders>`,
			xmlEscape(cal.ID), xmlEscape(name))
	}
	for _, ab := range abs {
		name := ab.DisplayName
		if name == "" {
			name = ab.Name
		}
		fmt.Fprintf(&b, `<t:Folders><t:ContactsFolder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.Contact</t:FolderClass><t:TotalCount>0</t:TotalCount></t:ContactsFolder></t:Folders>`,
			xmlEscape(ab.ID), xmlEscape(name))
	}
	for _, nf := range nfs {
		name := nf.DisplayName
		if name == "" {
			name = nf.Name
		}
		fmt.Fprintf(&b, `<t:Folders><t:Folder><t:FolderId Id="%s" ChangeKey="%s"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.StickyNote</t:FolderClass><t:TotalCount>0</t:TotalCount></t:Folder></t:Folders>`,
			xmlEscape(nf.ID), xmlEscape(nf.CTag), xmlEscape(name))
	}
	b.WriteString(`</m:RootFolder></m:FindFolderResponseMessage></m:ResponseMessages></m:FindFolderResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) findItem(ctx context.Context, u *storage.User, body string) (string, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	folderID := extractAttr(body, "FolderId", "Id")
	if folderID == "" {
		mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
		if err != nil {
			return "", err
		}
		folderID = mb.ID
	}

	if cal, err := h.store.GetCalendarByID(ctx, u.ID, folderID); err == nil {
		return h.findCalendarItems(ctx, cal.ID)
	}
	if ab, err := h.store.GetAddressBookByID(ctx, u.ID, folderID); err == nil {
		return h.findContactItems(ctx, ab.ID)
	}
	if _, err := h.store.NoteFolderRightsForUser(ctx, folderID, u.ID); err == nil {
		return h.findNoteItems(ctx, u, folderID)
	}
	return h.findMailItems(ctx, u, folderID)
}

func (h *ewsHandler) findMailItems(ctx context.Context, u *storage.User, mailboxID string) (string, error) {
	_ = u
	msgs, err := h.store.ListMessages(ctx, mailboxID)
	if err != nil {
		return "", err
	}
	if len(msgs) > 50 {
		msgs = msgs[len(msgs)-50:]
	}
	var b strings.Builder
	b.WriteString(`<m:FindItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder>`)
	b.WriteString(`<t:Items>`)
	for _, m := range msgs {
		hdr := readMsgHeaders(h.ms, m.FilePath)
		fmt.Fprintf(&b, `<t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:DateTimeReceived>%s</t:DateTimeReceived></t:Message>`,
			xmlEscape(m.ID), xmlEscape(m.ID), xmlEscape(hdr.Subject), m.InternalDate.UTC().Format("2006-01-02T15:04:05Z"))
	}
	b.WriteString(`</t:Items></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) findCalendarItems(ctx context.Context, calendarID string) (string, error) {
	objs, err := h.store.ListCalendarObjects(ctx, calendarID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<m:FindItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder><t:Items>`)
	for _, o := range objs {
		ev := parseICalEvent(o.Data)
		subject := ev.Summary
		if subject == "" {
			subject = o.UID
		}
		start := formatASTime(o.DTStart)
		if start == "" {
			start = ev.Start
		}
		end := formatASTime(o.DTEnd)
		if end == "" {
			end = ev.End
		}
		fmt.Fprintf(&b, `<t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:Start>%s</t:Start><t:End>%s</t:End><t:Location>%s</t:Location><t:UID>%s</t:UID></t:CalendarItem>`,
			xmlEscape(o.ID), xmlEscape(o.ID), xmlEscape(subject), xmlEscape(start), xmlEscape(end), xmlEscape(ev.Location), xmlEscape(o.UID))
	}
	b.WriteString(`</t:Items></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) findContactItems(ctx context.Context, addressBookID string) (string, error) {
	objs, err := h.store.ListAddressObjects(ctx, addressBookID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<m:FindItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder><t:Items>`)
	for _, o := range objs {
		c := parseVCard(o.Data)
		dn := c.FN
		if dn == "" {
			dn = strings.TrimSpace(c.FirstName + " " + c.LastName)
		}
		fmt.Fprintf(&b, `<t:Contact><t:ItemId Id="%s" ChangeKey="%s"/><t:DisplayName>%s</t:DisplayName><t:GivenName>%s</t:GivenName><t:Surname>%s</t:Surname><t:EmailAddresses><t:Entry Key="EmailAddress1">%s</t:Entry></t:EmailAddresses></t:Contact>`,
			xmlEscape(o.ID), xmlEscape(o.ID), xmlEscape(dn), xmlEscape(c.FirstName), xmlEscape(c.LastName), xmlEscape(c.Email))
	}
	b.WriteString(`</t:Items></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) getItem(ctx context.Context, u *storage.User, body string) (string, error) {
	id := extractAttr(body, "ItemId", "Id")
	if id == "" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}
	// Try mail first, then calendar object / contact by scanning user collections.
	if msg, err := h.store.GetMessageByID(ctx, id); err == nil {
		mbs, err := h.store.ListMailboxes(ctx, u.ID)
		if err != nil {
			return "", err
		}
		owned := false
		for _, mb := range mbs {
			if mb.ID == msg.MailboxID {
				owned = true
				break
			}
		}
		if !owned {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
		raw, err := h.ms.Read(msg.FilePath)
		if err != nil {
			raw = []byte("")
		}
		hdr := readMsgHeaders(h.ms, msg.FilePath)
		var b strings.Builder
		b.WriteString(`<m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
		b.WriteString(`<m:ResponseMessages><m:GetItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items>`)
		fmt.Fprintf(&b, `<t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:Body BodyType="Text">%s</t:Body></t:Message>`,
			xmlEscape(msg.ID), xmlEscape(msg.ID), xmlEscape(hdr.Subject), xmlEscape(truncate(string(raw), 64<<10)))
		b.WriteString(`</m:Items></m:GetItemResponseMessage></m:ResponseMessages></m:GetItemResponse>`)
		return b.String(), nil
	}

	cals, _ := h.store.ListCalendars(ctx, u.ID)
	for _, cal := range cals {
		objs, _ := h.store.ListCalendarObjects(ctx, cal.ID)
		for _, o := range objs {
			if o.ID != id {
				continue
			}
			ev := parseICalEvent(o.Data)
			var b strings.Builder
			b.WriteString(`<m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
			b.WriteString(`<m:ResponseMessages><m:GetItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items>`)
			fmt.Fprintf(&b, `<t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:Body BodyType="Text">%s</t:Body><t:UID>%s</t:UID></t:CalendarItem>`,
				xmlEscape(o.ID), xmlEscape(o.ID), xmlEscape(ev.Summary), xmlEscape(truncate(o.Data, 64<<10)), xmlEscape(o.UID))
			b.WriteString(`</m:Items></m:GetItemResponseMessage></m:ResponseMessages></m:GetItemResponse>`)
			return b.String(), nil
		}
	}
	abs, _ := h.store.ListAddressBooks(ctx, u.ID)
	for _, ab := range abs {
		objs, _ := h.store.ListAddressObjects(ctx, ab.ID)
		for _, o := range objs {
			if o.ID != id {
				continue
			}
			c := parseVCard(o.Data)
			var b strings.Builder
			b.WriteString(`<m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
			b.WriteString(`<m:ResponseMessages><m:GetItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items>`)
			fmt.Fprintf(&b, `<t:Contact><t:ItemId Id="%s" ChangeKey="%s"/><t:DisplayName>%s</t:DisplayName><t:Body BodyType="Text">%s</t:Body></t:Contact>`,
				xmlEscape(o.ID), xmlEscape(o.ID), xmlEscape(c.FN), xmlEscape(truncate(o.Data, 64<<10)))
			b.WriteString(`</m:Items></m:GetItemResponseMessage></m:ResponseMessages></m:GetItemResponse>`)
			return b.String(), nil
		}
	}
	if _, err := h.store.NoteRightsForUser(ctx, id, u.ID); err == nil {
		return h.getNoteItem(ctx, u, id)
	}
	return ewsFault("ErrorItemNotFound", "not found"), nil
}

func (h *ewsHandler) syncFolderItems(ctx context.Context, u *storage.User, body string) (string, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	folderID := extractAttr(body, "FolderId", "Id")
	if folderID == "" {
		mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
		if err != nil {
			return "", err
		}
		folderID = mb.ID
	}
	syncState := storage.NewID() // UUID sync state token
	var b strings.Builder
	b.WriteString(`<m:SyncFolderItemsResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:SyncFolderItemsResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode>`)
	fmt.Fprintf(&b, `<m:SyncState>%s</m:SyncState><m:IncludesLastItemInRange>true</m:IncludesLastItemInRange>`, xmlEscape(syncState))
	b.WriteString(`<m:Changes>`)

	if cal, err := h.store.GetCalendarByID(ctx, u.ID, folderID); err == nil {
		objs, err := h.store.ListCalendarObjects(ctx, cal.ID)
		if err != nil {
			return "", err
		}
		for _, o := range objs {
			fmt.Fprintf(&b, `<t:Create><t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/></t:CalendarItem></t:Create>`,
				xmlEscape(o.ID), xmlEscape(o.ID))
		}
	} else if ab, err := h.store.GetAddressBookByID(ctx, u.ID, folderID); err == nil {
		objs, err := h.store.ListAddressObjects(ctx, ab.ID)
		if err != nil {
			return "", err
		}
		for _, o := range objs {
			fmt.Fprintf(&b, `<t:Create><t:Contact><t:ItemId Id="%s" ChangeKey="%s"/></t:Contact></t:Create>`,
				xmlEscape(o.ID), xmlEscape(o.ID))
		}
	} else if _, err := h.store.NoteFolderRightsForUser(ctx, folderID, u.ID); err == nil {
		items, err := h.store.ListNoteItemsInFolder(ctx, folderID, false)
		if err != nil {
			return "", err
		}
		for _, n := range items {
			fmt.Fprintf(&b, `<t:Create><t:Message><t:ItemId Id="%s" ChangeKey="%s"/></t:Message></t:Create>`,
				xmlEscape(n.ID), xmlEscape(n.ETag))
		}
	} else {
		msgs, err := h.store.ListMessages(ctx, folderID)
		if err != nil {
			return "", err
		}
		for _, m := range msgs {
			fmt.Fprintf(&b, `<t:Create><t:Message><t:ItemId Id="%s" ChangeKey="%s"/></t:Message></t:Create>`,
				xmlEscape(m.ID), xmlEscape(m.ID))
		}
	}
	b.WriteString(`</m:Changes></m:SyncFolderItemsResponseMessage></m:ResponseMessages></m:SyncFolderItemsResponse>`)
	return b.String(), nil
}

func detectEWSOp(body string) string {
	decoder := xml.NewDecoder(strings.NewReader(body))
	inBody := false
	first := true
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch value := token.(type) {
		case xml.StartElement:
			if first {
				first = false
				if value.Name.Local != "Envelope" {
					return value.Name.Local
				}
			}
			if inBody {
				return value.Name.Local
			}
			if value.Name.Local == "Body" {
				inBody = true
			}
		}
	}
}

func soapEnvelope(inner string) string {
	return `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<s:Body>` + inner + `</s:Body></s:Envelope>`
}

func ewsFault(code, msg string) string {
	return fmt.Sprintf(`<m:FaultResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"><m:ResponseCode>%s</m:ResponseCode><m:MessageText>%s</m:MessageText></m:FaultResponse>`,
		xmlEscape(code), xmlEscape(msg))
}

func extractAttr(body, elem, attr string) string {
	needle := elem
	i := strings.Index(body, needle)
	if i < 0 {
		return ""
	}
	chunk := body[i:]
	if len(chunk) > 200 {
		chunk = chunk[:200]
	}
	key := attr + `="`
	j := strings.Index(chunk, key)
	if j < 0 {
		return ""
	}
	rest := chunk[j+len(key):]
	k := strings.Index(rest, `"`)
	if k < 0 {
		return ""
	}
	return rest[:k]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
