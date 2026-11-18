package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func (s *Server) mattermostSettings() *mmsettings.Service {
	if s.mattermostAPI != nil {
		return s.mattermostAPI.Settings()
	}
	cfg := s.cfgLive()
	return &mmsettings.Service{Store: s.store, Defaults: mmsettings.Defaults{Enabled: cfg.Mattermost.Enabled, PluginsEnabled: cfg.Mattermost.PluginsEnabled, SiteName: "Tayga Chat"}}
}
func (s *Server) canManageMattermostScope(r *http.Request, admin *storage.User, scope mmsettings.Scope) bool {
	if !s.isAdminUser(admin) {
		return false
	}
	if s.isGlobalAdminUser(admin) {
		return true
	}
	if scope.TenantID == "" || scope.TenantID != admin.TenantID {
		return false
	}
	if storage.HasRole(admin.Roles, storage.RoleTenantAdmin) {
		return true
	}
	return scope.DomainID != "" && s.adminCanManageDomain(r, admin, scope.DomainID)
}
func (s *Server) handleMattermostSettings(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	policy := s.mattermostSettings()
	if strings.TrimSuffix(r.URL.Path, "/") == "/api/v1/admin/mattermost/scopes" {
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		items := []any{}
		if s.isGlobalAdminUser(admin) {
			items = append(items, map[string]any{"scope": mmsettings.Scope{}, "kind": "server", "label": "Сервер"})
		}
		tenants, err := s.store.ListTenants(r.Context())
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "tenant lookup failed"})
			return
		}
		for _, tenant := range tenants {
			scope := mmsettings.Scope{TenantID: tenant.ID}
			if s.canManageMattermostScope(r, admin, scope) {
				items = append(items, map[string]any{"scope": scope, "kind": "tenant", "label": "Тенант: " + tenant.Name})
			}
			if !s.isGlobalAdminUser(admin) && tenant.ID != admin.TenantID {
				continue
			}
			domains, err := s.store.ListDomainsByTenant(r.Context(), tenant.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "domain lookup failed"})
				return
			}
			for _, domain := range domains {
				scope.DomainID = domain.ID
				if s.canManageMattermostScope(r, admin, scope) {
					items = append(items, map[string]any{"scope": scope, "kind": "domain", "label": "Домен: " + domain.Name + " · " + tenant.Name})
				}
			}
		}
		writeJSON(w, 200, map[string]any{"scopes": items, "global_admin": s.isGlobalAdminUser(admin)})
		return
	}
	if strings.TrimSuffix(r.URL.Path, "/") != "/api/v1/admin/mattermost" {
		http.NotFound(w, r)
		return
	}
	scope := mmsettings.Scope{TenantID: r.URL.Query().Get("tenant_id"), DomainID: r.URL.Query().Get("domain_id")}
	if !s.canManageMattermostScope(r, admin, scope) {
		writeJSON(w, 403, map[string]string{"error": "scope access denied"})
		return
	}
	if err := policy.Validate(r.Context(), scope); err != nil {
		status := 400
		if errors.Is(err, storage.ErrNotFound) {
			status = 404
		}
		writeJSON(w, status, map[string]string{"error": "invalid tenant or domain scope"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var value mmsettings.Overrides
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&value); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid Mattermost settings"})
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			writeJSON(w, 400, map[string]string{"error": "trailing JSON is not allowed"})
			return
		}
		if err := policy.Save(r.Context(), scope, value); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
	case http.MethodGet:
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	overrides, err := policy.Read(r.Context(), scope)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "scope read failed"})
		return
	}
	effective, err := policy.Resolve(r.Context(), scope)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "scope resolution failed"})
		return
	}
	catalog := map[string]any{"active": []any{}, "inactive": []any{}}
	if s.mattermostAPI != nil {
		if value, e := s.mattermostAPI.PluginCatalog(); e == nil {
			catalog = value
		}
	}
	writeJSON(w, 200, map[string]any{"catalog": catalog, "scope": scope, "overrides": mmsettings.Redact(overrides), "effective": mmsettings.Redact(effective), "global_admin": s.isGlobalAdminUser(admin), "host_enabled": s.cfgLive().Mattermost.Enabled, "plugin_host_enabled": s.cfgLive().Mattermost.PluginsEnabled})
}
