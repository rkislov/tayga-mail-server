package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/config"
)

// Manager holds the active server certificate and supports hot reload for all listeners.
type Manager struct {
	mu   sync.RWMutex
	cfg  *config.Config
	cert *tls.Certificate
	leaf *x509.Certificate
}

// Info is a public summary of the active certificate (no private key material).
type Info struct {
	Configured     bool     `json:"configured"`
	CertFile       string   `json:"cert_file"`
	KeyFile        string   `json:"key_file"`
	Subject        string   `json:"subject,omitempty"`
	Issuer         string   `json:"issuer,omitempty"`
	NotBefore      string   `json:"not_before,omitempty"`
	NotAfter       string   `json:"not_after,omitempty"`
	DNSNames       []string `json:"dns_names,omitempty"`
	IPSANs         []string `json:"ip_sans,omitempty"`
	Fingerprint    string   `json:"fingerprint_sha256,omitempty"`
	ExpiresInHours int      `json:"expires_in_hours,omitempty"`
}

// NewManager loads (or auto-generates) the configured certificate.
func NewManager(cfg *config.Config) (*Manager, error) {
	m := &Manager{cfg: cfg}
	if cfg == nil || !cfg.TLSEnabled() {
		return m, nil
	}
	if cfg.TLS.AutoGenerate {
		if err := EnsureSelfSigned(cfg.TLS.CertFile, cfg.TLS.KeyFile, hostsFor(cfg)); err != nil {
			return nil, err
		}
	}
	if !fileExists(cfg.TLS.CertFile) || !fileExists(cfg.TLS.KeyFile) {
		return m, nil
	}
	if err := m.reloadFromDisk(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cert != nil
}

// TLSConfig returns a config that always serves the current certificate.
func (m *Manager) TLSConfig() *tls.Config {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	ok := m.cert != nil
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: m.GetCertificate,
	}
}

// HTTPConfig is TLSConfig with HTTP/2 ALPN.
func (m *Manager) HTTPConfig() *tls.Config {
	base := m.TLSConfig()
	if base == nil {
		return nil
	}
	base.NextProtos = []string{"h2", "http/1.1"}
	return base
}

func (m *Manager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cert == nil {
		return nil, fmt.Errorf("tls: no certificate configured")
	}
	return m.cert, nil
}

func (m *Manager) Status() Info {
	info := Info{}
	if m == nil || m.cfg == nil {
		return info
	}
	info.CertFile = m.cfg.TLS.CertFile
	info.KeyFile = m.cfg.TLS.KeyFile
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.leaf == nil {
		return info
	}
	info.Configured = true
	info.Subject = m.leaf.Subject.String()
	info.Issuer = m.leaf.Issuer.String()
	info.NotBefore = m.leaf.NotBefore.UTC().Format(time.RFC3339)
	info.NotAfter = m.leaf.NotAfter.UTC().Format(time.RFC3339)
	info.DNSNames = append([]string(nil), m.leaf.DNSNames...)
	for _, ip := range m.leaf.IPAddresses {
		info.IPSANs = append(info.IPSANs, ip.String())
	}
	sum := sha256.Sum256(m.leaf.Raw)
	info.Fingerprint = hex.EncodeToString(sum[:])
	info.ExpiresInHours = int(time.Until(m.leaf.NotAfter).Hours())
	return info
}

// InstallPEM validates and atomically writes a new certificate/key pair, then activates it.
func (m *Manager) InstallPEM(certPEM, keyPEM []byte) error {
	if m == nil || m.cfg == nil || !m.cfg.TLSEnabled() {
		return fmt.Errorf("tls: cert_file/key_file not configured")
	}
	cert, leaf, err := parseKeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	if err := writePairAtomic(m.cfg.TLS.CertFile, m.cfg.TLS.KeyFile, certPEM, keyPEM); err != nil {
		return err
	}
	m.mu.Lock()
	m.cert = cert
	m.leaf = leaf
	m.mu.Unlock()
	return nil
}

// GenerateSelfSigned creates a new self-signed cert for the given hosts (or server defaults).
func (m *Manager) GenerateSelfSigned(hosts []string, days int) error {
	if m == nil || m.cfg == nil || !m.cfg.TLSEnabled() {
		return fmt.Errorf("tls: cert_file/key_file not configured")
	}
	if days <= 0 {
		days = 365
	}
	if len(hosts) == 0 {
		hosts = hostsFor(m.cfg)
	}
	certPEM, keyPEM, err := makeSelfSignedPEM(hosts, days)
	if err != nil {
		return err
	}
	return m.InstallPEM(certPEM, keyPEM)
}

func (m *Manager) reloadFromDisk() error {
	certPEM, err := os.ReadFile(m.cfg.TLS.CertFile)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(m.cfg.TLS.KeyFile)
	if err != nil {
		return err
	}
	cert, leaf, err := parseKeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cert = cert
	m.leaf = leaf
	m.mu.Unlock()
	return nil
}

func parseKeyPair(certPEM, keyPEM []byte) (*tls.Certificate, *x509.Certificate, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid certificate/key pair: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return nil, nil, fmt.Errorf("certificate chain empty")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, nil, fmt.Errorf("parse leaf: %w", err)
	}
	now := time.Now()
	if now.After(leaf.NotAfter) {
		return nil, nil, fmt.Errorf("certificate expired on %s", leaf.NotAfter.UTC().Format(time.RFC3339))
	}
	return &cert, leaf, nil
}

func writePairAtomic(certPath, keyPath string, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(filepath.Dir(certPath), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o750); err != nil {
		return err
	}
	certTmp := certPath + ".tmp"
	keyTmp := keyPath + ".tmp"
	if err := os.WriteFile(certTmp, certPEM, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(keyTmp, keyPEM, 0o600); err != nil {
		_ = os.Remove(certTmp)
		return err
	}
	if err := os.Rename(certTmp, certPath); err != nil {
		_ = os.Remove(certTmp)
		_ = os.Remove(keyTmp)
		return err
	}
	if err := os.Rename(keyTmp, keyPath); err != nil {
		_ = os.Remove(keyTmp)
		return err
	}
	return nil
}

func makeSelfSignedPEM(hosts []string, days int) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Tayga Mail"},
			CommonName:   firstHost(hosts),
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	return certPEM, keyPEM, nil
}
