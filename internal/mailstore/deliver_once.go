package mailstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DeliverOnce publishes an immutable message using its delivery UUID. The caller
// holds the database delivery-row lock until inserting the mailbox receipt, so
// stale workers cannot rewrite a message after it becomes visible to clients.
// On a rollback/crash the same path is reused rather than creating another file.
func (s *Store) DeliverOnce(email, folder, id string, data []byte) (string, int64, error) {
	if len(id) != 36 || strings.ContainsAny(id, "/\\\x00") {
		return "", 0, fmt.Errorf("invalid delivery ID")
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'f' || r >= '0' && r <= '9' || r == '-') {
			return "", 0, fmt.Errorf("invalid delivery ID")
		}
	}
	dir, err := s.EnsureFolder(email, folder)
	if err != nil {
		return "", 0, err
	}
	final := filepath.Join(dir, "new", "welcome-"+id+":2,")
	rel, err := filepath.Rel(s.Root, final)
	if err != nil {
		return "", 0, err
	}
	if existing, err := os.ReadFile(final); err == nil {
		if !bytes.Equal(existing, data) {
			return "", 0, fmt.Errorf("delivery content differs from the stored snapshot")
		}
		s.blobPut(rel, data)
		return rel, int64(len(data)), nil
	} else if !os.IsNotExist(err) {
		return "", 0, err
	}
	f, err := os.CreateTemp(filepath.Join(dir, "tmp"), "welcome-"+id+"-")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0640); err != nil {
		f.Close()
		return "", 0, err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", 0, err
	}
	if err = os.Rename(f.Name(), final); err != nil {
		return "", 0, err
	}
	s.blobPut(rel, data)
	return rel, int64(len(data)), nil
}
