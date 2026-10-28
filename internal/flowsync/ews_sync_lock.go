package flowsync

import "context"

type ewsSyncLockKey struct{ user, client, folder string }
type ewsSyncLock struct {
	token chan struct{}
	refs  int
}

// A cursor's read/diff/save must be atomic, including retries of its previous
// key. Unrelated folders, clients and users must not queue behind a large archive.
func (h *ewsHandler) lockEWSSync(ctx context.Context, key ewsSyncLockKey) (func(), error) {
	h.syncMu.Lock()
	if h.syncLocks == nil {
		h.syncLocks = make(map[ewsSyncLockKey]*ewsSyncLock)
	}
	lock := h.syncLocks[key]
	if lock == nil {
		lock = &ewsSyncLock{token: make(chan struct{}, 1)}
		h.syncLocks[key] = lock
	}
	lock.refs++
	h.syncMu.Unlock()
	releaseRef := func() {
		h.syncMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(h.syncLocks, key)
		}
		h.syncMu.Unlock()
	}
	select {
	case lock.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-lock.token
			releaseRef()
			return nil, err
		}
		return func() { <-lock.token; releaseRef() }, nil
	case <-ctx.Done():
		releaseRef()
		return nil, ctx.Err()
	}
}
