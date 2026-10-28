package flowsync

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type blockedEWSListing struct {
	storage.Driver
	folder           string
	entered, release chan struct{}
	once             sync.Once
}

func (s *blockedEWSListing) ListMessages(ctx context.Context, id string) ([]*storage.Message, error) {
	if id == s.folder {
		s.once.Do(func() { close(s.entered) })
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.Driver.ListMessages(ctx, id)
}

func TestEWSSyncArchiveDoesNotBlockInboxAndRetry(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	inbox, _ := st.EnsureMailbox(ctx, user.ID, "INBOX", "")
	archive, _ := st.EnsureMailbox(ctx, user.ID, "Archive", "")
	blocked := &blockedEWSListing{Driver: st, folder: archive.ID, entered: make(chan struct{}), release: make(chan struct{})}
	h := &ewsHandler{store: blocked, ms: eas.ms}
	request := func(folder, state string) string {
		return `<SyncFolderItems><SyncFolderId><FolderId Id="` + folder + `"/></SyncFolderId><SyncState>` + state + `</SyncState></SyncFolderItems>`
	}
	state := "ews:concurrent/start"
	if err := saveCollectionState(ctx, st, user.ID, "ews:concurrent", archive.ID, collectionState{Key: "start", Items: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	type result struct {
		body string
		err  error
	}
	done := make(chan result, 1)
	go func() { body, err := h.syncEWSItems(ctx, user, request(archive.ID, state)); done <- result{body, err} }()
	<-blocked.entered
	defer func() {
		select {
		case <-blocked.release:
		default:
			close(blocked.release)
		}
	}()
	short, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if got, err := h.syncEWSItems(short, user, request(inbox.ID, "")); err != nil || !strings.Contains(got, "NoError") {
		t.Fatalf("inbox waited for archive: %v %s", err, got)
	}
	// A duplicate cursor cannot run concurrently; cancellation releases its ref.
	cancelled, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	if unlock, err := h.lockEWSSync(cancelled, ewsSyncLockKey{user.ID, "ews:concurrent", archive.ID}); !errors.Is(err, context.DeadlineExceeded) {
		if unlock != nil {
			unlock()
		}
		t.Fatalf("duplicate cursor did not wait for the first sync: %v", err)
	}
	close(blocked.release)
	first := <-done
	if first.err != nil {
		t.Fatal(first.err)
	}
	replayed, err := h.syncEWSItems(ctx, user, request(archive.ID, state))
	if err != nil || replayed != first.body {
		t.Fatalf("retry changed the cursor response: %v", err)
	}
	if len(h.syncLocks) != 0 {
		t.Fatal("unused sync locks retained")
	}
}
