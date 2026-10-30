package httpapi

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/tayga/tms/internal/xmpp"
)

func TestChatRoomArchiveAPI(t *testing.T) {
	f := newFileFixture(t)
	f.s.cfg.Server.Hostname = "example.com"
	srv := xmpp.New(f.s.cfg, nil, f.s.store, f.s.authn, nil)
	f.s.SetXMPP(srv)
	ctx := context.Background()
	room, err := srv.CreateChatRoom(ctx, "owner@example.com", "Fixture", []string{"reader@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"first", "middle", "last"} {
		if _, err = srv.SendChatMessage(ctx, "owner@example.com", room.JID, body); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v1/chat/rooms/history?room=" + url.QueryEscape(room.JID) + "&limit=2"
	var page xmpp.RoomArchivePage
	w := f.call(1, "GET", path, "", 200)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Header())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.Complete || len(page.Messages) != 2 || page.Messages[0].Body != "middle" {
		t.Fatal(page)
	}
	w = f.call(1, "GET", path+"&before="+url.QueryEscape(page.First), "", 200)
	if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if !page.Complete || len(page.Messages) != 1 || page.Messages[0].Body != "first" {
		t.Fatal(page)
	}
	f.call(2, "GET", path, "", 403)
	f.call(-1, "GET", path, "", 401)
	f.call(1, "GET", path+"&before=missing", "", 404)
	f.call(1, "GET", "/api/v1/chat/rooms/history?room="+room.JID+"&limit=999", "", 400)
	if err = srv.UpdateChatRoom(ctx, "owner@example.com", room.JID, "reader@example.com", "none", false); err != nil {
		t.Fatal(err)
	}
	f.call(1, "GET", path, "", 403)
}
