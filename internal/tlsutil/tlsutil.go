package tlsutil

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"github.com/tayga/tms/internal/config"
)

// Load builds a tls.Config from cfg.TLS. Prefer Manager for hot-reload in production.
func Load(cfg *config.Config) (*tls.Config, error) {
	m, err := NewManager(cfg)
	if err != nil {
		return nil, err
	}
	return m.TLSConfig(), nil
}

// LoadHTTP is Load with HTTP/2 ALPN for HTTPS listeners.
func LoadHTTP(cfg *config.Config) (*tls.Config, error) {
	m, err := NewManager(cfg)
	if err != nil {
		return nil, err
	}
	return m.HTTPConfig(), nil
}

func hostsFor(cfg *config.Config) []string {
	seen := map[string]struct{}{}
	var hosts []string
	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	add(cfg.Server.Hostname)
	add("localhost")
	add("127.0.0.1")
	add("::1")
	return hosts
}

// EnsureSelfSigned writes a self-signed ECDSA P-256 cert/key if either file is missing.
func EnsureSelfSigned(certPath, keyPath string, hosts []string) error {
	if certPath == "" || keyPath == "" {
		return fmt.Errorf("tls: cert_file and key_file required for auto_generate")
	}
	if fileExists(certPath) && fileExists(keyPath) {
		return nil
	}
	certPEM, keyPEM, err := makeSelfSignedPEM(hosts, 365)
	if err != nil {
		return err
	}
	return writePairAtomic(certPath, keyPath, certPEM, keyPEM)
}

func firstHost(hosts []string) string {
	if len(hosts) == 0 {
		return "localhost"
	}
	return hosts[0]
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
