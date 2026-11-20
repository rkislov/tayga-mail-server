package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleNoteVersions(w http.ResponseWriter, r *http.Request, userID string, parts []string) {
	need := "read"
	if r.Method != http.MethodGet {
		need = "write"
	}
	n, rights, err := s.noteCan(r.Context(), userID, parts[0], need)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method == http.MethodGet && len(parts) == 2 {
		versions, err := s.store.ListNoteVersions(r.Context(), n.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot load versions"})
			return
		}

		out := []map[string]any{}
		for _, v := range versions {
			out = append(out, map[string]any{"id": v.ID, "saved_at": v.SavedAt, "title": v.Note.Title, "body_text": v.Note.BodyText})
		}
		writeJSON(w, http.StatusOK, map[string]any{"versions": out})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 4 && parts[3] == "restore" {
		var req struct {
			ETag string `json:"etag"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.ETag == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "etag required"})
			return
		}
		if req.ETag != n.ETag {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "etag mismatch"})
			return
		}
		v, err := s.store.GetNoteVersion(r.Context(), n.ID, parts[2])
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, storage.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{"error": "version unavailable"})
			return
		}
		// Restore content only: never revive an old folder or sharing policy.
		n.Title, n.DocumentJSON, n.BodyHTML, n.BodyText = v.Note.Title, v.Note.DocumentJSON, v.Note.BodyHTML, v.Note.BodyText
		if err := s.store.UpdateNoteItem(r.Context(), n); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, storage.ErrNoteConflict) {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, noteItemJSON(n, s.noteAttachmentsJSON(r, n.ID), rights))
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
