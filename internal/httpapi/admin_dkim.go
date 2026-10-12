package httpapi

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var dkimDomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
var dkimSelectorPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$`)

func generateDKIM(domain, selector string) (map[string]string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"domain": domain, "selector": selector,
		"dns_name":    selector + "._domainkey." + domain,
		"dns_value":   "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public),
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
	}, nil
}

func (s *Server) handleAdminDKIM(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireGlobalAdmin(w, r); !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input struct {
		Domain   string `json:"domain"`
		Selector string `json:"selector"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	input.Domain = strings.ToLower(strings.TrimSpace(input.Domain))
	input.Selector = strings.TrimSpace(input.Selector)
	if len(input.Domain) > 253 || !strings.Contains(input.Domain, ".") || !dkimDomainPattern.MatchString(input.Domain) || strings.Contains(input.Domain, "..") || !dkimSelectorPattern.MatchString(input.Selector) {
		writeJSON(w, 400, map[string]string{"error": "invalid domain or selector"})
		return
	}
	result, err := generateDKIM(input.Domain, input.Selector)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "key generation failed"})
		return
	}
	writeJSON(w, 200, result)
}

// handleDomainDKIM manages one active signing binding per administered domain.
func (s *Server) handleDomainDKIM(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		if name := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain"))); name != "" {
			domain, err := s.store.GetDomainByName(r.Context(), name)
			if err != nil || !s.adminCanManageDomain(r, admin, domain.ID) {
				writeJSON(w, 404, map[string]string{"error": "domain not found"})
				return
			}
			binding, err := storage.ReadDKIMBinding(r.Context(), s.store, name)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "key unavailable"})
				return
			}
			path := ""
			if binding != nil {
				path = binding.KeyFile
			} else if s.cfg.SMTP.DKIM.Domain == name {
				path = s.cfg.SMTP.DKIM.PrivateKeyFile
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "key unavailable"})
				return
			}
			w.Header().Set("Content-Type", "application/x-pem-file")
			w.Header().Set("Content-Disposition", `attachment; filename="dkim.pem"`)
			_, _ = w.Write(raw)
			return
		}
		tenants, err := s.store.ListTenants(r.Context())
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "domain lookup failed"})
			return
		}
		items := []map[string]any{}
		for _, tenant := range tenants {
			domains, err := s.store.ListDomainsByTenant(r.Context(), tenant.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "domain lookup failed"})
				return
			}
			for _, domain := range domains {
				if !s.adminCanManageDomain(r, admin, domain.ID) {
					continue
				}
				binding, err := storage.ReadDKIMBinding(r.Context(), s.store, domain.Name)
				if err != nil {
					writeJSON(w, 500, map[string]string{"error": "key metadata unavailable"})
					return
				}
				item := map[string]any{"domain": domain.Name, "enabled": false, "configured": false}
				if binding != nil {
					item["configured"] = true
					item["enabled"] = binding.Enabled
					item["selector"] = binding.Selector
					item["dns_name"] = binding.Selector + "._domainkey." + domain.Name
					item["dns_value"] = binding.DNSValue
					item["source"] = "managed"
				} else if s.cfg.SMTP.DKIM.Domain == domain.Name {
					item["configured"] = true
					item["enabled"] = s.cfg.SMTP.DKIM.Enabled
					item["selector"] = s.cfg.SMTP.DKIM.Selector
					item["source"] = "legacy"
					item["dns_name"] = s.cfg.SMTP.DKIM.Selector + "._domainkey." + domain.Name
					// Derive the public DNS record from the installed legacy RSA key.
					if raw, err := os.ReadFile(s.cfg.SMTP.DKIM.PrivateKeyFile); err == nil {
						if _, dns, err := dkimPEM(string(raw)); err == nil {
							item["dns_value"] = dns
						}
					}
				}
				items = append(items, item)
			}
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input struct {
		Domain     string `json:"domain"`
		Selector   string `json:"selector"`
		PrivateKey string `json:"private_key"`
		Enabled    bool   `json:"enabled"`
		Generate   bool   `json:"generate"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&input) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	input.Domain = strings.ToLower(strings.TrimSpace(input.Domain))
	input.Selector = strings.TrimSpace(input.Selector)
	domain, err := s.store.GetDomainByName(r.Context(), input.Domain)
	if err != nil || !s.adminCanManageDomain(r, admin, domain.ID) {
		writeJSON(w, 404, map[string]string{"error": "domain not found"})
		return
	}
	if !dkimSelectorPattern.MatchString(input.Selector) {
		writeJSON(w, 400, map[string]string{"error": "invalid selector"})
		return
	}
	binding, err := storage.ReadDKIMBinding(r.Context(), s.store, domain.Name)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "key metadata unavailable"})
		return
	}
	if binding == nil && s.cfg.SMTP.DKIM.Domain == domain.Name && input.Selector == s.cfg.SMTP.DKIM.Selector && !input.Generate && input.PrivateKey == "" {
		raw, readErr := os.ReadFile(s.cfg.SMTP.DKIM.PrivateKeyFile)
		if readErr != nil {
			writeJSON(w, 500, map[string]string{"error": "existing key unavailable"})
			return
		}
		_, dns, keyErr := dkimPEM(string(raw))
		if keyErr != nil {
			writeJSON(w, 400, map[string]string{"error": "existing key is not a supported RSA private key"})
			return
		}
		binding = &storage.DKIMBinding{Domain: domain.Name, Selector: input.Selector, KeyFile: s.cfg.SMTP.DKIM.PrivateKeyFile, DNSValue: dns, Enabled: input.Enabled}
	}
	newKeyPath := ""
	if input.Generate {
		result, err := generateDKIM(domain.Name, input.Selector)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "key generation failed"})
			return
		}
		input.PrivateKey = result["private_key"]
	}
	if input.PrivateKey != "" {
		raw, dns, err := dkimPEM(input.PrivateKey)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "valid RSA private PEM of at least 2048 bits required"})
			return
		}
		dir := filepath.Join(filepath.Dir(s.cfg.Mailstore.Root), "dkim-keys")
		if os.MkdirAll(dir, 0700) != nil {
			writeJSON(w, 500, map[string]string{"error": "key directory unavailable"})
			return
		}
		file, err := os.CreateTemp(dir, "key-*.pem")
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "key storage unavailable"})
			return
		}
		path := file.Name()
		newKeyPath = path
		_, err = file.Write(raw)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			_ = os.Remove(path)
			writeJSON(w, 500, map[string]string{"error": "key storage failed"})
			return
		}
		binding = &storage.DKIMBinding{Domain: domain.Name, Selector: input.Selector, KeyFile: path, DNSValue: dns, Enabled: input.Enabled}
	} else if binding == nil || binding.Selector != input.Selector {
		writeJSON(w, 400, map[string]string{"error": "generate or import a key first"})
		return
	} else {
		binding.Enabled = input.Enabled
	}
	raw, _ := json.Marshal(binding)
	if s.store.PutSetting(r.Context(), "dkim.domain."+domain.Name, string(raw)) != nil {
		if newKeyPath != "" {
			_ = os.Remove(newKeyPath)
		}
		writeJSON(w, 500, map[string]string{"error": "binding save failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"domain": domain.Name, "selector": binding.Selector, "enabled": binding.Enabled, "dns_name": binding.Selector + "._domainkey." + domain.Name, "dns_value": binding.DNSValue})
}

func dkimPEM(raw string) ([]byte, string, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, "", x509.IncorrectPasswordError
	}
	var key *rsa.PrivateKey
	var err error
	if block.Type == "RSA PRIVATE KEY" {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else if block.Type == "PRIVATE KEY" {
		var value any
		value, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		key, _ = value.(*rsa.PrivateKey)
	}
	if err != nil || key == nil || key.N.BitLen() < 2048 {
		return nil, "", x509.IncorrectPasswordError
	}
	if err = key.Validate(); err != nil {
		return nil, "", err
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, "", err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(pub), nil
}
