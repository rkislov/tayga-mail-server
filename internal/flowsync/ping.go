// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strconv"
	"strings"
	"time"
)

type pingSubscription struct {
	Heartbeat int
	Folders   []string
}

func (h *easHandler) collectionSnapshot(ctx context.Context, u *storage.User, id string) (map[string]string, error) {
	kind, err := h.resolveCollection(ctx, u.ID, id)
	if err != nil {
		return nil, err
	}
	var full string
	switch kind {
	case kindCalendar:
		full, _, err = h.syncCalendar(ctx, id, "snapshot", false, nil)
	case kindContacts:
		full, _, err = h.syncContacts(ctx, id, "snapshot", false, nil)
	case kindNotes:
		full, _, err = h.syncNotes(ctx, u, id, "snapshot", false, nil)
	default:
		full, _, err = h.syncMailCollection(context.WithValue(ctx, mailSnapshotKey{}, true), u, id, "snapshot", false, nil)
	}
	if err != nil {
		return nil, err
	}
	doc, err := parseProtocolXML([]byte(full))
	if err != nil {
		return nil, err
	}
	hashes := map[string]string{}
	if commands := doc.find("Commands"); commands != nil {
		for _, item := range commands.Children {
			hashes[item.value("ServerId")] = digest(item.render() + contextPreferences(ctx).hash())
		}
	}
	return hashes, nil
}

func (h *easHandler) ping(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, body []byte) (string, error) {
	status := func(n int) string { return fmt.Sprintf(`<Ping xmlns="Ping:"><Status>%d</Status></Ping>`, n) }
	raw, err := h.store.GetFlowSyncState(ctx, u.ID, dev.ID, "ping")
	if err != nil {
		return "", err
	}
	var sub pingSubscription
	if raw != "" {
		if err = json.Unmarshal([]byte(raw), &sub); err != nil {
			return "", err
		}
	}
	if len(body) > 0 {
		doc, err := parseProtocolXML(body)
		if err != nil {
			return status(4), nil
		}
		root := doc.find("Ping")
		if root == nil {
			return status(4), nil
		}
		if heartbeat := root.child("HeartbeatInterval"); heartbeat != nil {
			sub.Heartbeat, err = strconv.Atoi(heartbeat.Text)
			if err != nil {
				return status(4), nil
			}
		}
		if folders := root.child("Folders"); folders != nil {
			sub.Folders = nil
			for _, f := range folders.Children {
				if f.Name.Local == "Folder" {
					sub.Folders = append(sub.Folders, f.value("Id"))
				}
			}
		}
	}
	if sub.Heartbeat == 0 || len(sub.Folders) == 0 {
		return status(3), nil
	}
	if sub.Heartbeat < 60 || sub.Heartbeat > 900 {
		bound := 60
		if sub.Heartbeat > 900 {
			bound = 900
		}
		return fmt.Sprintf(`<Ping xmlns="Ping:"><Status>5</Status><HeartbeatInterval>%d</HeartbeatInterval></Ping>`, bound), nil
	}
	if len(sub.Folders) > 100 {
		return `<Ping xmlns="Ping:"><Status>6</Status><MaxFolders>100</MaxFolders></Ping>`, nil
	}
	for _, id := range sub.Folders {
		if _, err = h.resolveCollection(ctx, u.ID, id); err != nil {
			return status(7), nil
		}
	}
	encoded, _ := json.Marshal(sub)
	if err = h.store.PutFlowSyncState(ctx, u.ID, dev.ID, "ping", string(encoded)); err != nil {
		return "", err
	}
	deadline := time.NewTimer(time.Duration(sub.Heartbeat) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		var changed []string
		for _, id := range sub.Folders {
			state, err := loadCollectionState(ctx, h.store, u.ID, dev.ID, id)
			if err != nil {
				return "", err
			}
			snapshot, err := h.collectionSnapshot(context.WithValue(ctx, preferencesKey{}, state.Preferences), u, id)
			if err != nil {
				return status(7), nil
			}
			ids, _, _ := diffItems(state.Items, snapshot, 1)
			if len(ids) > 0 {
				changed = append(changed, id)
			}
		}
		if len(changed) > 0 {
			var b strings.Builder
			b.WriteString(`<Ping xmlns="Ping:"><Status>2</Status><Folders>`)
			for _, id := range changed {
				fmt.Fprintf(&b, `<Folder>%s</Folder>`, xmlEscape(id))
			}
			b.WriteString(`</Folders></Ping>`)
			return b.String(), nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return status(1), nil
		case <-ticker.C:
		}
	}
}
