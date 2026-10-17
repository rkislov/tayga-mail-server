// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
)

func TestGALFolderSyncLifecycleAndIsolation(t *testing.T) {
	ctx, st, u, _, _, _ := syncFixture(t)
	h := &ewsHandler{store: st}
	check := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		requireXML(t, out)
		return out
	}
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	peer, err := st.CreateUser(ctx, &storage.User{DomainID: u.DomainID, TenantID: u.TenantID, LocalPart: "peer", Email: "peer@example.com", DisplayName: "Коллега", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateDomain(ctx, u.TenantID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := st.CreateUser(ctx, &storage.User{DomainID: other.ID, TenantID: u.TenantID, LocalPart: "foreign", Email: "foreign@other.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: u.DomainID, LocalPart: "room201", DisplayName: "Комната 201", Kind: "room"})
	if err != nil {
		t.Fatal(err)
	}
	hierarchy := check(h.syncEWSHierarchy(ctx, u, "<SyncFolderHierarchy/>"))
	if !strings.Contains(hierarchy, galFolderID(u)) || !strings.Contains(hierarchy, "Общий адресный список") {
		t.Fatal(hierarchy)
	}
	hierarchyToken := extractTag(hierarchy, "SyncState")
	folder := `<FolderId Id="` + galFolderID(u) + `"/>`
	if out := check(h.getFolder(ctx, u, "<GetFolder>"+folder+"</GetFolder>")); !strings.Contains(out, "<t:Modify>false</t:Modify>") {
		t.Fatal(out)
	}
	items := check(h.findItem(ctx, u, "<FindItem>"+folder+"</FindItem>"))
	if !strings.Contains(items, peer.Email) || !strings.Contains(items, room.Email) || strings.Contains(items, foreign.Email) {
		t.Fatal(items)
	}
	itemID := galItemPrefix(u) + peer.ID
	if out := check(h.getItem(ctx, u, `<GetItem><ItemId Id="`+itemID+`"/></GetItem>`)); !strings.Contains(out, peer.Email) {
		t.Fatal(out)
	}
	if out := check(h.getItem(ctx, u, `<GetItem><ItemId Id="`+galItemPrefix(u)+foreign.ID+`"/></GetItem>`)); !strings.Contains(out, "ErrorItemNotFound") {
		t.Fatal(out)
	}
	if out := check(h.getItem(ctx, peer, `<GetItem><ItemId Id="`+itemID+`"/></GetItem>`)); !strings.Contains(out, "ErrorItemNotFound") {
		t.Fatal(out)
	}
	page := check(h.findItem(ctx, u, `<FindItem>`+folder+`<IndexedPageItemView Offset="0" MaxEntriesReturned="1" BasePoint="Beginning"/></FindItem>`))
	if strings.Count(page, "<t:Contact>") != 1 || !strings.Contains(page, `IncludesLastItemInRange="false"`) {
		t.Fatal(page)
	}
	filtered := check(h.findItem(ctx, u, `<FindItem>`+folder+`<Restriction><Contains ContainmentMode="Substring" ContainmentComparison="IgnoreCase"><FieldURI FieldURI="contacts:DisplayName"/><Constant Value="КОЛЛЕГА"/></Contains></Restriction></FindItem>`))
	if strings.Count(filtered, "<t:Contact>") != 1 || !strings.Contains(filtered, peer.Email) {
		t.Fatal(filtered)
	}
	syncReq := func(token string) string {
		return "<SyncFolderItems>" + folder + "<SyncState>" + token + "</SyncState></SyncFolderItems>"
	}
	first := check(h.syncEWSItems(ctx, u, syncReq("")))
	if !strings.Contains(first, peer.Email) {
		t.Fatal(first)
	}
	second := check(h.syncEWSItems(ctx, u, syncReq(extractTag(first, "SyncState"))))
	if strings.Contains(second, "<t:Create>") || strings.Contains(second, "<t:Update>") {
		t.Fatal(second)
	}
	if err := st.UpdateUserProfile(ctx, peer.ID, "Новое имя"); err != nil {
		t.Fatal(err)
	}
	third := check(h.syncEWSItems(ctx, u, syncReq(extractTag(second, "SyncState"))))
	if !strings.Contains(third, "<t:Update>") || !strings.Contains(third, "Новое имя") {
		t.Fatal(third)
	}
	if err := st.UpdateUserEnabled(ctx, peer.ID, false); err != nil {
		t.Fatal(err)
	}
	fourth := check(h.syncEWSItems(ctx, u, syncReq(extractTag(third, "SyncState"))))
	if !strings.Contains(fourth, `<t:Delete><t:ItemId Id="`+itemID) {
		t.Fatal(fourth)
	}
	for _, op := range []string{"UpdateItem", "DeleteItem", "CreateItem"} {
		body := "<" + op + ">" + folder + `<ItemId Id="` + itemID + `"/><Contact><DisplayName>Changed</DisplayName></Contact></` + op + ">"
		var out string
		var e error
		switch op {
		case "UpdateItem":
			out, e = h.updateItem(ctx, u, body)
		case "DeleteItem":
			out, e = h.deleteItem(ctx, u, body)
		case "CreateItem":
			out, e = h.createItem(ctx, u, body)
		}
		if out = check(out, e); !strings.Contains(out, "ErrorAccessDenied") {
			t.Fatal(out)
		}
	}
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "false"); err != nil {
		t.Fatal(err)
	}
	deleted := check(h.syncEWSHierarchy(ctx, u, "<SyncFolderHierarchy><SyncState>"+hierarchyToken+"</SyncState></SyncFolderHierarchy>"))
	if !strings.Contains(deleted, `<t:Delete><t:FolderId Id="`+galFolderID(u)) {
		t.Fatal(deleted)
	}
	if out := check(h.syncEWSItems(ctx, u, syncReq(extractTag(fourth, "SyncState")))); !strings.Contains(out, "ErrorAccessDenied") {
		t.Fatal(out)
	}
}

func TestGALStreamingSubscription(t *testing.T) {
	ctx, st, u, _, _, _ := syncFixture(t)
	h := &ewsHandler{store: st}
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	body := `<Subscribe><StreamingSubscriptionRequest><FolderIds><FolderId Id="` + galFolderID(u) + `"/></FolderIds><EventTypes><EventType>ModifiedEvent</EventType></EventTypes></StreamingSubscriptionRequest></Subscribe>`
	out, err := h.subscribeEWS(ctx, u, body)
	if err != nil || !strings.Contains(out, "NoError") {
		t.Fatal(out, err)
	}
	id := extractTag(out, "SubscriptionId")
	before := h.subs[id].objects
	if err := st.UpdateUserProfile(ctx, u.ID, "Updated"); err != nil {
		t.Fatal(err)
	}
	after, err := h.notificationSnapshot(ctx, u, h.subs[id])
	if err != nil {
		t.Fatal(err)
	}
	key := galItemPrefix(u) + u.ID
	if before[key].hash == after[key].hash {
		t.Fatal("directory profile update is not notified")
	}
}
