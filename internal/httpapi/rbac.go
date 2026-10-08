package httpapi

import (
	"net/http"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) isAdminUser(u *storage.User) bool {
	if u == nil {
		return false
	}
	if s.isAdmin(u.Email) {
		return true
	}
	return storage.IsDomainAdmin(u.Roles)
}

func (s *Server) isGlobalAdminUser(u *storage.User) bool {
	if u == nil {
		return false
	}
	if s.isAdmin(u.Email) {
		return true
	}
	return storage.IsGlobalAdmin(u.Roles)
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*storage.User, bool) {
	return s.requireDomainAdmin(w, r)
}

func (s *Server) requireGlobalAdmin(w http.ResponseWriter, r *http.Request) (*storage.User, bool) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return nil, false
	}
	su, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return nil, false
	}
	if !s.isGlobalAdminUser(su) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin required"})
		return nil, false
	}
	return su, true
}

func (s *Server) requireDomainAdmin(w http.ResponseWriter, r *http.Request) (*storage.User, bool) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return nil, false
	}
	su, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return nil, false
	}
	if !s.isAdminUser(su) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin required"})
		return nil, false
	}
	return su, true
}

func (s *Server) adminCanManageDomain(r *http.Request, admin *storage.User, domainID string) bool {
	if s.isGlobalAdminUser(admin) {
		return true
	}
	ok, err := s.store.UserAdministersDomain(r.Context(), admin.ID, domainID)
	return err == nil && ok
}

func (s *Server) adminCanManageUser(r *http.Request, admin, target *storage.User) bool {
	if s.isGlobalAdminUser(admin) {
		return true
	}
	if target.TenantID != admin.TenantID {
		return false
	}
	return s.adminCanManageDomain(r, admin, target.DomainID)
}
