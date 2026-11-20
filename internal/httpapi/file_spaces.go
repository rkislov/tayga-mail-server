package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/tayga/tms/internal/storage"
	"golang.org/x/net/webdav"
)

func (s *Server) fileOwnerActive(ctx context.Context, id string) bool {
	if s.spaceIsBlocked(id) {
		return false
	}
	if space, err := s.store.GetFileSpace(ctx, id); err == nil {
		return space != nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return false
	}
	user, err := s.store.GetUserByID(ctx, id)
	return err == nil && user != nil && user.Enabled
}
func (s *Server) fileOwnerLabel(ctx context.Context, id string) string {
	if space, err := s.store.GetFileSpace(ctx, id); err == nil {
		return space.Name
	}
	if user, err := s.store.GetUserByID(ctx, id); err == nil {
		return user.Email
	}
	return ""
}
func (s *Server) fileCanManage(ctx context.Context, owner, actor string) bool {
	if s.spaceIsBlocked(owner) {
		return false
	}
	if owner == actor {
		return s.fileOwnerActive(ctx, actor)
	}
	rights, err := s.store.FileSpaceRights(ctx, owner, actor)
	return err == nil && rights == "manage"
}
func fileSpaceError(w http.ResponseWriter, err error) {
	code := 500
	switch {
	case errors.Is(err, storage.ErrFileSpaceForbidden):
		code = 403
	case errors.Is(err, storage.ErrFileSpaceConflict):
		code = 409
	case errors.Is(err, storage.ErrFileSpaceInvalid):
		code = 400
	case errors.Is(err, storage.ErrNotFound):
		code = 404
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
func (s *Server) handleFileSpaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	user, err := s.store.GetUserByID(r.Context(), actor.ID)
	if err != nil {
		fileError(w, err)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/files/spaces"), "/")
	if parts := strings.SplitN(id, "/recovery", 2); len(parts) == 2 {
		s.handleSpaceRecovery(w, r, parts[0], strings.Trim(parts[1], "/"))
		return
	}
	if r.Method == http.MethodGet && id == "" {
		spaces, err := s.store.ListFileSpaces(r.Context(), actor.ID)
		if err != nil {
			fileSpaceError(w, err)
			return
		}
		for _, space := range spaces {
			if space.Rights != "manage" {
				space.Groups = []storage.FileSpaceGroup{}
			}
		}
		writeJSON(w, 200, map[string]any{"spaces": spaces, "tenant_id": user.TenantID, "can_create": storage.IsGlobalAdmin(user.Roles) || storage.HasRole(user.Roles, storage.RoleTenantAdmin)})
		return
	}
	if r.Method == http.MethodGet && id == "groups" {
		tenant := user.TenantID
		if value := r.URL.Query().Get("tenant_id"); value != "" {
			tenant = value
		}
		allowed := storage.IsGlobalAdmin(user.Roles) || (tenant == user.TenantID && storage.HasRole(user.Roles, storage.RoleTenantAdmin))
		if spaceID := r.URL.Query().Get("space_id"); spaceID != "" {
			space, e := s.store.GetFileSpace(r.Context(), spaceID)
			if e != nil {
				fileSpaceError(w, e)
				return
			}
			tenant = space.TenantID
			allowed = s.fileCanManage(r.Context(), spaceID, actor.ID)
		}
		if !allowed {
			writeJSON(w, 403, map[string]string{"error": "space management required"})
			return
		}
		domains, e := s.store.ListDomainsByTenant(r.Context(), tenant)
		if e != nil {
			fileError(w, e)
			return
		}
		groups := []map[string]string{}
		for _, domain := range domains {
			list, e := s.store.ListDirectoryGroups(r.Context(), domain.ID)
			if e != nil {
				fileError(w, e)
				return
			}
			for _, group := range list {
				groups = append(groups, map[string]string{"id": group.ID, "name": group.Name, "email": group.Email})
			}
		}
		writeJSON(w, 200, map[string]any{"groups": groups})
		return
	}
	if r.Method == http.MethodGet && id != "" {
		rights, e := s.store.FileSpaceRights(r.Context(), id, actor.ID)
		if e != nil {
			fileSpaceError(w, e)
			return
		}
		if rights == "" {
			writeJSON(w, 403, map[string]string{"error": "space access denied"})
			return
		}
		space, e := s.store.GetFileSpace(r.Context(), id)
		if e != nil {
			fileSpaceError(w, e)
			return
		}
		space.Rights = rights
		if rights != "manage" {
			space.Groups = []storage.FileSpaceGroup{}
		}
		writeJSON(w, 200, space)
		return
	}
	if (r.Method == http.MethodPost && id == "") || (r.Method == http.MethodPut && id != "" && id != "groups") {
		var space storage.FileSpace
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&space) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid space"})
			return
		}
		space.ID = id
		if space.TenantID == "" {
			space.TenantID = user.TenantID
		}
		s.fileMu.Lock()
		if s.spaceIsBlocked(id) {
			s.fileMu.Unlock()
			fileError(w, errSpaceRecoveryPending)
			return
		}
		saved, e := s.store.SaveFileSpace(r.Context(), actor.ID, &space)
		if e == nil {
			action := "space_updated"
			if id == "" {
				action = "space_created"
			}
			s.recordFileActivity(saved.ID, "", action, []fileActor{{ID: actor.ID, Source: "web"}})
		}
		s.fileMu.Unlock()
		if e != nil {
			fileSpaceError(w, e)
			return
		}
		saved.Rights, _ = s.store.FileSpaceRights(r.Context(), saved.ID, actor.ID)
		code := 200
		if id == "" {
			code = 201
		}
		writeJSON(w, code, saved)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
func (s *Server) serveSpacesDAVIndex(w http.ResponseWriter, r *http.Request, actor string) {
	if r.Method != "PROPFIND" && r.Method != "OPTIONS" {
		http.Error(w, "read-only collection", 403)
		return
	}
	if r.Method == "PROPFIND" && r.Header.Get("Depth") != "0" && r.Header.Get("Depth") != "1" {
		http.Error(w, "use Depth 0 or 1", 403)
		return
	}
	spaces, err := s.store.ListFileSpaces(r.Context(), actor)
	if err != nil {
		fileError(w, err)
		return
	}
	entries := []os.FileInfo{}
	for _, space := range spaces {
		entries = append(entries, davSharedInfo{name: space.ID})
	}
	handler := webdav.Handler{Prefix: "/dav/spaces", FileSystem: &sharedDAVIndex{entries: entries}, LockSystem: webdav.NewMemLS()}
	handler.ServeHTTP(w, r)
}

func (s *Server) fileOwnerNotificationUsers(ctx context.Context, owner string) []string {
	space, err := s.store.GetFileSpace(ctx, owner)
	if err != nil {
		return []string{owner}
	}
	users, err := s.store.ListUsersByTenant(ctx, space.TenantID)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, user := range users {
		if user.Enabled && s.fileCanManage(ctx, owner, user.ID) {
			out = append(out, user.ID)
		}
	}
	return out
}

func (s *Server) requireFileManagement(w http.ResponseWriter, r *http.Request, owner string) bool {
	actor, err := s.userFromBearer(r)
	if err != nil || !s.fileCanManage(r.Context(), owner, actor.ID) {
		writeJSON(w, 403, map[string]string{"error": "owner or space management required"})
		return false
	}
	return true
}
