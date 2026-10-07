package tlsutil

import (
	"crypto/tls"
	"net"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
)

func TestEnsureSelfSignedAndLoad(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "server.crt")
	key := filepath.Join(dir, "server.key")
	cfg := &config.Config{
		Server: config.ServerConfig{Hostname: "mail.test"},
		TLS: config.TLSConfig{
			CertFile:     cert,
			KeyFile:      key,
			AutoGenerate: true,
		},
	}
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Enabled() {
		t.Fatal("expected cert")
	}
	tc := m.TLSConfig()
	if tc == nil || tc.GetCertificate == nil {
		t.Fatalf("tls config: %+v", tc)
	}
	got, err := tc.GetCertificate(&tls.ClientHelloInfo{})
	if err != nil || got == nil {
		t.Fatal(err)
	}
	httpCfg := m.HTTPConfig()
	if len(httpCfg.NextProtos) == 0 {
		t.Fatal("expected ALPN")
	}
	st := m.Status()
	if !st.Configured || st.Fingerprint == "" {
		t.Fatalf("status %+v", st)
	}
}

func TestManagerInstallAndGenerate(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{Hostname: "mail.test"},
		TLS: config.TLSConfig{
			CertFile: filepath.Join(dir, "server.crt"),
			KeyFile:  filepath.Join(dir, "server.key"),
		},
	}
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Enabled() {
		t.Fatal("expected empty")
	}
	if err := m.GenerateSelfSigned([]string{"mail.test", "127.0.0.1"}, 30); err != nil {
		t.Fatal(err)
	}
	if !m.Enabled() {
		t.Fatal("expected cert after generate")
	}
}

func TestHTTPSDial(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "server.crt")
	key := filepath.Join(dir, "server.key")
	cfg := &config.Config{
		Server: config.ServerConfig{Hostname: "localhost"},
		TLS: config.TLSConfig{
			CertFile: cert, KeyFile: key, AutoGenerate: true,
		},
	}
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	tc := m.TLSConfig()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tc)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.(*tls.Conn).Handshake()
	}()
	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, ok := conn.LocalAddr().(*net.TCPAddr); !ok {
		t.Fatal("expected tcp")
	}
}
