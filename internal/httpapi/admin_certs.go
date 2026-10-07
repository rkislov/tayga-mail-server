package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleAdminCerts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireGlobalAdmin(w, r); !ok {
		return
	}
	if s.tls == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "tls manager unavailable (set tls.cert_file and tls.key_file)"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/certs")
	path = strings.Trim(path, "/")
	parts := splitPath(path)

	switch {
	case r.Method == http.MethodGet && len(parts) == 0:
		list, err := s.tls.ListCerts()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		active := s.tls.Status()
		writeJSON(w, http.StatusOK, map[string]any{"certificates": list, "active": active})

	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "self-signed":
		var req struct {
			Name  string   `json:"name"`
			Hosts []string `json:"hosts"`
			Days  int      `json:"days"`
			Activate bool  `json:"activate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		e, err := s.tls.AddSelfSignedCert(req.Name, req.Hosts, req.Days)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.Activate {
			e, err = s.tls.ActivateCert(e.ID)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			s.persistTLSActiveID(r.Context(), e.ID)
		}
		writeJSON(w, http.StatusOK, e)

	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "upload":
		var req struct {
			Name        string `json:"name"`
			Certificate string `json:"certificate"`
			PrivateKey  string `json:"private_key"`
			Activate    bool   `json:"activate"`
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
		e, err := s.tls.AddUploadedCert(req.Name, []byte(certPEM), []byte(keyPEM))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.Activate {
			e, err = s.tls.ActivateCert(e.ID)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			s.persistTLSActiveID(r.Context(), e.ID)
		}
		writeJSON(w, http.StatusOK, e)

	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "acme":
		var req struct {
			Name     string   `json:"name"`
			Domains  []string `json:"domains"`
			Email    string   `json:"email"`
			Staging  bool     `json:"staging"`
			Activate bool     `json:"activate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()
		e, err := s.tls.IssueACME(ctx, req.Name, req.Domains, req.Email, req.Staging, req.Activate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.Activate {
			s.persistTLSActiveID(r.Context(), e.ID)
		}
		writeJSON(w, http.StatusOK, e)

	case len(parts) == 2 && parts[1] == "activate" && r.Method == http.MethodPost:
		e, err := s.tls.ActivateCert(parts[0])
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.persistTLSActiveID(r.Context(), e.ID)
		writeJSON(w, http.StatusOK, e)

	case len(parts) == 2 && parts[1] == "renew" && r.Method == http.MethodPost:
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()
		e, err := s.tls.RenewACME(ctx, parts[0], true)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.persistTLSActiveID(r.Context(), e.ID)
		writeJSON(w, http.StatusOK, e)

	case len(parts) == 1 && r.Method == http.MethodDelete:
		if err := s.tls.DeleteCert(parts[0]); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	case len(parts) == 1 && r.Method == http.MethodGet:
		e, err := s.tls.GetCert(parts[0])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, e)

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) persistTLSActiveID(ctx context.Context, id string) {
	if s.hub == nil || id == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"cert_file":     s.cfgLive().TLS.CertFile,
		"key_file":      s.cfgLive().TLS.KeyFile,
		"certs_dir":     s.cfgLive().TLS.CertsDir,
		"active_id":     id,
		"auto_generate": s.cfgLive().TLS.AutoGenerate,
		"acme":          s.cfgLive().TLS.ACME,
	})
	_ = s.hub.PutSection(ctx, "tls", body)
}
