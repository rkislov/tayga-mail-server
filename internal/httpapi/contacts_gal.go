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
		if strings.HasPrefix(id, "gal-group-") {
			groups, err := s.store.ListDirectoryGroups(r.Context(), current.DomainID)
			if err == nil {
				for _, group := range groups {
					if id == "gal-group-"+group.ID {
						writeJSON(w, 200, groupContact(group))
						return
					}
				}
			}
			http.NotFound(w, r)
			return
		}
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
	groups, err := s.store.ListDirectoryGroups(r.Context(), current.DomainID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "directory unavailable"})
		return
	}
	for _, group := range groups {
		cards = append(cards, groupContact(group))
	}
	writeJSON(w, 200, map[string]any{"cards": cards, "readonly": true})
}

func groupContact(group *storage.DirectoryGroup) map[string]any {
	return map[string]any{"id": "gal-group-" + group.ID, "book_id": "gal", "uid": group.ID, "fn": group.Name,
		"email": group.Email, "note": group.Description, "readonly": true, "kind": "group"}
}

func (s *Server) domainXMPPGALEnabled(r *http.Request, domainID string) bool {
	value, ok, err := s.store.GetSetting(r.Context(), "domain.xmpp_gal."+domainID)
	return err == nil && ok && value == "true"
}
