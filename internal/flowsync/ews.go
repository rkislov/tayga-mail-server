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
	"sync"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// EWS-compatible SOAP endpoint handled by original FlowSync engine.
type ewsHandler struct {
	sender    *mailSubmission
	syncMu    sync.Mutex
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
	body, readErr := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
	if readErr != nil || len(body) > 8<<20 {
		http.Error(w, "invalid or oversized request", 400)
		return
	}
	if _, e := parseProtocolXML(body); e != nil {
		http.Error(w, "invalid XML", 400)
		return
	}
	op := detectEWSOp(string(body))
	if t := traceRequest(r); t != nil {
		t.Command = boundedLog(op)
		t.Folders = ewsFolderReferences(body)
	}

	var soap string
	var err error
	switch op {
	case "FindItem":
		soap, err = h.findItem(r.Context(), u, string(body))
	case "GetAttachment":
		soap, err = h.getAttachment(r.Context(), u, string(body))
	case "GetItem":
		soap, err = h.batchItems(r.Context(), u, "GetItem", string(body), h.getItem)
	case "SendItem":
		soap, err = h.batchItems(r.Context(), u, "SendItem", string(body), h.sendItem)
	case "CreateItem":
		soap, err = h.createItem(r.Context(), u, string(body))
	case "UpdateItem":
		soap, err = h.batchItems(r.Context(), u, "UpdateItem", string(body), h.updateItem)
	case "DeleteItem":
		soap, err = h.batchItems(r.Context(), u, "DeleteItem", string(body), h.deleteItem)
	case "SyncFolderHierarchy":
		soap, err = h.syncEWSHierarchy(r.Context(), u, string(body))
	case "SyncFolderItems":
		soap, err = h.syncEWSItems(r.Context(), u, string(body))
	case "GetFolder":
		soap, err = h.getFolder(r.Context(), u, string(body))
	case "FindFolder":
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
		soap = ewsFault("ErrorInternalServerError", "FlowSync operation failed")
	}
	if strings.Contains(soap, "<m:FaultResponse") && op != "" {
		soap = ewsOperationError(op, extractTag(soap, "ResponseCode"), extractTag(soap, "MessageText"))
	}
	if t := traceRequest(r); t != nil {
		decoder := xml.NewDecoder(strings.NewReader(soap))
		for {
			token, e := decoder.Token()
			if e != nil {
				break
			}
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "ResponseCode" {
				var code string
				if decoder.DecodeElement(&code, &start) == nil && code != "NoError" {
					t.Result = boundedLog(code)
					break
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(soapEnvelope(soap)))
}

func (h *ewsHandler) findFolder(ctx context.Context, u *storage.User) (string, error) {
	if err := h.store.EnsureDAVDefaults(ctx, u.ID); err != nil {
		return "", err
	}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		return "", err
	}
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
	b.WriteString(`<m:ResponseMessages><m:FindFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder IncludesLastItemInRange="true"><t:Folders>`)
	for _, mb := range mbs {
		fmt.Fprintf(&b, `<t:Folder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.Note</t:FolderClass><t:TotalCount>0</t:TotalCount></t:Folder>`,
			xmlEscape(mb.ID), xmlEscape(mb.Name))
	}
	for _, cal := range cals {
		name := cal.DisplayName
		if name == "" {
			name = cal.Name
		}
		fmt.Fprintf(&b, `<t:CalendarFolder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.Appointment</t:FolderClass><t:TotalCount>0</t:TotalCount></t:CalendarFolder>`,
			xmlEscape(cal.ID), xmlEscape(name))
	}
	for _, ab := range abs {
		name := ab.DisplayName
		if name == "" {
			name = ab.Name
		}
		fmt.Fprintf(&b, `<t:ContactsFolder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.Contact</t:FolderClass><t:TotalCount>0</t:TotalCount></t:ContactsFolder>`,
			xmlEscape(ab.ID), xmlEscape(name))
	}
	for _, nf := range nfs {
		name := nf.DisplayName
		if name == "" {
			name = nf.Name
		}
		fmt.Fprintf(&b, `<t:Folder><t:FolderId Id="%s" ChangeKey="%s"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>IPF.StickyNote</t:FolderClass><t:TotalCount>0</t:TotalCount></t:Folder>`,
			xmlEscape(nf.ID), xmlEscape(nf.CTag), xmlEscape(name))
	}
	b.WriteString(`</t:Folders></m:RootFolder></m:FindFolderResponseMessage></m:ResponseMessages></m:FindFolderResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) findItem(ctx context.Context, u *storage.User, body string) (string, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	folderID, resolveErr := h.resolveEWSFolder(ctx, u, body)
	if resolveErr != nil {
		return ewsFault("ErrorFolderNotFound", "folder unavailable"), nil
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
	if err := h.ensureMailboxOwned(ctx, u.ID, mailboxID); err != nil {
		return ewsFault("ErrorAccessDenied", "forbidden"), nil
	}
	msgs, err := h.store.ListMessages(ctx, mailboxID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<m:FindItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder IncludesLastItemInRange="true">`)
	b.WriteString(`<t:Items>`)
	for _, m := range msgs {
		hdr := readMsgHeaders(h.ms, m.FilePath)
		fmt.Fprintf(&b, `<t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:DateTimeReceived>%s</t:DateTimeReceived><t:IsRead>%t</t:IsRead></t:Message>`,
			xmlEscape(m.ID), xmlEscape(m.ID), xmlEscape(hdr.Subject), m.InternalDate.UTC().Format("2006-01-02T15:04:05Z"), storage.HasFlag(m.Flags, `\Seen`))
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
			xmlEscape(o.ID), xmlEscape(o.ETag), xmlEscape(subject), xmlEscape(start), xmlEscape(end), xmlEscape(ev.Location), xmlEscape(o.UID))
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
			xmlEscape(o.ID), xmlEscape(o.ETag), xmlEscape(dn), xmlEscape(c.FirstName), xmlEscape(c.LastName), xmlEscape(c.Email))
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
		return h.getMailItem(ctx, u, msg, body)
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
			fmt.Fprintf(&b, `<t:CalendarItem><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>%s</t:Subject><t:Body BodyType="Text">%s</t:Body><t:UID>%s</t:UID><t:Start>%s</t:Start><t:End>%s</t:End><t:IsAllDayEvent>%t</t:IsAllDayEvent><t:Location>%s</t:Location></t:CalendarItem>`,
				xmlEscape(o.ID), xmlEscape(o.ETag), xmlEscape(ev.Summary), xmlEscape(davTextProperty(o.Data, "DESCRIPTION")), xmlEscape(o.UID), ewsDate(o.DTStart), ewsDate(o.DTEnd), ev.AllDay, xmlEscape(ev.Location))
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
			fmt.Fprintf(&b, `<t:Contact><t:ItemId Id="%s" ChangeKey="%s"/><t:DisplayName>%s</t:DisplayName><t:GivenName>%s</t:GivenName><t:Surname>%s</t:Surname><t:CompanyName>%s</t:CompanyName><t:JobTitle>%s</t:JobTitle><t:EmailAddresses><t:Entry Key="EmailAddress1">%s</t:Entry></t:EmailAddresses><t:PhoneNumbers><t:Entry Key="MobilePhone">%s</t:Entry></t:PhoneNumbers></t:Contact>`,
				xmlEscape(o.ID), xmlEscape(o.ETag), xmlEscape(c.FN), xmlEscape(c.FirstName), xmlEscape(c.LastName), xmlEscape(c.Org), xmlEscape(c.Title), xmlEscape(c.Email), xmlEscape(c.Tel))
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
	folderID, resolveErr := h.resolveEWSFolder(ctx, u, body)
	if resolveErr != nil {
		return ewsFault("ErrorFolderNotFound", "folder unavailable"), nil
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
		if err := h.ensureMailboxOwned(ctx, u.ID, folderID); err != nil {
			return ewsFault("ErrorAccessDenied", "forbidden"), nil
		}
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
	decoder := xml.NewDecoder(strings.NewReader(body))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == elem {
			for _, a := range start.Attr {
				if a.Name.Local == attr {
					return a.Value
				}
			}
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func ewsFolderReferences(body []byte) string {
	decoder := xml.NewDecoder(strings.NewReader(string(body)))
	var ids []string
	for {
		token, e := decoder.Token()
		if e != nil {
			break
		}
		if start, ok := token.(xml.StartElement); ok && (start.Name.Local == "FolderId" || start.Name.Local == "DistinguishedFolderId") {
			for _, a := range start.Attr {
				if a.Name.Local == "Id" {
					ids = append(ids, boundedLog(a.Value))
				}
			}
		}
		if len(ids) >= 8 {
			break
		}
	}
	return boundedLog(strings.Join(ids, ","))
}
