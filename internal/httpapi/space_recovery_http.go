package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) restoreAdministrator(ctx context.Context, actor, tenant string) bool {
	user, err := s.store.GetUserByID(ctx, actor)
	return err == nil && user.Enabled && (storage.IsGlobalAdmin(user.Roles) || (user.TenantID == tenant && storage.HasRole(user.Roles, storage.RoleTenantAdmin)))
}
func (s *Server) canRestoreTarget(ctx context.Context, actor string, target *storage.FileSpaceSnapshot) bool {
	if target == nil || target.Space == nil {
		return false
	}
	if identity, e := s.readRecoveryOwner(target.Space.ID); e == nil && identity.Tenant != target.Space.TenantID {
		return false
	} else if e != nil && !os.IsNotExist(e) {
		return false
	}
	space, err := s.store.GetFileSpace(ctx, target.Space.ID)
	if err == nil {
		return space.TenantID == target.Space.TenantID && s.canRestoreSpace(ctx, space.ID, actor)
	}
	if !errors.Is(err, storage.ErrNotFound) || !s.restoreAdministrator(ctx, actor, target.Space.TenantID) {
		return false
	}
	user, e := s.store.GetUserByID(ctx, actor)
	if e != nil {
		return false
	}
	if storage.IsGlobalAdmin(user.Roles) {
		return true
	}
	for _, kind := range []string{"files", "file-state"} {
		entries, e := os.ReadDir(filepath.Join(s.ms.Root, kind, target.Space.ID))
		if e != nil && !os.IsNotExist(e) {
			return false
		}
		if len(entries) > 0 {
			return false
		}
	}
	return true
}
func spaceRecoveryPlanView(plan *spaceRestorePlan, point *spaceCheckpoint) map[string]any {
	changes := plan.Changes
	if len(changes) > 200 {
		changes = changes[:200]
	}
	return map[string]any{"plan_id": plan.ID, "space_id": plan.Space, "checkpoint_id": plan.Checkpoint, "checkpoint_at": plan.At, "mode": plan.Mode, "path": plan.Path, "expires": plan.Expires, "changes": changes, "total_changes": len(plan.Changes), "before_bytes": plan.BeforeBytes, "after_bytes": plan.AfterBytes, "metadata_restored": plan.Metadata, "settings": point.State.Space, "access_count": len(point.State.Access), "link_count": len(point.State.Shares), "safety_checkpoint": true, "folder_direct_access_revoked": plan.Mode == "folder", "projected_used_bytes": plan.ProjectedUsed, "quota_bytes": plan.Quota, "quota_blocked": plan.QuotaBlocked}
}
func (s *Server) handleSpaceRecovery(w http.ResponseWriter, r *http.Request, space, action string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	actor, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if !storage.ValidFileSpaceID(space) {
		fileError(w, errPathEscape)
		return
	}
	if action == "import" && r.Method == http.MethodPost {
		s.receiveSpaceBackup(w, r, actor.ID, space)
		return
	}
	if !s.canRestoreSpace(r.Context(), space, actor.ID) && action != "apply" && action != "preview" {
		writeJSON(w, 403, map[string]string{"error": "space management required"})
		return
	}
	if action == "" && r.Method == http.MethodGet {
		s.fileMu.Lock()
		points, e := s.listSpaceCheckpoints(space)
		health := s.spaceCheckpointErrors[space]
		s.fileMu.Unlock()
		if e != nil {
			fileError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"checkpoints": points, "automatic_limit": 50, "last_error": health, "pending": s.spaceIsBlocked(space)})
		return
	}
	if action == "retry" && r.Method == http.MethodPost {
		if e := s.recoverPendingSpaceRestores(r.Context(), space); e != nil {
			fileError(w, e)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "recovered"})
		return
	}
	if strings.HasPrefix(action, "checkpoints/") && r.Method == http.MethodDelete {
		id := strings.TrimPrefix(action, "checkpoints/")
		if !storage.ValidFileSpaceID(id) {
			fileError(w, errPathEscape)
			return
		}
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		if !s.requireFileManagement(w, r, space) {
			return
		}
		if s.spaceCheckpointReaders[space+"/"+id] > 0 {
			fileError(w, errFileConflict)
			return
		}
		if _, e := s.readSpaceCheckpoint(space, id); e != nil {
			fileError(w, e)
			return
		}
		if e := os.RemoveAll(filepath.Join(s.checkpointRoot(space), id)); e != nil {
			fileError(w, e)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	var req struct {
		Label      string    `json:"label"`
		Checkpoint string    `json:"checkpoint_id"`
		At         time.Time `json:"at"`
		Mode       string    `json:"mode"`
		Path       string    `json:"path"`
		Plan       string    `json:"plan_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid recovery request"})
		return
	}
	switch action {
	case "checkpoints":
		if len(req.Label) > 200 {
			fileError(w, storage.ErrFileSpaceInvalid)
			return
		}
		s.fileMu.Lock()
		if !s.requireFileManagement(w, r, space) {
			s.fileMu.Unlock()
			return
		}
		point, e := s.captureSpaceCheckpoint(r.Context(), space, req.Label, false)
		s.fileMu.Unlock()
		if e != nil {
			fileError(w, e)
			return
		}
		writeJSON(w, 201, point.Info)
	case "preview":
		if req.Checkpoint == "" && !req.At.IsZero() {
			s.fileMu.Lock()
			points, e := s.listSpaceCheckpoints(space)
			s.fileMu.Unlock()
			if e != nil {
				fileError(w, e)
				return
			}
			for _, point := range points {
				if !point.At.After(req.At) {
					req.Checkpoint = point.ID
					break
				}
			}
		}
		if req.Mode == "" {
			req.Mode = "full"
		}
		plan, e := s.createSpaceRestorePlan(r.Context(), actor.ID, space, req.Checkpoint, req.Mode, req.Path)
		if e != nil {
			recoveryHTTPError(w, e)
			return
		}
		point, e := readCheckpointFile(filepath.Join(s.ms.Root, "space-recovery", "plans", plan.ID, "target", "checkpoint.json"))
		if e != nil {
			fileError(w, e)
			return
		}
		writeJSON(w, 200, spaceRecoveryPlanView(plan, point))
	case "apply":
		if !storage.ValidFileSpaceID(req.Plan) {
			fileError(w, errPathEscape)
			return
		}
		data, e := os.ReadFile(filepath.Join(s.ms.Root, "space-recovery", "plans", req.Plan, "plan.json"))
		var plan spaceRestorePlan
		if e != nil || json.Unmarshal(data, &plan) != nil || plan.Space != space {
			fileError(w, errPathEscape)
			return
		}
		safety, e := s.applySpaceRestorePlan(r.Context(), actor.ID, req.Plan)
		if e != nil {
			recoveryHTTPError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"status": "restored", "space_id": space, "safety_checkpoint_id": safety})
	case "download":
		s.fileMu.Lock()
		if !s.requireFileManagement(w, r, space) {
			s.fileMu.Unlock()
			return
		}
		_, e := s.readSpaceCheckpoint(space, req.Checkpoint)
		if e != nil {
			s.fileMu.Unlock()
			fileError(w, e)
			return
		}
		now := time.Now()
		if s.spaceDownloadTickets == nil {
			s.spaceDownloadTickets = map[string]spaceDownloadTicket{}
		}
		for token, ticket := range s.spaceDownloadTickets {
			if now.After(ticket.Expires) {
				delete(s.spaceDownloadTickets, token)
			}
		}
		if len(s.spaceDownloadTickets) >= 128 {
			s.fileMu.Unlock()
			w.WriteHeader(429)
			return
		}
		token := storage.NewID() + storage.NewID()
		s.spaceDownloadTickets[token] = spaceDownloadTicket{Space: space, Point: req.Checkpoint, Actor: actor.ID, Expires: now.Add(2 * time.Minute)}
		s.fileMu.Unlock()
		writeJSON(w, 200, map[string]string{"url": "/api/v1/files/space-recovery/download?ticket=" + token})
	default:
		w.WriteHeader(404)
	}
}
func recoveryHTTPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrFileSpaceForbidden):
		writeJSON(w, 403, map[string]string{"error": err.Error()})
	case errors.Is(err, storage.ErrFileSpaceInvalid):
		writeJSON(w, 400, map[string]string{"error": err.Error()})
	default:
		fileError(w, err)
	}
}
func (s *Server) handleSpaceRecoveryGlobal(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/files/space-recovery/import" && r.Method == http.MethodPost {
		actor, e := s.userFromBearer(r)
		if e != nil {
			w.WriteHeader(401)
			return
		}
		user, e := s.store.GetUserByID(r.Context(), actor.ID)
		if e != nil || (!storage.IsGlobalAdmin(user.Roles) && !storage.HasRole(user.Roles, storage.RoleTenantAdmin)) {
			w.WriteHeader(403)
			return
		}
		s.receiveSpaceBackup(w, r, actor.ID, "")
		return
	}
	if r.URL.Path != "/api/v1/files/space-recovery/download" || r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	token := r.URL.Query().Get("ticket")
	s.fileMu.Lock()
	ticket, ok := s.spaceDownloadTickets[token]
	delete(s.spaceDownloadTickets, token)
	if !ok || time.Now().After(ticket.Expires) || !s.canRestoreSpace(r.Context(), ticket.Space, ticket.Actor) {
		s.fileMu.Unlock()
		w.WriteHeader(403)
		return
	}
	point, e := s.readSpaceCheckpoint(ticket.Space, ticket.Point)
	if e != nil {
		s.fileMu.Unlock()
		fileError(w, e)
		return
	}
	key := ticket.Space + "/" + ticket.Point
	if s.spaceCheckpointReaders == nil {
		s.spaceCheckpointReaders = map[string]int{}
	}
	s.spaceCheckpointReaders[key]++
	s.fileMu.Unlock()
	defer func() { s.fileMu.Lock(); s.spaceCheckpointReaders[key]--; s.fileMu.Unlock() }()
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="space-%s-%s.tar.gz"`, ticket.Space, ticket.Point))
	if e = writeSpaceArchive(r.Context(), filepath.Join(s.checkpointRoot(ticket.Space), ticket.Point), point, w); e != nil {
		s.log.Error("space archive download failed", "space", ticket.Space, "error", e)
	}
}

var spaceArchiveImportSlots = make(chan struct{}, 2)

func (s *Server) receiveSpaceBackup(w http.ResponseWriter, r *http.Request, actor, expected string) {
	w.Header().Set("Cache-Control", "no-store")
	if expected != "" {
		if !s.canRestoreSpace(r.Context(), expected, actor) {
			space, e := s.store.GetFileSpace(r.Context(), expected)
			if e == nil || space != nil {
				w.WriteHeader(403)
				return
			}
			user, e := s.store.GetUserByID(r.Context(), actor)
			if e != nil || !s.restoreAdministrator(r.Context(), actor, user.TenantID) {
				w.WriteHeader(403)
				return
			}
		}
	}
	select {
	case spaceArchiveImportSlots <- struct{}{}:
		defer func() { <-spaceArchiveImportSlots }()
	default:
		w.WriteHeader(429)
		return
	}
	temp, point, e := s.importSpaceArchive(r.Context(), http.MaxBytesReader(w, r.Body, recoveryMaxBytes+recoveryManifestLimit))
	if e != nil {
		var diskError *os.PathError
		var limitError *http.MaxBytesError
		switch {
		case errors.As(e, &limitError), errors.Is(e, errFileTooLarge):
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "archive exceeds recovery limits"})
		case errors.As(e, &diskError), errors.Is(e, context.Canceled), errors.Is(e, context.DeadlineExceeded):
			recoveryHTTPError(w, e)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or corrupt space archive"})
		}
		return
	}
	defer os.RemoveAll(temp)
	if (expected != "" && point.Info.SpaceID != expected) || !s.canRestoreTarget(r.Context(), actor, point.State) {
		w.WriteHeader(403)
		return
	}
	if e = s.store.ValidateFileSpaceSnapshot(r.Context(), point.State); e != nil {
		recoveryHTTPError(w, e)
		return
	}
	s.fileMu.Lock()
	root := s.checkpointRoot(point.Info.SpaceID)
	e = s.ensureRecoveryOwner(point.Info.SpaceID, point.State.Space.TenantID, point.State.Space.Generation)
	if e == nil {
		e = syncRecoveryTree(temp)
	}
	if e == nil {
		e = os.Rename(temp, filepath.Join(root, point.Info.ID))
	}
	if e == nil {
		e = syncRecoveryDir(root)
	}
	s.fileMu.Unlock()
	if e != nil {
		fileError(w, e)
		return
	}
	plan, e := s.createSpaceRestorePlan(r.Context(), actor, point.Info.SpaceID, point.Info.ID, "full", "")
	if e != nil {
		recoveryHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"checkpoint": point.Info, "preview": spaceRecoveryPlanView(plan, point)})
}
