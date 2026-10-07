package xmpp

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestC2SAuthBindRosterPEP(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sqlStore := store.(*storage.Store)
	ten, _ := sqlStore.CreateTenant(ctx, "t")
	dom, _ := sqlStore.CreateDomain(ctx, ten.ID, "example.com")
	layer := auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{
		Issuer: "Test", AccessTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour,
		ChallengeTTL: 5 * time.Minute, RequireTokenForMFAUsers: false,
	}, config.OIDCConfig{})
	hash, err := layer.Hasher.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlStore.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID,
		Email: "alice@example.com", LocalPart: "alice",
		DisplayName: "Alice", PasswordHash: hash, AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Server:  config.ServerConfig{Hostname: "example.com"},
		Storage: config.StorageConfig{Driver: "sqlite"},
		XMPP:    config.XMPPConfig{Enabled: true, Listen: "127.0.0.1:0", RequireTLS: false},
		MFA:     config.MFAConfig{RequireTokenForMFAUsers: false},
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := New(cfg, log, store, layer, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go srv.acceptLoop(ln, true) // alreadyTLS=true skips STARTTLS requirement

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
				t.Fatalf("read for %q: %v\nbuf=%s", substr, err, b.String())
			}
		}
		return b.String()
	}

	mustWrite(`<?xml version='1.0'?><stream:stream xmlns='jabber:client' xmlns:stream='http://etherx.jabber.org/streams' to='example.com' version='1.0'>`)
	feat := readUntil("mechanisms")
	if !strings.Contains(feat, "PLAIN") {
		t.Fatalf("features: %s", feat)
	}

	plain := base64.StdEncoding.EncodeToString([]byte("\x00alice@example.com\x00secret"))
	mustWrite(fmt.Sprintf(`<auth xmlns='urn:ietf:params:xml:ns:xmpp-sasl' mechanism='PLAIN'>%s</auth>`, plain))
	readUntil("success")

	mustWrite(`<?xml version='1.0'?><stream:stream xmlns='jabber:client' xmlns:stream='http://etherx.jabber.org/streams' to='example.com' version='1.0'>`)
	readUntil("bind")

	mustWrite(`<iq type='set' id='bind1'><bind xmlns='urn:ietf:params:xml:ns:xmpp-bind'><resource>test</resource></bind></iq>`)
	bindResp := readUntil("alice@example.com/test")
	if !strings.Contains(bindResp, "type='result'") && !strings.Contains(bindResp, `type="result"`) {
		t.Fatalf("bind: %s", bindResp)
	}

	mustWrite(`<iq type='set' id='r1'><query xmlns='jabber:iq:roster'><item jid='bob@example.com' name='Bob'/></query></iq>`)
	readUntil("result")

	mustWrite(`<iq type='get' id='r2'><query xmlns='jabber:iq:roster'/></iq>`)
	roster := readUntil("bob@example.com")
	if !strings.Contains(roster, "Bob") {
		t.Fatalf("roster: %s", roster)
	}

	mustWrite(`<iq type='set' id='p1' to='alice@example.com'><pubsub xmlns='http://jabber.org/protocol/pubsub'>` +
		`<publish node='urn:xmpp:omemo:2:devices'><item id='current'>` +
		`<list xmlns='urn:xmpp:omemo:2'><device id='99'/></list></item></publish></pubsub></iq>`)
	readUntil("result")

	mustWrite(`<iq type='get' id='p2' to='alice@example.com'><pubsub xmlns='http://jabber.org/protocol/pubsub'>` +
		`<items node='urn:xmpp:omemo:2:devices'/></pubsub></iq>`)
	pep := readUntil("device id='99'")
	if !strings.Contains(pep, "urn:xmpp:omemo:2:devices") {
		t.Fatalf("pep: %s", pep)
	}

	mustWrite(`<iq type='set' id='c1'><enable xmlns='urn:xmpp:carbons:2'/></iq>`)
	readUntil("result")
}
