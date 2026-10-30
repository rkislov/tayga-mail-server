package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestRoomBookmarksSyncPrivacyAndUserChoices(t *testing.T) {
	srv, st, alice := testServer(t)
	testRoomBookmarks(t, srv, st, alice)
}
func testRoomBookmarks(t *testing.T, srv *Server, st *storage.Store, alice *storage.User) {
	t.Helper()
	ctx := context.Background()
	bob, err := st.CreateUser(ctx, &storage.User{Email: "bookmarked@example.com", LocalPart: "bookmarked", DomainID: alice.DomainID, TenantID: alice.TenantID, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	var own, foreign bytes.Buffer
	phone := &Session{srv: srv, user: alice, JID: ParseJID(alice.Email + "/phone"), bw: bufio.NewWriter(&own), pepInterest: map[string]bool{bookmarksNode: true}}
	observer := &Session{srv: srv, user: bob, JID: ParseJID(bob.Email + "/phone"), bw: bufio.NewWriter(&foreign), pepInterest: map[string]bool{bookmarksNode: true}}
	srv.hub.Register(phone)
	srv.hub.Register(observer)
	if err = srv.upsertRoster(ctx, alice.ID, rosterItem{JID: bob.Email, Subscription: "both", GroupsJSON: "[]"}); err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, alice.Email, "Группа & команда", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(own.String(), bookmarksNode) || !strings.Contains(own.String(), room.JID) {
		t.Fatal("creator's other device not notified", own.String())
	}
	list := func(email string) []pepItem {
		t.Helper()
		it, e := srv.getPEPItems(ctx, email, bookmarksNode)
		if e != nil {
			t.Fatal(e)
		}
		return it
	}
	if len(list(alice.Email)) != 1 || len(list(bob.Email)) != 1 {
		t.Fatal("members not bookmarked")
	}
	call := func(typ, owner, xml string) string {
		t.Helper()
		own.Reset()
		if e := phone.handlePubSubIQ(ctx, "bm", typ, owner, xml); e != nil {
			t.Fatal(e)
		}
		return own.String()
	}
	get := `<pubsub xmlns='` + pepNS + `'><items node='` + bookmarksNode + `'/></pubsub>`
	if got := call("get", bob.Email, get); !strings.Contains(got, "forbidden") {
		t.Fatal("foreign bookmarks readable", got)
	}
	foreign.Reset()
	observer.sendLastPEP(ctx, alice.Email, bookmarksNode)
	if foreign.Len() != 0 {
		t.Fatal("private bookmarks leaked through presence", foreign.String())
	}
	publication := func(id, payload, access string) string {
		return `<pubsub xmlns='` + pepNS + `'><publish node='` + bookmarksNode + `'><item id='` + id + `'>` + payload + `</item></publish><publish-options><x xmlns='jabber:x:data' type='submit'><field var='pubsub#access_model'><value>` + access + `</value></field><field var='pubsub#max_items'><value>max</value></field></x></publish-options></pubsub>`
	}
	custom := `<conference xmlns='` + bookmarksNode + `' autojoin='false' name='My name'><nick>Custom nick</nick><extensions><pinned xmlns='urn:test'/></extensions></conference>`
	foreign.Reset()
	if got := call("set", "", publication(room.JID, custom, "whitelist")); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if foreign.Len() != 0 {
		t.Fatal("private publication leaked to contact", foreign.String())
	}
	if got := call("get", "", get); !strings.Contains(got, "Custom nick") || !strings.Contains(got, "urn:test") {
		t.Fatal("preference lost", got)
	}
	if got := call("set", "", publication(room.JID, custom, "open")); !strings.Contains(got, "not-acceptable") {
		t.Fatal("public bookmark allowed", got)
	}
	external := "team@conference.other.example"
	if got := call("set", "", publication(external, custom, "whitelist")); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if len(list(alice.Email)) != 2 {
		t.Fatal("publishing replaced other bookmark")
	}
	if got := call("set", "", `<pubsub xmlns='`+pepNS+`'><retract node='`+bookmarksNode+`'><item id='`+room.JID+`'/></retract></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	call("get", "", get)
	if len(list(alice.Email)) != 1 {
		t.Fatal("deleted bookmark resurrected")
	}
	// Upgrade/backfill: an old membership without any bookmark is seeded once.
	if _, err = srv.db.ExecContext(ctx, srv.rebind(`UPDATE web_chat_members SET bookmark_seeded=FALSE WHERE room_jid=? AND email=?`), room.JID, alice.Email); err != nil {
		t.Fatal(err)
	}
	if got := call("get", "", get); !strings.Contains(got, room.JID) {
		t.Fatal("old room not recovered", got)
	}
	// Removal and re-add are distinct memberships; removals reach every own device.
	foreign.Reset()
	if err = srv.LeaveChatRoom(ctx, bob.Email, room.JID); err != nil {
		t.Fatal(err)
	}
	if len(list(bob.Email)) != 0 || !strings.Contains(foreign.String(), "<retract") {
		t.Fatal("removed membership retained", foreign.String())
	}
	if err = srv.UpdateChatRoom(ctx, alice.Email, room.JID, bob.Email, "member", false); err != nil {
		t.Fatal(err)
	}
	if len(list(bob.Email)) != 1 {
		t.Fatal("new membership not seeded")
	}
	if err = srv.UpdateChatRoom(ctx, alice.Email, room.JID, "", "", true); err != nil {
		t.Fatal(err)
	}
	if len(list(bob.Email)) != 0 || len(list(alice.Email)) != 1 {
		t.Fatal("destroy affected external bookmark or retained local room")
	}
}

func TestSelfNotesHistorySurvivesRosterNormalization(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	if err := srv.UpsertChatRoster(ctx, u.ID, u.Email, u.Email); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.SendChatMessage(ctx, u.Email, u.Email, "private note"); err != nil {
		t.Fatal(err)
	}
	roster, err := srv.ListChatRoster(ctx, u.ID)
	if err != nil || len(roster) != 1 || !roster[0].Self {
		t.Fatal(roster, err)
	}
	history, err := srv.ChatHistory(ctx, u.Email, roster[0].JID, 80)
	if err != nil || len(history) != 1 || history[0].Body != "private note" {
		t.Fatal(history, err)
	}
}
