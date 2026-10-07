package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleAdminTenant(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	tenant, err := s.store.GetTenantByID(r.Context(), admin.TenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	domains, err := s.store.ListDomainsByTenant(r.Context(), admin.TenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	outDomains := make([]map[string]any, 0, len(domains))
	for _, d := range domains {
		n, _ := s.store.CountUsersByDomain(r.Context(), d.ID)
		outDomains = append(outDomains, map[string]any{
			"id": d.ID, "name": d.Name, "user_count": n, "created_at": d.CreatedAt,
		})
	}
	users, _ := s.store.ListUsersByTenant(r.Context(), admin.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant": map[string]any{
			"id": tenant.ID, "name": tenant.Name, "created_at": tenant.CreatedAt,
			"user_count": len(users),
		},
		"domains": outDomains,
	})
}

func (s *Server) handleAdminDomains(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/domains")
	path = strings.Trim(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		domains, err := s.store.ListDomainsByTenant(r.Context(), admin.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(domains))
		for _, d := range domains {
			if !s.adminCanManageDomain(r, admin, d.ID) {
				continue
			}
			n, _ := s.store.CountUsersByDomain(r.Context(), d.ID)
			out = append(out, map[string]any{
				"id": d.ID, "name": d.Name, "user_count": n, "created_at": d.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"domains": out})

	case r.Method == http.MethodPost && path == "":
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		name := strings.ToLower(strings.TrimSpace(req.Name))
		if name == "" || strings.Contains(name, " ") || !strings.Contains(name, ".") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid domain name required"})
			return
		}
		d, err := s.store.CreateDomain(r.Context(), admin.TenantID, name)
		if err != nil {
			if errors.Is(err, storage.ErrAlreadyExists) {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "domain already exists"})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": d.ID, "name": d.Name})

	case r.Method == http.MethodDelete && path != "":
		// path is domain name or UUID
		var target *storage.Domain
		if d, err := s.store.GetDomainByName(r.Context(), path); err == nil {
			target = d
		} else {
			domains, err := s.store.ListDomainsByTenant(r.Context(), admin.TenantID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for _, d := range domains {
				if d.ID == path {
					target = d
					break
				}
			}
		}
		if target == nil || target.TenantID != admin.TenantID {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		if err := s.store.DeleteDomain(r.Context(), admin.TenantID, target.ID); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
