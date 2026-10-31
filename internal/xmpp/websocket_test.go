package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

type wsWireClient struct {
	t    *testing.T
	conn *websocket.Conn
}

func wsFixture(t *testing.T) (*Server, *storage.Store, *storage.User, string) {
	t.Helper()
	srv, st, user := testServer(t)
	srv.cfg.XMPP.WebSocket, srv.cfg.XMPP.RequireTLS = true, true
	srv.authn = auth.NewLayer(st, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateUserPassword(context.Background(), user.ID, hash); err != nil {
		t.Fatal(err)
	}
	https := httptest.NewTLSServer(http.HandlerFunc(srv.ServeWebSocket))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		https.Close()
		deadline := time.Now().Add(5 * time.Second)
		for {
			srv.mu.Lock()
			n := srv.webSocketCount
			srv.mu.Unlock()
			if n == 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Error("WebSocket handlers did not stop")
				break
			}
			time.Sleep(time.Millisecond)
		}
	})
	return srv, st, user, "wss" + strings.TrimPrefix(https.URL, "https") + "/xmpp-websocket"
}

func wsDial(t *testing.T, endpoint string) *wsWireClient {
	t.Helper()
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, Subprotocols: []string{"xmpp"}, HandshakeTimeout: 3 * time.Second} // disposable loopback certificate
	c, _, err := dialer.Dial(endpoint, http.Header{"Origin": []string{strings.Replace(strings.TrimSuffix(endpoint, "/xmpp-websocket"), "wss:", "https:", 1)}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Subprotocol() != "xmpp" {
		t.Fatal("missing xmpp subprotocol")
	}
	t.Cleanup(func() { _ = c.Close() })
	return &wsWireClient{t, c}
}

func (c *wsWireClient) send(raw string) {
	c.t.Helper()
	_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
		c.t.Fatal(err)
	}
}

func (c *wsWireClient) next() resumeWireElement {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	typ, raw, err := c.conn.ReadMessage()
	if err != nil {
		c.t.Fatal(err)
	}
	if typ != websocket.TextMessage {
		c.t.Fatal("non-text frame")
	}
	if _, _, err = decodeWebSocketFrame(raw); err != nil {
		c.t.Fatalf("non-standalone XML: %s: %v", raw, err)
	}
	var e resumeWireElement
	if err := xml.Unmarshal(raw, &e); err != nil {
		c.t.Fatal(err)
	}
	return e
}

func (c *wsWireClient) want(local, id string) resumeWireElement {
	c.t.Helper()
	e := c.next()
	for e.XMLName == (xml.Name{Space: smNamespace, Local: "r"}) {
		e = c.next()
	}
	if e.XMLName.Local != local || (id != "" && e.ID != id) {
		c.t.Fatalf("want %s %s; got %+v", local, id, e)
	}
	return e
}

func (c *wsWireClient) open() {
	c.t.Helper()
	c.send(`<open xmlns='` + framingNS + `' to='example.com' version='1.0'/>`)
	if e := c.want("open", ""); e.XMLName.Space != framingNS || e.ID == "" {
		c.t.Fatal(e)
	}
	e := c.want("features", "")
	if e.XMLName.Space != streamNS || strings.Contains(e.Inner, "starttls") {
		c.t.Fatal(e)
	}
}

func (c *wsWireClient) login(password, resource string) {
	c.t.Helper()
	c.open()
	c.send(`<auth xmlns='urn:ietf:params:xml:ns:xmpp-sasl' mechanism='PLAIN'>` + base64.StdEncoding.EncodeToString([]byte("\x00alice\x00"+password)) + `</auth>`)
	c.want("success", "")
	c.open()
	if resource != "" {
		c.send(`<iq xmlns='jabber:client' type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>` + resource + `</resource></bind></iq>`)
		e := c.want("iq", "bind")
		if !strings.Contains(e.Inner, "alice@example.com/"+resource) {
			c.t.Fatal(e)
		}
	}
}

func TestWebSocketTransportTCPArchiveAndResume(t *testing.T) {
	srv, st, u, endpoint := wsFixture(t)
	ctx := context.Background()
	bob, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := srv.CreateChatRoom(ctx, u.Email, "WebSocket group", []string{bob.Email})
	if err != nil {
		t.Fatal(err)
	}
	audience, err := srv.RoomOMEMOContext(ctx, u.Email, room.JID)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := srv.SendRoomOMEMO(ctx, u.Email, room.JID, audience.Revision, groupCipherFixture)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	tcp := newResumeWireClient(t, srv, ln, "tcp")
	tcp.send(`<iq type='set' id='no-carbons'><disable xmlns='urn:xmpp:carbons:2'/></iq>`)
	tcp.want("iq", "no-carbons")
	ws := wsDial(t, endpoint)
	ws.login("fixture-password", "browser")
	ws.send(`<message xmlns='jabber:client' type='chat' to='alice@example.com/tcp' id='from-wss'><body>Across transports</body></message>`)
	if e := tcp.want("message", "from-wss"); !strings.Contains(e.Inner, "Across transports") {
		t.Fatal(e)
	}
	tcp.send(`<message type='chat' to='alice@example.com/browser' id='from-tcp'><body>Return path</body></message>`)
	if e := ws.want("message", "from-tcp"); !strings.Contains(e.Inner, "Return path") {
		t.Fatal(e)
	}
	ws.send(`<iq xmlns='jabber:client' type='get' id='roster'><query xmlns='jabber:iq:roster'/></iq>`)
	ws.want("iq", "roster")
	ws.send(`<iq xmlns='jabber:client' type='get' id='bookmarks'><pubsub xmlns='http://jabber.org/protocol/pubsub'><items node='urn:xmpp:bookmarks:1'/></pubsub></iq>`)
	if e := ws.want("iq", "bookmarks"); !strings.Contains(e.Inner, room.JID) {
		t.Fatal(e)
	}
	ws.send(`<iq xmlns='jabber:client' type='set' to='` + room.JID + `' id='archive'><query xmlns='urn:xmpp:mam:2' queryid='ws-history'><set xmlns='http://jabber.org/protocol/rsm'><max>10</max></set></query></iq>`)
	e := ws.want("message", "")
	if !strings.Contains(e.Inner, cipher.ID) || !strings.Contains(e.Inner, "encrypted") || !strings.Contains(e.Inner, "alice@example.com") {
		t.Fatal(e)
	}
	ws.want("iq", "archive")
	ws.send(`<presence xmlns='jabber:client' to='` + room.JID + `/Browser'><x xmlns='` + mucNS + `'><history maxstanzas='0'/></x></presence>`)
	if presence := ws.want("presence", ""); !strings.Contains(presence.Inner, "alice@example.com/browser") {
		t.Fatal("missing author identity in room presence", presence)
	}
	ws.want("message", "") // Room subject only; encrypted history uses MAM.
	tcp.send(`<presence to='` + room.JID + `/Desktop'><x xmlns='` + mucNS + `'><history maxstanzas='0'/></x></presence>`)
	tcp.want("presence", "")
	tcp.want("presence", "")
	tcp.want("message", "")
	ws.want("presence", "")
	ws.send(`<message xmlns='jabber:client' to='` + room.JID + `' type='groupchat' id='wss-omemo'><body>DO-NOT-LEAK</body>` + groupCipherFixture + `</message>`)
	for _, live := range []resumeWireElement{ws.want("message", ""), tcp.want("message", "")} {
		if !strings.Contains(live.Inner, "encrypted") || strings.Contains(live.Inner, "DO-NOT-LEAK") {
			t.Fatal(live)
		}
	}
	page, err := srv.RoomHistoryPage(ctx, u.Email, room.JID, "", 10)
	if err != nil || len(page.Messages) != 2 || !page.Messages[1].Encrypted || page.Messages[1].Body != "" {
		t.Fatal(page, err)
	}
	// Leave before the transport-only resume check to avoid room join traffic.
	ws.send(`<presence xmlns='jabber:client' to='` + room.JID + `/Browser' type='unavailable'/>`)
	ws.want("presence", "")
	ws.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := ws.want("enabled", "").ID
	if token == "" {
		t.Fatal("missing resume token")
	}
	if !srv.hub.Route(u.Email+"/browser", []byte(`<message id='pending' from='bob@example.com'><body>Unacknowledged</body></message>`)) {
		t.Fatal("routing failed")
	}
	ws.want("message", "pending")
	// Reconnect while the old connection is still live: only its transport is replaced.
	resumed := wsDial(t, endpoint)
	resumed.login("fixture-password", "")
	resumed.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='0'/>`)
	resumed.want("resumed", "")
	resumed.want("message", "pending")
	resumed.send(`<a xmlns='urn:xmpp:sm:3' h='1'/>`)
	resumed.send(`<iq xmlns='jabber:client' type='get' id='ping'><ping xmlns='urn:xmpp:ping'/></iq>`)
	resumed.want("iq", "ping")
	resumed.send(`<close xmlns='` + framingNS + `'/>`)
	resumed.want("close", "")
	_ = resumed.conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := resumed.conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("expected normal close, got %v", err)
	}
	srv.resumeMu.Lock()
	_, exists := srv.resumable[token]
	srv.resumeMu.Unlock()
	if exists {
		t.Fatal("clean close retained resumable state")
	}
}

func TestWebSocketDeviceRevocationAndAbruptRecovery(t *testing.T) {
	srv, _, u, endpoint := wsFixture(t)
	ctx := context.Background()
	device, password, err := srv.authn.CreateXMPPDevicePassword(ctx, u.ID, "WSS fixture")
	if err != nil {
		t.Fatal(err)
	}
	ws := wsDial(t, endpoint)
	ws.login(password, "device")
	ws.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := ws.want("enabled", "").ID
	sessions := srv.hub.Sessions(u.Email)
	if len(sessions) != 1 {
		t.Fatal("session not tracked")
	}
	ws.conn.Close()
	select {
	case <-sessions[0].done:
	case <-time.After(4 * time.Second):
		t.Fatal("reader remained")
	}
	reconnected := wsDial(t, endpoint)
	reconnected.login(password, "")
	reconnected.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='0'/>`)
	reconnected.want("resumed", "")
	if e := reconnected.next(); e.XMLName != (xml.Name{Space: smNamespace, Local: "r"}) {
		t.Fatal(e)
	}
	if err := srv.authn.RevokeXMPPDevicePassword(ctx, u.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	srv.DropDeviceStreams(u.ID, device.ID)
	_ = reconnected.conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := reconnected.conn.ReadMessage(); err == nil {
		t.Fatal("revoked connection stayed open")
	}
	failed := wsDial(t, endpoint)
	failed.open()
	failed.send(`<auth xmlns='urn:ietf:params:xml:ns:xmpp-sasl' mechanism='PLAIN'>` + base64.StdEncoding.EncodeToString([]byte("\x00alice\x00"+password)) + `</auth>`)
	failed.want("failure", "")
	if len(srv.authn.Sessions()) != 0 {
		t.Fatal("revoked session persisted")
	}
}

func TestWebSocketRejectsInvalidFramesAndState(t *testing.T) {
	_, _, _, endpoint := wsFixture(t)
	for _, tc := range []struct {
		name, raw, want string
		open, login     bool
		binary          bool
	}{
		{"missing-open", `<iq xmlns='jabber:client'/>`, "policy-violation", false, false, false},
		{"wrong-open-namespace", `<open xmlns='urn:wrong'/>`, "invalid-namespace", false, false, false},
		{"unknown-domain", `<open xmlns='` + framingNS + `' to='foreign.example' version='1.0'/>`, "host-unknown", false, false, false},
		{"wrong-version", `<open xmlns='` + framingNS + `' to='example.com' version='0.9'/>`, "unsupported-version", false, false, false},
		{"two-stanzas", `<iq xmlns='jabber:client'/><iq xmlns='jabber:client'/>`, "not-well-formed", true, false, false},
		{"partial-stanza", `<iq xmlns='jabber:client'>`, "not-well-formed", true, false, false},
		{"DTD", `<!DOCTYPE iq><iq xmlns='jabber:client'/>`, "not-well-formed", true, false, false},
		{"missing-prefix", `<iq xmlns='jabber:client'><q:query/></iq>`, "not-well-formed", true, false, false},
		{"duplicate-attribute", `<iq xmlns='jabber:client' id='a' id='b'/>`, "not-well-formed", true, false, false},
		{"whitespace", ` `, "not-well-formed", true, false, false},
		{"bad-UTF8", "<iq>\xff</iq>", "not-well-formed", true, false, false},
		{"deep", `<iq xmlns='jabber:client'>` + strings.Repeat("<x>", 65) + strings.Repeat("</x>", 65) + `</iq>`, "not-well-formed", true, false, false},
		{"binary", `<iq xmlns='jabber:client'/>`, "unsupported-stanza-type", true, false, true},
		{"preauth-bind", `<iq xmlns='jabber:client' type='set'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'/></iq>`, "not-authorized", true, false, false},
		{"preauth-message", `<message xmlns='jabber:client' to='alice@example.com'><body>unauthorized</body></message>`, "not-authorized", true, false, false},
		{"STARTTLS", `<starttls xmlns='urn:ietf:params:xml:ns:xmpp-tls'/>`, "policy-violation", true, false, false},
		{"reopen-bound", `<open xmlns='` + framingNS + `' to='example.com' version='1.0'/>`, "policy-violation", false, true, false},
		{"bound-missing-namespace", `<iq type='get'><ping xmlns='urn:xmpp:ping'/></iq>`, "invalid-namespace", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := wsDial(t, endpoint)
			if tc.open {
				c.open()
			}
			if tc.login {
				c.login("fixture-password", "bad-frame")
			}
			if tc.binary {
				if err := c.conn.WriteMessage(websocket.BinaryMessage, []byte(tc.raw)); err != nil {
					t.Fatal(err)
				}
			} else {
				c.send(tc.raw)
			}
			if !tc.open && !tc.login {
				c.want("open", "")
			}
			e := c.want("error", "")
			if e.XMLName.Space != streamNS || !strings.Contains(e.Inner, tc.want) {
				t.Fatal(e)
			}
			c.want("close", "")
		})
	}
	t.Run("oversized", func(t *testing.T) {
		c := wsDial(t, endpoint)
		c.open()
		c.send(`<message xmlns='jabber:client'><body>` + strings.Repeat("a", maxWebSocketFrame) + `</body></message>`)
		_ = c.conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, _, err := c.conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
			t.Fatal(err)
		}
	})
	t.Run("SASL-restart-required", func(t *testing.T) {
		c := wsDial(t, endpoint)
		c.open()
		c.send(`<auth xmlns='urn:ietf:params:xml:ns:xmpp-sasl' mechanism='PLAIN'>` + base64.StdEncoding.EncodeToString([]byte("\x00alice\x00fixture-password")) + `</auth>`)
		c.want("success", "")
		c.send(`<iq xmlns='jabber:client' type='set'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'/></iq>`)
		c.want("open", "") // SASL implicitly closed the previous XML stream.
		if e := c.want("error", ""); !strings.Contains(e.Inner, "policy-violation") {
			t.Fatal(e)
		}
	})
}

func TestWebSocketHTTPGuards(t *testing.T) {
	srv, _, _ := testServer(t)
	srv.cfg.XMPP.WebSocket = true
	for _, tc := range []struct {
		name, method, origin, protocol, query string
		tls                                   bool
		want                                  int
	}{
		{"plain", "GET", "", "xmpp", "", false, 403},
		{"cross-origin", "GET", "https://evil.example", "xmpp", "", true, 403},
		{"null-origin", "GET", "null", "xmpp", "", true, 403},
		{"insecure-origin", "GET", "http://example.com", "xmpp", "", true, 403},
		{"origin-with-path", "GET", "https://example.com/path", "xmpp", "", true, 403},
		{"missing-protocol", "GET", "https://example.com", "", "", true, 400},
		{"wrong-protocol", "GET", "https://example.com", "other", "", true, 400},
		{"query-token", "GET", "https://example.com", "xmpp", "?token=not-accepted", true, 403},
		{"method", "POST", "https://example.com", "xmpp", "", true, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "https://example.com/xmpp-websocket"+tc.query, nil)
			if !tc.tls {
				r.TLS = nil
				r.Header.Set("X-Forwarded-Proto", "https")
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			r.Header.Set("Sec-WebSocket-Protocol", tc.protocol)
			w := httptest.NewRecorder()
			srv.ServeWebSocket(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
	r := httptest.NewRequest("GET", "https://example.com/xmpp-websocket", nil)
	r.Header.Set("Sec-WebSocket-Protocol", "xmpp")
	srv.webSocketCount = maxWebSocketConnections
	w := httptest.NewRecorder()
	srv.ServeWebSocket(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	srv.cfg.XMPP.WebSocket = false
	w = httptest.NewRecorder()
	srv.ServeWebSocket(w, r)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	for _, origin := range []string{"https://example.com", "https://EXAMPLE.com:443"} {
		r.Header.Set("Origin", origin)
		if !webSocketOriginAllowed(r) {
			t.Fatal(origin)
		}
	}
	r.Header["Origin"] = []string{"https://example.com", "https://evil.example"}
	if webSocketOriginAllowed(r) {
		t.Fatal("multiple origins accepted")
	}
}

func TestWebSocketShutdownClosesPreauthConnection(t *testing.T) {
	srv, _, _, endpoint := wsFixture(t)
	c := wsDial(t, endpoint)
	c.open()
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := c.conn.ReadMessage(); err == nil {
		t.Fatal("preauth transport survived shutdown")
	}
}

func TestWebSocketFrameKeepsNamespacesAndLargeText(t *testing.T) {
	_, _, _, endpoint := wsFixture(t)
	c := wsDial(t, endpoint)
	c.login("fixture-password", "large")
	body := strings.Repeat("Привет", 3000)
	c.send(fmt.Sprintf(`<message xmlns='jabber:client' type='chat' to='alice@example.com/large' id='large'><body>%s</body></message>`, body))
	if e := c.want("message", "large"); !strings.Contains(e.Inner, body) {
		t.Fatal("fragmented message changed")
	}
}
