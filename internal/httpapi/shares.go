package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleFileShares(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/files/shares"), "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		list, err := s.store.ListFileShares(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(list))
		base := strings.TrimRight(s.publicBase(), "/")
		for _, sh := range list {
			item := map[string]any{
				"id": sh.ID, "path": sh.Path, "token": sh.Token,
				"url": base + "/s/" + sh.Token,
				"max_downloads": sh.MaxDownloads, "download_count": sh.DownloadCount,
				"created_at": sh.CreatedAt,
			}
			if sh.ExpiresAt != nil {
				item["expires_at"] = sh.ExpiresAt.UTC().Format(time.RFC3339)
			}
			out = append(out, item)
		}
		writeJSON(w, http.StatusOK, map[string]any{"shares": out})

	case r.Method == http.MethodPost && path == "":
		var req struct {
			Path         string `json:"path"`
			TTLHours     int    `json:"ttl_hours"`
			MaxDownloads int    `json:"max_downloads"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		rel := cleanRel(req.Path)
		if rel == "" || strings.HasSuffix(rel, "/") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file path required"})
			return
		}
		root, err := s.filesRoot(au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		abs, err := safeJoin(root, rel)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		st, err := os.Stat(abs)
		if err != nil || st.IsDir() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file not found"})
			return
		}
		sh := &storage.FileShare{UserID: au.ID, Path: rel, MaxDownloads: req.MaxDownloads}
		if req.TTLHours > 0 {
			exp := time.Now().UTC().Add(time.Duration(req.TTLHours) * time.Hour)
			sh.ExpiresAt = &exp
		} else {
			exp := time.Now().UTC().Add(7 * 24 * time.Hour)
			sh.ExpiresAt = &exp
		}
		saved, err := s.store.CreateFileShare(r.Context(), sh)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": saved.ID, "token": saved.Token, "url": strings.TrimRight(s.publicBase(), "/") + "/s/" + saved.Token,
			"expires_at": saved.ExpiresAt,
		})

	case r.Method == http.MethodDelete && path != "":
		if err := s.store.DeleteFileShare(r.Context(), au.ID, path); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) publicBase() string {
	cfg := s.cfgLive()
	if cfg != nil && cfg.HTTP.PublicURL != "" {
		return strings.TrimRight(cfg.HTTP.PublicURL, "/")
	}
	return "http://127.0.0.1:18080"
}

func (s *Server) handlePublicShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/s/"), "/")
	if token == "" || strings.Contains(token, "/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	sh, err := s.store.GetFileShareByToken(r.Context(), token)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if sh.ExpiresAt != nil && time.Now().UTC().After(*sh.ExpiresAt) {
		writeJSON(w, http.StatusGone, map[string]string{"error": "expired"})
		return
	}
	if sh.MaxDownloads > 0 && sh.DownloadCount >= sh.MaxDownloads {
		writeJSON(w, http.StatusGone, map[string]string{"error": "download limit reached"})
		return
	}
	root, err := s.filesRoot(sh.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	abs, err := safeJoin(root, sh.Path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	_ = s.store.IncrementFileShareDownload(r.Context(), sh.ID)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(abs)+`"`)
	http.ServeContent(w, r, filepath.Base(abs), st.ModTime(), f)
}
