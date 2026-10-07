package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleAdminTLS(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireGlobalAdmin(w, r); !ok {
		return
	}
	if s.tls == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tls manager unavailable (set tls.cert_file and tls.key_file)"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.tls.Status())

	case http.MethodPut:
		var req struct {
			Certificate string `json:"certificate"`
			PrivateKey  string `json:"private_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		certPEM := strings.TrimSpace(req.Certificate)
		keyPEM := strings.TrimSpace(req.PrivateKey)
		if certPEM == "" || keyPEM == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "certificate and private_key PEM required"})
			return
		}
		e, err := s.tls.AddUploadedCert("active", []byte(certPEM), []byte(keyPEM))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := s.tls.ActivateCert(e.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.persistTLSActiveID(r.Context(), e.ID)
		writeJSON(w, http.StatusOK, s.tls.Status())

	case http.MethodPost:
		var req struct {
			Hosts []string `json:"hosts"`
			Days  int      `json:"days"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		e, err := s.tls.AddSelfSignedCert("", req.Hosts, req.Days)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := s.tls.ActivateCert(e.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.persistTLSActiveID(r.Context(), e.ID)
		writeJSON(w, http.StatusOK, s.tls.Status())

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
