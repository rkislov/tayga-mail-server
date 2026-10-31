package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"
)

func pepTestIQ(t *testing.T, s *Session, out *bytes.Buffer, typ, to, body string) string {
	t.Helper()
	out.Reset()
	if err := s.handlePubSubIQ(context.Background(), "pep-test", typ, to, body); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestMonalPEPDiscoveryRequirements(t *testing.T) {
	srv, _, user := testServer(t)
	var out bytes.Buffer
	s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/monal"), bw: bufio.NewWriter(&out)}
	if err := s.writeDiscoInfo("features", user.Email); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<identity category='pubsub' type='pep'", "filtered-notifications", "publish-options", "publish", "subscribe", "create-nodes", "delete-items", "delete-nodes", "persistent-items", "retrieve-items", "auto-create", "pubsub#item-ids", "pubsub#multi-items", "pubsub#config-node-max", "urn:xmpp:bookmarks:1#compat-pep"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("missing %s: %s", expected, out.String())
		}
	}
}

func TestMonalPublishOptionsAndFilteredHeadline(t *testing.T) {
	srv, _, user := testServer(t)
	ctx := context.Background()
	var own, interested, uninterested, stranger bytes.Buffer
	node := "eu.siacs.conversations.axolotl.devicelist"
	s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/monal"), bw: bufio.NewWriter(&own), pepInterest: map[string]bool{node: true}}
	peer := &Session{srv: srv, JID: ParseJID("bob@example.com/phone"), bw: bufio.NewWriter(&interested), pepInterest: map[string]bool{node: true}}
	other := &Session{srv: srv, JID: ParseJID("bob@example.com/other"), bw: bufio.NewWriter(&uninterested)}
	unknown := &Session{srv: srv, JID: ParseJID("mallory@example.com/phone"), bw: bufio.NewWriter(&stranger), pepInterest: map[string]bool{node: true}}
	for _, session := range []*Session{s, peer, other, unknown} {
		srv.hub.Register(session)
	}
	if err := srv.upsertRoster(ctx, user.ID, rosterItem{JID: peer.JID.Bare(), Subscription: "from"}); err != nil {
		t.Fatal(err)
	}
	body := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='` + node + `'><item id='current'><list xmlns='eu.siacs.conversations.axolotl'><device id='42'/></list></item></publish><publish-options><x xmlns='jabber:x:data' type='submit'><field var='FORM_TYPE'><value>http://jabber.org/protocol/pubsub#publish-options</value></field><field var='pubsub#access_model'><value>open</value></field><field var='pubsub#persist_items'><value>true</value></field><field var='pubsub#notify_retract'><value>true</value></field><field var='pubsub#notify_delete'><value>true</value></field></x></publish-options></pubsub>`
	if got := pepTestIQ(t, s, &own, "set", "", body); !strings.Contains(got, "type='result'") || !strings.Contains(got, "type='headline'") {
		t.Fatal(got)
	}
	if !strings.Contains(interested.String(), "type='headline'") || !strings.Contains(interested.String(), "device id") {
		t.Fatal(interested.String())
	}
	if uninterested.Len() != 0 || stranger.Len() != 0 {
		t.Fatal("notification escaped resource interest or roster boundary")
	}
	interested.Reset()
	if got := pepTestIQ(t, s, &own, "set", "", `<pubsub xmlns='http://jabber.org/protocol/pubsub'><retract node='`+node+`'><item id='current'/></retract></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if !strings.Contains(interested.String(), "<retract") {
		t.Fatal(interested.String())
	}
	interested.Reset()
	if got := pepTestIQ(t, s, &own, "set", "", `<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><delete node='`+node+`'/></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if !strings.Contains(interested.String(), "<delete node=") {
		t.Fatal(interested.String())
	}
}

func TestPEPSubscriptionsPersistAndRespectAccess(t *testing.T) {
	srv, store, alice := testServer(t)
	ctx := context.Background()
	var ownerOut, peerOut bytes.Buffer
	owner := &Session{srv: srv, user: alice, JID: ParseJID(alice.Email + "/phone"), bw: bufio.NewWriter(&ownerOut)}
	// The protocol always uses the bound identity for subscription authorization.
	peer := &Session{srv: srv, user: alice, JID: ParseJID("bob@example.com/monal"), bw: bufio.NewWriter(&peerOut)}
	srv.hub.Register(peer)
	callOwner := func(typ, body string) string { return pepTestIQ(t, owner, &ownerOut, typ, "", body) }
	callPeer := func(typ, body string) string { return pepTestIQ(t, peer, &peerOut, typ, alice.Email, body) }
	create := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><create node='urn:test:private'/></pubsub>`
	if got := callOwner("set", create); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if got := callOwner("set", create); !strings.Contains(got, "conflict") {
		t.Fatal(got)
	}
	sub := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><subscribe node='urn:test:private' jid='bob@example.com'/></pubsub>`
	if got := callPeer("set", sub); !strings.Contains(got, "forbidden") {
		t.Fatal(got)
	}
	if got := callOwner("set", `<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><configure node='urn:test:private'><x xmlns='jabber:x:data'><field var='pubsub#access_model'><value>open</value></field></x></configure></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if got := callPeer("set", strings.Replace(sub, "jid='bob@example.com'", "jid='mallory@example.com'", 1)); !strings.Contains(got, "forbidden") {
		t.Fatal(got)
	}
	if got := callPeer("set", sub); !strings.Contains(got, "subscription='subscribed'") {
		t.Fatal(got)
	}
	if got := callPeer("set", sub); !strings.Contains(got, "subscription='subscribed'") {
		t.Fatal(got)
	}
	var count int
	if err := srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_pep_subscriptions`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d %v", count, err)
	}
	// Simulate a fresh process using the same durable subscription data.
	fresh := New(srv.cfg, srv.log, store, nil, nil)
	owner.srv = fresh
	peer.srv = fresh
	fresh.hub.Register(peer)
	pub := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='urn:test:private'><item><data xmlns='urn:test'>value</data></item></publish></pubsub>`
	peerOut.Reset()
	if got := callOwner("set", pub); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	if !strings.Contains(peerOut.String(), "type='headline'") {
		t.Fatal("explicit subscription lost on restart")
	}
	// Tightening access revokes delivery even though the explicit subscription persists.
	if got := callOwner("set", `<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><configure node='urn:test:private'><x xmlns='jabber:x:data'><field var='pubsub#access_model'><value>presence</value></field></x></configure></pubsub>`); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	peerOut.Reset()
	callOwner("set", pub)
	if peerOut.Len() != 0 {
		t.Fatal("private data leaked through old subscription")
	}
	if err := fresh.upsertRoster(ctx, alice.ID, rosterItem{JID: peer.JID.Bare(), Subscription: "from"}); err != nil {
		t.Fatal(err)
	}
	if got := callPeer("get", `<pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='urn:test:private'/></pubsub>`); !strings.Contains(got, ">value<") {
		t.Fatal(got)
	}
	if got := callPeer("set", strings.Replace(sub, "<subscribe ", "<unsubscribe ", 1)); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	peerOut.Reset()
	callOwner("set", pub)
	if peerOut.Len() != 0 {
		t.Fatal("unsubscribed resource still received events")
	}
	if got := callPeer("set", `<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><delete node='urn:test:private'/></pubsub>`); !strings.Contains(got, "forbidden") {
		t.Fatal(got)
	}
}

func TestPEPFeaturesCorrelatedAndSendLast(t *testing.T) {
	srv, _, user := testServer(t)
	ctx := context.Background()
	var out bytes.Buffer
	node := "eu.siacs.conversations.axolotl.devicelist"
	if err := srv.publishPEP(ctx, user.Email, node, "current", `<list xmlns='eu.siacs.conversations.axolotl'><device id='42'/></list>`); err != nil {
		t.Fatal(err)
	}
	s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/monal"), bw: bufio.NewWriter(&out)}
	s.requestPEPFeatures(`<c xmlns='http://jabber.org/protocol/caps' node='https://monal-im.org' ver='abc'/>`)
	var request struct {
		ID string `xml:"id,attr"`
	}
	if err := xml.Unmarshal(out.Bytes(), &request); err != nil {
		t.Fatal(err)
	}
	response := `<query xmlns='http://jabber.org/protocol/disco#info' node='https://monal-im.org#abc'><feature var='` + node + `+notify'/></query>`
	out.Reset()
	if s.handlePEPFeatures("wrong", "result", s.JID.Full(), s.JID.Domain, response) {
		t.Fatal("uncorrelated result accepted")
	}
	s.handlePEPFeatures(request.ID, "result", "other@example.com/phone", s.JID.Domain, response)
	if s.pepInterest[node] || out.Len() != 0 {
		t.Fatal("forged sender accepted")
	}
	if !s.handlePEPFeatures(request.ID, "result", s.JID.Full(), s.JID.Domain, response) {
		t.Fatal("result not consumed")
	}
	if !s.pepInterest[node] || !strings.Contains(out.String(), "type='headline'") || !strings.Contains(out.String(), "device id=") {
		t.Fatal(out.String())
	}
	out.Reset()
	s.requestPEPFeatures(`<c xmlns='http://jabber.org/protocol/caps' node='https://monal-im.org' ver='abc'/>`)
	if out.Len() != 0 {
		t.Fatal("unchanged presence caused discovery loop")
	}
}

func TestPEPCreateConfigureAndNamespaceValidation(t *testing.T) {
	srv, _, user := testServer(t)
	var out bytes.Buffer
	s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&out)}
	body := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><create node='urn:test:created'/><configure><x xmlns='jabber:x:data'><field var='pubsub#access_model'><value>open</value></field></x></configure></pubsub>`
	if got := pepTestIQ(t, s, &out, "set", "", body); !strings.Contains(got, "type='result'") {
		t.Fatal(got)
	}
	for _, body := range []string{
		`<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish xmlns='urn:wrong' node='urn:test:created'><item><x/></item></publish></pubsub>`,
		`<pubsub xmlns='http://jabber.org/protocol/pubsub'><delete node='urn:test:created'/></pubsub>`,
		`<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><configure node='urn:test:created'><x xmlns='jabber:x:data'><field var='pubsub#access_model'><value>open</value></field><field var='pubsub#access_model'><value>presence</value></field></x></configure></pubsub>`,
	} {
		if got := pepTestIQ(t, s, &out, "set", "", body); !strings.Contains(got, "bad-request") {
			t.Fatal(got)
		}
	}
}
