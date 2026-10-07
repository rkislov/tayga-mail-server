package flowsync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// EWS-compatible SOAP endpoint handled by proprietary FlowSync engine.
type ewsHandler struct {
	store     storage.Driver
	ms        *mailstore.Store
	publicURL string
}

func (h *ewsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
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

	var soap string
	var err error
	switch op {
	case "FindItem":
		soap, err = h.findItem(r.Context(), u)
	case "GetItem":
		soap, err = h.getItem(r.Context(), u, string(body))
	case "SyncFolderItems":
		soap, err = h.syncFolderItems(r.Context(), u)
	case "GetFolder", "FindFolder":
		soap, err = h.findFolder(r.Context(), u)
	default:
		soap = ewsFault("ErrorInvalidRequest", "FlowSync: unsupported operation "+op)
	}
	if err != nil {
		soap = ewsFault("ErrorInternalServerError", err.Error())
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(soapEnvelope(soap)))
}

func (h *ewsHandler) findFolder(ctx context.Context, u *storage.User) (string, error) {
	mbs, err := h.store.ListMailboxes(ctx, u.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<m:FindFolderResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:FindFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder>`)
	for _, mb := range mbs {
		fmt.Fprintf(&b, `<t:Folders><t:Folder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName><t:TotalCount>0</t:TotalCount></t:Folder></t:Folders>`,
			xmlEscape(mb.ID), xmlEscape(mb.Name))
	}
	b.WriteString(`</m:RootFolder></m:FindFolderResponseMessage></m:ResponseMessages></m:FindFolderResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) findItem(ctx context.Context, u *storage.User) (string, error) {
	mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		return "", err
	}
	msgs, err := h.store.ListMessages(ctx, mb.ID)
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
		fmt.Fprintf(&b, `<t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>Message UID %d</t:Subject><t:DateTimeReceived>%s</t:DateTimeReceived></t:Message>`,
			xmlEscape(m.ID), xmlEscape(m.ID), m.UID, m.InternalDate.UTC().Format("2006-01-02T15:04:05Z"))
	}
	b.WriteString(`</t:Items></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) getItem(ctx context.Context, u *storage.User, body string) (string, error) {
	id := extractAttr(body, "ItemId", "Id")
	if id == "" {
		return ewsFault("ErrorInvalidId", "missing ItemId"), nil
	}
	msg, err := h.store.GetMessageByID(ctx, id)
	if err != nil {
		return ewsFault("ErrorItemNotFound", err.Error()), nil
	}
	// ownership: message must belong to user's mailbox
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
	var b strings.Builder
	b.WriteString(`<m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:GetItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items>`)
	fmt.Fprintf(&b, `<t:Message><t:ItemId Id="%s" ChangeKey="%s"/><t:Subject>Message UID %d</t:Subject><t:Body BodyType="Text">%s</t:Body></t:Message>`,
		xmlEscape(msg.ID), xmlEscape(msg.ID), msg.UID, xmlEscape(truncate(string(raw), 64<<10)))
	b.WriteString(`</m:Items></m:GetItemResponseMessage></m:ResponseMessages></m:GetItemResponse>`)
	return b.String(), nil
}

func (h *ewsHandler) syncFolderItems(ctx context.Context, u *storage.User) (string, error) {
	mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		return "", err
	}
	msgs, err := h.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return "", err
	}
	syncState := storage.NewID() // UUID sync state token
	var b strings.Builder
	b.WriteString(`<m:SyncFolderItemsResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">`)
	b.WriteString(`<m:ResponseMessages><m:SyncFolderItemsResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode>`)
	fmt.Fprintf(&b, `<m:SyncState>%s</m:SyncState><m:IncludesLastItemInRange>true</m:IncludesLastItemInRange>`, xmlEscape(syncState))
	b.WriteString(`<m:Changes>`)
	for _, m := range msgs {
		fmt.Fprintf(&b, `<t:Create><t:Message><t:ItemId Id="%s" ChangeKey="%s"/></t:Message></t:Create>`,
			xmlEscape(m.ID), xmlEscape(m.ID))
	}
	b.WriteString(`</m:Changes></m:SyncFolderItemsResponseMessage></m:ResponseMessages></m:SyncFolderItemsResponse>`)
	return b.String(), nil
}

func detectEWSOp(body string) string {
	for _, op := range []string{"SyncFolderItems", "FindItem", "GetItem", "FindFolder", "GetFolder"} {
		if strings.Contains(body, op) {
			return op
		}
	}
	return ""
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
	// find Id="uuid" near ItemId
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
