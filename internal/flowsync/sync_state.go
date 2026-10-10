// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type collectionState struct {
	Preferences syncPreferences
	Key         string
	Items       map[string]string
	PreviousKey string
	RequestHash string
	Response    string
}

func digest(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func loadCollectionState(ctx context.Context, st storage.Driver, user, client, collection string) (collectionState, error) {
	raw, err := st.GetFlowSyncState(ctx, user, client, collection)
	if err != nil {
		return collectionState{}, err
	}
	s := collectionState{Items: map[string]string{}}
	if raw != "" {
		if err = json.Unmarshal([]byte(raw), &s); err != nil {
			return s, err
		}
	}
	return s, nil
}
func saveCollectionState(ctx context.Context, st storage.Driver, user, client, collection string, s collectionState) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return st.PutFlowSyncState(ctx, user, client, collection, string(raw))
}

// Synchronization state records only acknowledged/sent item hashes. A window
// advances the snapshot by precisely those changes included in that response.
func diffItems(previous map[string]string, current map[string]string, limit int) ([]string, map[string]string, bool) {
	ids := []string{}
	for id, hash := range current {
		if previous[id] != hash {
			ids = append(ids, id)
		}
	}
	for id := range previous {
		if _, ok := current[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	more := len(ids) > limit
	if more {
		ids = ids[:limit]
	}
	next := map[string]string{}
	for id, hash := range previous {
		next[id] = hash
	}
	for _, id := range ids {
		if hash, ok := current[id]; ok {
			next[id] = hash
		} else {
			delete(next, id)
		}
	}
	return ids, next, more
}

func syncStatus(collection string, status int) string {
	return fmt.Sprintf(`<Collection xmlns="AirSync:"><SyncKey>0</SyncKey><CollectionId>%s</CollectionId><Status>%d</Status></Collection>`, xmlEscape(collection), status)
}

func (h *easHandler) syncCollections(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, body []byte) (string, error) {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	doc, err := parseProtocolXML(body)
	if err != nil {
		return "", err
	}
	collections := doc.find("Collections")
	if collections == nil {
		return `<Sync xmlns="AirSync:"><Status>13</Status></Sync>`, nil
	}
	var response strings.Builder
	response.WriteString(`<Sync xmlns="AirSync:"><Collections>`)
	if len(collections.Children) > 100 {
		return `<Sync xmlns="AirSync:"><Status>13</Status></Sync>`, nil
	}
	for _, request := range collections.Children {
		if request.Name.Local != "Collection" {
			continue
		}
		result, err := h.syncOne(ctx, u, dev, request)
		if err != nil {
			return "", err
		}
		response.WriteString(result)
	}
	response.WriteString(`</Collections></Sync>`)
	return response.String(), nil
}

func (h *easHandler) syncOne(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, request *protocolNode) (string, error) {
	id, key := request.value("CollectionId"), request.value("SyncKey")
	kind, err := h.resolveCollection(ctx, u.ID, id)
	if err != nil {
		return syncStatus(id, 8), nil
	}
	state, err := loadCollectionState(ctx, h.store, u.ID, dev.ID, id)
	if err != nil {
		return "", err
	}
	fingerprint := digest(request.render())
	if key != "0" && key == state.PreviousKey && fingerprint == state.RequestHash && state.Response != "" {
		return state.Response, nil
	}
	if key != "0" && (key == "" || key != state.Key) {
		return syncStatus(id, 3), nil
	}
	preferences, err := preferencesFrom(request, state.Preferences)
	if err != nil || !validFilter(kind, preferences) {
		return syncStatus(id, 4), nil
	}
	ctx = context.WithValue(ctx, preferencesKey{}, preferences)
	if key == "0" {
		if changes := request.child("GetChanges"); changes != nil && changes.Text != "0" {
			return syncStatus(id, 4), nil
		}
	}
	if key == "0" {
		next := storage.NewID()
		result := fmt.Sprintf(`<Collection xmlns="AirSync:"><SyncKey>%s</SyncKey><CollectionId>%s</CollectionId><Status>1</Status></Collection>`, next, xmlEscape(id))
		err = saveCollectionState(ctx, h.store, u.ID, dev.ID, id, collectionState{Preferences: preferences, Key: next, Items: map[string]string{}, Response: result, PreviousKey: key, RequestHash: fingerprint})
		return result, err
	}
	// The key is validated before accepting any client changes.
	writes := h.applyWritebacks(context.WithValue(ctx, deleteMovesKey{}, request.value("DeletesAsMoves") != "0"), u, kind, id, parseXMLClientOps(request.render()))
	if request.value("GetChanges") == "0" {
		next := storage.NewID()
		out := fmt.Sprintf(`<Collection xmlns="AirSync:"><SyncKey>%s</SyncKey><CollectionId>%s</CollectionId><Status>1</Status>%s</Collection>`, next, xmlEscape(id), renderWritebackResponses(writes))
		return out, saveCollectionState(ctx, h.store, u.ID, dev.ID, id, collectionState{Preferences: preferences, Key: next, Items: state.Items, PreviousKey: key, RequestHash: fingerprint, Response: out})
	}
	var full string
	switch kind {
	case kindCalendar:
		full, _, err = h.syncCalendar(ctx, id, "unused", false, writes)
	case kindContacts:
		full, _, err = h.syncContacts(ctx, id, "unused", false, writes)
	case kindNotes:
		full, _, err = h.syncNotes(ctx, u, id, "unused", false, writes)
	default:
		full, _, err = h.syncMailCollection(context.WithValue(ctx, mailSnapshotKey{}, true), u, id, "unused", false, writes)
	}
	if err != nil {
		return "", err
	}
	document, err := parseProtocolXML([]byte(full))
	if err != nil {
		return "", err
	}
	commands := document.find("Commands")
	items := map[string]*protocolNode{}
	hashes := map[string]string{}
	if commands != nil {
		for _, add := range commands.Children {
			sid := add.value("ServerId")
			items[sid] = add
			hashes[sid] = digest(add.render() + preferences.hash())
		}
	}
	window := 100
	if n, e := strconv.Atoi(request.value("WindowSize")); e == nil && n > 0 {
		window = n
	}
	if window > 512 {
		window = 512
	}
	ids, nextItems, more := diffItems(state.Items, hashes, window)
	if kind == kindMail && len(ids) > 0 {
		selected := map[string]bool{}
		for _, sid := range ids {
			selected[sid] = true
		}
		rich, _, err := h.syncMailCollection(context.WithValue(ctx, mailSelectionKey{}, selected), u, id, "unused", false, nil)
		if err != nil {
			return "", err
		}
		doc, err := parseProtocolXML([]byte(rich))
		if err != nil {
			return "", err
		}
		if commands := doc.find("Commands"); commands != nil {
			for _, item := range commands.Children {
				items[item.value("ServerId")] = item
			}
		}
	}
	nextKey := storage.NewID()
	var result strings.Builder
	fmt.Fprintf(&result, `<Collection xmlns="AirSync:"><SyncKey>%s</SyncKey><CollectionId>%s</CollectionId><Status>1</Status>`, nextKey, xmlEscape(id))
	result.WriteString(renderWritebackResponses(writes))
	if more {
		result.WriteString(`<MoreAvailable/>`)
	}
	if len(ids) > 0 {
		result.WriteString(`<Commands>`)
	}
	for _, sid := range ids {
		item, exists := items[sid]
		if !exists {
			op := "Delete"
			if kind == kindMail {
				if msg, e := h.store.GetMessageByID(ctx, sid); e == nil && msg.MailboxID == id {
					op = "SoftDelete"
				}
			}
			fmt.Fprintf(&result, `<%s><ServerId>%s</ServerId></%s>`, op, xmlEscape(sid), op)
			continue
		}
		if _, seen := state.Items[sid]; seen {
			item.Name.Local = "Change"
		}
		result.WriteString(item.render())
	}
	if len(ids) > 0 {
		result.WriteString(`</Commands>`)
	}
	result.WriteString(`</Collection>`)
	out := result.String()
	err = saveCollectionState(ctx, h.store, u.ID, dev.ID, id, collectionState{Preferences: preferences, Key: nextKey, Items: nextItems, PreviousKey: key, RequestHash: fingerprint, Response: out})
	return out, err
}
