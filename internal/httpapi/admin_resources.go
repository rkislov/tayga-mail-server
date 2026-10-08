package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func resourceToJSON(res *storage.CalendarResource) map[string]any {
	return map[string]any{
		"id":           res.ID,
		"tenant_id":    res.TenantID,
		"domain_id":    res.DomainID,
		"user_id":      res.UserID,
		"email":        res.Email,
		"local_part":   res.LocalPart,
		"display_name": res.DisplayName,
		"kind":         res.Kind,
		"capacity":     res.Capacity,
		"description":  res.Description,
		"auto_accept":  res.AutoAccept,
		"enabled":      res.Enabled,
		"created_at":   res.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) handleAdminResources(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/resources")
	path = strings.Trim(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		domainID := strings.TrimSpace(r.URL.Query().Get("domain_id"))
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		var (
			list []*storage.CalendarResource
			err  error
		)
		switch {
		case domainID != "":
			dom, derr := s.store.GetDomainByID(r.Context(), domainID)
			if derr != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
				return
			}
			if !s.adminCanAccessTenant(r, admin, dom.TenantID) || !s.adminCanManageDomain(r, admin, dom.ID) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			list, err = s.store.ListCalendarResourcesByDomain(r.Context(), domainID, false)
		case tenantID != "":
			if !s.adminCanAccessTenant(r, admin, tenantID) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			list, err = s.store.ListCalendarResourcesByTenant(r.Context(), tenantID, false)
		default:
			list, err = s.store.ListCalendarResourcesByTenant(r.Context(), admin.TenantID, false)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, res := range list {
			out = append(out, resourceToJSON(res))
		}
		writeJSON(w, http.StatusOK, map[string]any{"resources": out})

	case r.Method == http.MethodPost && path == "":
		var req struct {
			DomainID    string `json:"domain_id"`
			LocalPart   string `json:"local_part"`
			DisplayName string `json:"display_name"`
			Kind        string `json:"kind"`
			Capacity    int    `json:"capacity"`
			Description string `json:"description"`
			AutoAccept  *bool  `json:"auto_accept"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		domainID := strings.TrimSpace(req.DomainID)
		if domainID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain_id required"})
			return
		}
		dom, err := s.store.GetDomainByID(r.Context(), domainID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		if !s.adminCanAccessTenant(r, admin, dom.TenantID) || !s.adminCanManageDomain(r, admin, dom.ID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		auto := true
		if req.AutoAccept != nil {
			auto = *req.AutoAccept
		}
		res, err := s.store.CreateCalendarResource(r.Context(), &storage.CalendarResource{
			DomainID: domainID, LocalPart: req.LocalPart, DisplayName: strings.TrimSpace(req.DisplayName),
			Kind: req.Kind, Capacity: req.Capacity, Description: strings.TrimSpace(req.Description),
			AutoAccept: auto, Enabled: true,
		})
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, resourceToJSON(res))

	case path != "" && r.Method == http.MethodPatch:
		res, err := s.store.GetCalendarResource(r.Context(), path)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if !s.adminCanAccessTenant(r, admin, res.TenantID) || !s.adminCanManageDomain(r, admin, res.DomainID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		var req struct {
			DisplayName *string `json:"display_name"`
			Kind        *string `json:"kind"`
			Capacity    *int    `json:"capacity"`
			Description *string `json:"description"`
			AutoAccept  *bool   `json:"auto_accept"`
			Enabled     *bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.DisplayName != nil {
			res.DisplayName = strings.TrimSpace(*req.DisplayName)
		}
		if req.Kind != nil {
			res.Kind = *req.Kind
		}
		if req.Capacity != nil {
			res.Capacity = *req.Capacity
		}
		if req.Description != nil {
			res.Description = strings.TrimSpace(*req.Description)
		}
		if req.AutoAccept != nil {
			res.AutoAccept = *req.AutoAccept
		}
		if req.Enabled != nil {
			res.Enabled = *req.Enabled
		}
		if err := s.store.UpdateCalendarResource(r.Context(), res); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resourceToJSON(res))

	case path != "" && r.Method == http.MethodDelete:
		res, err := s.store.GetCalendarResource(r.Context(), path)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if !s.adminCanAccessTenant(r, admin, res.TenantID) || !s.adminCanManageDomain(r, admin, res.DomainID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if err := s.store.DeleteCalendarResource(r.Context(), res.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}
