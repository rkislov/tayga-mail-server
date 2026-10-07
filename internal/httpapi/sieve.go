package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/sieve"
)

func (s *Server) handleSieveScripts(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sieve/scripts")
	path = strings.Trim(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		scripts, err := s.store.ListSieveScripts(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(scripts))
		for _, sc := range scripts {
			out = append(out, map[string]any{
				"id": sc.ID, "name": sc.Name, "script": sc.Script,
				"active": sc.Active, "created_at": sc.CreatedAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"scripts": out})

	case r.Method == http.MethodPut && path != "" && !strings.Contains(path, "/"):
		var req struct {
			Script string `json:"script"`
			Active *bool  `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		name := path
		if name == "" || req.Script == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and script required"})
			return
		}
		if err := sieve.CheckScript(req.Script); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sieve: " + err.Error()})
			return
		}
		sc, err := s.store.PutSieveScript(r.Context(), au.ID, name, req.Script)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if req.Active != nil && *req.Active {
			if err := s.store.SetActiveSieveScript(r.Context(), au.ID, name); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			sc.Active = true
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id": sc.ID, "name": sc.Name, "script": sc.Script, "active": sc.Active,
		})

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/activate"):
		name := strings.Trim(strings.TrimSuffix(path, "/activate"), "/")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		if err := s.store.SetActiveSieveScript(r.Context(), au.ID, name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})

	case r.Method == http.MethodDelete && path != "" && !strings.Contains(path, "/"):
		if err := s.store.DeleteSieveScript(r.Context(), au.ID, path); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
