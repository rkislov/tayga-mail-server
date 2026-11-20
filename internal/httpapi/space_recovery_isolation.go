package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) initializeSpaceRecovery(ctx context.Context) error {
	// Lightweight API embeddings may omit the mail/file store entirely.
	if s.ms == nil || s.store == nil {
		return nil
	}
	s.spaceRecoveryOnce.Do(func() {
		s.spaceRecoveryError = s.recoverPendingSpaceRestores(ctx)
		if s.spaceRecoveryError != nil {
			return
		}
		ids, err := s.store.ListFileSpaceIDs(ctx)
		if err != nil {
			s.spaceRecoveryError = err
			return
		}
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		for _, id := range ids {
			points, err := s.listSpaceCheckpoints(id)
			if err != nil {
				s.spaceRecoveryError = err
				return
			}
			if len(points) == 0 {
				if _, err = s.captureSpaceCheckpoint(ctx, id, "Initial recovery point", true); err != nil {
					s.log.Error("initial space checkpoint failed", "space", id, "error", err)
					if s.spaceCheckpointErrors == nil {
						s.spaceCheckpointErrors = map[string]string{}
					}
					s.spaceCheckpointErrors[id] = "Initial checkpoint failed"
				}
			}
		}
	})
	return s.spaceRecoveryError
}

// Protect live reads during the short filesystem/DB handover. Long downloads
// make restore return 423 instead of exposing half-restored state or hanging.
func (s *Server) spaceRecoveryReadIsolation(next http.Handler) http.Handler {
	if s.ms == nil || s.store == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != "PROPFIND" {
			next.ServeHTTP(w, r)
			return
		}
		owner := ""
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/v1/files/") || path == "/api/v1/files" {
			owner = r.URL.Query().Get("owner_id")
		}
		if strings.HasPrefix(path, "/dav/spaces/") {
			owner = strings.SplitN(strings.TrimPrefix(path, "/dav/spaces/"), "/", 2)[0]
		}
		if strings.HasPrefix(path, "/s/") {
			if share, err := s.store.GetFileShareByToken(r.Context(), strings.Trim(strings.TrimPrefix(path, "/s/"), "/")); err == nil {
				owner = share.UserID
			}
		}
		if strings.HasPrefix(path, "/wopi/") || strings.HasPrefix(path, "/office/") {
			token := r.URL.Query().Get("access_token")
			s.fileMu.Lock()
			if session, ok := s.officeSessions[token]; ok {
				owner = session.Owner
			}
			if session, ok := s.fileComparisons[token]; ok {
				owner = session.Owner
			}
			s.fileMu.Unlock()
		}
		if owner != "" {
			if _, err := s.store.GetFileSpace(r.Context(), owner); err == nil {
				gate := s.spaceReadLock(owner)
				gate.RLock()
				defer gate.RUnlock()
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) cleanSpaceRecoveryPlans(ctx context.Context) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	for _, kind := range []string{"plans", "imports"} {
		root := filepath.Join(s.ms.Root, "space-recovery", kind)
		dirs, _ := os.ReadDir(root)
		for _, dir := range dirs {
			if ctx.Err() != nil {
				return
			}
			if !dir.IsDir() {
				continue
			}
			info, err := dir.Info()
			if err != nil {
				continue
			}
			if time.Since(info.ModTime()) > 24*time.Hour {
				_ = os.RemoveAll(filepath.Join(root, dir.Name()))
			}
		}
	}
}
