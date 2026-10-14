package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
	"testing"
)

func TestOMEMOLegacyAndModernPublishRetrieve(t *testing.T) {
	srv, _, user := testServer(t)
	ctx := context.Background()
	var out bytes.Buffer
	session := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&out)}
	call := func(typ, to, body string) string {
		t.Helper()
		out.Reset()
		if err := session.handlePubSubIQ(ctx, "test", typ, to, body); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	for _, node := range []string{"eu.siacs.conversations.axolotl.devicelist", "eu.siacs.conversations.axolotl.bundles:42", "urn:xmpp:omemo:2:bundles"} {
		pub := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='` + node + `'><item id='42'><bundle xmlns='urn:test:bundle'><key>abc</key></bundle></item></publish><publish-options><x xmlns='jabber:x:data' type='submit'><field var='FORM_TYPE'><value>http://jabber.org/protocol/pubsub#publish-options</value></field><field var='pubsub#access_model'><value>open</value></field><field var='pubsub#persist_items'><value>true</value></field></x></publish-options></pubsub>`
		if got := call("set", "", pub); !strings.Contains(got, "type='result'") {
			t.Fatal(got)
		}
		if got := call("set", "other@example.com", pub); !strings.Contains(got, "forbidden") {
			t.Fatal(got)
		}
		session.JID = ParseJID("bob@example.com/laptop")
		got := call("get", user.Email, `<pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='`+node+`'><item id='42'/></items></pubsub>`)
		if !strings.Contains(got, "abc") {
			t.Fatal(got)
		}
		if got = call("get", user.Email, `<pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='`+node+`'><item id='missing'/></items></pubsub>`); strings.Contains(got, "abc") {
			t.Fatal(got)
		}
		session.JID = ParseJID(user.Email + "/phone")
		if got = call("set", "", `<pubsub xmlns='http://jabber.org/protocol/pubsub'><retract node='`+node+`'><item id='42'/></retract></pubsub>`); !strings.Contains(got, "result") {
			t.Fatal(got)
		}
		items, err := srv.getPEPItems(ctx, user.Email, node)
		if err != nil || len(items) != 0 {
			t.Fatalf("%+v %v", items, err)
		}
	}
}
func TestForwardedOMEMOKeepsInheritedNamespaces(t *testing.T) {
	dec := xml.NewDecoder(strings.NewReader(`<message xmlns='jabber:client' xmlns:o='eu.siacs.conversations.axolotl'><o:encrypted><o:header sid='42'><o:key rid='7'>abc</o:key></o:header><o:payload>ciphertext</o:payload></o:encrypted></message>`))
	token, _ := dec.Token()
	inner, err := readInnerXML(dec, token.(xml.StartElement))
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		XMLName xml.Name
		Header  struct {
			XMLName xml.Name
			Key     struct {
				XMLName xml.Name
				Value   string `xml:",chardata"`
			} `xml:"key"`
		} `xml:"header"`
	}
	if err := xml.Unmarshal([]byte(inner), &data); err != nil {
		t.Fatal(err)
	}
	if data.XMLName.Space != "eu.siacs.conversations.axolotl" || data.Header.Key.XMLName.Space != data.XMLName.Space || data.Header.Key.Value != "abc" {
		t.Fatalf("lost namespace: %s", inner)
	}
}

func TestOMEMONotificationsConfigurationAndPersistence(t *testing.T) {
	srv, store, user := testServer(t)
	ctx := context.Background()
	var own, peer bytes.Buffer
	sender := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&own)}
	receiver := &Session{srv: srv, JID: ParseJID("bob@example.com/laptop"), bw: bufio.NewWriter(&peer)}
	srv.hub.Register(receiver)
	if err := srv.upsertRoster(ctx, user.ID, rosterItem{JID: "bob@example.com", Name: "Bob", Subscription: "from", GroupsJSON: "[]"}); err != nil {
		t.Fatal(err)
	}
	node := "eu.siacs.conversations.axolotl.devicelist"
	for _, id := range []string{"old", "current"} {
		body := `<pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='` + node + `'><item id='` + id + `'><list xmlns='eu.siacs.conversations.axolotl'><device id='42'/></list></item></publish><publish-options><x xmlns='jabber:x:data'><field var='pubsub#max_items'><value>1</value></field></x></publish-options></pubsub>`
		if err := sender.handlePubSubIQ(ctx, "p", "set", "", body); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(peer.String(), "to='bob@example.com/laptop'") || !strings.Contains(peer.String(), "current") {
		t.Fatal(peer.String())
	}
	// A fresh server over the same database can read the published key material.
	fresh := New(srv.cfg, srv.log, store, nil, nil)
	items, err := fresh.getPEPItems(ctx, user.Email, node)
	if err != nil || len(items) != 1 || items[0].ItemID != "current" {
		t.Fatalf("%+v %v", items, err)
	}
	own.Reset()
	if err := sender.handlePubSubIQ(ctx, "c", "get", "", `<pubsub xmlns='http://jabber.org/protocol/pubsub#owner'><configure node='`+node+`'/></pubsub>`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(own.String(), "<value>1</value>") {
		t.Fatal(own.String())
	}
	if err := srv.changeSubscription(ctx, user.Email, "bob@example.com", true, true); err != nil {
		t.Fatal(err)
	}
	roster, err := srv.listRoster(ctx, user.ID)
	if err != nil || len(roster) != 1 || roster[0].Subscription != "both" || roster[0].Name != "Bob" {
		t.Fatalf("%+v %v", roster, err)
	}
}

func TestMissingOMEMONodeReturnsItemNotFound(t *testing.T) {
	srv, _, user := testServer(t)
	var out bytes.Buffer
	session := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&out)}
	err := session.handlePubSubIQ(context.Background(), "missing", "get", user.Email, `<pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='eu.siacs.conversations.axolotl.bundles:1151778467'/></pubsub>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "item-not-found") {
		t.Fatal(out.String())
	}
}

func TestDiscoveryUsesAccountDomainInsteadOfHostname(t *testing.T) {
	srv, _, user := testServer(t)
	srv.domain = "mail.example.com"
	for _, to := range []string{"example.com", "", user.Email, "mail.example.com"} {
		var out bytes.Buffer
		session := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&out)}
		if err := session.writeDiscoInfo("disco", to); err != nil {
			t.Fatal(err)
		}
		var reply struct {
			From string `xml:"from,attr"`
		}
		if err := xml.Unmarshal(out.Bytes(), &reply); err != nil {
			t.Fatal(err)
		}
		want := to
		if want == "" {
			want = session.JID.Domain
		}
		if reply.From != want {
			t.Fatalf("query %q: from=%q, want %q", to, reply.From, want)
		}
		if !strings.Contains(out.String(), "pubsub#publish-options") {
			t.Fatal("PEP capabilities missing")
		}
	}
}

func TestMAMResponseCorrelation(t *testing.T) {
	srv, _, user := testServer(t)
	ctx := context.Background()
	if err := srv.archiveMAM(ctx, user.Email, "bob@example.com", "msg1", `<message xmlns='jabber:client'><body>test</body></message>`); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	session := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&out)}
	if err := session.handleMAMIQ(ctx, "iq1", "set", `<query xmlns='urn:xmpp:mam:2' queryid='client-query'/>`); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"queryid='client-query'", "from='" + user.Email + "'", "id='iq1'", "<fin"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s: %s", want, out.String())
		}
	}
}
