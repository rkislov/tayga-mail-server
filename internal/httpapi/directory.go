package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleDirectory(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	user, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil || !s.domainGALEnabled(r, user.DomainID) {
		http.NotFound(w, r)
		return
	}
	s.writeDirectory(w, r, user.DomainID, false)
}

func (s *Server) writeDirectory(w http.ResponseWriter, r *http.Request, domainID string, admin bool) {
	domain, err := s.store.GetDomainByID(r.Context(), domainID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	groups, err := s.store.ListDirectoryGroups(r.Context(), domainID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "directory unavailable"})
		return
	}
	users, err := s.store.ListUsersByDomain(r.Context(), domainID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "directory unavailable"})
		return
	}
	members := []map[string]any{}
	visible := map[string]bool{}
	for _, user := range users {
		if user.AuthSource == "resource" || (!admin && !user.Enabled) {
			continue
		}
		visible[user.ID] = true
		members = append(members, map[string]any{"id": user.ID, "name": user.DisplayName, "email": user.Email, "enabled": user.Enabled})
	}
	for _, group := range groups {
		ids := []string{}
		for _, id := range group.MemberIDs {
			if visible[id] {
				ids = append(ids, id)
			}
		}
		group.MemberIDs = ids
	}
	writeJSON(w, 200, map[string]any{"domain": map[string]string{"id": domain.ID, "name": domain.Name}, "groups": groups, "users": members, "editable": admin})
}

func directoryError(w http.ResponseWriter, err error) {
	code := 500
	switch {
	case errors.Is(err, storage.ErrNotFound):
		code = 404
	case errors.Is(err, storage.ErrDirectoryInvalid):
		code = 400
	case errors.Is(err, storage.ErrAlreadyExists), errors.Is(err, storage.ErrDirectoryConflict), errors.Is(err, storage.ErrDirectoryChildren):
		code = 409
	}
	message := err.Error()
	if code == 500 {
		message = "directory unavailable"
	}
	writeJSON(w, code, map[string]string{"error": message})
}

func (s *Server) handleAdminDirectory(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	domainID := r.URL.Query().Get("domain_id")
	domain, err := s.store.GetDomainByID(r.Context(), domainID)
	if err != nil || !s.adminCanAccessTenant(r, admin, domain.TenantID) || !s.adminCanManageDomain(r, admin, domainID) {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/directory"), "/")
	switch {
	case r.Method == http.MethodGet && id == "":
		s.writeDirectory(w, r, domainID, true)
	case (r.Method == http.MethodPost && id == "") || (r.Method == http.MethodPut && id != "" && !strings.Contains(id, "/")):
		var group storage.DirectoryGroup
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&group); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		group.ID, group.DomainID = id, domainID
		saved, err := s.store.SaveDirectoryGroup(r.Context(), &group)
		if err != nil {
			directoryError(w, err)
			return
		}
		code := 200
		if id == "" {
			code = 201
		}
		writeJSON(w, code, saved)
	case r.Method == http.MethodDelete && id != "" && !strings.Contains(id, "/"):
		revision, _ := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		if err := s.store.DeleteDirectoryGroup(r.Context(), domainID, id, revision); err != nil {
			directoryError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
