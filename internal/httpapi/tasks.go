package httpapi

import (
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks")
	id = strings.Trim(id, "/")
	switch r.Method {
	case http.MethodGet:
		tasks, err := s.store.ListTasks(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot load tasks"})
			return
		}
		writeJSON(w, 200, map[string]any{"tasks": tasks})
	case http.MethodPost, http.MethodPut:
		var task storage.Task
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&task) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid task"})
			return
		}
		if strings.TrimSpace(task.Title) == "" || len(task.Title) > 1024 || len(task.Body) > 65536 {
			writeJSON(w, 400, map[string]string{"error": "invalid title or body"})
			return
		}
		if task.Due != "" {
			if _, err := time.Parse(time.RFC3339, task.Due); err != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid due date"})
				return
			}
		}
		task.ID = id
		task.SourceMessageID = ""
		if (r.Method == http.MethodPost && id != "") || (r.Method == http.MethodPut && id == "") {
			writeJSON(w, 400, map[string]string{"error": "invalid task ID"})
			return
		}
		if err := s.store.SaveTask(r.Context(), au.ID, &task); err != nil {
			if err == storage.ErrNotFound {
				writeJSON(w, 409, map[string]string{"error": "Task changed or unavailable; refresh"})
			} else {
				writeJSON(w, 500, map[string]string{"error": "cannot save task"})
			}
			return
		}
		writeJSON(w, 200, task)
	case http.MethodDelete:
		if err := s.store.DeleteTask(r.Context(), au.ID, id); err != nil {
			writeJSON(w, 404, map[string]string{"error": "task unavailable"})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
