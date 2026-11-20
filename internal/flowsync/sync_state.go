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
	// Options used to build Items can differ from the sticky Fetch options.
	ItemPreferences *syncPreferences `json:",omitempty"`
	ReadKeys        []string         `json:",omitempty"`
}

func (s collectionState) itemPreferences() syncPreferences {
	if s.ItemPreferences != nil {
		return *s.ItemPreferences
	}
	return s.Preferences
}

// Read-only Fetch may race a window of Sync responses. Accept a bounded set of
// keys issued to this device and collection, but never use them for writebacks.
func (s collectionState) canFetch(key string) bool {
	if key == "" || key == "0" {
		return false
	}
	if key == s.Key || key == s.PreviousKey {
		return true
	}
	for _, old := range s.ReadKeys {
		if old == key {
			return true
		}
	}
	return false
}

func (s collectionState) nextReadKeys() []string {
	keys := append([]string(nil), s.ReadKeys...)
	if s.Key != "" && s.Key != "0" {
		keys = append(keys, s.Key)
	}
	if len(keys) > 32 {
		keys = keys[len(keys)-32:]
	}
	return keys
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
	if isReadOnlySyncFetch(request) {
		if !state.canFetch(key) {
			return syncStatus(id, 3), nil
		}
		preferences, err := preferencesFrom(request, state.Preferences)
		if err != nil || !validFilter(kind, preferences) {
			return syncStatus(id, 4), nil
		}
		fetches, err := h.fetchSyncItems(context.WithValue(ctx, preferencesKey{}, preferences), u, kind, id, request)
		if err != nil {
			return "", err
		}
		// Fetch does not acknowledge a Sync window or change its key/replay.
		// Keep sticky Options separately from the snapshot's representation.
		if state.ItemPreferences == nil {
			p := state.Preferences
			state.ItemPreferences = &p
		}
		state.Preferences = preferences
		out := fmt.Sprintf(`<Collection xmlns="AirSync:"><SyncKey>%s</SyncKey><CollectionId>%s</CollectionId><Status>1</Status>%s</Collection>`, xmlEscape(key), xmlEscape(id), renderWritebackResponses(fetches))
		return out, saveCollectionState(ctx, h.store, u.ID, dev.ID, id, state)
	}
	if key != "0" && key == state.PreviousKey && fingerprint == state.RequestHash && state.Response != "" {
		// Older builds cached successful responses that silently omitted Fetch.
		// Repair only the read response; never reapply Add/Change/Delete on retry.
		repaired, err := h.repairSyncFetchReplay(ctx, u, kind, id, request, state)
		if err != nil {
			return "", err
		}
		if repaired != state.Response {
			state.Response = repaired
			if err := saveCollectionState(ctx, h.store, u.ID, dev.ID, id, state); err != nil {
				return "", err
			}
		}
		return state.Response, nil
	}
	// Earlier builds advanced the key for a Fetch. Its previous key describes
	// the same snapshot; allow the next Sync to recover without a folder reset.
	legacyFetchKey := key != "" && key == state.PreviousKey && cachedFetchOnly(state.Response)
	if key != "0" && (key == "" || (key != state.Key && !legacyFetchKey)) {
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
	if kind == kindNotes {
		ctx = context.WithValue(ctx, noteStateKey{}, &noteWriteState{Snapshot: state, Written: map[string]storage.NoteItem{}})
	}
	ctx = context.WithValue(ctx, taskDeviceKey{}, dev.ID)
	writes := h.applyWritebacks(context.WithValue(ctx, deleteMovesKey{}, request.value("DeletesAsMoves") != "0"), u, kind, id, parseXMLClientOps(request.render()))
	fetches, err := h.fetchSyncItems(ctx, u, kind, id, request)
	if err != nil {
		return "", err
	}
	writes = append(writes, fetches...)
	if kind == kindTasks {
		// A successful Add response already supplies the server ID. Including
		// it again as a server Add would create a duplicate on some clients.
		acknowledgeTaskAdds(writes, &state)
	}
	if kind == kindNotes {
		if state.Items == nil {
			state.Items = map[string]string{}
		}
		// Acknowledge the exact revision written by this client, including send-only
		// Sync. Re-reading here could accidentally acknowledge a concurrent edit.
		for id, n := range ctx.Value(noteStateKey{}).(*noteWriteState).Written {
			hash, err := noteSnapshotHash(&n, state.itemPreferences())
			if err != nil {
				return "", err
			}
			state.Items[id] = hash
		}
	}
	if kind == kindCalendar {
		if state.Items == nil {
			state.Items = map[string]string{}
		}
		for _, w := range writes {
			if w.Kind == "add" && w.Status == 1 {
				// The Add response already maps the client item to its server ID.
				// Canonical scheduling metadata may follow as Change, never Add.
				state.Items[w.ServerID] = "calendar-created"
			}
		}
	}
	if request.value("GetChanges") == "0" {
		next := storage.NewID()
		out := fmt.Sprintf(`<Collection xmlns="AirSync:"><SyncKey>%s</SyncKey><CollectionId>%s</CollectionId><Status>1</Status>%s</Collection>`, next, xmlEscape(id), renderWritebackResponses(writes))
		p := state.itemPreferences()
		return out, saveCollectionState(ctx, h.store, u.ID, dev.ID, id, collectionState{Preferences: preferences, ItemPreferences: &p, ReadKeys: state.nextReadKeys(), Key: next, Items: state.Items, PreviousKey: key, RequestHash: fingerprint, Response: out})
	}
	var full string
	var taskHashes map[string]string
	switch kind {
	case kindTasks:
		full, taskHashes, err = h.taskSnapshot(ctx, u, id, "unused", writes)
	case kindCalendar:
		full, _, err = h.syncCalendar(ctx, u, id, "unused", false, writes)
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
	hashes, previous := snapshotItemHashes(kind, commands, preferences, state.Items, state.itemPreferences(), state.Preferences)
	if kind == kindTasks {
		hashes = taskHashes
	}
	if commands != nil {
		for _, add := range commands.Children {
			sid := add.value("ServerId")
			items[sid] = add
		}
	}
	window := 100
	if n, e := strconv.Atoi(request.value("WindowSize")); e == nil && n > 0 {
		window = n
	}
	if window > 512 {
		window = 512
	}
	ids, nextItems, more := diffItems(previous, hashes, window)
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
			if kind == kindTasks && preferences.Filter == 8 {
				if _, e := (&ewsHandler{store: h.store}).getTask(ctx, u, sid); e == nil {
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
	err = saveCollectionState(ctx, h.store, u.ID, dev.ID, id, collectionState{Preferences: preferences, ItemPreferences: &preferences, ReadKeys: state.nextReadKeys(), Key: nextKey, Items: nextItems, PreviousKey: key, RequestHash: fingerprint, Response: out})
	return out, err
}
