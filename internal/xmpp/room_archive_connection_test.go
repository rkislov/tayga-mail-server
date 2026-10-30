package xmpp

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestRoomMAMTCPReconnectAndRemovedMember(t *testing.T) {
	srv, st, u := testServer(t)
	ctx := context.Background()
	srv.authn = auth.NewLayer(st, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateUserPassword(ctx, u.ID, hash); err != nil {
		t.Fatal(err)
	}
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	phone := newResumeWireClient(t, srv, ln, "phone")
	desktop := newResumeWireClient(t, srv, ln, "desktop")
	room, err := srv.CreateChatRoom(ctx, bob.Email, "TCP archive", []string{u.Email})
	if err != nil {
		t.Fatal(err)
	}
	phone.want("message", "")
	desktop.want("message", "") // invitations, not user messages
	var ids []string
	encrypted := map[string]bool{}
	audience, err := srv.RoomOMEMOContext(ctx, bob.Email, room.JID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 257; i++ {
		var ev ChatEvent
		var e error
		if i%2 == 0 {
			ev, e = srv.SendRoomOMEMO(ctx, bob.Email, room.JID, audience.Revision, groupCipherFixture)
			encrypted[ev.ID] = true
		} else {
			ev, e = srv.SendChatMessage(ctx, bob.Email, room.JID, fmt.Sprint(i))
		}
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, ev.ID)
	}
	phone.send(`<presence to='` + room.JID + `/Alice'><x xmlns='` + mucNS + `'><history maxstanzas='0'/></x></presence>`)
	phone.want("presence", "")
	phone.want("message", "") // self-presence and subject
	phone.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := phone.want("enabled", "").ID
	query := func(c *resumeWireClient, id, rsm string) mamWirePage {
		c.send(`<iq to='` + room.JID + `' type='set' id='` + id + `'><query xmlns='urn:xmpp:mam:2' queryid='` + id + `'><set xmlns='http://jabber.org/protocol/rsm'>` + rsm + `</set></query></iq>`)
		page := readArchiveWirePage(t, c, id, "", room.JID)
		var want []string
		for _, id := range page.IDs {
			if encrypted[id] {
				want = append(want, id)
			}
		}
		if !reflect.DeepEqual(page.Encrypted, want) {
			t.Fatalf("ciphertexts lost across TCP/MAM: %+v", page)
		}
		return page
	}
	first := query(phone, "first", `<max>100</max>`)
	if !reflect.DeepEqual(first.IDs, ids[:100]) || first.Complete {
		t.Fatal(first)
	}
	last := query(desktop, "last", `<max>100</max><before/>`)
	if !reflect.DeepEqual(last.IDs, ids[157:]) {
		t.Fatal(last)
	}
	// Archived results go only to the requesting resource even with carbons enabled.
	desktop.send(`<iq type='get' id='barrier'><ping xmlns='urn:xmpp:ping'/></iq>`)
	desktop.want("iq", "barrier")
	phone.send(`<a xmlns='urn:xmpp:sm:3' h='101'/><r xmlns='urn:xmpp:sm:3'/>`)
	phone.want("a", "")
	phone.conn.Close()
	select {
	case <-phone.done:
	case <-time.After(4 * time.Second):
		t.Fatal("disconnect timeout")
	}
	restored := newResumeWireClient(t, srv, ln, "")
	restored.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='101'/>`)
	restored.want("resumed", "")
	// Resuming rejoins the room and rechecks membership.
	restored.want("presence", "")
	restored.want("message", "")
	middle := query(restored, "middle", `<max>100</max><after>`+first.Last+`</after>`)
	if !reflect.DeepEqual(middle.IDs, ids[100:200]) {
		t.Fatal(middle)
	}
	end := query(restored, "end", `<max>100</max><after>`+middle.Last+`</after>`)
	if !reflect.DeepEqual(end.IDs, ids[200:]) || !end.Complete {
		t.Fatal(end)
	}
	// 101 acknowledged before resume; two join stanzas + 101 + 58 since then.
	restored.send(`<a xmlns='urn:xmpp:sm:3' h='262'/><r xmlns='urn:xmpp:sm:3'/>`)
	restored.want("a", "")
	restored.conn.Close()
	select {
	case <-restored.done:
	case <-time.After(4 * time.Second):
		t.Fatal("disconnect timeout")
	}
	if err = srv.UpdateChatRoom(ctx, bob.Email, room.JID, u.Email, "none", false); err != nil {
		t.Fatal(err)
	}
	if _, err = srv.SendChatMessage(ctx, bob.Email, room.JID, "after removal"); err != nil {
		t.Fatal(err)
	}
	removed := newResumeWireClient(t, srv, ln, "")
	removed.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='262'/>`)
	removed.want("resumed", "")
	e := removed.want("presence", "")
	if !strings.Contains(e.Inner, "registration-required") {
		t.Fatal(e)
	}
	removed.send(`<iq to='` + room.JID + `' type='set' id='denied'><query xmlns='urn:xmpp:mam:2'/></iq>`)
	if e = removed.want("iq", "denied"); !strings.Contains(e.Inner, "forbidden") {
		t.Fatal(e)
	}
}

// A room sender can leave/reuse a nickname while the receiver is disconnected.
// Require a fresh MAM catchup instead of a successful SM resume that drops a
// ciphertext or feeds it to a Signal session belonging to the current nick.
func TestGroupOMEMOReconnectFallsBackToMAM(t *testing.T) {
	srv, st, u := testServer(t)
	ctx := context.Background()
	srv.authn = auth.NewLayer(st, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateUserPassword(ctx, u.ID, hash); err != nil {
		t.Fatal(err)
	}
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, u.Email, "Encrypted reconnect", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	phone := newResumeWireClient(t, srv, ln, "phone")
	join := func(c *resumeWireClient) {
		c.send(`<presence to='` + room.JID + `/Alice'><x xmlns='` + mucNS + `'><history maxstanzas='0'/></x></presence>`)
		c.want("presence", "")
		c.want("message", "")
	}
	join(phone)
	phone.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := phone.want("enabled", "").ID
	phone.conn.Close()
	select {
	case <-phone.done:
	case <-time.After(4 * time.Second):
		t.Fatal("disconnect timeout")
	}
	audience, err := srv.RoomOMEMOContext(ctx, bob.Email, room.JID)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := srv.SendRoomOMEMO(ctx, bob.Email, room.JID, audience.Revision, groupCipherFixture)
	if err != nil {
		t.Fatal(err)
	}
	restored := newResumeWireClient(t, srv, ln, "")
	restored.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='0'/>`)
	if failure := restored.want("failed", ""); !strings.Contains(failure.Inner, "item-not-found") {
		t.Fatal(failure)
	}
	restored.send(`<iq type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>phone</resource></bind></iq>`)
	restored.want("iq", "bind")
	join(restored)
	restored.send(`<iq to='` + room.JID + `' type='set' id='catchup'><query xmlns='urn:xmpp:mam:2' queryid='catchup'/></iq>`)
	page := readArchiveWirePage(t, restored, "catchup", "", room.JID)
	if !reflect.DeepEqual(page.Encrypted, []string{ev.ID}) || !page.Complete {
		t.Fatal(page)
	}
}

// Monal pulls bookmarks from its own bare JID after login and joins autojoin
// entries. The creator on another device must get its room without an invite.
func TestRoomBookmarksTCPReconnect(t *testing.T) {
	srv, st, u := testServer(t)
	ctx := context.Background()
	srv.authn = auth.NewLayer(st, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateUserPassword(ctx, u.ID, hash); err != nil {
		t.Fatal(err)
	}
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, u.Email, "From web", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	for _, resource := range []string{"phone", "second-phone"} {
		phone := newResumeWireClient(t, srv, ln, resource)
		phone.send(`<iq type='get' id='bookmarks' to='` + u.Email + `'><pubsub xmlns='` + pepNS + `'><items node='` + bookmarksNode + `'/></pubsub></iq>`)
		iq := phone.want("iq", "bookmarks")
		if !strings.Contains(iq.Inner, room.JID) || !strings.Contains(iq.Inner, `autojoin='true'`) {
			t.Fatal(iq)
		}
		phone.send(`<presence to='` + room.JID + `/Alice'><x xmlns='` + mucNS + `'><history maxstanzas='0'/></x></presence>`)
		presence := phone.want("presence", "")
		if strings.Contains(presence.Inner, "<error") || !strings.Contains(presence.Inner, "110") {
			t.Fatal(presence)
		}
		phone.want("message", "")
		phone.send(`<presence type='unavailable' to='` + room.JID + `/Alice'/>`)
		phone.want("presence", "")
	}
}
