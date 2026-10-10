// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strconv"
	"strings"
)

func (h *ewsHandler) syncEWSItems(ctx context.Context, u *storage.User, body string) (string, error) {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	folder, err := h.resolveEWSFolder(ctx, u, body)
	if err != nil {
		return ewsOperationError("SyncFolderItems", "ErrorFolderNotFound", "folder unavailable"), nil
	}
	// Resolve and check ownership before loading any state or listing items.
	if _, err := (&easHandler{store: h.store}).resolveCollection(ctx, u.ID, folder); err != nil {
		return ewsOperationError("SyncFolderItems", "ErrorAccessDenied", "folder unavailable"), nil
	}
	token := extractTag(body, "SyncState")
	client, key := "ews:"+storage.NewID(), ""
	if token != "" {
		parts := strings.SplitN(token, "/", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "ews:") {
			return ewsOperationError("SyncFolderItems", "ErrorInvalidSyncStateData", "invalid sync state"), nil
		}
		client, key = parts[0], parts[1]
	}
	state, err := loadCollectionState(ctx, h.store, u.ID, client, folder)
	if err != nil {
		return "", err
	}
	fingerprint := digest(body)
	if key != "" && key == state.PreviousKey && fingerprint == state.RequestHash {
		return state.Response, nil
	}
	if token != "" && (state.Key == "" || state.Key != key) {
		return ewsOperationError("SyncFolderItems", "ErrorInvalidSyncStateData", "invalid sync state"), nil
	}
	full, err := h.findItem(ctx, u, body)
	if err != nil {
		return "", err
	}
	doc, err := parseProtocolXML([]byte(full))
	if err != nil {
		return "", err
	}
	nodes := map[string]*protocolNode{}
	hashes := map[string]string{}
	if items := doc.find("Items"); items != nil {
		for _, item := range items.Children {
			id := extractAttr(item.render(), "ItemId", "Id")
			nodes[id] = item
			hashes[id] = digest(item.render())
		}
	}
	limit := 100
	if n, e := strconv.Atoi(extractTag(body, "MaxChangesReturned")); e == nil && n > 0 {
		limit = n
	}
	if limit > 512 {
		limit = 512
	}
	ids, nextItems, more := diffItems(state.Items, hashes, limit)
	next := storage.NewID()
	var out strings.Builder
	fmt.Fprintf(&out, `<m:SyncFolderItemsResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:SyncFolderItemsResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:SyncState>%s/%s</m:SyncState><m:IncludesLastItemInRange>%t</m:IncludesLastItemInRange><m:Changes>`, client, next, !more)
	for _, id := range ids {
		item, exists := nodes[id]
		if !exists {
			fmt.Fprintf(&out, `<t:Delete><t:ItemId Id="%s"/></t:Delete>`, xmlEscape(id))
			continue
		}
		op := "Create"
		if _, seen := state.Items[id]; seen {
			op = "Update"
		}
		fmt.Fprintf(&out, `<t:%s>%s</t:%s>`, op, item.render(), op)
	}
	out.WriteString(`</m:Changes></m:SyncFolderItemsResponseMessage></m:ResponseMessages></m:SyncFolderItemsResponse>`)
	result := out.String()
	err = saveCollectionState(ctx, h.store, u.ID, client, folder, collectionState{Key: next, Items: nextItems, PreviousKey: key, RequestHash: fingerprint, Response: result})
	return result, err
}

func (h *ewsHandler) syncEWSHierarchy(ctx context.Context, u *storage.User, body string) (string, error) {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	folder := "hierarchy"
	token := extractTag(body, "SyncState")
	client, key := "ews:"+storage.NewID(), ""
	if token != "" {
		parts := strings.SplitN(token, "/", 2)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], "ews:") {
			return ewsOperationError("SyncFolderHierarchy", "ErrorInvalidSyncStateData", "invalid sync state"), nil
		}
		client, key = parts[0], parts[1]
	}
	state, err := loadCollectionState(ctx, h.store, u.ID, client, folder)
	if err != nil {
		return "", err
	}
	fingerprint := digest(body)
	if key != "" && key == state.PreviousKey && fingerprint == state.RequestHash {
		return state.Response, nil
	}
	if token != "" && (state.Key == "" || state.Key != key) {
		return ewsOperationError("SyncFolderHierarchy", "ErrorInvalidSyncStateData", "invalid sync state"), nil
	}
	full, err := h.findFolder(ctx, u)
	if err != nil {
		return "", err
	}
	doc, err := parseProtocolXML([]byte(full))
	if err != nil {
		return "", err
	}
	nodes := map[string]*protocolNode{}
	hashes := map[string]string{}
	if items := doc.find("Folders"); items != nil {
		for _, item := range items.Children {
			id := extractAttr(item.render(), "FolderId", "Id")
			nodes[id] = item
			hashes[id] = digest(item.render())
		}
	}
	limit := 100
	if n, e := strconv.Atoi(extractTag(body, "MaxChangesReturned")); e == nil && n > 0 {
		limit = n
	}
	if limit > 512 {
		limit = 512
	}
	ids, nextItems, more := diffItems(state.Items, hashes, limit)
	next := storage.NewID()
	var out strings.Builder
	fmt.Fprintf(&out, `<m:SyncFolderHierarchyResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:SyncFolderHierarchyResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:SyncState>%s/%s</m:SyncState><m:IncludesLastFolderInRange>%t</m:IncludesLastFolderInRange><m:Changes>`, client, next, !more)
	for _, id := range ids {
		item, exists := nodes[id]
		if !exists {
			fmt.Fprintf(&out, `<t:Delete><t:FolderId Id="%s"/></t:Delete>`, xmlEscape(id))
			continue
		}
		op := "Create"
		if _, seen := state.Items[id]; seen {
			op = "Update"
		}
		fmt.Fprintf(&out, `<t:%s>%s</t:%s>`, op, item.render(), op)
	}
	out.WriteString(`</m:Changes></m:SyncFolderHierarchyResponseMessage></m:ResponseMessages></m:SyncFolderHierarchyResponse>`)
	result := out.String()
	err = saveCollectionState(ctx, h.store, u.ID, client, folder, collectionState{Key: next, Items: nextItems, PreviousKey: key, RequestHash: fingerprint, Response: result})
	return result, err
}
