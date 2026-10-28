package xmpp

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
)

type resumeWireElement struct {
	XMLName xml.Name
	ID      string `xml:"id,attr"`
	H       string `xml:"h,attr"`
	Inner   string `xml:",innerxml"`
}

type resumeWireClient struct {
	t       *testing.T
	conn    net.Conn
	decoder *xml.Decoder
	session *Session
	done    chan struct{}
}

func (c *resumeWireClient) send(raw string) {
	c.t.Helper()
	_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(c.conn, raw); err != nil {
		c.t.Fatal(err)
	}
}

func (c *resumeWireClient) next() resumeWireElement {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	for {
		token, err := c.decoder.Token()
		if err != nil {
			c.t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local == "stream" {
			continue
		}
		var e resumeWireElement
		if err := c.decoder.DecodeElement(&e, &start); err != nil {
			c.t.Fatal(err)
		}
		return e
	}
}

func (c *resumeWireClient) want(local, id string) resumeWireElement {
	c.t.Helper()
	e := c.next()
	// Acknowledgement requests can follow ordinary stanzas.
	for e.XMLName == (xml.Name{Space: smNamespace, Local: "r"}) {
		e = c.next()
	}
	if e.XMLName.Local != local || (id != "" && e.ID != id) {
		c.t.Fatalf("want %s %s; got %+v", local, id, e)
	}
	return e
}

func newResumeWireClient(t *testing.T, srv *Server, listener net.Listener, resource string) *resumeWireClient {
	t.Helper()
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := listener.Accept()
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	s := newSession(srv, serverConn, true) // local fixture; TLS transport has separate tests
	c := &resumeWireClient{t: t, conn: conn, decoder: xml.NewDecoder(conn), session: s, done: make(chan struct{})}
	go func() { s.serve(); close(c.done) }()
	t.Cleanup(func() {
		conn.Close()
		select {
		case <-c.done:
		case <-time.After(4 * time.Second):
			t.Error("session did not stop")
		}
	})
	header := `<stream:stream xmlns='jabber:client' xmlns:stream='http://etherx.jabber.org/streams' to='example.com' version='1.0'>`
	c.send(header)
	c.want("features", "")
	c.send(`<auth xmlns='urn:ietf:params:xml:ns:xmpp-sasl' mechanism='PLAIN'>` + base64.StdEncoding.EncodeToString([]byte("\x00alice\x00fixture-password")) + `</auth>`)
	c.want("success", "")
	c.decoder = xml.NewDecoder(conn)
	c.send(header)
	c.want("features", "")
	if resource != "" {
		c.send(`<iq type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>` + resource + `</resource></bind></iq>`)
		c.want("iq", "bind")
		c.send(`<iq type='set' id='carbons'><enable xmlns='urn:xmpp:carbons:2'/></iq>`)
		c.want("iq", "carbons")
	}
	return c
}

func TestResumeReplacesUndetectedConnectionAndKeepsSecondDevice(t *testing.T) {
	srv, store, u := testServer(t)
	srv.authn = auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	hash, err := srv.authn.Hasher.Hash("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateUserPassword(context.Background(), u.ID, hash); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	phone := newResumeWireClient(t, srv, ln, "phone")
	desktop := newResumeWireClient(t, srv, ln, "desktop")
	phone.send(`<enable xmlns='urn:xmpp:sm:3' resume='true'/>`)
	token := phone.want("enabled", "").ID
	if token == "" {
		t.Fatal("missing SM token")
	}
	for _, id := range []string{"first", "second"} {
		if !srv.hub.Route(u.Email+"/phone", []byte(`<message from='bob@example.com' to='`+u.Email+`/phone' type='chat' id='`+id+`'><body>`+id+`</body></message>`)) {
			t.Fatal("route failed")
		}
		phone.want("message", id)
		carbon := desktop.want("message", "")
		if !strings.Contains(carbon.Inner, "<received") || !strings.Contains(carbon.Inner, id) {
			t.Fatalf("missing received carbon: %s", carbon.Inner)
		}
	}
	phone.send(`<a xmlns='urn:xmpp:sm:3' h='1'/><r xmlns='urn:xmpp:sm:3'/>`)
	phone.want("a", "") // barrier: the server has handled h=1
	// The old TCP socket deliberately stays open, as with an undetected Wi-Fi
	// loss. An invalid counter must not evict it, while a valid resume must.
	reconnected := newResumeWireClient(t, srv, ln, "")
	reconnected.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='99'/>`)
	if failed := reconnected.want("failed", ""); !strings.Contains(failed.Inner, "bad-request") {
		t.Fatal(failed)
	}
	phone.send(`<r xmlns='urn:xmpp:sm:3'/>`)
	phone.want("a", "")
	reconnected.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='1'/>`)
	if resumed := reconnected.want("resumed", ""); resumed.H != "0" {
		t.Fatal(resumed)
	}
	reconnected.want("message", "second")
	select {
	case <-phone.done:
	case <-time.After(4 * time.Second):
		t.Fatal("superseded connection still running")
	}
	// Ping acts as a barrier on the other device. Replaying for the phone must
	// not fan out a second carbon for an already delivered message.
	desktop.send(`<iq type='get' id='barrier'><ping xmlns='urn:xmpp:ping'/></iq>`)
	desktop.want("iq", "barrier")
	if len(srv.hub.Sessions(u.Email)) != 2 {
		t.Fatal("other device lost during resume")
	}
	if !srv.hub.Route(u.Email+"/phone", []byte(`<message from='bob@example.com' to='`+u.Email+`/phone' type='chat' id='third'><body>third</body></message>`)) {
		t.Fatal("resumed route missing")
	}
	reconnected.want("message", "third")
	if carbon := desktop.want("message", ""); !strings.Contains(carbon.Inner, "third") {
		t.Fatal(carbon)
	}
	reconnected.send(`<a xmlns='urn:xmpp:sm:3' h='3'/><r xmlns='urn:xmpp:sm:3'/>`)
	reconnected.want("a", "")
	reconnected.session.mu.Lock()
	sent, acknowledged, pending := reconnected.session.sm.sent, reconnected.session.sm.acknowledged, len(reconnected.session.sm.pending)
	reconnected.session.mu.Unlock()
	if sent != 3 || acknowledged != 3 || pending != 0 {
		t.Fatalf("replay changed counters: sent=%d ack=%d pending=%d", sent, acknowledged, pending)
	}
	// A second, detected disconnect must preserve the same stream token too.
	reconnected.send(`<message to='bob@example.com' id='incomplete'><body>`)
	reconnected.conn.Close()
	select {
	case <-reconnected.done:
	case <-time.After(4 * time.Second):
		t.Fatal("disconnected session did not stop")
	}
	resumedAgain := newResumeWireClient(t, srv, ln, "")
	resumedAgain.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='3'/>`)
	if e := resumedAgain.want("resumed", ""); e.H != "0" {
		t.Fatal("partial stanza was acknowledged", e)
	}
	resumedAgain.send(`<message to='bob@example.com' type='chat' id='outgoing'><body>from resumed phone</body></message>`)
	carbon := desktop.want("message", "")
	if !strings.Contains(carbon.Inner, "<sent") || !strings.Contains(carbon.Inner, "outgoing") {
		t.Fatalf("sent carbons not restored: %s", carbon.Inner)
	}
	// Explicit stream closure is different from the transport losses above.
	resumedAgain.send(`</stream:stream>`)
	select {
	case <-resumedAgain.done:
	case <-time.After(4 * time.Second):
		t.Fatal("clean close did not finish")
	}
	closedClient := newResumeWireClient(t, srv, ln, "")
	closedClient.send(`<resume xmlns='urn:xmpp:sm:3' previd='` + token + `' h='3'/>`)
	if e := closedClient.want("failed", ""); !strings.Contains(e.Inner, "item-not-found") {
		t.Fatal("cleanly closed stream resumed", e)
	}
}
