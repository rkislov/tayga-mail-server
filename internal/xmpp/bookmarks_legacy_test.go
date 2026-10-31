package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestLegacyBookmarksBridge(t *testing.T) {
	srv, st, alice := testServer(t)
	testLegacyBookmarksBridge(t, srv, st, alice)
}

func testLegacyBookmarksBridge(t *testing.T, srv *Server, st *storage.Store, alice *storage.User) {
	t.Helper()
	ctx := context.Background()
	bob, err := st.CreateUser(ctx, &storage.User{Email: "legacyfriend@example.com", LocalPart: "legacyfriend", DomainID: alice.DomainID, TenantID: alice.TenantID, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, alice.Email, "Bookmarked & private", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	var own, legacy, modern, foreign bytes.Buffer
	client := &Session{srv: srv, user: alice, JID: ParseJID(alice.Email + "/writer"), bw: bufio.NewWriter(&own)}
	for _, session := range []*Session{
		{srv: srv, user: alice, JID: ParseJID(alice.Email + "/legacy"), bw: bufio.NewWriter(&legacy), pepInterest: map[string]bool{legacyBookmarksNode: true}},
		{srv: srv, user: alice, JID: ParseJID(alice.Email + "/modern"), bw: bufio.NewWriter(&modern), pepInterest: map[string]bool{bookmarksNode: true}},
		{srv: srv, user: alice, JID: ParseJID("stranger@example.com/phone"), bw: bufio.NewWriter(&foreign), pepInterest: map[string]bool{bookmarksNode: true, legacyBookmarksNode: true}},
	} {
		srv.hub.Register(session)
	}
	call := func(typ, body string) string { return pepTestIQ(t, client, &own, typ, "", body) }
	get := `<pubsub xmlns='` + pepNS + `'><items node='storage:bookmarks'/></pubsub>`
	if got := call("get", get); !strings.Contains(got, room.JID) || !strings.Contains(got, `autojoin="true"`) {
		t.Fatal("legacy client cannot discover web room", got)
	}
	publish := func(payload string) string {
		return `<pubsub xmlns='` + pepNS + `'><publish node='storage:bookmarks'><item id='current'>` + payload + `</item></publish><publish-options><x xmlns='jabber:x:data'><field var='pubsub#access_model'><value>whitelist</value></field></x></publish-options></pubsub>`
	}
	initial := `<conference xmlns='urn:xmpp:bookmarks:1' autojoin='true'><extensions><settings xmlns='urn:client:private' xmlns:v='urn:client:attrs' v:pin='yes'>value</settings></extensions></conference>`
	if err = client.publishAndNotifyPEP(ctx, alice.Email, bookmarksNode, room.JID, initial); err != nil {
		t.Fatal(err)
	}
	legacy.Reset()
	modern.Reset()
	updated := `<storage xmlns='storage:bookmarks'><url name='Reference' url='https://example.org'/><conference jid='` + room.JID + `' name='Private name' autojoin='false'><nick>Other &amp; nick</nick></conference><conference jid='external@conference.example.org' autojoin='true'/></storage>`
	if got := call("set", publish(updated)); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if !strings.Contains(legacy.String(), "external@conference.example.org") || !strings.Contains(modern.String(), "Other &amp; nick") || foreign.Len() != 0 {
		t.Fatal("notifications missing or crossed owner boundary", legacy.String(), modern.String(), foreign.String())
	}
	if got := call("get", get); !strings.Contains(got, "https://example.org") || !strings.Contains(got, "urn:client:attrs") || !strings.Contains(got, `autojoin="false"`) {
		t.Fatal("round trip lost preferences, extensions or URLs", got)
	}
	// Legacy writes that omit extensions must preserve those created by new clients.
	items, err := srv.getPEPItems(ctx, alice.Email, bookmarksNode)
	if err != nil || len(items) < 2 {
		t.Fatal(items, err)
	}
	found := false
	for _, item := range items {
		if item.ItemID == room.JID {
			tokens, err := bookmarkTokens(item.Payload, xml.Name{Space: bookmarksNode, Local: "conference"})
			if err != nil {
				t.Fatal(err)
			}
			for _, token := range tokens {
				if start, ok := token.(xml.StartElement); ok && start.Name.Space == "urn:client:private" {
					for _, a := range start.Attr {
						found = found || a.Name == (xml.Name{Space: "urn:client:attrs", Local: "pin"}) && a.Value == "yes"
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("extension namespace/attribute lost")
	}
	modern.Reset()
	if got := call("set", publish(updated)); !strings.Contains(got, "type='result'") || modern.Len() != 0 {
		t.Fatal("unchanged legacy write repeated native updates", got, modern.String())
	}
	before := call("get", get)
	for _, bad := range []string{
		`<storage xmlns='storage:bookmarks'><conference jid='bad jid'/></storage>`,
		`<storage xmlns='storage:bookmarks'><conference jid='room@example.org'/><conference jid='room@example.org'/></storage>`,
		`<storage xmlns='storage:bookmarks'/><extra/>`,
	} {
		if got := call("set", publish(bad)); !strings.Contains(got, "bad-request") {
			t.Fatal("invalid replacement accepted", got)
		}
		if after := call("get", get); before != after {
			t.Fatal("invalid replacement modified bookmarks")
		}
	}
	if got := call("set", strings.Replace(publish(updated), "<value>whitelist</value>", "<value>open</value>", 1)); !strings.Contains(got, "not-acceptable") {
		t.Fatal("public legacy bookmarks accepted", got)
	}
	stranger := &Session{srv: srv, user: alice, JID: ParseJID("stranger@example.com/reader"), bw: bufio.NewWriter(&foreign)}
	if got := pepTestIQ(t, stranger, &foreign, "get", alice.Email, get); !strings.Contains(got, "forbidden") {
		t.Fatal("legacy privacy failure", got)
	}
	foreign.Reset()
	stranger.sendLastPEP(ctx, alice.Email, legacyBookmarksNode)
	if foreign.Len() != 0 {
		t.Fatal("legacy presence privacy failure")
	}
	// A modern retract is reflected in the old representation, preserving URLs.
	if got := call("set", `<pubsub xmlns='`+pepNS+`'><retract node='`+bookmarksNode+`'><item id='external@conference.example.org'/></retract></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if got := call("get", get); strings.Contains(got, "external@conference.example.org") || !strings.Contains(got, "https://example.org") {
		t.Fatal("legacy projection stale", got)
	}
	// Removing the old single item also clears native bookmarks, and does not
	// leave the actual room or resurrect its managed bookmark on reconnect.
	if got := call("set", `<pubsub xmlns='`+pepNS+`'><retract node='storage:bookmarks'><item id='current'/></retract></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if got := call("get", get); strings.Contains(got, "<conference") {
		t.Fatal("deleted bookmarks resurrected", got)
	}
	if err = srv.requireRoomMember(ctx, alice.Email, room.JID); err != nil {
		t.Fatal("bookmark removal changed membership", err)
	}
	call("set", publish(updated))
	if got := call("set", `<pubsub xmlns='`+pepNS+`#owner'><delete node='`+bookmarksNode+`'/></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if got := call("get", get); strings.Contains(got, "<conference") || !strings.Contains(got, "https://example.org") {
		t.Fatal("native node deletion lost URLs or retained rooms", got)
	}
	call("set", publish(updated))
	if got := call("set", `<pubsub xmlns='`+pepNS+`#owner'><delete node='storage:bookmarks'/></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if got := call("get", get); strings.Contains(got, "<conference") || strings.Contains(got, "https://example.org") {
		t.Fatal("legacy node deletion retained bookmarks", got)
	}
}

func TestLegacyBookmarksUpgradeImport(t *testing.T) {
	srv, _, alice := testServer(t)
	ctx := context.Background()
	if err := srv.publishPEP(ctx, alice.Email, legacyBookmarksNode, "old", `<storage xmlns='storage:bookmarks'><conference jid='old@conference.example.org' autojoin='false'><nick>Old nick</nick></conference><url url='https://example.org'/></storage>`); err != nil {
		t.Fatal(err)
	}
	if err := srv.syncRoomBookmarks(ctx, alice.Email); err != nil {
		t.Fatal(err)
	}
	items, err := srv.getPEPItems(ctx, alice.Email, bookmarksNode)
	if err != nil || len(items) != 1 || items[0].ItemID != "old@conference.example.org" || !strings.Contains(items[0].Payload, "Old nick") {
		t.Fatal(items, err)
	}
	if err = srv.removeRoomBookmark(ctx, alice.Email, items[0].ItemID); err != nil {
		t.Fatal(err)
	}
	if err = srv.syncRoomBookmarks(ctx, alice.Email); err != nil {
		t.Fatal(err)
	}
	item, err := srv.legacyBookmarkItem(ctx, alice.Email)
	if err != nil || strings.Contains(item.Payload, "<conference") || !strings.Contains(item.Payload, "https://example.org") {
		t.Fatal("old conference resurrected or URL lost", item, err)
	}
}
