package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/storage"
)

func (s *Server) SetMigrate(m *migrate.Service) {
	s.migrate = m
}

func (s *Server) migrationAllowed(r *http.Request, u *storage.User) bool {
	cos := s.userCoS(r, u)
	return migrate.ResolveForUser(r.Context(), s.store, u, cos.Features)
}

func (s *Server) handleMigration(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	su, err := s.store.GetUserByID(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	allowed := s.migrationAllowed(r, su)
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/migration"), "/")

	if !allowed && path == "" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"allowed": false, "jobs": []any{}})
		return
	}
	if !allowed {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "migration not enabled"})
		return
	}
	if s.migrate == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "migration unavailable"})
		return
	}

	switch {
	case path == "" && r.Method == http.MethodGet:
		jobs, err := s.migrate.ListJobs(r.Context(), su.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if jobs == nil {
			jobs = []migrate.Job{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"allowed": true, "jobs": jobs})

	case path == "jobs" && r.Method == http.MethodPost:
		var req migrate.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		job, err := s.migrate.CreateJob(r.Context(), su.ID, req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, job)

	case strings.HasPrefix(path, "jobs/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "jobs/")
		job, err := s.migrate.GetJob(r.Context(), su.ID, id)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, job)

	case strings.HasSuffix(path, "/cancel") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "jobs/"), "/cancel")
		id = strings.Trim(id, "/")
		if err := s.migrate.CancelJob(r.Context(), su.ID, id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAdminMigration(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireDomainAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.migrate == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "migration unavailable"})
		return
	}
	jobs, err := s.migrate.ListJobsByTenant(r.Context(), admin.TenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = []migrate.Job{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}
