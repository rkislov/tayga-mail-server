package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

func (s *Server) fileAccessAllowed(r *http.Request, owner, grantee, path string, write bool) bool {
	return s.fileAccessAllowedContext(r.Context(), owner, grantee, path, write)
}

func (s *Server) fileAccessAllowedContext(ctx context.Context, owner, grantee, path string, write bool) bool {
	if s.spaceIsBlocked(owner) {
		return false
	}
	if space, e := s.store.GetFileSpace(ctx, owner); e == nil {
		rights, e := s.store.FileSpaceRights(ctx, owner, grantee)
		if e != nil {
			return false
		}
		if rights != "" && (!write || rights == "write" || rights == "manage") {
			return true
		}
		user, e := s.store.GetUserByID(ctx, grantee)
		if e != nil || !user.Enabled || user.TenantID != space.TenantID {
			return false
		}
	}
	grants, err := s.store.ListFileAccess(ctx, owner, grantee)
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
	ownerID := au.ID
	if requested := r.URL.Query().Get("owner_id"); requested != "" && requested != au.ID {
		if !s.fileCanManage(r.Context(), requested, au.ID) {
			writeJSON(w, 403, map[string]string{"error": "space management required"})
			return
		}
		ownerID = requested
	}
	if r.Method == http.MethodGet {
		owner, grantee := ownerID, ""
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
	root, err := s.filesRoot(ownerID)
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
		if space, e := s.store.GetFileSpace(r.Context(), ownerID); e == nil && space.TenantID != user.TenantID {
			writeJSON(w, 400, map[string]string{"error": "recipient must belong to space tenant"})
			return
		}
		if user.ID == au.ID {
			writeJSON(w, 400, map[string]string{"error": "owner already has access"})
			return
		}
		s.fileMu.Lock()
		if !s.requireFileManagement(w, r, ownerID) {
			s.fileMu.Unlock()
			return
		}
		if _, err = os.Stat(abs); err == nil {
			err = s.store.SetFileAccess(r.Context(), ownerID, path, user.ID, req.Rights)
		}
		if err == nil {
			s.recordFileEvent(ownerID, fileActivityEvent{Path: path, Action: "access_changed", Actor: fileActor{ID: au.ID, Source: "web"}, Subject: user.Email, Rights: req.Rights})
		}
		s.fileMu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot save access"})
			return
		}
	case http.MethodDelete:
		if req.GranteeID == "" {
			writeJSON(w, 400, map[string]string{"error": "grantee_id required"})
			return
		}
		s.fileMu.Lock()
		if !s.requireFileManagement(w, r, ownerID) {
			s.fileMu.Unlock()
			return
		}
		existing, e := s.store.ListFileAccess(r.Context(), ownerID, req.GranteeID)
		err = e
		if err == nil {
			err = s.store.DeleteFileAccess(r.Context(), ownerID, path, req.GranteeID)
		}
		if err == nil {
			for _, g := range existing {
				if g.Path == path {
					subject := req.GranteeID
					if u, e := s.store.GetUserByID(r.Context(), req.GranteeID); e == nil {
						subject = u.Email
					}
					s.recordFileEvent(ownerID, fileActivityEvent{Path: path, Action: "access_revoked", Actor: fileActor{ID: au.ID, Source: "web"}, Subject: subject})
					break
				}
			}
		}
		s.fileMu.Unlock()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot revoke access"})
			return
		}
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
