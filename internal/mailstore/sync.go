package mailstore

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// SyncReport summarizes a local↔object-store reconcile pass.
type SyncReport struct {
	Messages   int
	Pushed     int
	Pulled     int
	Missing    int
	Unchanged  int
	Errors     int
}

// SyncOptions controls reconcile behaviour.
type SyncOptions struct {
	DryRun bool
	Log    *slog.Logger
}

// SyncObjects walks all indexed messages and ensures local maildir and blob stay aligned:
// local present → push to blob; local missing → pull from blob into cache.
func SyncObjects(ctx context.Context, store storage.Driver, ms *Store, blob Blob, opts SyncOptions) (*SyncReport, error) {
	if ms == nil {
		return nil, errors.New("mailstore required")
	}
	if blob == nil {
		blob = ms.blob
	}
	if blob == nil {
		return nil, errors.New("object store not configured")
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	rep := &SyncReport{}

	tenants, err := store.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tenants {
		users, err := store.ListUsersByTenant(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			mboxes, err := store.ListMailboxes(ctx, u.ID)
			if err != nil {
				return nil, err
			}
			for _, mb := range mboxes {
				msgs, err := store.ListMessages(ctx, mb.ID)
				if err != nil {
					return nil, err
				}
				for _, msg := range msgs {
					if msg.FilePath == "" {
						continue
					}
					rep.Messages++
					if err := syncOne(ctx, ms, blob, msg.FilePath, opts.DryRun, rep, log); err != nil {
						rep.Errors++
						log.Warn("sync-objects message failed", "path", msg.FilePath, "err", err)
					}
				}
			}
		}
	}
	return rep, nil
}

func syncOne(ctx context.Context, ms *Store, blob Blob, rel string, dryRun bool, rep *SyncReport, log *slog.Logger) error {
	abs := ms.Abs(rel)
	_, localErr := os.Stat(abs)
	localOK := localErr == nil

	if localOK {
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		// Cheap existence check: Get; if identical skip (optional). Always Put when missing or dry-run count.
		remote, gerr := blob.Get(ctx, blobKey(rel))
		if gerr == nil && string(remote) == string(data) {
			rep.Unchanged++
			return nil
		}
		if dryRun {
			rep.Pushed++
			return nil
		}
		putCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := blob.Put(putCtx, blobKey(rel), data); err != nil {
			return err
		}
		rep.Pushed++
		return nil
	}

	if localErr != nil && !os.IsNotExist(localErr) {
		return localErr
	}

	getCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	data, err := blob.Get(getCtx, blobKey(rel))
	if err != nil {
		if errors.Is(err, errBlobNotFound) {
			rep.Missing++
			log.Debug("sync-objects missing both sides", "path", rel)
			return nil
		}
		return err
	}
	if dryRun {
		rep.Pulled++
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(abs, data, 0o640); err != nil {
		return err
	}
	rep.Pulled++
	return nil
}
