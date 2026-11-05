package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func aliasError(w http.ResponseWriter, err error) {
	code, message := 500, "cannot save alias"
	switch {
	case errors.Is(err, storage.ErrAliasInvalid):
		code, message = 400, "invalid alias or target mailbox"
	case errors.Is(err, storage.ErrNotFound):
		code, message = 404, "alias or target not found"
	case errors.Is(err, storage.ErrAlreadyExists):
		code, message = 409, "address already exists"
	}
	writeJSON(w, code, map[string]string{"error": message})
}

// Nested paths bind every mutation to a domain or user in the admin's scope.
func (s *Server) handleDomainAliases(w http.ResponseWriter, r *http.Request, admin *storage.User, parts []string) {
	if len(parts) < 2 || len(parts) > 3 {
		http.NotFound(w, r)
		return
	}
	domain, err := s.store.GetDomainByID(r.Context(), parts[0])
	if err != nil {
		aliasError(w, storage.ErrNotFound)
		return
	}
	if !s.adminCanAccessTenant(r, admin, domain.TenantID) || !s.adminCanManageDomain(r, admin, domain.ID) {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	id := ""
	if len(parts) == 3 {
		id = parts[2]
	}
	switch {
	case r.Method == http.MethodGet && id == "":
		aliases, err := s.store.ListDomainAliases(r.Context(), domain.ID)
		if err != nil {
			aliasError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"aliases": aliases})
	case r.Method == http.MethodPost && id == "" || (r.Method == http.MethodPatch || r.Method == http.MethodPut) && id != "":
		// Claiming another domain name changes the global routing namespace.
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, 403, map[string]string{"error": "global admin required"})
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		alias, err := s.store.SaveDomainAlias(r.Context(), &storage.DomainAlias{ID: id, DomainID: domain.ID, Name: req.Name})
		if err != nil {
			aliasError(w, err)
			return
		}
		code := 200
		if id == "" {
			code = 201
		}
		writeJSON(w, code, alias)
	case r.Method == http.MethodDelete && id != "":
		if !s.isGlobalAdminUser(admin) {
			writeJSON(w, 403, map[string]string{"error": "global admin required"})
			return
		}
		if err = s.store.DeleteDomainAlias(r.Context(), domain.ID, id); err != nil {
			aliasError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleMailboxAliases(w http.ResponseWriter, r *http.Request, admin *storage.User, parts []string) {
	if len(parts) < 2 || len(parts) > 3 {
		http.NotFound(w, r)
		return
	}
	user := s.adminTenantUser(w, r, admin, parts[0])
	if user == nil {
		return
	}
	id := ""
	if len(parts) == 3 {
		id = parts[2]
	}
	aliases, err := s.store.ListAliases(r.Context(), user.ID)
	if err != nil {
		aliasError(w, err)
		return
	}
	if r.Method == http.MethodGet && id == "" {
		visible := []*storage.Alias{}
		for _, a := range aliases {
			if s.adminCanManageDomain(r, admin, a.DomainID) {
				visible = append(visible, a)
			}
		}
		writeJSON(w, 200, map[string]any{"aliases": visible})
		return
	}
	var current *storage.Alias
	if id != "" {
		for _, a := range aliases {
			if a.ID == id {
				current = a
				break
			}
		}
		if current == nil {
			aliasError(w, storage.ErrNotFound)
			return
		}
	}
	switch {
	case r.Method == http.MethodPost && id == "" || (r.Method == http.MethodPatch || r.Method == http.MethodPut) && id != "":
		var req struct {
			LocalPart string `json:"local_part"`
			DomainID  string `json:"domain_id"`
		}
		if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		domainID := strings.TrimSpace(req.DomainID)
		if domainID == "" {
			domainID = user.DomainID
		}
		if current != nil {
			if req.DomainID != "" && req.DomainID != current.DomainID {
				aliasError(w, storage.ErrAliasInvalid)
				return
			}
			domainID = current.DomainID
		}
		domain, err := s.store.GetDomainByID(r.Context(), domainID)
		if err != nil {
			aliasError(w, storage.ErrNotFound)
			return
		}
		if domain.TenantID != user.TenantID || !s.adminCanManageDomain(r, admin, domainID) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		alias, err := s.store.SaveAlias(r.Context(), &storage.Alias{ID: id, DomainID: domainID, UserID: user.ID, LocalPart: req.LocalPart})
		if err != nil {
			aliasError(w, err)
			return
		}
		code := 200
		if id == "" {
			code = 201
		}
		writeJSON(w, code, alias)
	case r.Method == http.MethodDelete && current != nil:
		if !s.adminCanManageDomain(r, admin, current.DomainID) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		if err = s.store.DeleteAlias(r.Context(), current.DomainID, user.ID, id); err != nil {
			aliasError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
