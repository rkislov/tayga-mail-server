package tlsutil

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/acme"
)

const letsEncryptStagingURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

// ACME challenge tokens served at /.well-known/acme-challenge/{token}.
type challengeStore struct {
	mu   sync.RWMutex
	toks map[string]string // token -> key authorization
}

func (s *challengeStore) put(token, keyAuth string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.toks == nil {
		s.toks = map[string]string{}
	}
	s.toks[token] = keyAuth
}

func (s *challengeStore) del(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.toks, token)
}

func (s *challengeStore) get(token string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.toks[token]
	return v, ok
}

// HTTPChallengeHandler serves ACME HTTP-01 tokens; other paths fall through to next.
func (m *Manager) HTTPChallengeHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/.well-known/acme-challenge/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			if next != nil {
				next.ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		token := strings.TrimPrefix(r.URL.Path, prefix)
		token = strings.Trim(token, "/")
		if m == nil {
			http.NotFound(w, r)
			return
		}
		keyAuth, ok := m.challenges.get(token)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(keyAuth))
	})
}

// IssueACME obtains a certificate via ACME (HTTP-01) and stores it in the catalog.
// Domains must resolve to this host on port 80. Does not activate unless activate is true.
func (m *Manager) IssueACME(ctx context.Context, name string, domains []string, email string, staging bool, activate bool) (*CertEntry, error) {
	if m == nil || m.cfg == nil || !m.cfg.TLSEnabled() {
		return nil, fmt.Errorf("tls: cert_file/key_file not configured")
	}
	domains = cleanDomains(domains)
	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one domain required")
	}
	if strings.TrimSpace(name) == "" {
		name = domains[0]
	}
	if email == "" && m.cfg != nil {
		email = strings.TrimSpace(m.cfg.TLS.ACME.Email)
	}
	if email == "" {
		return nil, fmt.Errorf("ACME account email required (tls.acme.email or request)")
	}
	if m.cfg != nil && m.cfg.TLS.ACME.Staging {
		staging = true
	}

	client, err := m.acmeClient(staging)
	if err != nil {
		return nil, err
	}
	if _, err := client.Register(ctx, &acme.Account{Contact: []string{"mailto:" + email}}, acme.AcceptTOS); err != nil {
		// already registered is fine
		if !isACMEAlreadyExists(err) {
			return nil, fmt.Errorf("acme register: %w", err)
		}
	}

	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domains...))
	if err != nil {
		return nil, fmt.Errorf("acme order: %w", err)
	}

	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return nil, fmt.Errorf("acme authz: %w", err)
		}
		if authz.Status == acme.StatusValid {
			continue
		}
		var chal *acme.Challenge
		for i := range authz.Challenges {
			if authz.Challenges[i].Type == "http-01" {
				chal = authz.Challenges[i]
				break
			}
		}
		if chal == nil {
			return nil, fmt.Errorf("acme: no http-01 challenge for %s", authz.Identifier.Value)
		}
		resp, err := client.HTTP01ChallengeResponse(chal.Token)
		if err != nil {
			return nil, err
		}
		m.challenges.put(chal.Token, resp)
		defer m.challenges.del(chal.Token)

		if _, err := client.Accept(ctx, chal); err != nil {
			return nil, fmt.Errorf("acme accept: %w", err)
		}
		if _, err := client.WaitAuthorization(ctx, authz.URI); err != nil {
			return nil, fmt.Errorf("acme wait authz %s: %w", authz.Identifier.Value, err)
		}
	}

	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return nil, fmt.Errorf("acme wait order: %w", err)
	}

	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domains[0]},
		DNSNames: domains,
	}, certKey)
	if err != nil {
		return nil, err
	}

	ders, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return nil, fmt.Errorf("acme finalize: %w", err)
	}
	var certPEM []byte
	for _, der := range ders {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	keyBytes, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	entry, err := m.addPEM(name, SourceACME, domains, certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	if activate {
		return m.ActivateCert(entry.ID)
	}
	return entry, nil
}

func (m *Manager) acmeClient(staging bool) (*acme.Client, error) {
	dirURL := ""
	if m.cfg != nil {
		dirURL = strings.TrimSpace(m.cfg.TLS.ACME.Directory)
		if staging || m.cfg.TLS.ACME.Staging {
			if dirURL == "" {
				dirURL = letsEncryptStagingURL
			}
		}
	}
	if staging && dirURL == "" {
		dirURL = letsEncryptStagingURL
	}
	key, err := m.loadOrCreateAccountKey()
	if err != nil {
		return nil, err
	}
	return &acme.Client{Key: key, DirectoryURL: dirURL}, nil
}

func (m *Manager) loadOrCreateAccountKey() (crypto.Signer, error) {
	path := filepath.Join(m.certsDir(), "acme", "account.key")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return key, nil
			}
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				if s, ok := key.(crypto.Signer); ok {
					return s, nil
				}
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func cleanDomains(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimSuffix(d, ".")
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

func isACMEAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "already") || strings.Contains(s, "conflict") || strings.Contains(s, "409")
}

// RenewACME re-issues an ACME catalog entry for the same domains.
func (m *Manager) RenewACME(ctx context.Context, id string, activate bool) (*CertEntry, error) {
	e, err := m.GetCert(id)
	if err != nil {
		return nil, err
	}
	if e.Source != SourceACME {
		return nil, fmt.Errorf("certificate is not ACME-issued")
	}
	email := ""
	if m.cfg != nil {
		email = m.cfg.TLS.ACME.Email
	}
	staging := m.cfg != nil && m.cfg.TLS.ACME.Staging
	wasActive := e.Active
	newE, err := m.IssueACME(ctx, e.Name, e.Domains, email, staging, false)
	if err != nil {
		return nil, err
	}
	if activate || wasActive {
		if _, err := m.ActivateCert(newE.ID); err != nil {
			return nil, err
		}
	}
	_ = m.DeleteCert(id)
	return m.GetCert(newE.ID)
}