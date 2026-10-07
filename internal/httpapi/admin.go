package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type adminQuotaRequest struct {
	QuotaBytes int64 `json:"quota_bytes"`
}

type adminCreateUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	QuotaBytes  int64  `json:"quota_bytes"`
}

// adminTenantUser loads a user in the admin's tenant/domain scope or writes an error response.
func (s *Server) adminTenantUser(w http.ResponseWriter, r *http.Request, admin *storage.User, id string) *storage.User {
	target, err := s.store.GetUserByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return nil
	}
	if !s.adminCanManageUser(r, admin, target) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return nil
	}
	return target
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users")
	path = strings.Trim(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		users, err := s.store.ListUsersByTenant(r.Context(), admin.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			if !s.adminCanManageUser(r, admin, u) {
				continue
			}
			used, _ := s.store.SumMailboxBytes(r.Context(), u.ID)
			out = append(out, map[string]any{
				"id":               u.ID,
				"email":            u.Email,
				"display_name":     u.DisplayName,
				"quota_bytes":      u.QuotaBytes,
				"used_bytes":       used,
				"enabled":          u.Enabled,
				"auth_source":      u.AuthSource,
				"roles":            storage.ParseRoles(u.Roles),
				"domain_id":        u.DomainID,
				"service_class_id": u.ServiceClassID,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": out})

	case r.Method == http.MethodPost && path == "":
		var req adminCreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))
		if email == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
			return
		}
		at := strings.LastIndex(email, "@")
		if at < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email"})
			return
		}
		domainName := email[at+1:]
		local := email[:at]
		dom, err := s.store.GetDomainByName(r.Context(), domainName)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain not found"})
			return
		}
		if dom.TenantID != admin.TenantID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "domain outside tenant"})
			return
		}
		if !s.adminCanManageDomain(r, admin, dom.ID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "domain outside admin scope"})
			return
		}
		hash, err := s.authn.Hasher.Hash(req.Password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash failed"})
			return
		}
		u, err := s.store.CreateUser(r.Context(), &storage.User{
			TenantID: admin.TenantID, DomainID: dom.ID,
			Email: email, LocalPart: local, DisplayName: req.DisplayName,
			PasswordHash: hash, AuthSource: "local", QuotaBytes: req.QuotaBytes, Enabled: true,
		})
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if s.ms != nil {
			root, _ := s.ms.EnsureUser(email)
			_, _ = s.store.EnsureMailbox(r.Context(), u.ID, "INBOX", root)
			_ = s.store.EnsureDAVDefaults(r.Context(), u.ID)
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": u.ID, "email": u.Email})

	case strings.HasSuffix(path, "/quota") && r.Method == http.MethodPut:
		id := strings.Trim(strings.TrimSuffix(path, "/quota"), "/")
		var req adminQuotaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if s.adminTenantUser(w, r, admin, id) == nil {
			return
		}
		if err := s.store.UpdateUserQuota(r.Context(), id, req.QuotaBytes); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	case strings.HasSuffix(path, "/password") && r.Method == http.MethodPut:
		id := strings.Trim(strings.TrimSuffix(path, "/password"), "/")
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if len(req.Password) < 8 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
			return
		}
		target := s.adminTenantUser(w, r, admin, id)
		if target == nil {
			return
		}
		if target.AuthSource != "local" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only local users support password reset"})
			return
		}
		hash, err := s.authn.Hasher.Hash(req.Password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash failed"})
			return
		}
		if err := s.store.UpdateUserPassword(r.Context(), id, hash); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	case path != "" && r.Method == http.MethodPatch:
		var req struct {
			Enabled        *bool    `json:"enabled"`
			Roles          []string `json:"roles"`
			DomainIDs      []string `json:"domain_ids"`
			ServiceClassID *string  `json:"service_class_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if s.adminTenantUser(w, r, admin, path) == nil {
			return
		}
		if req.Enabled != nil {
			if err := s.store.UpdateUserEnabled(r.Context(), path, *req.Enabled); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		if req.Roles != nil {
			if !s.isGlobalAdminUser(admin) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required to set roles"})
				return
			}
			if err := s.store.UpdateUserRoles(r.Context(), path, storage.JoinRoles(req.Roles)); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		if req.DomainIDs != nil {
			if !s.isGlobalAdminUser(admin) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
				return
			}
			if err := s.store.SetDomainAdminDomains(r.Context(), path, req.DomainIDs); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		if req.ServiceClassID != nil {
			if err := s.store.UpdateUserServiceClass(r.Context(), path, *req.ServiceClassID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
