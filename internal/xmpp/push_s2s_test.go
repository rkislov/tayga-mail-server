package xmpp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func pushTestCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"example.com", "push.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}
func fakePushGateway(ctx context.Context, conn net.Conn, config *tls.Config, mode string) error {
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	d := xml.NewDecoder(conn)
	open := func() error {
		se, e := nextPushElement(d)
		if e != nil {
			return e
		}
		if se.Name != (xml.Name{Space: streamNS, Local: "stream"}) || attr(se, "from") != "example.com" || attr(se, "to") != "push.example.com" {
			return fmt.Errorf("wrong stream opening")
		}
		return nil
	}
	write := func(raw string) error { _, e := io.WriteString(conn, raw); return e }
	stream := `<stream:stream xmlns:stream='http://etherx.jabber.org/streams' xmlns='jabber:server' from='push.example.com' to='example.com' version='1.0'>`
	if e := open(); e != nil {
		return e
	}
	if mode == "no-tls" {
		return write(stream + `<stream:features/>`)
	}
	if e := write(stream + `<stream:features><starttls xmlns='` + tlsNS + `'/></stream:features>`); e != nil {
		return e
	}
	se, e := nextPushElement(d)
	if e != nil {
		return e
	}
	if se.Name != (xml.Name{Space: tlsNS, Local: "starttls"}) {
		return fmt.Errorf("missing TLS request")
	}
	if e = d.Skip(); e != nil {
		return e
	}
	if e = write(`<proceed xmlns='` + tlsNS + `'/>`); e != nil {
		return e
	}
	secure := tls.Server(conn, config)
	if e = secure.HandshakeContext(ctx); e != nil {
		return e
	}
	conn = secure
	d = xml.NewDecoder(conn)
	if e = open(); e != nil {
		return e
	}
	bidi := `<bidi xmlns='urn:xmpp:features:bidi'/>`
	if strings.HasPrefix(mode, "monal-bidi") {
		bidi = `<bidi xmlns='urn:xmpp:bidi'/>`
	}
	if mode == "wrong-bidi-namespace" {
		bidi = `<bidi xmlns='urn:unrelated:bidi'/>`
	}
	if mode == "no-bidi" {
		bidi = ""
	}
	if e = write(stream + `<stream:features><mechanisms xmlns='` + saslNS + `'><mechanism>EXTERNAL</mechanism></mechanisms>` + bidi + `</stream:features>`); e != nil {
		return e
	}
	if mode == "no-bidi" || mode == "wrong-bidi-namespace" {
		return nil
	}
	se, e = nextPushElement(d)
	if e != nil {
		return e
	}
	if se.Name != (xml.Name{Space: "urn:xmpp:bidi", Local: "bidi"}) {
		return fmt.Errorf("missing bidi request")
	}
	if e = d.Skip(); e != nil {
		return e
	}
	se, e = nextPushElement(d)
	if e != nil {
		return e
	}
	if se.Name != (xml.Name{Space: saslNS, Local: "auth"}) || attr(se, "mechanism") != "EXTERNAL" {
		return fmt.Errorf("wrong authentication")
	}
	var identity string
	if e = d.DecodeElement(&identity, &se); e != nil {
		return e
	}
	decoded, _ := base64.StdEncoding.DecodeString(identity)
	if string(decoded) != "example.com" {
		return fmt.Errorf("wrong auth identity")
	}
	if mode == "auth-failure" || mode == "monal-bidi-auth-failure" {
		return write(`<failure xmlns='` + saslNS + `'><not-authorized/></failure>`)
	}
	if e = write(`<success xmlns='` + saslNS + `'/>`); e != nil {
		return e
	}
	d = xml.NewDecoder(conn)
	if e = open(); e != nil {
		return e
	}
	if e = write(stream + `<stream:features/>`); e != nil {
		return e
	}
	se, e = nextPushElement(d)
	if e != nil {
		return e
	}
	if se.Name != (xml.Name{Space: "jabber:server", Local: "iq"}) || attr(se, "from") != "alice@example.com" || attr(se, "to") != "push.example.com" {
		return fmt.Errorf("wrong publish stanza")
	}
	inner, e := readInnerXML(d, se)
	if e != nil {
		return e
	}
	if !strings.Contains(inner, "urn:xmpp:push:0") {
		return fmt.Errorf("missing push payload")
	}
	from := "push.example.com"
	if mode == "spoofed-reply" {
		from = "other.example.com"
	}
	return write(`<iq type='result' from='` + from + `' to='alice@example.com' id='request'/>`)
}
func TestPushS2SVerifiedTLSExternalAndReply(t *testing.T) {
	cert, pool := pushTestCertificate(t)
	for _, mode := range []string{"success", "monal-bidi", "monal-bidi-auth-failure", "untrusted", "wrong-name", "no-tls", "no-bidi", "wrong-bidi-namespace", "auth-failure", "spoofed-reply"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			done := make(chan error, 1)
			go func() {
				done <- fakePushGateway(ctx, b, &tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, MinVersion: tls.VersionTLS12}, mode)
			}()
			cfg := &tls.Config{ServerName: "push.example.com", RootCAs: pool, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
			if mode == "untrusted" {
				cfg.RootCAs = x509.NewCertPool()
			}
			if mode == "wrong-name" {
				cfg.ServerName = "wrong.example.com"
			}
			typ, _, err := pushS2SExchange(ctx, a, cfg, "example.com", "push.example.com", "alice@example.com", "push.example.com", "request", []byte(`<iq from='alice@example.com' to='push.example.com' type='set' id='request'><pubsub xmlns='http://jabber.org/protocol/pubsub'><publish node='token'><item><notification xmlns='urn:xmpp:push:0'/></item></publish></pubsub></iq>`))
			a.Close()
			serverErr := <-done
			if mode == "success" || mode == "monal-bidi" {
				if err != nil || typ != "result" || serverErr != nil {
					t.Fatalf("exchange: %s %v gateway: %v", typ, err, serverErr)
				}
			} else if err == nil {
				t.Fatal("unsafe gateway accepted")
			}
		})
	}
}
func TestPushPublicAddressesAndCertificateScope(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "2001:db8::1"} {
		if publicPushIP(net.ParseIP(ip)) {
			t.Fatal("accepted", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !publicPushIP(net.ParseIP(ip)) {
			t.Fatal("rejected", ip)
		}
	}
	srv, _, u := testServer(t)
	cert, _ := pushTestCertificate(t)
	srv.tlsCfg = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.cfg.XMPP.PushServices = []string{"push.example.com"}
	if _, err := srv.pushClientCertificate("uncovered.example.com"); err == nil {
		t.Fatal("uncovered domain accepted")
	}
	session := &Session{srv: srv, user: u, JID: ParseJID(u.Email + "/phone")}
	if session.pushDiscoveryFeature(u.Email) == "" {
		t.Fatal("publisher discovery missing")
	}
	if srv.pushS2SAllowed("attacker.example.com") {
		t.Fatal("allowlist bypass")
	}
}
