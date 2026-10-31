package httpapi

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tayga/tms/internal/xmpp"
)

func TestXMPPWebSocketRouteDoesNotUseWebCredentials(t *testing.T) {
	f := newFileFixture(t)
	f.call(-1, "GET", "/xmpp-websocket", "", 404)
	f.s.cfg.XMPP.Enabled = true
	f.s.cfg.XMPP.WebSocket = true
	srv := xmpp.New(f.s.cfg, f.s.log, f.s.store, f.s.authn, nil)
	f.s.SetXMPP(srv)
	f.call(0, "GET", "/xmpp-websocket", "", 403) // Even a valid web token cannot enable plaintext WS.
	https := httptest.NewTLSServer(f.s.Handler())
	defer https.Close()
	defer srv.Shutdown(context.Background())
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, Subprotocols: []string{"xmpp"}} // local test certificate
	c, _, err := dialer.Dial("wss"+strings.TrimPrefix(https.URL, "https")+"/xmpp-websocket", http.Header{
		"Origin":        []string{https.URL},
		"Authorization": []string{"Bearer " + f.tokens[0]},
		"Cookie":        []string{"access_token=" + f.tokens[0]},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	read := func(want string) {
		t.Helper()
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, b, err := c.ReadMessage()
		if err != nil || !strings.Contains(string(b), want) {
			t.Fatalf("want %s; got %s (%v)", want, b, err)
		}
	}
	if err := c.WriteMessage(websocket.TextMessage, []byte(`<open xmlns='urn:ietf:params:xml:ns:xmpp-framing' to='example.com' version='1.0'/>`)); err != nil {
		t.Fatal(err)
	}
	read("<open")
	read("mechanisms")
	if err := c.WriteMessage(websocket.TextMessage, []byte(`<iq xmlns='jabber:client' type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'/></iq>`)); err != nil {
		t.Fatal(err)
	}
	read("not-authorized")
	read("<close")
}
