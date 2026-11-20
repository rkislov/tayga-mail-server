package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/tayga/tms/internal/storage"
)

// Attribution is rendered by the application and cannot be changed by branding.
type brandingConfig struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	Image   string `json:"image"`
}

func validBrandingImage(value string) bool {
	if value == "" {
		return true
	}
	kind, encoded, ok := strings.Cut(value, ";base64,")
	if !ok || (kind != "data:image/png" && kind != "data:image/jpeg") {
		return false
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) > 1<<20 {
		return false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 {
		return false
	}
	if "data:image/"+format != kind {
		return false
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	return err == nil
}

func (s *Server) readBranding(ctx context.Context, key string) (brandingConfig, error) {
	raw, _, err := s.store.GetSetting(ctx, key)
	value := brandingConfig{}
	if err == nil && raw != "" {
		err = json.Unmarshal([]byte(raw), &value)
	}
	return value, err
}

// Users may only read the branding resolved from their own domain and tenant.
func (s *Server) handleBranding(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	su, err := s.store.GetUserByID(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "user lookup failed"})
		return
	}
	value, err := s.readBranding(r.Context(), "branding.domains."+su.DomainID)
	if err == nil && !value.Enabled {
		value, err = s.readBranding(r.Context(), "branding.tenants."+su.TenantID)
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "branding lookup failed"})
		return
	}
	if !value.Enabled {
		value = brandingConfig{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, value)
}

func (s *Server) handleAdminBranding(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/branding/"), "/")
	if len(parts) != 2 || parts[1] == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch parts[0] {
	case "domains":
		domain, err := s.store.GetDomainByID(r.Context(), parts[1])
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "domain not found"})
			return
		}
		if !s.adminCanManageDomain(r, admin, domain.ID) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
	case "tenants":
		if !s.isGlobalAdminUser(admin) && (!storage.HasRole(admin.Roles, storage.RoleTenantAdmin) || admin.TenantID != parts[1]) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		if _, err := s.store.GetTenantByID(r.Context(), parts[1]); err != nil {
			writeJSON(w, 404, map[string]string{"error": "tenant not found"})
			return
		}
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	key := "branding." + parts[0] + "." + parts[1]
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		value, err := s.readBranding(r.Context(), key)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "branding lookup failed"})
			return
		}
		writeJSON(w, 200, value)
	case http.MethodPut:
		var value brandingConfig
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1500000))
		dec.DisallowUnknownFields()
		if dec.Decode(&value) != nil || dec.Decode(new(any)) != io.EOF {
			writeJSON(w, 400, map[string]string{"error": "invalid branding"})
			return
		}
		value.Name = strings.TrimSpace(value.Name)
		if utf8.RuneCountInString(value.Name) > 60 || strings.ContainsAny(value.Name, "\x00\r\n") || !validBrandingImage(value.Image) {
			writeJSON(w, 400, map[string]string{"error": "use a name up to 60 characters and a PNG or JPEG up to 1 MB, at most 4096 × 4096 pixels"})
			return
		}
		if !value.Enabled {
			value = brandingConfig{}
		}
		raw, _ := json.Marshal(value)
		if s.store.PutSetting(r.Context(), key, string(raw)) != nil {
			writeJSON(w, 500, map[string]string{"error": "branding save failed"})
			return
		}
		writeJSON(w, 200, value)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
