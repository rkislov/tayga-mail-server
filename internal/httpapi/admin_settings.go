package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/settings"
)

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireGlobalAdmin(w, r); !ok {
		return
	}
	if s.hub == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "settings hub unavailable"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/settings")
	path = strings.Trim(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		out, err := s.hub.ListAll()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, out)
	case path != "" && r.Method == http.MethodGet:
		raw, err := s.hub.GetSection(path)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"section":          path,
			"value":            jsonRaw(raw),
			"restart_required": settings.RequiresRestart(path),
		})
	case path != "" && r.Method == http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body failed"})
			return
		}
		if err := s.hub.PutSection(r.Context(), path, body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "saved",
			"section":          path,
			"restart_required": s.hub.RestartRequired(),
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// jsonRaw unwraps RawMessage for nested encoding.
func jsonRaw(b []byte) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m == nil {
		return map[string]any{}
	}
	return m
}
