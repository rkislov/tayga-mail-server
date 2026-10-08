package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleAdminOutbound(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/outbound")
	path = strings.Trim(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		s.listOutbound(w, r)
	case path != "" && r.Method == http.MethodDelete:
		s.deleteOutbound(w, r, path)
	case path != "" && r.Method == http.MethodPost && strings.HasSuffix(path, "/retry"):
		id := strings.TrimSuffix(path, "/retry")
		id = strings.Trim(id, "/")
		s.retryOutbound(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) listOutbound(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	items, err := s.store.ListOutbound(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	total, _ := s.store.CountOutbound(r.Context())
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"id":            it.ID,
			"envelope_from": it.EnvelopeFrom,
			"envelope_to":   it.EnvelopeTo,
			"message_id":    it.MessageID,
			"attempts":      it.Attempts,
			"max_attempts":  it.MaxAttempts,
			"next_attempt":  it.NextAttempt.UTC().Format(time.RFC3339),
			"last_error":    it.LastError,
			"size":          len(it.Data),
			"created_at":    it.CreatedAt.UTC().Format(time.RFC3339),
			"updated_at":    it.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": out})
}

func (s *Server) deleteOutbound(w http.ResponseWriter, r *http.Request, id string) {
	if _, err := s.store.GetOutbound(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.DeleteOutbound(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) retryOutbound(w http.ResponseWriter, r *http.Request, id string) {
	it, err := s.store.GetOutbound(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.RescheduleOutbound(r.Context(), it.ID, it.Attempts, time.Now().UTC(), it.LastError); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	detail := "admin retry"
	if it.LastError != "" {
		detail = "admin retry: " + it.LastError
	}
	s.writeMailLog(r.Context(), &storage.User{Email: it.EnvelopeFrom, TenantID: ""}, "queued", "outbound", it.EnvelopeTo, it.MessageID, int64(len(it.Data)), detail)
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}
