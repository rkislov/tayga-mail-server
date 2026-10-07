package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/config"
)

// Load builds a tls.Config from cfg.TLS. When AutoGenerate is set and files are
// missing, a development self-signed certificate is written first.
func Load(cfg *config.Config) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}
	if !cfg.TLSEnabled() {
		return nil, nil
	}
	if cfg.TLS.AutoGenerate {
		if err := EnsureSelfSigned(cfg.TLS.CertFile, cfg.TLS.KeyFile, hostsFor(cfg)); err != nil {
			return nil, err
		}
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("tls: load key pair: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// LoadHTTP is Load with HTTP/2 ALPN for HTTPS listeners.
func LoadHTTP(cfg *config.Config) (*tls.Config, error) {
	base, err := Load(cfg)
	if err != nil || base == nil {
		return base, err
	}
	out := base.Clone()
	out.NextProtos = []string{"h2", "http/1.1"}
	return out, nil
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
	if err := os.MkdirAll(filepath.Dir(certPath), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o750); err != nil {
		return err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Tayga Mail Dev"},
			CommonName:   firstHost(hosts),
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
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
