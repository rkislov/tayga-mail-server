package httpapi

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/tayga/tms/internal/xmpp"
)

func TestChatRoomOMEMOAPI(t *testing.T) {
	f := newFileFixture(t)
	f.s.cfg.Server.Hostname = "example.com"
	srv := xmpp.New(f.s.cfg, nil, f.s.store, f.s.authn, nil)
	f.s.SetXMPP(srv)
	room, e := srv.CreateChatRoom(context.Background(), "owner@example.com", "Encrypted", []string{"reader@example.com"})
	if e != nil {
		t.Fatal(e)
	}
	path := "/api/v1/chat/rooms/omemo"
	var audience xmpp.RoomOMEMOContext
	w := f.call(0, "GET", path+"?room="+url.QueryEscape(room.JID), "", 200)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header())
	}
	if e = json.Unmarshal(w.Body.Bytes(), &audience); e != nil {
		t.Fatal(e)
	}
	cipher := `<encrypted xmlns="eu.siacs.conversations.axolotl"><header sid="42"><key rid="7">YWJj</key><iv>AAAAAAAAAAAAAAAA</iv></header><payload>YWJj</payload></encrypted>`
	request := func(revision string) string {
		b, _ := json.Marshal(map[string]string{"room": room.JID, "revision": revision, "xml": cipher})
		return string(b)
	}
	f.call(0, "POST", path, request(audience.Revision), 200)
	f.call(0, "POST", path, request(""), 409)
	f.call(-1, "GET", path+"?room="+room.JID, "", 401)
	f.call(2, "GET", path+"?room="+room.JID, "", 403)
	f.call(2, "POST", path, request(audience.Revision), 403)
	if e = srv.UpdateChatRoom(context.Background(), "owner@example.com", room.JID, "reader@example.com", "none", false); e != nil {
		t.Fatal(e)
	}
	f.call(0, "POST", path, request(audience.Revision), 409)
	f.call(1, "POST", path, request(audience.Revision), 403)
}
