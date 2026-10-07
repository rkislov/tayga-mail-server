package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

// DefaultCoSConfig is applied when a user has no class assigned.
const DefaultCoSConfig = `{
  "quota_bytes": 0,
  "max_mail_size": 26214400,
  "large_attach_bytes": 10485760,
  "share_max_ttl_sec": 604800,
  "features": {"files": true, "dav": true, "flowsync": true, "sieve": true, "shares": true, "delegates": true}
}`

type cosConfig struct {
	QuotaBytes        int64 `json:"quota_bytes"`
	MaxMailSize       int64 `json:"max_mail_size"`
	LargeAttachBytes  int64 `json:"large_attach_bytes"`
	ShareMaxTTLSec    int64 `json:"share_max_ttl_sec"`
	Features          map[string]bool `json:"features"`
}

func parseCoSConfig(raw string) cosConfig {
	cfg := cosConfig{
		MaxMailSize: 25 << 20, LargeAttachBytes: 10 << 20, ShareMaxTTLSec: 7 * 24 * 3600,
		Features: map[string]bool{"files": true, "dav": true, "flowsync": true, "sieve": true, "shares": true, "delegates": true},
	}
	if raw == "" {
		_ = json.Unmarshal([]byte(DefaultCoSConfig), &cfg)
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	return cfg
}

func (s *Server) userCoS(r *http.Request, u *storage.User) cosConfig {
	if u != nil && u.ServiceClassID != "" {
		if sc, err := s.store.GetServiceClass(r.Context(), u.ServiceClassID); err == nil {
			return parseCoSConfig(sc.Config)
		}
	}
	return parseCoSConfig(DefaultCoSConfig)
}

func (s *Server) handleAdminCoS(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireDomainAdmin(w, r)
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/service-classes"), "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		list, err := s.store.ListServiceClasses(r.Context(), admin.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, sc := range list {
			var cfg any
			_ = json.Unmarshal([]byte(sc.Config), &cfg)
			out = append(out, map[string]any{"id": sc.ID, "name": sc.Name, "config": cfg, "created_at": sc.CreatedAt})
		}
		writeJSON(w, http.StatusOK, map[string]any{"service_classes": out})

	case r.Method == http.MethodPost && path == "":
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		var req struct {
			Name   string          `json:"name"`
			Config json.RawMessage `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		cfg := string(req.Config)
		if cfg == "" {
			cfg = DefaultCoSConfig
		}
		sc, err := s.store.CreateServiceClass(r.Context(), &storage.ServiceClass{
			TenantID: admin.TenantID, Name: strings.TrimSpace(req.Name), Config: cfg,
		})
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": sc.ID, "name": sc.Name})

	case r.Method == http.MethodPut && path != "":
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		var req struct {
			Name   string          `json:"name"`
			Config json.RawMessage `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		sc := &storage.ServiceClass{ID: path, TenantID: admin.TenantID, Name: req.Name, Config: string(req.Config)}
		if sc.Config == "" {
			sc.Config = "{}"
		}
		if err := s.store.UpdateServiceClass(r.Context(), sc); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	case r.Method == http.MethodDelete && path != "":
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		if err := s.store.DeleteServiceClass(r.Context(), admin.TenantID, path); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
