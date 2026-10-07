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
	tc, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tc == nil || len(tc.Certificates) != 1 {
		t.Fatalf("tls config: %+v", tc)
	}
	// Second load reuses files.
	tc2, err := Load(cfg)
	if err != nil || tc2 == nil {
		t.Fatal(err)
	}
	httpCfg, err := LoadHTTP(cfg)
	if err != nil || httpCfg == nil {
		t.Fatal(err)
	}
	if len(httpCfg.NextProtos) == 0 {
		t.Fatal("expected ALPN")
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
	tc, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
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
