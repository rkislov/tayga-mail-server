// SPDX-License-Identifier: Apache-2.0
package flowsync

import "encoding/json"

// Mail snapshots are metadata-only. Their Body/Data is a digest of the stored
// file, size and flags, not a rendered preview. Presentation preferences and
// protocol additions must never turn unchanged mail into a folder-wide Change.
func snapshotItemHashes(kind collectionKind, commands *protocolNode, preferences syncPreferences, previous map[string]string, legacyPreferences ...syncPreferences) (map[string]string, map[string]string) {
	current := map[string]string{}
	acknowledged := map[string]string{}
	for id, hash := range previous {
		acknowledged[id] = hash
	}
	if commands == nil {
		return current, acknowledged
	}
	for _, item := range commands.Children {
		id := item.value("ServerId")
		if kind != kindMail {
			current[id] = digest(item.render() + preferences.hash())
			continue
		}
		data := item.child("ApplicationData")
		fields := []string{id, data.value("Subject"), data.value("From"), data.value("DateReceived"), data.value("Read"), data.find("Body").value("Data")}
		encoded, _ := json.Marshal(fields)
		current[id] = "mail-v1:" + digest(string(encoded))
		old := previous[id]
		if old == "" || old == current[id] {
			continue
		}
		// Upgrade only when an old fingerprint proves the item is unchanged.
		// Unsent items, real edits and deletions retain their pending changes.
		legacy := item.render()
		beforeNative := *item
		beforeData := *data
		beforeData.Children = nil
		for _, child := range data.Children {
			if child.Name.Local != "NativeBodyType" {
				beforeData.Children = append(beforeData.Children, child)
			}
		}
		beforeNative.Children = []*protocolNode{item.child("ServerId"), &beforeData}
		for _, prefs := range append(legacyPreferences, preferences) {
			if old == digest(legacy+prefs.hash()) || old == digest(beforeNative.render()+prefs.hash()) {
				acknowledged[id] = current[id]
				break
			}
		}
	}
	return current, acknowledged
}
