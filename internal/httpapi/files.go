package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/files"), "/") == "access" {
		s.handleFileAccess(w, r, au)
		return
	}
	ownerID := au.ID
	if requested := r.URL.Query().Get("owner_id"); requested != "" && requested != au.ID {
		rel := r.URL.Query().Get("path")
		operation := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/files"), "/")
		if operation != "move" && operation != "mkdir" && !s.fileAccessAllowed(r, requested, au.ID, rel, r.Method != http.MethodGet && r.Method != http.MethodHead) {
			writeJSON(w, 403, map[string]string{"error": "file access denied"})
			return
		}
		// Mutating JSON operations need independent source and destination checks.
		if operation == "move" || operation == "mkdir" {
			raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
			if err != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid json"})
				return
			}
			var payload struct {
				Path string `json:"path"`
				From string `json:"from"`
				To   string `json:"to"`
			}
			if json.Unmarshal(raw, &payload) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid json"})
				return
			}
			targets := []string{payload.Path}
			if operation == "move" {
				targets = []string{payload.From, payload.To}
			}
			for _, target := range targets {
				if !s.fileAccessAllowed(r, requested, au.ID, target, true) {
					writeJSON(w, 403, map[string]string{"error": "file access denied"})
					return
				}
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
		}
		ownerID = requested
	}
	root, err := s.filesRoot(ownerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/files"), "/")

	switch {
	case r.Method == http.MethodGet && path == "archive":
		s.downloadFileArchive(w, r, root)
	case r.Method == http.MethodGet && path == "":
		rel := r.URL.Query().Get("path")
		abs, err := safeJoin(root, rel)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			parent := cleanRel(filepath.Dir(rel))
			writeJSON(w, 200, map[string]any{"path": parent, "entries": []map[string]any{{"name": filepath.Base(abs), "is_dir": false, "size": info.Size(), "mtime": info.ModTime().UTC().Format(time.RFC3339)}}})
			return
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, http.StatusOK, map[string]any{"path": rel, "entries": []any{}})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			name := e.Name()
			item := map[string]any{
				"name": name, "is_dir": e.IsDir(),
				"mtime": info.ModTime().UTC().Format(time.RFC3339),
			}
			if !e.IsDir() {
				item["size"] = info.Size()
			}
			out = append(out, item)
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": cleanRel(rel), "entries": out})

	case r.Method == http.MethodGet && path == "content":
		rel := r.URL.Query().Get("path")
		abs, err := safeJoin(root, rel)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not a file"})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(abs)}))
		http.ServeContent(w, r, filepath.Base(abs), st.ModTime(), f)

	case r.Method == http.MethodPut && path == "content":
		rel := r.URL.Query().Get("path")
		abs, err := safeJoin(root, rel)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file exceeds 64 MiB or could not be read"})
			return
		}
		if err := os.WriteFile(abs, data, 0o640); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": cleanRel(rel), "size": len(data)})

	case r.Method == http.MethodPost && path == "mkdir":
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
			return
		}
		abs, err := safeJoin(root, req.Path)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := os.MkdirAll(abs, 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "created", "path": cleanRel(req.Path)})

	case r.Method == http.MethodDelete && path == "":
		rel := r.URL.Query().Get("path")
		if rel == "" || rel == "." || rel == "/" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
			return
		}
		abs, err := safeJoin(root, rel)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if abs == root {
			writeJSON(w, 400, map[string]string{"error": "cannot delete root"})
			return
		}
		if err := os.RemoveAll(abs); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	case r.Method == http.MethodPost && path == "move":
		var req struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to required"})
			return
		}
		src, err := safeJoin(root, req.From)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		dst, err := safeJoin(root, req.To)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if src == root || dst == root || src == dst || strings.HasPrefix(dst, src+string(os.PathSeparator)) {
			writeJSON(w, 400, map[string]string{"error": "invalid move"})
			return
		}
		if _, err := os.Lstat(dst); err == nil {
			writeJSON(w, 409, map[string]string{"error": "destination already exists"})
			return
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := os.Rename(src, dst); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "moved", "path": cleanRel(req.To)})

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) filesRoot(userID string) (string, error) {
	root := filepath.Join(s.ms.Root, "files", userID)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	return root, nil
}

func cleanRel(rel string) string {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = filepath.ToSlash(filepath.Clean("/" + rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." {
		return ""
	}
	return rel
}

func safeJoin(root, rel string) (string, error) {
	for _, part := range strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/") {
		if part == ".." {
			return "", errPathEscape
		}
	}
	rel = cleanRel(rel)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	abs = filepath.Clean(abs)
	rootClean := filepath.Clean(root)
	if abs != rootClean && !strings.HasPrefix(abs, rootClean+string(os.PathSeparator)) {
		return "", errPathEscape
	}
	// Reject symbolic-link ancestors, including paths for files not yet created.
	// User uploads cannot create links, and a local link must not escape isolation.
	for current := abs; current != rootClean; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errPathEscape
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return abs, nil
}

var errPathEscape = errString("invalid path")

type errString string

func (e errString) Error() string { return string(e) }
