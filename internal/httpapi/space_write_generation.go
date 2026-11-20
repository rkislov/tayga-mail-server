package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) spaceWriteGeneration(owner string) (int64, bool, error) {
	space, err := s.store.GetFileSpace(context.Background(), owner)
	if errors.Is(err, storage.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return space.Generation, true, nil
}
func (s *Server) saveStageGeneration(temp, owner string) error {
	generation, space, err := s.spaceWriteGeneration(owner)
	if err != nil || !space {
		return err
	}
	data, _ := json.Marshal(struct {
		Owner      string
		Generation int64
	}{owner, generation})
	return os.WriteFile(temp+".generation", data, 0600)
}
func (s *Server) checkStageGeneration(temp, owner string) error {
	if s.spaceIsBlocked(owner) {
		return errSpaceRecoveryPending
	}
	data, err := os.ReadFile(temp + ".generation")
	if os.IsNotExist(err) {
		generation, space, e := s.spaceWriteGeneration(owner)
		if e != nil {
			return e
		}
		base := filepath.Base(temp)
		if space && generation > 0 && (strings.HasPrefix(base, "upload-") || strings.HasPrefix(base, "dav-")) {
			return errFileConflict
		}
		return nil
	}
	if err != nil {
		return err
	}
	var expected struct {
		Owner      string
		Generation int64
	}
	if json.Unmarshal(data, &expected) != nil || expected.Owner != owner {
		return errFileConflict
	}
	current, _, err := s.spaceWriteGeneration(owner)
	if err != nil {
		return err
	}
	if expected.Generation != current {
		return errFileConflict
	}
	return nil
}
func (s *Server) removeFileStage(temp string) {
	_ = os.Remove(temp)
	_ = os.Remove(temp + ".generation")
}

func (s *Server) spaceIsBlocked(id string) bool {
	_, blocked := s.spaceRecoveryBlocked.Load(id)
	return blocked
}
