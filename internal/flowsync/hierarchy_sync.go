// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func (h *easHandler) hierarchySync(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, body []byte) (string, error) {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	key := extractTag(string(body), "SyncKey")
	if key == "" {
		key = "0"
	}
	state, err := loadCollectionState(ctx, h.store, u.ID, dev.ID, "hierarchy")
	if err != nil {
		return "", err
	}
	fingerprint := digest(string(body))
	if key != "0" && key == state.PreviousKey && fingerprint == state.RequestHash {
		return state.Response, nil
	}
	if key != "0" && key != state.Key {
		return `<FolderSync xmlns="FolderHierarchy:"><Status>9</Status></FolderSync>`, nil
	}
	if key == "0" {
		state.Items = map[string]string{}
	}
	full, _, err := h.folderSync(ctx, u, dev, false)
	if err != nil {
		return "", err
	}
	doc, err := parseProtocolXML([]byte(full))
	if err != nil {
		return "", err
	}
	nodes := map[string]*protocolNode{}
	current := map[string]string{}
	if changes := doc.find("Changes"); changes != nil {
		for _, n := range changes.Children {
			if n.Name.Local != "Add" {
				continue
			}
			id := n.value("ServerId")
			nodes[id] = n
			current[id] = digest(n.render())
		}
	}
	ids, nextItems, _ := diffItems(state.Items, current, len(current)+len(state.Items)+1)
	next := storage.NewID()
	var out strings.Builder
	fmt.Fprintf(&out, `<FolderSync xmlns="FolderHierarchy:"><Status>1</Status><SyncKey>%s</SyncKey><Changes><Count>%d</Count>`, next, len(ids))
	for _, id := range ids {
		n, exists := nodes[id]
		if !exists {
			fmt.Fprintf(&out, `<Delete><ServerId>%s</ServerId></Delete>`, xmlEscape(id))
			continue
		}
		if _, ok := state.Items[id]; ok {
			n.Name.Local = "Update"
		}
		out.WriteString(n.render())
	}
	out.WriteString(`</Changes></FolderSync>`)
	response := out.String()
	err = saveCollectionState(ctx, h.store, u.ID, dev.ID, "hierarchy", collectionState{Key: next, Items: nextItems, PreviousKey: key, RequestHash: fingerprint, Response: response})
	return response, err
}

// Folder mutations share the same key and replay cache as FolderSync.
func (h *easHandler) mutateHierarchy(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, command string, body []byte, fn func(context.Context, *storage.User, []byte, bool) (string, []byte, error)) (string, error) {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	state, err := loadCollectionState(ctx, h.store, u.ID, dev.ID, "hierarchy")
	if err != nil {
		return "", err
	}
	key := extractTag(string(body), "SyncKey")
	fingerprint := digest(command + string(body))
	if key != "" && key == state.PreviousKey && fingerprint == state.RequestHash {
		return state.Response, nil
	}
	if key == "" || key != state.Key {
		return folderOpStatusXML(command, 9, ""), nil
	}
	response, _, err := fn(ctx, u, body, false)
	if err != nil {
		return "", err
	}
	if extractTag(response, "Status") != "1" {
		return response, nil
	}
	id := extractTag(response, "ServerId")
	if id == "" {
		id = extractTag(string(body), "ServerId")
	}
	if command == "FolderDelete" {
		delete(state.Items, id)
	} else {
		full, _, err := h.folderSync(ctx, u, dev, false)
		if err != nil {
			return "", err
		}
		doc, err := parseProtocolXML([]byte(full))
		if err != nil {
			return "", err
		}
		if changes := doc.find("Changes"); changes != nil {
			for _, n := range changes.Children {
				if n.Name.Local == "Add" && n.value("ServerId") == id {
					state.Items[id] = digest(n.render())
				}
			}
		}
	}
	next := storage.NewID()
	response = strings.Replace(response, "</Status>", "</Status><SyncKey>"+next+"</SyncKey>", 1)
	state.Key = next
	state.PreviousKey = key
	state.RequestHash = fingerprint
	state.Response = response
	return response, saveCollectionState(ctx, h.store, u.ID, dev.ID, "hierarchy", state)
}
