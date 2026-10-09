package httpapi

import (
	"net/http"
	"strings"
)

func (s *Server) handleAdminSessions(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/sessions"), "/")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet && id == "" {
		out := []any{}
		for _, item := range s.authn.Sessions() {
			user, err := s.store.GetUserByID(r.Context(), item.UserID)
			if err == nil && s.adminCanManageDomain(r, admin, user.DomainID) {
				out = append(out, item)
			}
		}
		writeJSON(w, 200, map[string]any{"sessions": out})
		return
	}
	if r.Method == http.MethodDelete && id != "" {
		owner := s.authn.SessionOwner(id)
		user, err := s.store.GetUserByID(r.Context(), owner)
		if err != nil || !s.adminCanManageDomain(r, admin, user.DomainID) {
			writeJSON(w, 404, map[string]string{"error": "session not found"})
			return
		}
		if err = s.authn.CloseSession(id); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot close session"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "closed"})
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
