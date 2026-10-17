package xmpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

type sidMessage struct {
	XMLName xml.Name
	ID      string `xml:"id,attr"`
	From    string `xml:"from,attr"`
	Body    string `xml:"jabber:client body"`
	IDs     []struct {
		By string `xml:"by,attr"`
		ID string `xml:"id,attr"`
	} `xml:"urn:xmpp:sid:0 stanza-id"`
	Origin struct {
		ID string `xml:"id,attr"`
	} `xml:"urn:xmpp:sid:0 origin-id"`
}

func decodeSIDMessage(t *testing.T, raw string) sidMessage {
	t.Helper()
	var m sidMessage
	if err := xml.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err, raw)
	}
	if m.XMLName != (xml.Name{Space: "jabber:client", Local: "message"}) {
		t.Fatal("missing message namespace", raw)
	}
	return m
}
func requireSID(t *testing.T, raw, owner string) string {
	t.Helper()
	m := decodeSIDMessage(t, raw)
	id := ""
	for _, value := range m.IDs {
		if value.By == owner {
			if id != "" {
				t.Fatal("duplicate by", raw)
			}
			id = value.ID
		}
	}
	if id == "" {
		t.Fatal("no stable ID for", owner, raw)
	}
	return id
}
func sidPeer(t *testing.T, store *storage.Store, owner *storage.User) *storage.User {
	t.Helper()
	u, err := store.CreateUser(context.Background(), &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, Email: "bob@example.com", LocalPart: "bob", DisplayName: "Bob", PasswordHash: "x", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return u
}
func sendSIDClient(t *testing.T, srv *Server, user *storage.User, raw string) {
	t.Helper()
	var out bytes.Buffer
	s := &Session{srv: srv, log: srv.log, user: user, JID: ParseJID(user.Email + "/phone"), bw: bufio.NewWriter(&out), dec: xml.NewDecoder(strings.NewReader(raw))}
	token, err := s.dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.handleMessage(token.(xml.StartElement)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "type='error'") {
		t.Fatal(out.String())
	}
}
func mamForwarded(t *testing.T, srv *Server, user *storage.User) []struct{ ID, Raw string } {
	t.Helper()
	var out bytes.Buffer
	s := &Session{srv: srv, user: user, JID: ParseJID(user.Email + "/history"), bw: bufio.NewWriter(&out)}
	if err := s.handleMAMIQ(context.Background(), "mam", "set", `<query xmlns='urn:xmpp:mam:2' queryid='test'/>`); err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(strings.NewReader(out.String()))
	var messages []struct{ ID, Raw string }
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "message" {
			if err := dec.Skip(); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var env struct {
			Result struct {
				ID        string `xml:"id,attr"`
				Forwarded struct {
					Inner string `xml:",innerxml"`
				} `xml:"urn:xmpp:forward:0 forwarded"`
			} `xml:"urn:xmpp:mam:2 result"`
		}
		if err := dec.DecodeElement(&env, &se); err != nil {
			t.Fatal(err)
		}
		inner := env.Result.Forwarded.Inner
		i := strings.Index(inner, "<message")
		if i < 0 {
			t.Fatal("missing forwarded message", inner)
		}
		messages = append(messages, struct{ ID, Raw string }{env.Result.ID, inner[i:]})
	}
	return messages
}

func TestStableIDsAcrossMessageEntryPoints(t *testing.T) {
	for _, kind := range []string{"c2s", "web", "omemo", "bot", "component"} {
		t.Run(kind, func(t *testing.T) {
			srv, store, alice := testServer(t)
			bob := sidPeer(t, store, alice)
			ctx := context.Background()
			var live, carbon bytes.Buffer
			srv.hub.Register(&Session{JID: ParseJID(bob.Email + "/phone"), bw: bufio.NewWriter(&live)})
			srv.hub.Register(&Session{JID: ParseJID(alice.Email + "/desktop"), bw: bufio.NewWriter(&carbon), carbons: true})
			events, cancel := srv.hub.subscribeWeb(bob.Email)
			defer cancel()
			body := "Привет & hello"
			from := alice.Email
			var ev ChatEvent
			var err error
			switch kind {
			case "c2s":
				sendSIDClient(t, srv, alice, `<message to='bob@example.com' id='client-id' type='chat'><body>Привет &amp; hello</body><origin-id xmlns='urn:xmpp:sid:0' id='origin'/></message>`)
			case "web":
				ev, err = srv.SendChatMessage(ctx, alice.Email, bob.Email, body)
			case "omemo":
				ev, err = srv.SendWebOMEMO(ctx, alice.Email, bob.Email, `<encrypted xmlns="eu.siacs.conversations.axolotl"><header sid="42"><key rid="7" prekey="true">YWJj</key><iv>AAAAAAAAAAAAAAAA</iv></header><payload>YWJj</payload></encrypted>`)
			case "bot":
				err = srv.SendBotMessage(ctx, alice.Email, bob.Email, body, "chat")
			case "component":
				from = "helper@bots.example.com"
				cs := &componentSession{srv: srv, domain: "bots.example.com", dec: xml.NewDecoder(strings.NewReader(`<message xmlns='jabber:component:accept' from='helper@bots.example.com' to='bob@example.com' type='chat'><body>Привет &amp; hello</body><stanza-id xmlns='urn:xmpp:sid:0' by='bob@example.com' id='forged'/></message>`))}
				tok, e := cs.dec.Token()
				if e != nil {
					t.Fatal(e)
				}
				err = cs.forwardStanza(tok.(xml.StartElement))
			}
			if err != nil {
				t.Fatal(err)
			}
			id := requireSID(t, live.String(), bob.Email)
			if id == "forged" || id == "client-id" {
				t.Fatal("untrusted ID used", id)
			}
			if ev.ID != "" && ev.ID != id {
				t.Fatal("send response ID mismatch", ev.ID, id)
			}
			m := decodeSIDMessage(t, live.String())
			if kind != "omemo" && m.Body != body {
				t.Fatal("body changed", m.Body)
			}
			if kind == "c2s" && (m.ID != "client-id" || m.Origin.ID != "origin") {
				t.Fatal("client ID lost", m)
			}
			if kind == "omemo" {
				var enc encryptedWire
				if xml.Unmarshal([]byte(encryptedElement(live.String())), &enc) != nil || enc.Payload != "YWJj" || enc.Header.SID != 42 {
					t.Fatal("ciphertext changed", live.String())
				}
			}
			rows, e := srv.queryMAM(ctx, bob.Email, from, 10)
			if e != nil || len(rows) != 1 || rows[0].StanzaID != id || rows[0].Stanza != live.String() {
				t.Fatal("MAM mismatch", rows, e)
			}
			select {
			case event := <-events:
				if event.ID != id {
					t.Fatal("SSE mismatch", event.ID, id)
				}
			default:
				t.Fatal("no SSE message")
			}
			history, e := srv.ChatHistory(ctx, bob.Email, from, 10)
			if e != nil || len(history) != 1 || history[0].ID != id {
				t.Fatal("web history mismatch", history, e)
			}
			forwarded := mamForwarded(t, srv, bob)
			if len(forwarded) != 1 || forwarded[0].ID != id || requireSID(t, forwarded[0].Raw, bob.Email) != id {
				t.Fatal("MAM wire mismatch", forwarded)
			}
			if kind != "component" && (!strings.Contains(carbon.String(), "<sent") || !strings.Contains(carbon.String(), id)) {
				t.Fatal("carbon lost ID", carbon.String())
			}
		})
	}
}
func TestStableIDsIgnoreCollidingClientIDsAndPersistOffline(t *testing.T) {
	srv, store, alice := testServer(t)
	bob := sidPeer(t, store, alice)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		sendSIDClient(t, srv, alice, fmt.Sprintf(`<message to='bob@example.com' id='same'><body>message %d</body></message>`, i))
	}
	rows, err := srv.queryMAM(ctx, bob.Email, alice.Email, 10)
	if err != nil || len(rows) != 2 || rows[0].StanzaID == rows[1].StanzaID {
		t.Fatal("collision lost history", rows, err)
	}
	offline := map[string]bool{}
	if err = srv.flushOffline(ctx, bob.ID, func(raw []byte) { offline[requireSID(t, string(raw), bob.Email)] = true }); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if !offline[row.StanzaID] {
			t.Fatal("offline ID changed", row.StanzaID)
		}
	}
	// History cursors remain valid after a new Server instance reads the same DB.
	restarted := New(srv.cfg, srv.log, store, nil, nil)
	page, _, _, err := restarted.mamPage(ctx, bob.Email, mamFilter{After: &rows[0].StanzaID, Limit: 10})
	if err != nil || len(page) != 1 || page[0].StanzaID != rows[1].StanzaID {
		t.Fatal("cursor instability", page, err)
	}
}
func TestStableIDsReplaceSpoofsEvenWhenArchiveDisabled(t *testing.T) {
	srv, _, alice := testServer(t)
	ctx := context.Background()
	var out bytes.Buffer
	sess := &Session{srv: srv, user: alice, JID: ParseJID(alice.Email + "/phone"), bw: bufio.NewWriter(&out)}
	if err := sess.handleMAMIQ(ctx, "prefs", "set", `<prefs xmlns='urn:xmpp:mam:2' default='never'/>`); err != nil {
		t.Fatal(err)
	}
	raw := `<message from='alice@example.com/phone' to='remote@example.net' id='client'><body>hello</body><stanza-id xmlns='urn:xmpp:sid:0' by='ALICE@EXAMPLE.COM' id='forged1'/><stanza-id xmlns='urn:xmpp:sid:0' by='alice@example.com' id='forged2'/><stanza-id xmlns='urn:xmpp:sid:0' by='remote@example.net' id='remote'/><origin-id xmlns='urn:xmpp:sid:0' id='origin'/><extra xmlns='urn:test'><stanza-id xmlns='urn:xmpp:sid:0' by='alice@example.com' id='nested'/></extra></message>`
	stanza, id, err := srv.prepareArchivedMessage(ctx, raw)
	if err != nil || strings.Contains(stanza, "forged") || requireSID(t, stanza, alice.Email) != id {
		t.Fatal(stanza, err)
	}
	if !strings.Contains(stanza, "nested") || requireSID(t, stanza, "remote@example.net") != "remote" || decodeSIDMessage(t, stanza).Origin.ID != "origin" {
		t.Fatal("unrelated IDs changed", stanza)
	}
	rows, err := srv.queryMAM(ctx, alice.Email, "", 10)
	if err != nil || len(rows) != 0 {
		t.Fatal("preferences ignored", rows, err)
	}
}
func TestArchiveFailureDoesNotDeliverOrCommitPartialHistory(t *testing.T) {
	srv, store, alice := testServer(t)
	bob := sidPeer(t, store, alice)
	_, err := srv.db.Exec(`CREATE TRIGGER fail_bob_mam BEFORE INSERT ON xmpp_mam WHEN NEW.owner_bare_jid='bob@example.com' BEGIN SELECT RAISE(ABORT,'test failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	var live bytes.Buffer
	srv.hub.Register(&Session{JID: ParseJID(bob.Email + "/phone"), bw: bufio.NewWriter(&live)})
	if _, err = srv.SendChatMessage(context.Background(), alice.Email, bob.Email, "hello"); err == nil {
		t.Fatal("failure hidden")
	}
	var count int
	if err = srv.db.QueryRow(`SELECT COUNT(*) FROM xmpp_mam`).Scan(&count); err != nil || count != 0 || live.Len() != 0 {
		t.Fatal("partial delivery", count, live.String(), err)
	}
}
func TestLegacyMAMIDsAndNamespacesPreserved(t *testing.T) {
	srv, _, u := testServer(t)
	if err := srv.archiveMAM(context.Background(), u.Email, "bob@example.com", "old-cursor", `<message from='bob@example.com' to='alice@example.com' id='client'><body>old</body><stanza-id xmlns='urn:xmpp:sid:0' by='alice@example.com' id='fake'/></message>`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		messages := mamForwarded(t, srv, u)
		if len(messages) != 1 || messages[0].ID != "old-cursor" || requireSID(t, messages[0].Raw, u.Email) != "old-cursor" || decodeSIDMessage(t, messages[0].Raw).Body != "old" {
			t.Fatal(messages)
		}
	}
}
func TestStableIDDiscoveryScope(t *testing.T) {
	srv, store, alice := testServer(t)
	bob := sidPeer(t, store, alice)
	for _, to := range []string{"", srv.domain, alice.Email, bob.Email, "missing@example.com", "alice@example.com/phone", "remote.invalid"} {
		var out bytes.Buffer
		s := &Session{srv: srv, user: alice, JID: ParseJID(alice.Email + "/phone"), bw: bufio.NewWriter(&out)}
		if err := s.writeDiscoInfo("disco", to); err != nil {
			t.Fatal(err)
		}
		want := to == "" || to == srv.domain || to == alice.Email || to == bob.Email
		if strings.Contains(out.String(), `var='urn:xmpp:sid:0'`) != want {
			t.Fatal(to, out.String())
		}
		if want && !strings.Contains(out.String(), "pubsub#retrieve-items") {
			t.Fatal("account PEP discovery lost", to, out.String())
		}
		if strings.Contains(out.String(), `var='urn:xmpp:push:0'`) {
			t.Fatal("unavailable push advertised")
		}
	}
}

func TestSelfMessageUsesOneArchiveAndOneWebEvent(t *testing.T) {
	srv, _, u := testServer(t)
	ctx := context.Background()
	events, cancel := srv.hub.subscribeWeb(u.Email)
	defer cancel()
	ev, err := srv.SendChatMessage(ctx, u.Email, u.Email, "note to self")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := srv.queryMAM(ctx, u.Email, u.Email, 10)
	if err != nil || len(rows) != 1 || rows[0].StanzaID != ev.ID {
		t.Fatal(rows, err)
	}
	if event := <-events; event.ID != ev.ID {
		t.Fatal(event)
	}
	select {
	case duplicate := <-events:
		t.Fatal("duplicate self event", duplicate)
	default:
	}
}
func TestMessagesWithoutClientIDsHaveDistinctStableIDs(t *testing.T) {
	srv, _, u := testServer(t)
	raw := `<message to='alice@example.com'><body>no id</body></message>`
	sendSIDClient(t, srv, u, raw)
	sendSIDClient(t, srv, u, raw)
	rows, err := srv.queryMAM(context.Background(), u.Email, u.Email, 10)
	if err != nil || len(rows) != 2 || rows[0].StanzaID == rows[1].StanzaID {
		t.Fatal(rows, err)
	}
	for _, row := range rows {
		if requireSID(t, row.Stanza, u.Email) != row.StanzaID {
			t.Fatal(row)
		}
	}
}
