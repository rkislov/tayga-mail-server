package httpapi

import (
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"strings"
)

func (s *Server) domainGALEnabled(r *http.Request, domainID string) bool {
	value, ok, err := s.store.GetSetting(r.Context(), "domain.gal."+domainID)
	return err == nil && ok && value == "true"
}
func (s *Server) galContact(user *storage.User) map[string]any {
	return map[string]any{"id": "gal-" + user.ID, "book_id": "gal", "uid": user.ID, "fn": user.DisplayName, "email": user.Email, "readonly": true}
}
func (s *Server) handleGAL(w http.ResponseWriter, r *http.Request, au *authUser, id string) {
	current, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil || !s.domainGALEnabled(r, current.DomainID) {
		http.NotFound(w, r)
		return
	}
	if id != "" {
		user, err := s.store.GetUserByID(r.Context(), strings.TrimPrefix(id, "gal-"))
		if err != nil || user.DomainID != current.DomainID || !user.Enabled {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, s.galContact(user))
		return
	}
	users, err := s.store.ListUsersByDomain(r.Context(), current.DomainID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "directory unavailable"})
		return
	}
	cards := []map[string]any{}
	for _, user := range users {
		if user.Enabled {
			cards = append(cards, s.galContact(user))
		}
	}
	writeJSON(w, 200, map[string]any{"cards": cards, "readonly": true})
}
