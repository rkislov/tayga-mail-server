package httpapi

import (
	"encoding/json"
	"io"
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
	root, err := s.filesRoot(au.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/files"), "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		rel := r.URL.Query().Get("path")
		abs, err := safeJoin(root, rel)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
		w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(abs)+`"`)
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
		data, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read failed"})
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
	rel = cleanRel(rel)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	abs = filepath.Clean(abs)
	rootClean := filepath.Clean(root)
	if abs != rootClean && !strings.HasPrefix(abs, rootClean+string(os.PathSeparator)) {
		return "", errPathEscape
	}
	return abs, nil
}

var errPathEscape = errString("invalid path")

type errString string

func (e errString) Error() string { return string(e) }
