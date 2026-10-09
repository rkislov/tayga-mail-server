package httpapi

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
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
