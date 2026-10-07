package tlsutil

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SourceSelfSigned = "self_signed"
	SourceUploaded   = "uploaded"
	SourceACME       = "acme"
)

// CertEntry is one certificate in the catalog (no private key in JSON API).
type CertEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Source      string   `json:"source"` // self_signed | uploaded | acme
	Domains     []string `json:"domains,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	Issuer      string   `json:"issuer,omitempty"`
	NotBefore   string   `json:"not_before,omitempty"`
	NotAfter    string   `json:"not_after,omitempty"`
	Fingerprint string   `json:"fingerprint_sha256,omitempty"`
	Active      bool     `json:"active"`
	CreatedAt   string   `json:"created_at,omitempty"`
	CertRel     string   `json:"-"`
	KeyRel      string   `json:"-"`
}

type catalogFile struct {
	ActiveID string             `json:"active_id"`
	Certs    []catalogDiskEntry `json:"certs"`
}

type catalogDiskEntry struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Source    string   `json:"source"`
	Domains   []string `json:"domains,omitempty"`
	CertRel   string   `json:"cert_file"`
	KeyRel    string   `json:"key_file"`
	CreatedAt string   `json:"created_at"`
}

func (m *Manager) certsDir() string {
	if m == nil || m.cfg == nil {
		return ""
	}
	dir := strings.TrimSpace(m.cfg.TLS.CertsDir)
	if dir == "" {
		dir = filepath.Join(filepath.Dir(m.cfg.TLS.CertFile), "store")
	}
	return dir
}

func (m *Manager) catalogPath() string {
	return filepath.Join(m.certsDir(), "catalog.json")
}

func (m *Manager) loadCatalog() (*catalogFile, error) {
	dir := m.certsDir()
	if dir == "" {
		return &catalogFile{}, fmt.Errorf("tls: certs_dir not configured")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	path := m.catalogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &catalogFile{}, nil
		}
		return nil, err
	}
	var cat catalogFile
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (m *Manager) saveCatalog(cat *catalogFile) error {
	if err := os.MkdirAll(m.certsDir(), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.catalogPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, m.catalogPath())
}

// ListCerts returns catalog entries with parsed metadata and active flag.
func (m *Manager) ListCerts() ([]CertEntry, error) {
	cat, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}
	activeID := cat.ActiveID
	if activeID == "" && m.cfg != nil {
		activeID = m.cfg.TLS.ActiveID
	}
	out := make([]CertEntry, 0, len(cat.Certs))
	for _, d := range cat.Certs {
		out = append(out, m.entryFromDisk(d, activeID))
	}
	return out, nil
}

func (m *Manager) entryFromDisk(d catalogDiskEntry, activeID string) CertEntry {
	e := CertEntry{
		ID: d.ID, Name: d.Name, Source: d.Source, Domains: append([]string(nil), d.Domains...),
		CreatedAt: d.CreatedAt, CertRel: d.CertRel, KeyRel: d.KeyRel,
		Active: d.ID == activeID && activeID != "",
	}
	certPath := filepath.Join(m.certsDir(), d.CertRel)
	if pemData, err := os.ReadFile(certPath); err == nil {
		fillEntryMeta(&e, pemData)
	}
	return e
}

func fillEntryMeta(e *CertEntry, certPEM []byte) {
	leaf := firstLeaf(certPEM)
	if leaf == nil {
		return
	}
	e.Subject = leaf.Subject.String()
	e.Issuer = leaf.Issuer.String()
	e.NotBefore = leaf.NotBefore.UTC().Format(time.RFC3339)
	e.NotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
	if len(e.Domains) == 0 {
		e.Domains = append([]string(nil), leaf.DNSNames...)
	}
	sum := sha256.Sum256(leaf.Raw)
	e.Fingerprint = hex.EncodeToString(sum[:])
}

func firstLeaf(certPEM []byte) *x509.Certificate {
	rest := certPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		leaf, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		return leaf
	}
}

// AddSelfSignedCert creates a self-signed cert in the catalog (does not activate).
func (m *Manager) AddSelfSignedCert(name string, hosts []string, days int) (*CertEntry, error) {
	if days <= 0 {
		days = 365
	}
	if len(hosts) == 0 {
		hosts = hostsFor(m.cfg)
	}
	if strings.TrimSpace(name) == "" {
		name = firstHost(hosts)
	}
	certPEM, keyPEM, err := makeSelfSignedPEM(hosts, days)
	if err != nil {
		return nil, err
	}
	return m.addPEM(name, SourceSelfSigned, hosts, certPEM, keyPEM)
}

// AddUploadedCert stores a commercial / external PEM pair in the catalog.
func (m *Manager) AddUploadedCert(name string, certPEM, keyPEM []byte) (*CertEntry, error) {
	_, leaf, err := parseKeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	hosts := append([]string(nil), leaf.DNSNames...)
	if strings.TrimSpace(name) == "" {
		name = leaf.Subject.CommonName
		if name == "" && len(hosts) > 0 {
			name = hosts[0]
		}
		if name == "" {
			name = "uploaded"
		}
	}
	return m.addPEM(name, SourceUploaded, hosts, certPEM, keyPEM)
}

func (m *Manager) addPEM(name, source string, domains []string, certPEM, keyPEM []byte) (*CertEntry, error) {
	if m == nil || m.cfg == nil || !m.cfg.TLSEnabled() {
		return nil, fmt.Errorf("tls: cert_file/key_file not configured")
	}
	if _, _, err := parseKeyPair(certPEM, keyPEM); err != nil {
		return nil, err
	}
	cat, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	dirRel := id
	absDir := filepath.Join(m.certsDir(), dirRel)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return nil, err
	}
	certRel := filepath.ToSlash(filepath.Join(dirRel, "cert.pem"))
	keyRel := filepath.ToSlash(filepath.Join(dirRel, "key.pem"))
	if err := writePairAtomic(filepath.Join(m.certsDir(), certRel), filepath.Join(m.certsDir(), keyRel), certPEM, keyPEM); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}
	d := catalogDiskEntry{
		ID: id, Name: name, Source: source, Domains: domains,
		CertRel: certRel, KeyRel: keyRel,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	cat.Certs = append(cat.Certs, d)
	if err := m.saveCatalog(cat); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}
	e := m.entryFromDisk(d, cat.ActiveID)
	return &e, nil
}

// ActivateCert makes the catalog entry the live server certificate (hot reload).
func (m *Manager) ActivateCert(id string) (*CertEntry, error) {
	cat, err := m.loadCatalog()
	if err != nil {
		return nil, err
	}
	var found *catalogDiskEntry
	for i := range cat.Certs {
		if cat.Certs[i].ID == id {
			found = &cat.Certs[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("certificate %q not found", id)
	}
	certPEM, err := os.ReadFile(filepath.Join(m.certsDir(), found.CertRel))
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(filepath.Join(m.certsDir(), found.KeyRel))
	if err != nil {
		return nil, err
	}
	if err := m.InstallPEM(certPEM, keyPEM); err != nil {
		return nil, err
	}
	cat.ActiveID = id
	if m.cfg != nil {
		m.cfg.TLS.ActiveID = id
	}
	if err := m.saveCatalog(cat); err != nil {
		return nil, err
	}
	e := m.entryFromDisk(*found, id)
	return &e, nil
}

// DeleteCert removes a catalog entry (refuses if it is the active one).
func (m *Manager) DeleteCert(id string) error {
	cat, err := m.loadCatalog()
	if err != nil {
		return err
	}
	activeID := cat.ActiveID
	if activeID == "" && m.cfg != nil {
		activeID = m.cfg.TLS.ActiveID
	}
	if id == activeID {
		return fmt.Errorf("cannot delete the active certificate; activate another first")
	}
	var keep []catalogDiskEntry
	var removed *catalogDiskEntry
	for _, d := range cat.Certs {
		if d.ID == id {
			cp := d
			removed = &cp
			continue
		}
		keep = append(keep, d)
	}
	if removed == nil {
		return fmt.Errorf("certificate %q not found", id)
	}
	cat.Certs = keep
	if err := m.saveCatalog(cat); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(m.certsDir(), removed.ID))
	return nil
}

// GetCert returns one catalog entry.
func (m *Manager) GetCert(id string) (*CertEntry, error) {
	list, err := m.ListCerts()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("certificate %q not found", id)
}

// EnsureCatalogSeed imports the active on-disk pair into the catalog if empty.
func (m *Manager) EnsureCatalogSeed() error {
	if m == nil || m.cfg == nil || !m.cfg.TLSEnabled() {
		return nil
	}
	cat, err := m.loadCatalog()
	if err != nil {
		return err
	}
	if len(cat.Certs) > 0 {
		return nil
	}
	if !fileExists(m.cfg.TLS.CertFile) || !fileExists(m.cfg.TLS.KeyFile) {
		return nil
	}
	certPEM, err := os.ReadFile(m.cfg.TLS.CertFile)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(m.cfg.TLS.KeyFile)
	if err != nil {
		return err
	}
	e, err := m.addPEM("default", SourceUploaded, nil, certPEM, keyPEM)
	if err != nil {
		return err
	}
	_, err = m.ActivateCert(e.ID)
	return err
}
