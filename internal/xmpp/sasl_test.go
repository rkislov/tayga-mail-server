package xmpp

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"github.com/xdg-go/scram"
)

const saslHeader = `<stream:stream xmlns='jabber:client' xmlns:stream='http://etherx.jabber.org/streams' to='example.com' from='alice@example.com' version='1.0'>`

func saslFixture(t *testing.T) (*Server, *storage.Store, *storage.User) {
	t.Helper()
	srv, st, u := testServer(t)
	srv.cfg.XMPP.RequireTLS = true
	srv.authn = auth.NewLayer(st, config.LDAPConfig{}, config.MFAConfig{}, config.OIDCConfig{})
	if err := srv.authn.SetLocalPassword(context.Background(), u.ID, "fixture-password"); err != nil {
		t.Fatal(err)
	}
	return srv, st, u
}
func saslWire(t *testing.T, srv *Server, direct bool, version uint16) (*resumeWireClient, resumeWireElement) {
	t.Helper()
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	certServer.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	remote, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	sess := newSession(srv, remote, direct)
	sess.tlsCfg = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: version, MaxVersion: version}
	if direct {
		tc := tls.Server(remote, sess.sessionTLSConfig())
		sess.conn = tc
		sess.br = bufio.NewReader(tc)
		sess.bw = bufio.NewWriter(tc)
		sess.dec = xml.NewDecoder(sess.br)
	}
	c := &resumeWireClient{t: t, conn: conn, decoder: xml.NewDecoder(conn), session: sess, done: make(chan struct{})}
	go func() { sess.serve(); close(c.done) }()
	t.Cleanup(func() {
		c.conn.Close()
		select {
		case <-c.done:
		case <-time.After(4 * time.Second):
			t.Error("SASL connection stuck")
		}
	})
	if !direct {
		c.send(saslHeader)
		f := c.want("features", "")
		if strings.Contains(f.Inner, "mechanism") || strings.Contains(f.Inner, "sasl-cb") {
			t.Fatal("authentication before TLS")
		}
		c.send(`<starttls xmlns='urn:ietf:params:xml:ns:xmpp-tls'/>`)
		c.want("proceed", "")
	}
	tc := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, MinVersion: version, MaxVersion: version}) // disposable loopback certificate
	if err = tc.Handshake(); err != nil {
		t.Fatal(err)
	}
	c.conn = tc
	c.decoder = xml.NewDecoder(tc)
	c.send(saslHeader)
	return c, c.want("features", "")
}

func saslStart(t *testing.T, c *resumeWireClient, ns, mechanism, password, cbType string) (*scram.ClientConversation, string) {
	t.Helper()
	client, err := scram.SHA256.NewClient("alice", password, "")
	if err != nil {
		t.Fatal(err)
	}
	conversation := client.NewConversation()
	state := c.conn.(*tls.Conn).ConnectionState()
	if cbType != "" {
		var cb scram.ChannelBinding
		if cbType == "tls-server-end-point" {
			cb, err = scram.NewTLSServerEndpointBinding(&state)
		} else {
			cb, err = scram.NewTLSExporterBinding(&state)
		}
		if err != nil {
			t.Fatal(err)
		}
		if cbType == "wrong" {
			cb.Data[0] ^= 1
		}
		conversation = client.NewConversationWithChannelBinding(cb)
	}
	first, err := conversation.Step("")
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(first))
	if ns == sasl2NS {
		c.send(`<authenticate xmlns='` + ns + `' mechanism='` + mechanism + `'><initial-response>` + b64 + `</initial-response><user-agent id='6e4ca628-9159-4ca3-94a6-94d25b6cfc67'><software>Test</software><device>Loopback</device></user-agent></authenticate>`)
	} else {
		c.send(`<auth xmlns='` + ns + `' mechanism='` + mechanism + `'>` + b64 + `</auth>`)
	}
	challenge := c.want("challenge", "")
	raw, err := base64.StdEncoding.DecodeString(challenge.Inner)
	if err != nil {
		t.Fatal(err)
	}
	return conversation, string(raw)
}
func saslComplete(t *testing.T, c *resumeWireClient, conv *scram.ClientConversation, ns, challenge string, success bool) {
	t.Helper()
	response, err := conv.Step(challenge)
	if err != nil {
		t.Fatal(err)
	}
	c.send(`<response xmlns='` + ns + `'>` + base64.StdEncoding.EncodeToString([]byte(response)) + `</response>`)
	if !success {
		c.want("failure", "")
		return
	}
	result := c.want("success", "")
	data := result.Inner
	if ns == sasl2NS {
		var decoded struct {
			Data string `xml:"additional-data"`
			JID  string `xml:"authorization-identifier"`
		}
		if err = xml.Unmarshal([]byte("<success>"+result.Inner+"</success>"), &decoded); err != nil {
			t.Fatal(err)
		}
		data = decoded.Data
		if decoded.JID != "alice@example.com" {
			t.Fatal(decoded.JID)
		}
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conv.Step(string(raw)); err != nil || !conv.Valid() {
		t.Fatal("invalid server proof", err)
	}
	if ns == saslNS {
		c.decoder = xml.NewDecoder(c.conn)
		c.send(saslHeader)
	}
	f := c.want("features", "")
	if !strings.Contains(f.Inner, "xmpp-bind") || strings.Contains(f.Inner, "mechanism") {
		t.Fatal(f.Inner)
	}
	c.send(`<iq type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>sasl-fixture</resource></bind></iq>`)
	c.want("iq", "bind")
	c.send(`<iq type='get' id='ping' to='example.com'><ping xmlns='urn:xmpp:ping'/></iq>`)
	c.want("iq", "ping")
}

func TestSASL2SCRAMTLS(t *testing.T) {
	for _, row := range []struct {
		name, ns, cb string
		direct       bool
		version      uint16
	}{
		{"starttls-exporter", sasl2NS, "tls-exporter", false, tls.VersionTLS13},
		{"direct-exporter", sasl2NS, "tls-exporter", true, tls.VersionTLS13},
		{"starttls-endpoint", sasl2NS, "tls-server-end-point", false, tls.VersionTLS12},
		{"legacy-plus", saslNS, "tls-exporter", false, tls.VersionTLS13},
		{"sasl2-nonplus", sasl2NS, "", false, tls.VersionTLS13},
	} {
		t.Run(row.name, func(t *testing.T) {
			srv, _, _ := saslFixture(t)
			c, f := saslWire(t, srv, row.direct, row.version)
			if !strings.Contains(f.Inner, "SCRAM-SHA-256-PLUS") || !strings.Contains(f.Inner, "tls-server-end-point") {
				t.Fatal(f.Inner)
			}
			mech := "SCRAM-SHA-256"
			if row.cb != "" {
				mech += "-PLUS"
			}
			conv, challenge := saslStart(t, c, row.ns, mech, "fixture-password", row.cb)
			expected := scramAdvertisementHash([]string{"PLAIN", "X-OAUTH2", "SCRAM-SHA-256", "SCRAM-SHA-256-PLUS"}, []string{"tls-exporter", "tls-server-end-point"})
			if !strings.HasSuffix(challenge, ",h="+expected) {
				t.Fatal("missing downgrade protection")
			}
			saslComplete(t, c, conv, row.ns, challenge, true)
		})
	}
}

func TestSASL2SCRAMRejects(t *testing.T) {
	for _, mode := range []string{"wrong-password", "wrong-binding", "tampered-advertisement", "revoked-device", "changed-password", "disabled", "bad-nonce", "replayed-proof", "authzid", "duplicate-attribute", "downgrade-y", "plus-without-binding", "interleaved-stanza"} {
		t.Run(mode, func(t *testing.T) {
			srv, st, u := saslFixture(t)
			password := "fixture-password"
			var device *storage.XMPPDevicePassword
			var err error
			if mode == "revoked-device" {
				device, password, err = srv.authn.CreateXMPPDevicePassword(context.Background(), u.ID, "fixture")
				if err != nil {
					t.Fatal(err)
				}
			}
			c, _ := saslWire(t, srv, false, tls.VersionTLS13)
			switch mode {
			case "authzid", "duplicate-attribute", "downgrade-y", "plus-without-binding":
				first := "n,,n=alice,r=abcdef0123456789"
				mech := "SCRAM-SHA-256"
				if mode == "authzid" {
					first = "n,a=bob,n=alice,r=abcdef0123456789"
				}
				if mode == "duplicate-attribute" {
					first += ",n=alice"
				}
				if mode == "downgrade-y" {
					first = "y,,n=alice,r=abcdef0123456789"
				}
				if mode == "plus-without-binding" {
					mech += "-PLUS"
				}
				c.send(`<authenticate xmlns='` + sasl2NS + `' mechanism='` + mech + `'><initial-response>` + base64.StdEncoding.EncodeToString([]byte(first)) + `</initial-response></authenticate>`)
				c.want("failure", "")
				return
			}
			cb := "tls-exporter"
			if mode == "wrong-binding" {
				cb = "wrong"
			}
			if mode == "wrong-password" {
				password = "incorrect-password"
			}
			conv, challenge := saslStart(t, c, sasl2NS, "SCRAM-SHA-256-PLUS", password, cb)
			switch mode {
			case "tampered-advertisement":
				challenge = challenge[:strings.LastIndex(challenge, ",h=")] + ",h=" + scramAdvertisementHash([]string{"SCRAM-SHA-256"}, nil)
			case "revoked-device":
				if err = srv.authn.RevokeXMPPDevicePassword(context.Background(), u.ID, device.ID); err != nil {
					t.Fatal(err)
				}
			case "changed-password":
				if err = srv.authn.SetLocalPassword(context.Background(), u.ID, "replacement-password"); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if _, err = st.DB().Exec(`UPDATE users SET enabled=0 WHERE id=?`, u.ID); err != nil {
					t.Fatal(err)
				}
			case "interleaved-stanza":
				c.send(`<iq type='get' id='bad'><ping xmlns='urn:xmpp:ping'/></iq>`)
				select {
				case <-c.done:
				case <-time.After(time.Second):
					t.Fatal("accepted interleaved traffic")
				}
				return
			}
			if mode == "bad-nonce" || mode == "replayed-proof" {
				response, err := conv.Step(challenge)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "bad-nonce" {
					response = strings.Replace(response, ",r=", ",r=wrong", 1)
				} else {
					response = response[:strings.LastIndex(response, ",p=")] + ",p=" + base64.StdEncoding.EncodeToString(make([]byte, 32))
				}
				c.send(`<response xmlns='` + sasl2NS + `'>` + base64.StdEncoding.EncodeToString([]byte(response)) + `</response>`)
				c.want("failure", "")
				return
			}
			saslComplete(t, c, conv, sasl2NS, challenge, false)
		})
	}
}

func TestSASL2WebSocket(t *testing.T) {
	srv, _, u, endpoint := wsFixture(t)
	if err := srv.authn.SetLocalPassword(context.Background(), u.ID, "fixture-password"); err != nil {
		t.Fatal(err)
	}
	c := wsDial(t, endpoint)
	c.send(`<open xmlns='urn:ietf:params:xml:ns:xmpp-framing' to='example.com' from='alice@example.com' version='1.0'/>`)
	c.want("open", "")
	f := c.want("features", "")
	if !strings.Contains(f.Inner, "SCRAM-SHA-256") || strings.Contains(f.Inner, "-PLUS") {
		t.Fatal(f.Inner)
	}
	client, _ := scram.SHA256.NewClient("alice", "fixture-password", "")
	conv := client.NewConversation()
	first, _ := conv.Step("")
	c.send(`<authenticate xmlns='` + sasl2NS + `' mechanism='SCRAM-SHA-256'><initial-response>` + base64.StdEncoding.EncodeToString([]byte(first)) + `</initial-response></authenticate>`)
	challenge, _ := base64.StdEncoding.DecodeString(c.want("challenge", "").Inner)
	response, err := conv.Step(string(challenge))
	if err != nil {
		t.Fatal(err)
	}
	c.send(`<response xmlns='` + sasl2NS + `'>` + base64.StdEncoding.EncodeToString([]byte(response)) + `</response>`)
	result := c.want("success", "")
	var decoded struct {
		Data string `xml:"additional-data"`
	}
	xml.Unmarshal([]byte("<success>"+result.Inner+"</success>"), &decoded)
	proof, _ := base64.StdEncoding.DecodeString(decoded.Data)
	if _, err = conv.Step(string(proof)); err != nil || !conv.Valid() {
		t.Fatal(err)
	}
	c.want("features", "")
	c.send(`<iq xmlns='jabber:client' type='set' id='bind'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>ws-sasl2</resource></bind></iq>`)
	c.want("iq", "bind")
}

func TestSCRAMMigrationAndDevicePolicies(t *testing.T) {
	srv, st, u := saslFixture(t)
	ctx := context.Background()
	_, salt, keys, err := srv.authn.XMPPSCRAMCredentials(ctx, u.Email)
	if err != nil || len(keys) != 1 {
		t.Fatal(err)
	}
	// A legacy device with no SCRAM material prevents premature advertisement.
	legacy := &storage.XMPPDevicePassword{ID: storage.NewID(), UserID: u.ID, Name: "legacy", SecretHash: "digest", CreatedAt: time.Now().UTC()}
	if err = st.CreateXMPPDevicePassword(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	c, f := saslWire(t, srv, false, tls.VersionTLS13)
	if strings.Contains(f.Inner, "SCRAM") {
		t.Fatal("advertised partial migration")
	}
	c.conn.Close()
	if err = st.RevokeXMPPDevicePassword(ctx, u.ID, legacy.ID); err != nil {
		t.Fatal(err)
	}
	device, password, err := srv.authn.CreateXMPPDevicePassword(ctx, u.ID, "new device")
	if err != nil {
		t.Fatal(err)
	}
	if err = srv.authn.SetLocalPassword(ctx, u.ID, "new-primary"); err != nil {
		t.Fatal(err)
	}
	_, newSalt, keys, err := srv.authn.XMPPSCRAMCredentials(ctx, u.Email)
	if err != nil || newSalt != salt || len(keys) != 2 {
		t.Fatal("password change broke device salt", err)
	}
	c, _ = saslWire(t, srv, false, tls.VersionTLS13)
	conv, challenge := saslStart(t, c, sasl2NS, "SCRAM-SHA-256-PLUS", password, "tls-exporter")
	saslComplete(t, c, conv, sasl2NS, challenge, true)
	if c.session.credentialID != device.ID {
		t.Fatal("device scope lost")
	}
	c.conn.Close()
	if err = srv.authn.SetXMPPDevicesOnly(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	_, _, keys, err = srv.authn.XMPPSCRAMCredentials(ctx, u.Email)
	if err != nil || len(keys) != 1 || keys[0].CredentialID != device.ID {
		t.Fatal("strict policy leaked primary verifier", err)
	}
	c, _ = saslWire(t, srv, false, tls.VersionTLS13)
	conv, challenge = saslStart(t, c, sasl2NS, "SCRAM-SHA-256", "new-primary", "")
	saslComplete(t, c, conv, sasl2NS, challenge, false)
	// LDAP password verification must never be replaced by a cached primary key.
	if _, err = st.DB().Exec(`UPDATE users SET auth_source='ldap' WHERE id=?`, u.ID); err != nil {
		t.Fatal(err)
	}
	_, _, keys, err = srv.authn.XMPPSCRAMCredentials(ctx, u.Email)
	if err != nil || len(keys) != 1 || keys[0].CredentialID != device.ID {
		t.Fatal("LDAP device-only policy", err)
	}
	if err = srv.authn.SetXMPPDevicesOnly(ctx, u.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = srv.authn.XMPPSCRAMCredentials(ctx, u.Email); err == nil {
		t.Fatal("LDAP password cached")
	}
}

func TestSASL2PlainAndLazyMigration(t *testing.T) {
	srv, st, u := saslFixture(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`DELETE FROM xmpp_scram_keys WHERE user_id=?`, u.ID); err != nil {
		t.Fatal(err)
	}
	c, f := saslWire(t, srv, false, tls.VersionTLS13)
	if strings.Contains(f.Inner, "SCRAM") {
		t.Fatal("SCRAM before enrollment")
	}
	c.send(`<authenticate xmlns='` + sasl2NS + `' mechanism='PLAIN'><initial-response>` + base64.StdEncoding.EncodeToString([]byte("\x00alice\x00fixture-password")) + `</initial-response></authenticate>`)
	c.want("success", "")
	c.want("features", "")
	c.conn.Close()
	if _, _, keys, err := srv.authn.XMPPSCRAMCredentials(ctx, u.Email); err != nil || len(keys) != 1 {
		t.Fatal("PLAIN migration failed", err)
	}
	c, f = saslWire(t, srv, false, tls.VersionTLS13)
	if !strings.Contains(f.Inner, "SCRAM-SHA-256-PLUS") {
		t.Fatal("ready account not advertised")
	}
	conv, challenge := saslStart(t, c, sasl2NS, "SCRAM-SHA-256-PLUS", "fixture-password", "tls-exporter")
	saslComplete(t, c, conv, sasl2NS, challenge, true)
}

func TestSCRAMMFAAndCredentialVersion(t *testing.T) {
	srv, st, u := saslFixture(t)
	ctx := context.Background()
	srv.authn = auth.NewLayer(st, config.LDAPConfig{}, config.MFAConfig{RequireTokenForMFAUsers: true}, config.OIDCConfig{})
	if _, err := st.DB().Exec(`INSERT INTO user_mfa(user_id,totp_enabled) VALUES(?,1)`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := srv.authn.XMPPSCRAMCredentials(ctx, u.Email); err == nil {
		t.Fatal("SCRAM bypassed TOTP policy")
	}
	d, password, err := srv.authn.CreateXMPPDevicePassword(ctx, u.ID, "MFA device")
	if err != nil {
		t.Fatal(err)
	}
	_, _, keys, err := srv.authn.XMPPSCRAMCredentials(ctx, u.Email)
	if err != nil || len(keys) != 1 || keys[0].CredentialID != d.ID {
		t.Fatal("scoped credentials blocked by MFA", err)
	}
	c, _ := saslWire(t, srv, true, tls.VersionTLS13)
	conv, challenge := saslStart(t, c, sasl2NS, "SCRAM-SHA-256-PLUS", password, "tls-exporter")
	saslComplete(t, c, conv, sasl2NS, challenge, true)
}

// RFC 7677 section 3's fixed transcript, independently of the live client
// conversation tests. The account is a disposable fixture, never a real user.
func TestSCRAMRFC7677Vector(t *testing.T) {
	srv, st, u := saslFixture(t)
	u, err := st.GetUserByID(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	salt, _ := base64.StdEncoding.DecodeString("W22ZaJ0SNY7soEsUEjb6gQ==")
	client, _ := scram.SHA256.NewClient("user", "pencil", "")
	credentials := client.GetStoredCredentials(scram.KeyFactors{Salt: string(salt), Iters: 4096})
	k := storage.XMPPSCRAMKey{SourceHash: u.PasswordHash, StoredKey: base64.StdEncoding.EncodeToString(credentials.StoredKey), ServerKey: base64.StdEncoding.EncodeToString(credentials.ServerKey)}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	s := newSession(srv, a, true)
	defer s.Close()
	nonce := "rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0"
	s.scram = &scramExchange{namespace: sasl2NS, mechanism: "SCRAM-SHA-256", gs2: "n,,", firstBare: "n=user,r=rOprNGfwEbeRWgbNEkqO", serverFirst: "r=" + nonce + ",s=W22ZaJ0SNY7soEsUEjb6gQ==,i=4096", nonce: nonce, user: u, keys: []storage.XMPPSCRAMKey{k}}
	result, err := s.finishSCRAM("c=biws,r=" + nonce + ",p=dHzbZapWIk4jUhN+Ute9ytag9zjfMHgsqmmiz7AndVQ=")
	if err != nil || result != "v=6rriTRBi23WpRR/wtup+mMhUZUn/dB5nLTJRsjl95G4=" {
		t.Fatal("RFC vector", result, err)
	}
}
