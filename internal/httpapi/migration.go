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
	if targetID := r.URL.Query().Get("target_user_id"); targetID != "" && targetID != su.ID {
		target, lookupErr := s.store.GetUserByID(r.Context(), targetID)
		if lookupErr != nil || !s.isAdminUser(su) || !s.adminCanManageUser(r, su, target) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "target account not permitted"})
			return
		}
		su = target
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

	case strings.HasPrefix(path, "jobs/") && strings.HasSuffix(path, "/retry") && r.Method == http.MethodPost:
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid request"})
			return
		}
		id := strings.TrimSuffix(strings.TrimPrefix(path, "jobs/"), "/retry")
		job, err := s.migrate.RetryJob(r.Context(), su.ID, id, req.Password)
		if err != nil {
			code := http.StatusConflict
			if errors.Is(err, storage.ErrNotFound) {
				code = 404
			}
			writeJSON(w, code, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, job)

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
	if r.URL.Query().Get("targets") == "1" {
		tenants, err := s.store.ListTenants(r.Context())
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "account lookup failed"})
			return
		}
		targets := []map[string]any{}
		for _, tenant := range tenants {
			if !s.isGlobalAdminUser(admin) && tenant.ID != admin.TenantID {
				continue
			}
			users, err := s.store.ListUsersByTenant(r.Context(), tenant.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "account lookup failed"})
				return
			}
			for _, target := range users {
				if s.adminCanManageUser(r, admin, target) {
					targets = append(targets, map[string]any{"id": target.ID, "email": target.Email, "allowed": s.migrationAllowed(r, target)})
				}
			}
		}
		writeJSON(w, 200, map[string]any{"targets": targets})
		return
	}
	jobs, err := s.migrate.ListJobsByTenant(r.Context(), admin.TenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	visible := []migrate.Job{}
	for _, job := range jobs {
		target, err := s.store.GetUserByID(r.Context(), job.UserID)
		if err == nil && s.adminCanManageUser(r, admin, target) {
			visible = append(visible, job)
		}
	}
	jobs = visible
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}
