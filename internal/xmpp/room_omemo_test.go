package xmpp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const groupCipherFixture = `<encrypted xmlns="eu.siacs.conversations.axolotl"><header sid="42"><key rid="7" prekey="true">YWJj</key><iv>AAAAAAAAAAAAAAAA</iv></header><payload>YWJj</payload></encrypted>`

func TestGroupOMEMO(t *testing.T) { srv, st, a := testServer(t); testGroupOMEMO(t, srv, st, a) }
func testGroupOMEMO(t *testing.T, srv *Server, st *storage.Store, alice *storage.User) {
	t.Helper()
	ctx := context.Background()
	member := func(local string) *storage.User {
		u, e := st.CreateUser(ctx, &storage.User{Email: local + "@example.com", LocalPart: local, TenantID: alice.TenantID, DomainID: alice.DomainID, Enabled: true, DisplayName: local})
		if e != nil {
			t.Fatal(e)
		}
		return u
	}
	bob, c := member("group-bob"), member("group-carol")
	room, e := srv.CreateChatRoom(ctx, alice.Email, "Encrypted", []string{bob.Email, c.Email})
	if e != nil {
		t.Fatal(e)
	}
	events, stop := srv.SubscribeChat(alice.Email)
	defer stop()
	_, stopSecond := srv.SubscribeChat(alice.Email)
	a, ab := mucTestSession(srv, alice, "phone")
	b, bb := mucTestSession(srv, bob, "phone")
	join := func(s *Session, nick string) {
		mucInput(t, s, `<presence to='`+room.JID+`/`+nick+`'><x xmlns='`+mucNS+`'><history maxstanzas='0'/></x></presence>`)
	}
	join(a, "Alice")
	join(b, "Bob")
	ab.Reset()
	bb.Reset()
	audience, e := srv.RoomOMEMOContext(ctx, alice.Email, room.JID)
	if e != nil || len(audience.Members) != 3 || audience.Revision == "" {
		t.Fatal(audience, e)
	}
	ev, e := srv.SendRoomOMEMO(ctx, alice.Email, room.JID, audience.Revision, groupCipherFixture)
	if e != nil || !ev.Encrypted || ev.Body != "" || ev.OMEMO == "" {
		t.Fatal(ev, e)
	}
	deadline := time.After(time.Second)
received:
	for {
		select {
		case got := <-events:
			if got.ID == ev.ID {
				if got.OMEMO != ev.OMEMO {
					t.Fatal(got)
				}
				break received
			}
		case <-deadline:
			t.Fatal("encrypted SSE message missing")
		}
	}
	live := bb.String()
	if !strings.Contains(live, "jid='"+alice.Email+"/web'") || !strings.Contains(live, ev.ID) || strings.Contains(live, "<body>") || strings.Index(live, "<presence") > strings.Index(live, "<message") {
		t.Fatal("missing authoritative web presence before ciphertext", live)
	}
	// A new native resource learns about existing web occupants before messages.
	b2, b2out := mucTestSession(srv, bob, "tablet")
	join(b2, "Bob")
	if !strings.Contains(b2out.String(), "jid='"+alice.Email+"/web'") {
		t.Fatal(b2out.String())
	}
	bb.Reset()
	stopSecond()
	if strings.Contains(bb.String(), "unavailable") {
		t.Fatal("one of two tabs disconnected web occupant")
	}
	bb.Reset()
	stop()
	if !strings.Contains(bb.String(), "unavailable") {
		t.Fatal("last web connection didn't leave room", bb.String())
	}
	// Reserved nick cannot be claimed by another participant.
	bb.Reset()
	join(b, webRoomNick(alice.Email))
	if !strings.Contains(bb.String(), "jid-malformed") {
		t.Fatal(bb.String())
	}
	ab.Reset()
	bb.Reset()
	native := `<message to='` + room.JID + `' type='groupchat' id='native-cipher'><body>PLAINTEXT MUST NOT BE STORED</body>` + groupCipherFixture + `<x xmlns='` + mucNS + `#user'><item jid='forged@example.com'/></x></message>`
	mucInput(t, b, native)
	if !strings.Contains(ab.String(), "<encrypted") || strings.Contains(ab.String(), "PLAINTEXT") || strings.Contains(ab.String(), "forged") {
		t.Fatal(ab.String())
	}
	page, e := srv.RoomHistoryPage(ctx, alice.Email, room.JID, "", 80)
	if e != nil || len(page.Messages) != 2 {
		t.Fatal(page, e)
	}
	nativeID := page.Messages[1].ID
	for _, m := range page.Messages {
		if !m.Encrypted || m.Body != "" || m.OMEMO != ev.OMEMO {
			t.Fatal(m)
		}
	}
	ab.Reset()
	bb.Reset()
	mucInput(t, b, native)
	if ab.Len() != 0 || !strings.Contains(bb.String(), nativeID) || !strings.Contains(bb.String(), "<encrypted") {
		t.Fatal("duplicate delivery or missing encrypted retry", ab.String(), bb.String())
	}
	// MAM keeps authoritative identities and the very same payload and stanza IDs.
	bb.Reset()
	mucInput(t, b, `<iq to='`+room.JID+`' type='set' id='mam'><query xmlns='urn:xmpp:mam:2'/></iq>`)
	if strings.Count(bb.String(), "<encrypted") != 2 || !strings.Contains(bb.String(), "jid='"+alice.Email+"/web'") || !strings.Contains(bb.String(), "jid='"+bob.Email+"/phone'") || strings.Contains(bb.String(), "forged") {
		t.Fatal(bb.String())
	}
	ab.Reset()
	bb.Reset()
	if e = srv.UpdateChatRoom(ctx, alice.Email, room.JID, c.Email, "none", false); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(bb.String(), "jid='"+c.Email+"' affiliation='none'") {
		t.Fatal("offline membership change not broadcast", bb.String())
	}
	if _, e = srv.SendRoomOMEMO(ctx, alice.Email, room.JID, audience.Revision, groupCipherFixture); !errors.Is(e, ErrRoomChanged) {
		t.Fatal("stale audience accepted", e)
	}
	if e = srv.UpdateChatRoom(ctx, alice.Email, room.JID, c.Email, "member", false); e != nil {
		t.Fatal(e)
	}
	if _, e = srv.SendRoomOMEMO(ctx, alice.Email, room.JID, audience.Revision, groupCipherFixture); !errors.Is(e, ErrRoomChanged) {
		t.Fatal("remove/readd reused old audience", e)
	}
	// Disabled users are not encryption recipients, even without a membership edit.
	current, e := srv.RoomOMEMOContext(ctx, alice.Email, room.JID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = srv.db.ExecContext(ctx, srv.rebind(`UPDATE users SET enabled=? WHERE id=?`), false, c.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = srv.SendRoomOMEMO(ctx, alice.Email, room.JID, current.Revision, groupCipherFixture); !errors.Is(e, ErrRoomChanged) {
		t.Fatal(e)
	}
	if _, e = srv.RoomOMEMOContext(ctx, c.Email, room.JID); !errors.Is(e, ErrRoomAccess) {
		t.Fatal(e)
	}
	if e = srv.UpdateChatRoom(ctx, alice.Email, room.JID, bob.Email, "none", false); e != nil {
		t.Fatal(e)
	}
	if _, e = srv.RoomOMEMOContext(ctx, bob.Email, room.JID); !errors.Is(e, ErrRoomAccess) {
		t.Fatal(e)
	}
	ab.Reset()
	bb.Reset()
	current, e = srv.RoomOMEMOContext(ctx, alice.Email, room.JID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = srv.SendRoomOMEMO(ctx, alice.Email, room.JID, current.Revision, groupCipherFixture); e != nil {
		t.Fatal(e)
	}
	if bb.Len() != 0 {
		t.Fatal("removed participant received ciphertext", bb.String())
	}
	if _, e = srv.SendRoomOMEMO(ctx, alice.Email, room.JID, current.Revision, strings.Replace(groupCipherFixture, `sid="42"`, `sid="0"`, 1)); e == nil {
		t.Fatal("invalid ciphertext accepted")
	}
	page, e = srv.RoomHistoryPage(ctx, alice.Email, room.JID, "", 80)
	if e != nil || len(page.Messages) != 3 {
		t.Fatal(page, e)
	}
	if e = srv.PublishWebOMEMO(ctx, alice.Email, legacyOMEMO+".bundles:42", retirementBundle); e != nil {
		t.Fatal(e)
	}
	if e = srv.RetireWebOMEMO(ctx, alice.Email, 42, "fixture-key"); e != nil {
		t.Fatal(e)
	}
	if _, e = srv.SendRoomOMEMO(ctx, alice.Email, room.JID, current.Revision, groupCipherFixture); !errors.Is(e, ErrOMEMORetired) {
		t.Fatal("retired group sender accepted", e)
	}
	ab.Reset()
	mucInput(t, a, `<message to='`+room.JID+`' type='groupchat' id='retired'>`+groupCipherFixture+`</message>`)
	if !strings.Contains(ab.String(), "not-allowed") {
		t.Fatal(ab.String())
	}
}
