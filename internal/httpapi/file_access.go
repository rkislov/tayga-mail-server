package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func (s *Server) fileAccessAllowed(r *http.Request, owner, grantee, path string, write bool) bool {
	grants, err := s.store.ListFileAccess(r.Context(), owner, grantee)
	if err != nil {
		return false
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		return false
	}
	if _, err = safeJoin(root, path); err != nil {
		return false
	}
	path = cleanRel(path)
	for _, grant := range grants {
		if write && grant.Rights != "write" {
			continue
		}
		if path == grant.Path {
			return true
		}
		base, err := safeJoin(root, grant.Path)
		if err != nil {
			continue
		}
		info, err := os.Stat(base)
		if err == nil && info.IsDir() && strings.HasPrefix(path, grant.Path+"/") {
			return true
		}
	}
	return false
}
func (s *Server) handleFileAccess(w http.ResponseWriter, r *http.Request, au *authUser) {
	if r.Method == http.MethodGet {
		owner, grantee := au.ID, ""
		if r.URL.Query().Get("shared") == "1" {
			owner, grantee = "", au.ID
		}
		list, err := s.store.ListFileAccess(r.Context(), owner, grantee)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot list access"})
			return
		}
		out := []map[string]any{}
		for _, entry := range list {
			email := ""
			if u, err := s.store.GetUserByID(r.Context(), entry.GranteeID); err == nil {
				email = u.Email
			}
			root, _ := s.filesRoot(entry.OwnerID)
			abs, pathErr := safeJoin(root, entry.Path)
			isDir := false
			if pathErr == nil {
				if info, err := os.Stat(abs); err == nil {
					isDir = info.IsDir()
				}
			}
			out = append(out, map[string]any{"is_dir": isDir, "owner_id": entry.OwnerID, "path": entry.Path, "grantee_id": entry.GranteeID, "email": email, "rights": entry.Rights})
		}
		writeJSON(w, 200, map[string]any{"access": out})
		return
	}
	var req struct {
		Path      string `json:"path"`
		Email     string `json:"email"`
		Rights    string `json:"rights"`
		GranteeID string `json:"grantee_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	root, err := s.filesRoot(au.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "files unavailable"})
		return
	}
	abs, err := safeJoin(root, req.Path)
	if err != nil || cleanRel(req.Path) == "" {
		writeJSON(w, 400, map[string]string{"error": "file or folder path required"})
		return
	}
	path := cleanRel(req.Path)
	switch r.Method {
	case http.MethodPost:
		if _, err = os.Stat(abs); err != nil {
			writeJSON(w, 404, map[string]string{"error": "file not found"})
			return
		}
		if req.Rights != "read" && req.Rights != "write" {
			writeJSON(w, 400, map[string]string{"error": "rights must be read or write"})
			return
		}
		user, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "user not found"})
			return
		}
		if user.ID == au.ID {
			writeJSON(w, 400, map[string]string{"error": "owner already has access"})
			return
		}
		err = s.store.SetFileAccess(r.Context(), au.ID, path, user.ID, req.Rights)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot save access"})
			return
		}
	case http.MethodDelete:
		if err = s.store.DeleteFileAccess(r.Context(), au.ID, path, req.GranteeID); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot revoke access"})
			return
		}
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
