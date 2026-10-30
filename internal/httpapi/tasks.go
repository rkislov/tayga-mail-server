package httpapi

import (
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"strconv"
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
	parts := strings.Split(id, "/")
	if id == "assignees" || id == "notifications" || len(parts) == 2 {
		s.handleTaskActions(w, r, au.ID, parts)
		return
	}
	if len(parts) > 1 {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		if id != "" {
			task, e := s.store.GetTask(r.Context(), au.ID, id)
			if e != nil {
				writeJSON(w, 404, map[string]string{"error": "task unavailable"})
				return
			}
			writeJSON(w, 200, task)
			return
		}
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
			if err == storage.ErrTaskSeries {
				writeJSON(w, 409, map[string]string{"error": err.Error()})
			} else if err == storage.ErrInvalidTaskProgress || err == storage.ErrTaskAssignment {
				writeJSON(w, 400, map[string]string{"error": "invalid task status or percentage"})
			} else if err == storage.ErrUnauthorized {
				writeJSON(w, 403, map[string]string{"error": "Only the author can change task details; accept the assignment to update progress"})
			} else if err == storage.ErrNotFound {
				writeJSON(w, 409, map[string]string{"error": "Task changed or unavailable; refresh"})
			} else {
				writeJSON(w, 500, map[string]string{"error": "cannot save task"})
			}
			return
		}
		saved, e := s.store.GetTask(r.Context(), au.ID, task.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot load saved task"})
			return
		}
		writeJSON(w, 200, saved)
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

func (s *Server) handleTaskActions(w http.ResponseWriter, r *http.Request, user string, parts []string) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if len(parts) == 1 && parts[0] == "assignees" && r.Method == http.MethodGet {
		q := r.URL.Query().Get("q")
		if len(q) > 256 {
			writeJSON(w, 400, map[string]string{"error": "query too long"})
			return
		}
		users, total, err := s.store.TaskAssignees(r.Context(), user, q, limit, offset)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot load assignees"})
			return
		}
		writeJSON(w, 200, map[string]any{"users": users, "total": total})
		return
	}
	if len(parts) == 1 && parts[0] == "notifications" {
		switch r.Method {
		case http.MethodGet:
			rows, total, unread, err := s.store.TaskNotifications(r.Context(), user, limit, offset)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "cannot load notifications"})
				return
			}
			writeJSON(w, 200, map[string]any{"notifications": rows, "total": total, "unread": unread})
			return
		case http.MethodPost:
			var req struct {
				ID string `json:"id"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || req.ID == "" {
				writeJSON(w, 400, map[string]string{"error": "invalid notification"})
				return
			}
			if err := s.store.ReadTaskNotification(r.Context(), user, req.ID); err != nil {
				writeJSON(w, 404, map[string]string{"error": "notification unavailable"})
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	if len(parts) == 2 && parts[1] == "history" && r.Method == http.MethodGet {
		rows, total, err := s.store.TaskHistory(r.Context(), user, parts[0], limit, offset)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "task unavailable"})
			return
		}
		writeJSON(w, 200, map[string]any{"events": rows, "total": total})
		return
	}
	if len(parts) == 2 && parts[1] == "assignment" && r.Method == http.MethodPost {
		var req struct {
			Action     string `json:"action"`
			AssigneeID string `json:"assignee_id"`
			Revision   int64  `json:"revision"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid assignment"})
			return
		}
		task, err := s.store.AssignTask(r.Context(), user, parts[0], req.Action, req.AssigneeID, req.Revision)
		if err != nil {
			code := 500
			message := "cannot change assignment"
			switch err {
			case storage.ErrNotFound:
				code = 409
				message = "Task or assignee changed or unavailable; refresh"
			case storage.ErrUnauthorized:
				code = 403
				message = "Action not allowed for this task"
			case storage.ErrTaskSeries:
				code = 409
				message = err.Error()
			case storage.ErrTaskAssignment:
				code = 400
				message = "Invalid assignment transition"
			}
			writeJSON(w, code, map[string]string{"error": message})
			return
		}
		writeJSON(w, 200, map[string]any{"task": task, "ok": true})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}
