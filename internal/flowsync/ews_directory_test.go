// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
)

func TestEWSDirectoryIsolationAndRooms(t *testing.T) {
	ctx, st, u, _, _, _ := syncFixture(t)
	h := &ewsHandler{store: st}
	run := func(op, body string) string {
		t.Helper()
		out, e := h.directoryEWS(ctx, u, op, "<"+op+">"+body+"</"+op+">")
		if e != nil {
			t.Fatal(e)
		}
		requireXML(t, out)
		return out
	}
	if err := st.PutSetting(ctx, "domain.gal."+u.DomainID, "true"); err != nil {
		t.Fatal(err)
	}
	peer, err := st.CreateUser(ctx, &storage.User{DomainID: u.DomainID, TenantID: u.TenantID, LocalPart: "slava", Email: "slava@example.com", DisplayName: "Вячеслав Андреенко", Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateDomain(ctx, u.TenantID, "other.example")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateUser(ctx, &storage.User{DomainID: other.ID, TenantID: u.TenantID, LocalPart: "slava", Email: "slava@other.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := st.CreateCalendarResource(ctx, &storage.CalendarResource{DomainID: u.DomainID, LocalPart: "room201", DisplayName: "Комната 201", Kind: "room"})
	if err != nil {
		t.Fatal(err)
	}
	out := run("ResolveNames", "<UnresolvedEntry>slava</UnresolvedEntry>")
	if !strings.Contains(out, peer.Email) || strings.Contains(out, "slava@other.example") {
		t.Fatal(out)
	}
	out = run("ResolveNames", "<UnresolvedEntry>Вячеслав</UnresolvedEntry>")
	if !strings.Contains(out, peer.Email) {
		t.Fatal(out)
	}
	out = run("ResolveNames", "<UnresolvedEntry>smtp:room201@example.com</UnresolvedEntry>")
	if !strings.Contains(out, room.Email) {
		t.Fatal(out)
	}
	for _, scope := range []string{"ActiveDirectory", "Contacts"} {
		folders := ""
		if scope == "Contacts" {
			folders = `<ParentFolderIds><FolderId Id="` + galFolderID(u) + `"/></ParentFolderIds>`
		}
		out, err = h.directoryEWS(ctx, u, "ResolveNames", `<ResolveNames SearchScope="`+scope+`" ReturnFullContactData="true">`+folders+`<UnresolvedEntry>roo</UnresolvedEntry></ResolveNames>`)
		if err != nil || !strings.Contains(out, room.Email) || !strings.Contains(out, `PropertyTag="0x3905"`) || !strings.Contains(out, "<t:Value>7</t:Value>") {
			t.Fatal(scope, out, err)
		}
		requireXML(t, out)
	}
	lists := run("GetRoomLists", "")
	id := extractTag(lists, "EmailAddress")
	if id == "" {
		t.Fatal(lists)
	}
	out = run("GetRooms", "<RoomList><EmailAddress>"+id+"</EmailAddress></RoomList>")
	if !strings.Contains(out, room.Email) {
		t.Fatal(out)
	}
	out = run("GetRooms", "<RoomList><EmailAddress>rooms@other.example</EmailAddress></RoomList>")
	if !strings.Contains(out, "ErrorNameResolutionNoResults") {
		t.Fatal(out)
	}
	if err = st.PutSetting(ctx, "domain.gal."+u.DomainID, "false"); err != nil {
		t.Fatal(err)
	}
	if out = run("ResolveNames", "<UnresolvedEntry>slava</UnresolvedEntry>"); !strings.Contains(out, "ErrorNameResolutionNoResults") {
		t.Fatal(out)
	}
	if out = run("GetRoomLists", ""); strings.Contains(out, id) {
		t.Fatal(out)
	}
}
