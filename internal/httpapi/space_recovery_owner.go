package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/tayga/tms/internal/storage"
)

type recoverySpaceOwner struct {
	Space, Tenant string
	Generation    int64
}

func (s *Server) readRecoveryOwner(space string) (recoverySpaceOwner, error) {
	var owner recoverySpaceOwner
	data, err := os.ReadFile(filepath.Join(s.checkpointRoot(space), "owner.json"))
	if err != nil {
		return owner, err
	}
	err = json.Unmarshal(data, &owner)
	if err == nil && (owner.Space != space || !storage.ValidFileSpaceID(owner.Tenant)) {
		err = storage.ErrFileSpaceInvalid
	}
	return owner, err
}
func (s *Server) ensureRecoveryOwner(space, tenant string, generation int64) error {
	old, err := s.readRecoveryOwner(space)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && old.Tenant != tenant {
		return storage.ErrFileSpaceInvalid
	}
	if old.Generation > generation {
		generation = old.Generation
	}
	if err = os.MkdirAll(s.checkpointRoot(space), 0700); err != nil {
		return err
	}
	return writeFileStateJSON(filepath.Join(s.checkpointRoot(space), "owner.json"), recoverySpaceOwner{Space: space, Tenant: tenant, Generation: generation})
}
