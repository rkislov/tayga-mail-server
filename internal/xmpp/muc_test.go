// SPDX-License-Identifier: Apache-2.0
package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
)

func mucTestSession(srv *Server, u *storage.User, resource string) (*Session, *bytes.Buffer) {
	b := &bytes.Buffer{}
	return &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/" + resource), bw: bufio.NewWriter(b)}, b
}
func mucInput(t *testing.T, s *Session, raw string) {
	t.Helper()
	s.dec = xml.NewDecoder(strings.NewReader(raw))
	token, e := s.dec.Token()
	if e != nil {
		t.Fatal(e)
	}
	start := token.(xml.StartElement)
	switch start.Name.Local {
	case "iq":
		e = s.handleIQ(start)
	case "message":
		e = s.handleMessage(start)
	case "presence":
		e = s.handlePresence(start)
	}
	if e != nil {
		t.Fatal(e)
	}
}
func TestMUCNativeWebMembershipAndReplay(t *testing.T) {
	srv, st, alice := testServer(t)
	ctx := context.Background()
	bob, e := st.CreateUser(ctx, &storage.User{TenantID: alice.TenantID, DomainID: alice.DomainID, LocalPart: "bob", Email: "bob@example.com", Enabled: true, AuthSource: "local"})
	if e != nil {
		t.Fatal(e)
	}
	a, ab := mucTestSession(srv, alice, "a")
	b, bb := mucTestSession(srv, bob, "b")
	srv.hub.Register(a)
	srv.hub.Register(b)
	room, e := srv.CreateChatRoom(ctx, alice.Email, "Team", []string{bob.Email})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(bb.String(), "<invite") {
		t.Fatal("missing native invite", bb.String())
	}
	ab.Reset()
	bb.Reset()
	join := func(s *Session, nick string) {
		mucInput(t, s, `<presence to='`+room.JID+`/`+nick+`'><x xmlns='`+mucNS+`'><history maxstanzas='10'/></x></presence>`)
	}
	join(a, "Alice")
	join(b, "Bob")
	if !strings.Contains(bb.String(), "code='110'") || !strings.Contains(bb.String(), "/Alice") {
		t.Fatal(bb.String())
	}
	ab.Reset()
	bb.Reset()
	ch, cancel := srv.SubscribeChat(alice.Email)
	defer cancel()
	message := `<message to='` + room.JID + `' type='groupchat' id='client1'><body>Hello from native</body><origin-id xmlns='urn:xmpp:sid:0' id='origin1'/></message>`
	mucInput(t, b, message)
	event := <-ch
	if event.Body != "Hello from native" {
		t.Fatal(event)
	}
	if !strings.Contains(ab.String(), "/Bob") || !strings.Contains(ab.String(), event.ID) {
		t.Fatal(ab.String())
	}
	ab.Reset()
	bb.Reset()
	mucInput(t, b, message)
	if ab.Len() != 0 || !strings.Contains(bb.String(), event.ID) {
		t.Fatal("retry duplicated broadcast", ab.String(), bb.String())
	}
	if _, e = srv.SendChatMessage(ctx, alice.Email, room.JID, "Web reply"); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(bb.String(), "Web reply") {
		t.Fatal(bb.String())
	}
	history, e := srv.ChatHistory(ctx, alice.Email, room.JID, 80)
	if e != nil || len(history) != 2 {
		t.Fatal(history, e)
	}
	bb.Reset()
	mucInput(t, b, `<presence type='unavailable' to='`+room.JID+`/Bob'/>`)
	bb.Reset()
	join(b, "Bob")
	if !strings.Contains(bb.String(), "urn:xmpp:delay") || !strings.Contains(bb.String(), event.ID) {
		t.Fatal(bb.String())
	}
	bb.Reset()
	mucInput(t, b, `<iq to='`+room.JID+`' type='set' id='bad'><query xmlns='`+mucNS+`#admin'><item jid='`+alice.Email+`' affiliation='none'/></query></iq>`)
	if !strings.Contains(bb.String(), "type='error'") {
		t.Fatal("member changed owner", bb.String())
	}
	if e = srv.UpdateChatRoom(ctx, alice.Email, room.JID, bob.Email, "none", false); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(bb.String(), "code='321'") {
		t.Fatal(bb.String())
	}
	if _, e = srv.ChatHistory(ctx, bob.Email, room.JID, 80); e == nil {
		t.Fatal("former member has history")
	}
	bb.Reset()
	join(b, "Bob")
	if !strings.Contains(bb.String(), "registration-required") {
		t.Fatal(bb.String())
	}
	if e = srv.LeaveChatRoom(ctx, alice.Email, room.JID); e == nil {
		t.Fatal("owner left orphaned room")
	}
	if e = srv.UpdateChatRoom(ctx, alice.Email, room.JID, "", "", true); e != nil {
		t.Fatal(e)
	}
}
func TestMUCNativeCreateConfigureAndIsolation(t *testing.T) {
	srv, st, alice := testServer(t)
	a, ab := mucTestSession(srv, alice, "a")
	room := "created@web-groups.example.com"
	mucInput(t, a, `<presence to='`+room+`/Alice'><x xmlns='`+mucNS+`'/></presence>`)
	if !strings.Contains(ab.String(), "code='201'") {
		t.Fatal(ab.String())
	}
	ab.Reset()
	mucInput(t, a, `<iq type='set' id='config' to='`+room+`'><query xmlns='`+mucNS+`#owner'><x xmlns='jabber:x:data' type='submit'/></query></iq>`)
	if !strings.Contains(ab.String(), "type='result'") {
		t.Fatal(ab.String())
	}
	ctx := context.Background()
	tenant, _ := st.CreateTenant(ctx, "foreign")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "foreign.example")
	outsider, e := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "outsider", Email: "outsider@foreign.example", Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = srv.UpdateChatRoom(ctx, alice.Email, room, outsider.Email, "member", false); e == nil {
		t.Fatal("cross tenant invitation")
	}
	o, ob := mucTestSession(srv, outsider, "phone")
	mucInput(t, o, `<iq type='get' id='list' to='web-groups.example.com'><query xmlns='`+discoItems+`'/></iq>`)
	if strings.Contains(ob.String(), room) {
		t.Fatal("room leaked", ob.String())
	}
	ab.Reset()
	mucInput(t, a, `<iq type='get' id='disco' to='example.com'><query xmlns='`+discoItems+`'/></iq>`)
	if !strings.Contains(ab.String(), "web-groups.example.com") {
		t.Fatal(ab.String())
	}
}

func TestMUCMultipleDevicesAndStreamResume(t *testing.T) {
	srv, st, alice := testServer(t)
	ctx := context.Background()
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: alice.TenantID, DomainID: alice.DomainID, LocalPart: "bob", Email: "bob@example.com", Enabled: true, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, alice.Email, "Shared", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	a, ab := mucTestSession(srv, alice, "desktop")
	phone, pb := mucTestSession(srv, alice, "phone")
	b, bb := mucTestSession(srv, bob, "phone")
	join := func(s *Session, nick string) {
		mucInput(t, s, `<presence to='`+room.JID+`/`+nick+`'><x xmlns='`+mucNS+`'/></presence>`)
	}
	join(a, "Alice")
	join(phone, "Alice")
	join(b, "Bob")
	if strings.Count(bb.String(), "/Alice' to=") != 1 {
		t.Fatal("duplicate occupant", bb.String())
	}
	bb.Reset()
	ab.Reset()
	pb.Reset()
	mucInput(t, phone, `<presence type='unavailable' to='`+room.JID+`/Alice'/>`)
	if strings.Contains(bb.String(), "type='unavailable'") {
		t.Fatal("remaining device marked offline", bb.String())
	}
	bb.Reset()
	mucInput(t, a, `<presence to='`+room.JID+`/Invalid'><x xmlns='`+mucNS+`'><history maxstanzas='-1'/></x></presence>`)
	if bb.Len() != 0 || srv.muc[room.JID][a].Nick != "Alice" {
		t.Fatal("invalid rename changed presence", bb.String())
	}
	a.authed = true
	a.bound = true
	a.sm = streamManagement{enabled: true, resumable: true, token: "room-resume"}
	a.leaveMUC()
	a.retainStream()
	if _, err = srv.SendChatMessage(ctx, bob.Email, room.JID, "While disconnected"); err != nil {
		t.Fatal(err)
	}
	resumed, rb := mucTestSession(srv, alice, "unused")
	resumed.authed = true
	start := xml.StartElement{Attr: []xml.Attr{{Name: xml.Name{Local: "previd"}, Value: "room-resume"}, {Name: xml.Name{Local: "h"}, Value: "0"}}}
	if err = resumed.resumeStream(start); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rb.String(), "<resumed") || !strings.Contains(rb.String(), "While disconnected") || srv.muc[room.JID][resumed].Nick != "Alice" {
		t.Fatal(rb.String())
	}
	rb.Reset()
	if _, err = srv.SendChatMessage(ctx, bob.Email, room.JID, "After resume"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rb.String(), "After resume") {
		t.Fatal("resumed resource not receiving", rb.String())
	}
}
