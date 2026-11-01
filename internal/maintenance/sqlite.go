// Package maintenance coordinates offline maintenance with all Tayga SQLite
// users. Locks are advisory: external SQLite tools must also be stopped.
package maintenance

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

const LockSuffix = ".tayga-lock"

func SQLite(path string, exclusive bool) (io.Closer, error) {
	if path == ":memory:" {
		return nil, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if resolved, e := filepath.EvalSymlinks(abs); e == nil {
		abs = resolved
	} else if os.IsNotExist(e) {
		parent, e := filepath.EvalSymlinks(filepath.Dir(abs))
		if e != nil {
			return nil, e
		}
		abs = filepath.Join(parent, filepath.Base(abs))
	} else {
		return nil, e
	}
	l := flock.New(abs+LockSuffix, flock.SetPermissions(0600))
	if exclusive {
		// Maintenance commonly runs as root. Never create a root-owned lock
		// that would prevent the unprivileged server from starting afterwards.
		l = flock.New(abs+LockSuffix, flock.SetFlag(os.O_RDONLY))
	}
	var ok bool
	if exclusive {
		ok, err = l.TryLock()
	} else {
		ok, err = l.TryRLock()
	}
	if err != nil || !ok {
		l.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("SQLite is in use by Tayga or offline maintenance; stop the other process first")
	}
	return l, nil
}
