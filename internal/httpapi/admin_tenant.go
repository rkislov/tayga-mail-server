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
		if !s.adminCanManageDomain(r, admin, d.ID) {
			continue
		}
		n, _ := s.store.CountUsersByDomain(r.Context(), d.ID)
		outDomains = append(outDomains, map[string]any{
			"id": d.ID, "name": d.Name, "user_count": n,
			"migration_enabled": d.MigrationEnabled, "created_at": d.CreatedAt,
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

func (s *Server) handleAdminTenants(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/tenants")
	path = strings.Trim(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		s.listAdminTenants(w, r, admin)
	case path == "" && r.Method == http.MethodPost:
		s.createAdminTenant(w, r, admin)
	case path != "" && !strings.Contains(path, "/") && r.Method == http.MethodGet:
		s.getAdminTenant(w, r, admin, path)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) listAdminTenants(w http.ResponseWriter, r *http.Request, admin *storage.User) {
	var tenants []*storage.Tenant
	if s.isGlobalAdminUser(admin) {
		all, err := s.store.ListTenants(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		tenants = all
	} else {
		t, err := s.store.GetTenantByID(r.Context(), admin.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		tenants = []*storage.Tenant{t}
	}
	out := make([]map[string]any, 0, len(tenants))
	for _, t := range tenants {
		st, _ := s.store.TenantStats(r.Context(), t.ID)
		item := map[string]any{
			"id": t.ID, "name": t.Name, "created_at": t.CreatedAt,
		}
		if st != nil {
			item["domain_count"] = st.Domains
			item["user_count"] = st.Users
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenants": out,
		"scope":   map[string]any{"global": s.isGlobalAdminUser(admin)},
	})
}

func (s *Server) createAdminTenant(w http.ResponseWriter, r *http.Request, admin *storage.User) {
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
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	t, err := s.store.CreateTenant(r.Context(), name)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "tenant already exists"})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": t.ID, "name": t.Name})
}

func (s *Server) getAdminTenant(w http.ResponseWriter, r *http.Request, admin *storage.User, id string) {
	if !s.adminCanAccessTenant(r, admin, id) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	t, err := s.store.GetTenantByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	st, _ := s.store.TenantStats(r.Context(), t.ID)
	out := map[string]any{"id": t.ID, "name": t.Name, "created_at": t.CreatedAt}
	if st != nil {
		out["domain_count"] = st.Domains
		out["user_count"] = st.Users
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant": out})
}

func (s *Server) adminCanAccessTenant(r *http.Request, admin *storage.User, tenantID string) bool {
	if s.isGlobalAdminUser(admin) {
		return true
	}
	return admin.TenantID == tenantID
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
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenantID == "" {
			tenantID = admin.TenantID
		}
		if !s.adminCanAccessTenant(r, admin, tenantID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		domains, err := s.store.ListDomainsByTenant(r.Context(), tenantID)
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
				"id": d.ID, "name": d.Name, "tenant_id": d.TenantID, "user_count": n,
				"migration_enabled": d.MigrationEnabled, "created_at": d.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"domains": out, "tenant_id": tenantID})

	case path != "" && r.Method == http.MethodPatch:
		var req struct {
			MigrationEnabled *string `json:"migration_enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		target := s.resolveDomain(r, admin, path)
		if target == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if !s.adminCanManageDomain(r, admin, target.ID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if req.MigrationEnabled != nil {
			if err := s.store.UpdateDomainMigration(r.Context(), target.ID, *req.MigrationEnabled); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	case r.Method == http.MethodPost && path == "":
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		var req struct {
			Name     string `json:"name"`
			TenantID string `json:"tenant_id"`
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
		tenantID := strings.TrimSpace(req.TenantID)
		if tenantID == "" {
			tenantID = admin.TenantID
		}
		if !s.adminCanAccessTenant(r, admin, tenantID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		d, err := s.store.CreateDomain(r.Context(), tenantID, name)
		if err != nil {
			if errors.Is(err, storage.ErrAlreadyExists) {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "domain already exists"})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": d.ID, "name": d.Name, "tenant_id": d.TenantID})

	case r.Method == http.MethodDelete && path != "":
		target := s.resolveDomain(r, admin, path)
		if target == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
			return
		}
		if err := s.store.DeleteDomain(r.Context(), target.TenantID, target.ID); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) resolveDomain(r *http.Request, admin *storage.User, path string) *storage.Domain {
	var d *storage.Domain
	if x, err := s.store.GetDomainByName(r.Context(), path); err == nil {
		d = x
	} else if x, err := s.store.GetDomainByID(r.Context(), path); err == nil {
		d = x
	} else {
		return nil
	}
	if s.isGlobalAdminUser(admin) {
		return d
	}
	if d.TenantID == admin.TenantID {
		return d
	}
	return nil
}
