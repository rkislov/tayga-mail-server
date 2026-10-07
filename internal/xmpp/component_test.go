package xmpp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
)

func TestComponentConnectAndRoute(t *testing.T) {
	srv, _, u := testServer(t)
	srv.cfg.XMPP.Components = []config.XMPPComponentConfig{{
		Name: "bots", Subdomain: "bots", Secret: "comp-secret",
	}}
	srv.domain = "example.com"

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go srv.acceptComponents(ln)

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(conn)

	mustWrite := func(s string) {
		t.Helper()
		if _, err := io.WriteString(conn, s); err != nil {
			t.Fatal(err)
		}
	}
	readUntil := func(substr string) string {
		t.Helper()
		var b strings.Builder
		buf := make([]byte, 4096)
		deadline := time.Now().Add(3 * time.Second)
		for !strings.Contains(b.String(), substr) {
			_ = conn.SetReadDeadline(deadline)
			n, err := br.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				t.Fatalf("waiting %q: %v\n%s", substr, err, b.String())
			}
		}
		return b.String()
	}

	mustWrite(`<?xml version='1.0'?><stream:stream xmlns='jabber:component:accept' xmlns:stream='http://etherx.jabber.org/streams' to='bots.example.com'>`)
	open := readUntil(`id='`)
	i := strings.Index(open, `id='`)
	rest := open[i+4:]
	j := strings.IndexByte(rest, '\'')
	streamID := rest[:j]
	mustWrite(fmt.Sprintf(`<handshake>%s</handshake>`, ComponentHandshakeDigest(streamID, "comp-secret")))
	readUntil("handshake")

	mustWrite(fmt.Sprintf(
		`<message from='echo@bots.example.com' to='%s' type='chat' id='m1'><body>from-bot</body></message>`,
		u.Email,
	))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := srv.queryMAM(context.Background(), u.Email, "echo@bots.example.com", 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected MAM entry from component message")
}
