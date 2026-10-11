// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestEWSStreamingSubscriptionChangesAndOwnership(t *testing.T) {
	ctx, st, u, eas, _, ab := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	request := `<Subscribe><StreamingSubscriptionRequest SubscribeToAllFolders="true"><EventTypes><EventType>CreatedEvent</EventType><EventType>ModifiedEvent</EventType><EventType>DeletedEvent</EventType></EventTypes></StreamingSubscriptionRequest></Subscribe>`
	response, err := h.subscribeEWS(ctx, u, request)
	if err != nil || !strings.Contains(response, "NoError") {
		t.Fatalf("subscribe: %s %v", response, err)
	}
	requireXML(t, response)
	id := extractTag(response, "SubscriptionId")
	if id == "" {
		t.Fatal("missing subscription ID")
	}
	get := `<GetStreamingEvents><SubscriptionIds><SubscriptionId>` + id + `</SubscriptionId></SubscriptionIds><ConnectionTimeout>1</ConnectionTimeout></GetStreamingEvents>`
	other := *u
	other.ID = "other"
	response, err = h.streamingEventsEWS(ctx, &other, get)
	if err != nil || !strings.Contains(response, "ErrorInvalidSubscription") {
		t.Fatalf("cross-user subscription: %s %v", response, err)
	}
	card, err := st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: ab, UID: "notification", HrefName: "notification.vcf", Data: "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Notification\r\nEND:VCARD\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	response, err = h.streamingEventsEWS(ctx, u, get)
	if err != nil || !strings.Contains(response, "CreatedEvent") || !strings.Contains(response, card.ID) {
		t.Fatalf("new contact notification: %s %v", response, err)
	}
	requireXML(t, response)
	if !strings.Contains(response, `<m:ConnectionStatus>Closed</m:ConnectionStatus>`) {
		t.Fatal("missing stream closure")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = h.streamingEventsEWS(cancelled, u, get); err == nil {
		t.Fatal("ignored cancellation or replayed already delivered events")
	}
	if got := h.unsubscribeEWS(&other, `<SubscriptionId>`+id+`</SubscriptionId>`); !strings.Contains(got, "ErrorInvalidSubscription") {
		t.Fatal("foreign unsubscribe accepted")
	}
	if got := h.unsubscribeEWS(u, `<SubscriptionId>`+id+`</SubscriptionId>`); !strings.Contains(got, "NoError") {
		t.Fatal(got)
	}
	response, _ = h.streamingEventsEWS(ctx, u, get)
	if !strings.Contains(response, "ErrorInvalidSubscription") {
		t.Fatal("unsubscribed ID remains valid")
	}
}

func TestEWSNotificationWindowAndMove(t *testing.T) {
	s := &ewsSubscription{events: map[string]bool{"CreatedEvent": true, "MovedEvent": true, "DeletedEvent": true}, objects: map[string]ewsNotificationObject{}}
	current := map[string]ewsNotificationObject{}
	for i := 0; i < 60; i++ {
		current[fmt.Sprint(i)] = ewsNotificationObject{parent: "inbox", hash: "1"}
	}
	first := notificationChanges(s, current)
	if strings.Count(first, "<t:CreatedEvent>") != 50 {
		t.Fatal("missing event window")
	}
	second := notificationChanges(s, current)
	if strings.Count(second, "<t:CreatedEvent>") != 10 {
		t.Fatal("lost pending events")
	}
	if notificationChanges(s, current) != "" {
		t.Fatal("repeated unchanged events")
	}
	current["1"] = ewsNotificationObject{parent: "archive", hash: "1"}
	delete(current, "2")
	changed := notificationChanges(s, current)
	if !strings.Contains(changed, "MovedEvent") || !strings.Contains(changed, `<t:OldParentFolderId Id="inbox"/>`) || !strings.Contains(changed, "DeletedEvent") {
		t.Fatal(changed)
	}
}

func TestEWSHierarchyParentsAndMessageCounts(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	parent, _ := st.EnsureMailbox(ctx, u.ID, "Archive", "")
	child, _ := st.EnsureMailbox(ctx, u.ID, "Archive/2026", "")
	_, err := st.InsertMessage(ctx, &storage.Message{MailboxID: child.ID, UID: 1, FilePath: "unused", Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	response, err := h.getFolder(ctx, u, `<GetFolder><FolderIds><FolderId Id="`+child.ID+`"/></FolderIds></GetFolder>`)
	if err != nil || !strings.Contains(response, `ParentFolderId Id="`+parent.ID+`"`) || !strings.Contains(response, `<t:TotalCount>1</t:TotalCount>`) || !strings.Contains(response, `<t:UnreadCount>1</t:UnreadCount>`) {
		t.Fatalf("invalid hierarchy: %s %v", response, err)
	}
}
