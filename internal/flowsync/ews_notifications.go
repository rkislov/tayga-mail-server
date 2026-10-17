// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.
package flowsync

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// Subscriptions are ephemeral, like Exchange streaming subscriptions. After a
// restart the client must subscribe again; durable item sync state is separate.
type ewsSubscription struct {
	mu      sync.Mutex
	user    string
	expires time.Time // guarded by ewsHandler.subsMu
	all     bool
	folders map[string]bool
	events  map[string]bool
	objects map[string]ewsNotificationObject
}
type ewsNotificationObject struct {
	parent, hash  string
	folder, inbox bool
}

func ewsNotificationSuccess(op, content string) string {
	return fmt.Sprintf(`<m:%sResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:%sResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode>%s</m:%sResponseMessage></m:ResponseMessages></m:%sResponse>`, op, op, content, op, op)
}

func (h *ewsHandler) subscribeEWS(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return ewsOperationError("Subscribe", "ErrorInvalidRequest", "Invalid XML"), nil
	}
	req := doc.find("StreamingSubscriptionRequest")
	if req == nil {
		return ewsOperationError("Subscribe", "ErrorInvalidRequest", "Only streaming subscriptions are supported"), nil
	}
	s := &ewsSubscription{user: u.ID, folders: map[string]bool{}, events: map[string]bool{}}
	for _, a := range req.Attr {
		if a.Name.Local == "SubscribeToAllFolders" && (a.Value == "true" || a.Value == "1") {
			s.all = true
		}
	}
	if events := req.child("EventTypes"); events != nil {
		for _, e := range events.Children {
			switch e.Text {
			case "CreatedEvent", "DeletedEvent", "ModifiedEvent", "MovedEvent", "CopiedEvent", "NewMailEvent":
				s.events[e.Text] = true
			default:
				return ewsOperationError("Subscribe", "ErrorInvalidEvent", "Unsupported event type"), nil
			}
		}
	}
	if len(s.events) == 0 {
		return ewsOperationError("Subscribe", "ErrorInvalidRequest", "Event types required"), nil
	}
	if folders := req.child("FolderIds"); folders != nil {
		if len(folders.Children) > 100 {
			return ewsOperationError("Subscribe", "ErrorInvalidRequest", "Too many folders"), nil
		}
		for _, f := range folders.Children {
			id, e := h.resolveEWSFolder(ctx, u, f.render())
			if e != nil {
				return ewsOperationError("Subscribe", "ErrorFolderNotFound", "Folder unavailable"), nil
			}
			if e = h.readableEWSCollection(ctx, u, id); e != nil && systemFolderName(u, id) != "tasks" {
				return ewsOperationError("Subscribe", "ErrorAccessDenied", "Folder unavailable"), nil
			}
			s.folders[id] = true
		}
	}
	if !s.all && len(s.folders) == 0 {
		return ewsOperationError("Subscribe", "ErrorInvalidRequest", "Folder scope required"), nil
	}
	s.objects, err = h.notificationSnapshot(ctx, u, s)
	if err != nil {
		return "", err
	}
	h.subsMu.Lock()
	defer h.subsMu.Unlock()
	if h.subs == nil {
		h.subs = map[string]*ewsSubscription{}
	}
	count := 0
	for id, existing := range h.subs {
		if time.Now().After(existing.expires) {
			delete(h.subs, id)
		} else if existing.user == u.ID {
			count++
		}
	}
	if count >= 20 || len(h.subs) >= 2000 {
		return ewsOperationError("Subscribe", "ErrorExceededSubscriptionCount", "Subscription limit reached"), nil
	}
	id := storage.NewID()
	s.expires = time.Now().Add(30 * time.Minute)
	h.subs[id] = s
	return ewsNotificationSuccess("Subscribe", `<m:SubscriptionId>`+xmlEscape(id)+`</m:SubscriptionId>`), nil
}

func (h *ewsHandler) unsubscribeEWS(u *storage.User, body string) string {
	id := extractTag(body, "SubscriptionId")
	h.subsMu.Lock()
	defer h.subsMu.Unlock()
	s := h.subs[id]
	if s == nil || s.user != u.ID {
		return ewsOperationError("Unsubscribe", "ErrorInvalidSubscription", "Subscription unavailable")
	}
	delete(h.subs, id)
	return ewsNotificationSuccess("Unsubscribe", "")
}

// Only IDs and change digests are retained. Mail bodies and attachments are never
// read to detect notifications. Folder enumeration rechecks the user's access.
func (h *ewsHandler) notificationSnapshot(ctx context.Context, u *storage.User, s *ewsSubscription) (map[string]ewsNotificationObject, error) {
	out := map[string]ewsNotificationObject{}
	listing, err := h.findFolder(ctx, u)
	if err != nil {
		return nil, err
	}
	doc, err := parseProtocolXML([]byte(listing))
	if err != nil {
		return nil, err
	}
	for _, f := range doc.find("Folders").Children {
		id := extractAttr(f.render(), "FolderId", "Id")
		if !s.all && !s.folders[id] {
			continue
		}
		out[id] = ewsNotificationObject{parent: extractAttr(f.render(), "ParentFolderId", "Id"), hash: digest(f.render()), folder: true}
		add := func(item, hash string) {
			out[item] = ewsNotificationObject{parent: id, hash: hash, inbox: f.value("DisplayName") == "INBOX"}
		}
		switch f.value("FolderClass") {
		case "IPF.Appointment":
			items, e := h.store.ListCalendarObjects(ctx, id)
			if e != nil {
				return nil, e
			}
			for _, v := range items {
				add(v.ID, v.ETag)
			}
		case "IPF.Contact":
			var items []*storage.AddressObject
			var e error
			if id == galFolderID(u) {
				items, e = h.galContacts(ctx, u)
			} else {
				items, e = h.store.ListAddressObjects(ctx, id)
			}
			if e != nil {
				return nil, e
			}
			for _, v := range items {
				add(v.ID, v.ETag)
			}
		case "IPF.Task":
			if systemFolderName(u, id) == "tasks" {
				tasks, e := h.store.ListTasks(ctx, u.ID)
				if e != nil {
					return nil, e
				}
				for _, task := range tasks {
					add(task.ID, digest(taskXML(task, u.ID)))
				}
			}
		case "IPF.StickyNote":
			items, e := h.store.ListNoteItemsInFolder(ctx, id, false)
			if e != nil {
				return nil, e
			}
			for _, v := range items {
				add(v.ID, v.ETag)
			}
		default:
			items, e := h.store.ListMessages(ctx, id)
			if e != nil {
				return nil, e
			}
			for _, v := range items {
				metadata, e := h.store.EWSMailMetadata(ctx, v.ID)
				if e != nil {
					return nil, e
				}
				add(v.ID, digest(fmt.Sprintf("%s/%d/%s/%s", v.Flags, v.Size, v.FilePath, metadata)))
			}
		}
	}
	return out, nil
}

func notificationChanges(s *ewsSubscription, current map[string]ewsNotificationObject) string {
	ids := make([]string, 0, len(current)+len(s.objects))
	for id, v := range current {
		if old, ok := s.objects[id]; !ok || old != v {
			ids = append(ids, id)
		}
	}
	for id := range s.objects {
		if _, ok := current[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var b strings.Builder
	count := 0
	for _, id := range ids {
		old, had := s.objects[id]
		v, exists := current[id]
		kind := "ModifiedEvent"
		if !exists {
			kind = "DeletedEvent"
			v = old
		} else if !had {
			kind = "CreatedEvent"
		} else if old.parent != v.parent {
			kind = "MovedEvent"
		}
		kinds := []string{kind}
		if kind == "CreatedEvent" && v.inbox && !v.folder {
			kinds = append(kinds, "NewMailEvent")
		}
		for _, event := range kinds {
			if !s.events[event] {
				continue
			}
			tag := "ItemId"
			if v.folder {
				tag = "FolderId"
			}
			fmt.Fprintf(&b, `<t:%s><t:TimeStamp>%s</t:TimeStamp><t:%s Id="%s"/><t:ParentFolderId Id="%s"/>`, event, time.Now().UTC().Format(time.RFC3339), tag, xmlEscape(id), xmlEscape(v.parent))
			if event == "MovedEvent" {
				fmt.Fprintf(&b, `<t:Old%s Id="%s"/><t:OldParentFolderId Id="%s"/>`, tag, xmlEscape(id), xmlEscape(old.parent))
			}
			fmt.Fprintf(&b, `</t:%s>`, event)
			count++
		}
		if exists {
			s.objects[id] = v
		} else {
			delete(s.objects, id)
		}
		if count >= 50 {
			break
		}
	}
	return b.String()
}

func (h *ewsHandler) streamingEventsEWS(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	ids := doc.find("SubscriptionIds")
	timeout, e := strconv.Atoi(extractTag(body, "ConnectionTimeout"))
	if ids == nil || len(ids.Children) == 0 || len(ids.Children) > 20 || e != nil || timeout < 1 || timeout > 30 {
		return ewsOperationError("GetStreamingEvents", "ErrorInvalidRequest", "Invalid subscription IDs or timeout"), nil
	}
	// Close and let the client reconnect within the HTTP server's write timeout.
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		var notifications strings.Builder
		for _, idNode := range ids.Children {
			id := strings.TrimSpace(idNode.Text)
			h.subsMu.Lock()
			s := h.subs[id]
			valid := s != nil && s.user == u.ID && time.Now().Before(s.expires)
			if valid {
				s.expires = time.Now().Add(30 * time.Minute)
			}
			h.subsMu.Unlock()
			if !valid {
				return ewsOperationError("GetStreamingEvents", "ErrorInvalidSubscription", "Subscription unavailable"), nil
			}
			s.mu.Lock()
			current, e := h.notificationSnapshot(ctx, u, s)
			changes := ""
			if e == nil {
				changes = notificationChanges(s, current)
			}
			s.mu.Unlock()
			if e != nil {
				return "", e
			}
			if changes != "" {
				fmt.Fprintf(&notifications, `<m:Notification><t:SubscriptionId>%s</t:SubscriptionId>%s</m:Notification>`, xmlEscape(id), changes)
			}
		}
		if notifications.Len() > 0 {
			return ewsNotificationSuccess("GetStreamingEvents", `<m:Notifications>`+notifications.String()+`</m:Notifications><m:ConnectionStatus>Closed</m:ConnectionStatus>`), nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return ewsNotificationSuccess("GetStreamingEvents", `<m:ConnectionStatus>Closed</m:ConnectionStatus>`), nil
		case <-ticker.C:
		}
	}
}
