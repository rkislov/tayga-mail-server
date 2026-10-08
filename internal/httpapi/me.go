package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type patchMeRequest struct {
	DisplayName *string `json:"display_name"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	su, err := s.store.GetUserByID(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		used, err := s.store.SumMailboxBytes(r.Context(), su.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		domainIDs, _ := s.store.ListDomainAdminDomains(r.Context(), su.ID)
		cos := s.userCoS(r, su)
		features := map[string]bool{}
		for k, v := range cos.Features {
			features[k] = v
		}
		features["migration"] = s.migrationAllowed(r, su)
		writeJSON(w, http.StatusOK, map[string]any{
			"id":                su.ID,
			"email":             su.Email,
			"display_name":      su.DisplayName,
			"auth_source":       su.AuthSource,
			"quota_bytes":       su.QuotaBytes,
			"used_bytes":        used,
			"enabled":           su.Enabled,
			"roles":             storage.ParseRoles(su.Roles),
			"admin_scope":       storage.AdminScope(su.Roles),
			"is_admin":          s.isAdminUser(su),
			"is_global_admin":   s.isGlobalAdminUser(su),
			"domain_id":         su.DomainID,
			"domain_ids":        domainIDs,
			"tenant_id":         su.TenantID,
			"service_class_id":  su.ServiceClassID,
			"migration_enabled": su.MigrationEnabled,
			"features":          features,
		})
	case http.MethodPatch:
		var req patchMeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.DisplayName == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "display_name required"})
			return
		}
		if err := s.store.UpdateUserProfile(r.Context(), su.ID, strings.TrimSpace(*req.DisplayName)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) isAdmin(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	cfg := s.cfgLive()
	if cfg == nil {
		return false
	}
	for _, a := range cfg.HTTP.Admins {
		if strings.EqualFold(strings.TrimSpace(a), email) {
			return true
		}
	}
	return false
}
