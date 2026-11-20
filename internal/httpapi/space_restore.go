package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type spaceRestoreChange struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Before int64  `json:"before_bytes"`
	After  int64  `json:"after_bytes"`
}
type spaceRestorePlan struct {
	ID            string               `json:"id"`
	Actor         string               `json:"actor"`
	Space         string               `json:"space_id"`
	Checkpoint    string               `json:"checkpoint_id"`
	At            time.Time            `json:"checkpoint_at"`
	Expires       time.Time            `json:"expires"`
	Mode          string               `json:"mode"`
	Path          string               `json:"path"`
	Expected      string               `json:"expected"`
	Changes       []spaceRestoreChange `json:"changes"`
	BeforeBytes   int64                `json:"before_bytes"`
	AfterBytes    int64                `json:"after_bytes"`
	Metadata      bool                 `json:"metadata_restored"`
	ProjectedUsed int64                `json:"projected_used_bytes"`
	Quota         int64                `json:"quota_bytes"`
	QuotaBlocked  bool                 `json:"quota_blocked"`
}
type spaceRestoreIntent struct {
	ID, Space, Actor, Safety, Mode, Path, Status string
	At                                           time.Time
	HadFiles, HadState                           bool
	State                                        *storage.FileSpaceSnapshot
	Bindings                                     map[string]string
	Affected                                     []string
}

func (s *Server) spaceReadLock(space string) *sync.RWMutex {
	value, _ := s.spaceRecoveryViews.LoadOrStore(space, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}
func (s *Server) stageSpaceCheckpoint(ctx context.Context, space, label, root string) (*spaceCheckpoint, error) {
	state, err := s.store.SnapshotFileSpace(ctx, space)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	live := filepath.Join(s.ms.Root, "files", space)
	if err = os.MkdirAll(live, 0750); err != nil {
		return nil, err
	}
	if err = cloneRecoveryTree(ctx, live, filepath.Join(root, "files"), false); err != nil {
		return nil, err
	}
	source := filepath.Join(s.ms.Root, "file-state", space)
	if _, e := os.Stat(source); e == nil {
		if err = cloneRecoveryTree(ctx, source, filepath.Join(root, "state"), true); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if err = os.MkdirAll(filepath.Join(root, "state"), 0700); err != nil {
		return nil, err
	}
	entries, err := recoveryEntries(ctx, root)
	if err != nil {
		return nil, err
	}
	point := &spaceCheckpoint{Format: 1, State: state, Files: entries, Info: spaceCheckpointInfo{ID: storage.NewID(), SpaceID: space, At: time.Now().UTC(), Label: label, Entries: len(entries)}}
	for _, entry := range entries {
		point.Info.Bytes += entry.Size
	}
	if err = writeFileStateJSON(filepath.Join(root, "checkpoint.json"), point); err != nil {
		return nil, err
	}
	return point, nil
}
func spaceSnapshotFingerprint(point *spaceCheckpoint) string {
	value := struct {
		State *storage.FileSpaceSnapshot
		Files []recoveryEntry
	}{point.State, append([]recoveryEntry{}, point.Files...)}
	for i := range value.Files {
		value.Files[i].SHA256 = ""
		if value.Files[i].Dir {
			value.Files[i].Modified = time.Time{}
		}
	}
	data, _ := json.Marshal(value)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
func (s *Server) createSpaceRestorePlan(ctx context.Context, actor, space, checkpoint, mode, path string) (*spaceRestorePlan, error) {
	if mode != "full" && mode != "folder" {
		return nil, storage.ErrFileSpaceInvalid
	}
	if mode == "folder" && (path == "" || !checkpointPathAllowed("files/"+path)) {
		return nil, errPathEscape
	}
	root := filepath.Join(s.ms.Root, "space-recovery", "plans", storage.NewID())
	success := false
	defer func() {
		if !success {
			os.RemoveAll(root)
		}
	}()
	s.fileMu.Lock()
	point, err := s.readSpaceCheckpoint(space, checkpoint)
	if err == nil && !s.canRestoreTarget(ctx, actor, point.State) {
		err = storage.ErrFileSpaceForbidden
	}
	if err == nil {
		err = cloneRecoveryTree(ctx, filepath.Join(s.checkpointRoot(space), checkpoint, "files"), filepath.Join(root, "target", "files"), false)
	}
	if err == nil {
		err = cloneRecoveryTree(ctx, filepath.Join(s.checkpointRoot(space), checkpoint, "state"), filepath.Join(root, "target", "state"), true)
	}
	if err == nil {
		err = writeFileStateJSON(filepath.Join(root, "target", "checkpoint.json"), point)
	}
	var current *spaceCheckpoint
	if err == nil {
		current, err = s.stageRestoreCurrent(ctx, space, filepath.Join(root, "current"))
	}
	s.fileMu.Unlock()
	if err != nil {
		return nil, err
	}
	if mode == "folder" && current.State == nil {
		return nil, storage.ErrFileSpaceInvalid
	}
	if mode == "full" {
		if err = s.store.ValidateFileSpaceSnapshot(ctx, point.State); err != nil {
			return nil, err
		}
	}
	plan := &spaceRestorePlan{ID: filepath.Base(root), Actor: actor, Space: space, Checkpoint: checkpoint, At: point.Info.At, Expires: time.Now().UTC().Add(15 * time.Minute), Mode: mode, Path: path, Expected: spaceSnapshotFingerprint(current), Metadata: mode == "full"}
	before, err := recoveryLiveHashes(ctx, filepath.Join(root, "current"), path)
	if err != nil {
		return nil, err
	}
	after, err := recoveryLiveHashes(ctx, filepath.Join(root, "target"), path)
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{}
	for p := range before {
		paths[p] = true
	}
	for p := range after {
		paths[p] = true
	}
	for p := range paths {
		a, aok := before[p]
		b, bok := after[p]
		if aok {
			plan.BeforeBytes += a.Size
		}
		if bok {
			plan.AfterBytes += b.Size
		}
		if aok && bok && a.Dir == b.Dir && a.SHA256 == b.SHA256 {
			continue
		}
		kind := "modified"
		if !aok {
			kind = "added"
		}
		if !bok {
			kind = "deleted"
		}
		plan.Changes = append(plan.Changes, spaceRestoreChange{Path: p, Kind: kind, Before: a.Size, After: b.Size})
	}
	if mode == "folder" {
		used, e := recoverySnapshotUsage(filepath.Join(root, "current"))
		if e != nil {
			return nil, e
		}
		plan.ProjectedUsed = used + plan.AfterBytes
		plan.Quota = current.State.Space.QuotaBytes
		plan.QuotaBlocked = plan.Quota > 0 && plan.ProjectedUsed > plan.Quota
	} else {
		used, e := recoverySnapshotUsage(filepath.Join(root, "target"))
		if e != nil {
			return nil, e
		}
		plan.ProjectedUsed = used
		plan.Quota = point.State.Space.QuotaBytes
	}
	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].Path < plan.Changes[j].Path })
	if err = writeFileStateJSON(filepath.Join(root, "plan.json"), plan); err != nil {
		return nil, err
	}
	success = true
	return plan, nil
}
func recoveryLiveHashes(ctx context.Context, root, prefix string) (map[string]recoveryEntry, error) {
	out := map[string]recoveryEntry{}
	base := filepath.Join(root, "files")
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == base {
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		rel = filepath.ToSlash(rel)
		if prefix != "" && rel != prefix && !strings.HasPrefix(rel, prefix+"/") {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		entry := recoveryEntry{Path: rel, Dir: d.IsDir()}
		if !d.IsDir() {
			entry.Size = info.Size()
			entry.SHA256, e = recoveryFileHash(ctx, path)
			if e != nil {
				return e
			}
		}
		out[rel] = entry
		return nil
	})
	return out, err
}
func (s *Server) canRestoreSpace(ctx context.Context, space, actor string) bool {
	rights, err := s.store.FileSpaceRights(ctx, space, actor)
	if err == nil {
		return rights == "manage"
	}
	return false
}
func (s *Server) applySpaceRestorePlan(ctx context.Context, actor, id string) (string, error) {
	if !storage.ValidFileSpaceID(id) {
		return "", errPathEscape
	}
	root := filepath.Join(s.ms.Root, "space-recovery", "plans", id)
	data, err := os.ReadFile(filepath.Join(root, "plan.json"))
	if err != nil {
		return "", err
	}
	var plan spaceRestorePlan
	if json.Unmarshal(data, &plan) != nil || plan.ID != id || plan.Actor != actor || !time.Now().Before(plan.Expires) {
		return "", storage.ErrFileSpaceForbidden
	}
	gate := s.spaceReadLock(plan.Space)
	if !gate.TryLock() {
		return "", errFileLocked
	}
	defer gate.Unlock()
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	target, err := readCheckpointFile(filepath.Join(root, "target", "checkpoint.json"))
	if err != nil {
		return "", err
	}
	if target.State.Space.ID != plan.Space || !s.canRestoreTarget(ctx, actor, target.State) {
		return "", storage.ErrFileSpaceForbidden
	}
	check := filepath.Join(root, "check")
	os.RemoveAll(check)
	current, err := s.stageRestoreCurrent(ctx, plan.Space, check)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(check)
	if spaceSnapshotFingerprint(current) != plan.Expected {
		return "", errFileConflict
	}
	target, err = readCheckpointFile(filepath.Join(root, "target", "checkpoint.json"))
	if err != nil {
		return "", err
	}
	desired := target.State
	if plan.Mode == "folder" && current.State == nil {
		return "", storage.ErrFileSpaceInvalid
	}
	if plan.Mode == "folder" {
		desired, err = storage.CloneFileSpaceSnapshot(current.State)
		if err != nil {
			return "", err
		}
	}
	if err = s.store.ValidateFileSpaceSnapshot(ctx, desired); err != nil {
		return "", err
	}
	desired, err = storage.CloneFileSpaceSnapshot(desired)
	if err != nil {
		return "", err
	}
	if current.State != nil && current.State.Space.Generation > desired.Space.Generation {
		desired.Space.Generation = current.State.Space.Generation
	}
	if identity, e := s.readRecoveryOwner(plan.Space); e == nil && identity.Generation > desired.Space.Generation {
		desired.Space.Generation = identity.Generation
	}
	if err = s.ensureRecoveryOwner(plan.Space, desired.Space.TenantID, desired.Space.Generation+1); err != nil {
		return "", err
	}
	// A durable, protected safety point is always published before touching live data.
	if current.State == nil {
		current.State, err = storage.CloneFileSpaceSnapshot(target.State)
		if err != nil {
			return "", err
		}
	}
	safety := current.Info.ID
	current.Info.Automatic = false
	current.Info.Label = "Safety before restore"
	if err = writeFileStateJSON(filepath.Join(check, "checkpoint.json"), current); err != nil {
		return "", err
	}
	if err = os.MkdirAll(s.checkpointRoot(plan.Space), 0700); err != nil {
		return "", err
	}
	if err = syncRecoveryTree(check); err != nil {
		return "", err
	}
	if err = os.Rename(check, filepath.Join(s.checkpointRoot(plan.Space), safety)); err != nil {
		return "", err
	}
	if err = syncRecoveryDir(s.checkpointRoot(plan.Space)); err != nil {
		return "", err
	}
	opID := storage.NewID()
	opRoot := filepath.Join(s.ms.Root, "space-recovery", "operations", opID)
	if err = os.MkdirAll(opRoot, 0700); err != nil {
		return "", err
	}
	source := filepath.Join(root, "target")
	if plan.Mode == "folder" {
		source = filepath.Join(s.checkpointRoot(plan.Space), safety)
	}
	if err = cloneRecoveryTree(ctx, filepath.Join(source, "files"), filepath.Join(opRoot, "new-files"), false); err != nil {
		return "", err
	}
	if err = cloneRecoveryTree(ctx, filepath.Join(source, "state"), filepath.Join(opRoot, "new-state"), true); err != nil {
		return "", err
	}
	if plan.Mode == "folder" {
		if err = prepareFolderRollback(ctx, opRoot, filepath.Join(root, "target"), plan.Path, desired, s.fileActivityBindings(plan.Space, plan.Path)); err != nil {
			return "", err
		}
		if limit := desired.Space.QuotaBytes; limit > 0 {
			used, e := recoveryWorkingUsage(opRoot)
			if e != nil {
				return "", e
			}
			if used > limit {
				return "", errFileQuota
			}
		}
	}
	live, state := filepath.Join(s.ms.Root, "files", plan.Space), filepath.Join(s.ms.Root, "file-state", plan.Space)
	_, liveErr := os.Stat(live)
	_, stateErr := os.Stat(state)
	intent := &spaceRestoreIntent{ID: opID, Space: plan.Space, Actor: actor, Safety: safety, Mode: plan.Mode, Path: plan.Path, Status: "pending", At: time.Now().UTC(), HadFiles: liveErr == nil, HadState: stateErr == nil, State: desired}
	if plan.Mode == "folder" {
		intent.Bindings = map[string]string{}
		for p, id := range s.fileActivityBindings(plan.Space, plan.Path) {
			intent.Bindings[p] = ""
			intent.Affected = append(intent.Affected, id)
		}
		bindings, e := recoveryActivityBindings(filepath.Join(root, "target"), plan.Path)
		if e != nil {
			return "", e
		}
		for p, id := range bindings {
			intent.Bindings[p] = id
			intent.Affected = append(intent.Affected, id)
		}
	}
	if err = syncRecoveryTree(opRoot); err != nil {
		return "", err
	}
	if err = writeFileStateJSON(filepath.Join(opRoot, "intent.json"), intent); err != nil {
		return "", err
	}
	if err = s.finishSpaceRestore(ctx, opRoot, intent); err != nil {
		return safety, err
	}
	os.RemoveAll(root)
	return safety, nil
}
func recoveryWorkingUsage(root string) (int64, error) {
	sum, err := fileTreeBytes(filepath.Join(root, "new-files"))
	if err != nil {
		return 0, err
	}
	for _, kind := range []string{"versions", "trash"} {
		base := filepath.Join(root, "new-state", kind)
		dirs, e := os.ReadDir(base)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return 0, e
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			n, e := fileTreeBytes(filepath.Join(base, dir.Name(), "content"))
			if e != nil {
				return 0, e
			}
			sum += n
		}
	}
	return sum, nil
}
func recoveryActivityBindings(root, prefix string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "state", "activity", "events.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	state := &fileActivityState{Paths: map[string]string{}}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var event fileActivityEvent
		if json.Unmarshal([]byte(line), &event) != nil {
			return nil, storage.ErrFileSpaceInvalid
		}
		applyFileActivity(state, event)
	}
	out := map[string]string{}
	for p, id := range state.Paths {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			out[p] = id
		}
	}
	return out, nil
}
func prepareFolderRollback(ctx context.Context, root, target, prefix string, state *storage.FileSpaceSnapshot, bindings map[string]string) error {
	live := filepath.Join(root, "new-files")
	path, err := safeJoin(live, prefix)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if info != nil {
		rev := fileRevision{ID: storage.NewID(), Path: prefix, Name: filepath.Base(prefix), Size: info.Size(), IsDir: info.IsDir(), At: time.Now().UTC(), ActivityIDs: bindings}
		dir := filepath.Join(root, "new-state", "trash", rev.ID)
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		if err = writeFileStateJSON(filepath.Join(dir, "metadata.json"), rev); err != nil {
			return err
		}
		if err = os.Rename(path, filepath.Join(dir, "content")); err != nil {
			return err
		}
	}
	source, err := safeJoin(filepath.Join(target, "files"), prefix)
	if err != nil {
		return err
	}
	old, err := os.Stat(source)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if old != nil {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if old.IsDir() {
			err = cloneRecoveryTree(ctx, source, path, false)
		} else {
			err = os.Link(source, path)
		}
		if err != nil {
			return err
		}
	}
	access := state.Access[:0]
	for _, a := range state.Access {
		if a.Path != prefix && !strings.HasPrefix(a.Path, prefix+"/") {
			access = append(access, a)
		}
	}
	state.Access = access
	shares := state.Shares[:0]
	for _, a := range state.Shares {
		if a.Path != prefix && !strings.HasPrefix(a.Path, prefix+"/") {
			shares = append(shares, a)
		}
	}
	state.Shares = shares
	return nil
}
func swapRecoveryTree(current, fresh, old string, had bool) error {
	_, oldErr := os.Stat(old)
	_, freshErr := os.Stat(fresh)
	_, currentErr := os.Stat(current)
	if oldErr == nil && currentErr == nil && freshErr == nil {
		return errFileConflict
	}
	if os.IsNotExist(oldErr) && had && currentErr == nil && freshErr == nil {
		if err := os.Rename(current, old); err != nil {
			return err
		}
		if err := syncRecoveryDir(filepath.Dir(old)); err != nil {
			return err
		}
		if err := syncRecoveryDir(filepath.Dir(current)); err != nil {
			return err
		}
		currentErr = os.ErrNotExist
	}
	if os.IsNotExist(currentErr) && freshErr == nil {
		if err := os.MkdirAll(filepath.Dir(current), 0700); err != nil {
			return err
		}
		if err := os.Rename(fresh, current); err != nil {
			return err
		}
		if err := syncRecoveryDir(filepath.Dir(fresh)); err != nil {
			return err
		}
		return syncRecoveryDir(filepath.Dir(current))
	}
	if currentErr == nil && os.IsNotExist(freshErr) {
		return nil
	}
	return fmt.Errorf("incomplete recovery tree")
}
func rollbackRecoveryTree(current, fresh, old string, had bool) error {
	if _, err := os.Stat(old); err == nil {
		if _, err = os.Stat(current); err == nil {
			if err = os.Rename(current, fresh); err != nil {
				return err
			}
		}
		if err = os.Rename(old, current); err != nil {
			return err
		}
		return syncRecoveryDir(filepath.Dir(current))
	}
	if !had {
		if _, e := os.Stat(fresh); os.IsNotExist(e) {
			if _, e = os.Stat(current); e == nil {
				return os.Rename(current, fresh)
			}
		}
	}
	return nil
}
func (s *Server) finishSpaceRestore(ctx context.Context, root string, intent *spaceRestoreIntent) error {
	if !storage.ValidFileSpaceID(intent.ID) || !storage.ValidFileSpaceID(intent.Space) || intent.State == nil || intent.State.Space == nil || intent.State.Space.ID != intent.Space {
		return storage.ErrFileSpaceInvalid
	}
	s.spaceRecoveryBlocked.Store(intent.Space, true)
	if s.spaceRecoverySuppressed == nil {
		s.spaceRecoverySuppressed = map[string]bool{}
	}
	s.spaceRecoverySuppressed[intent.Space] = true
	defer delete(s.spaceRecoverySuppressed, intent.Space)
	live, state := filepath.Join(s.ms.Root, "files", intent.Space), filepath.Join(s.ms.Root, "file-state", intent.Space)
	err := swapRecoveryTree(live, filepath.Join(root, "new-files"), filepath.Join(root, "old-files"), intent.HadFiles)
	if err == nil {
		err = swapRecoveryTree(state, filepath.Join(root, "new-state"), filepath.Join(root, "old-state"), intent.HadState)
	}
	swapFailed := err != nil
	if err == nil {
		err = s.store.RestoreFileSpaceSnapshot(ctx, intent.State, intent.ID)
	}
	committed, checkErr := s.store.FileSpaceRestoreCommitted(context.Background(), intent.ID)
	if checkErr != nil {
		return errSpaceRecoveryPending
	}
	if err != nil && !committed {
		a := rollbackRecoveryTree(live, filepath.Join(root, "new-files"), filepath.Join(root, "old-files"), intent.HadFiles)
		b := rollbackRecoveryTree(state, filepath.Join(root, "new-state"), filepath.Join(root, "old-state"), intent.HadState)
		if a != nil || b != nil {
			return errSpaceRecoveryPending
		}
		intent.Status = "failed"
		if e := writeFileStateJSON(filepath.Join(root, "intent.json"), intent); e != nil {
			return errSpaceRecoveryPending
		}
		s.spaceRecoveryBlocked.Delete(intent.Space)
		return err
	}
	if !committed || swapFailed {
		return errSpaceRecoveryPending
	}
	delete(s.fileActivities, intent.Space)
	s.invalidateOfficePath(intent.Space, "")
	for token, sess := range s.officeSessions {
		if sess.Owner == intent.Space {
			delete(s.officeSessions, token)
		}
	}
	for token, sess := range s.fileComparisons {
		if sess.Owner == intent.Space {
			delete(s.fileComparisons, token)
		}
	}
	delete(s.fileDAVLocks, intent.Space)
	s.fileSearchMu.Lock()
	for key := range s.fileSearchCache {
		if strings.HasPrefix(key, intent.Space+"\x00") {
			delete(s.fileSearchCache, key)
		}
	}
	s.fileSearchMu.Unlock()
	s.spaceRecoveryBlocked.Delete(intent.Space)
	if err = s.appendFileActivity(intent.Space, fileActivityEvent{ID: intent.ID, Action: "space_restored", Path: intent.Path, Actor: fileActor{ID: intent.Actor, Source: "recovery"}, Subject: intent.Safety, Bindings: intent.Bindings, Affected: intent.Affected}); err != nil {
		s.spaceRecoveryBlocked.Store(intent.Space, true)
		return errSpaceRecoveryPending
	}
	if _, e := s.captureSpaceCheckpoint(context.Background(), intent.Space, "After restore", true); e != nil {
		s.log.Error("post-restore checkpoint failed", "space", intent.Space, "error", e)
	}
	intent.Status = "complete"
	if err = writeFileStateJSON(filepath.Join(root, "intent.json"), intent); err != nil {
		s.spaceRecoveryBlocked.Store(intent.Space, true)
		return errSpaceRecoveryPending
	}
	os.RemoveAll(filepath.Join(root, "old-files"))
	os.RemoveAll(filepath.Join(root, "old-state"))
	return nil
}
func (s *Server) recoverPendingSpaceRestores(ctx context.Context, filter ...string) error {
	root := filepath.Join(s.ms.Root, "space-recovery", "operations")
	dirs, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	for _, dir := range dirs {
		if !dir.IsDir() || !storage.ValidFileSpaceID(dir.Name()) {
			continue
		}
		path := filepath.Join(root, dir.Name())
		data, e := os.ReadFile(filepath.Join(path, "intent.json"))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		var intent spaceRestoreIntent
		if json.Unmarshal(data, &intent) != nil || intent.ID != dir.Name() {
			return storage.ErrFileSpaceInvalid
		}
		if len(filter) > 0 && filter[0] != "" && intent.Space != filter[0] {
			continue
		}
		if intent.Status != "pending" {
			continue
		}
		gate := s.spaceReadLock(intent.Space)
		if !gate.TryLock() {
			return errSpaceRecoveryPending
		}
		e = s.finishSpaceRestore(ctx, path, &intent)
		gate.Unlock()
		if e != nil {
			if intent.Status == "failed" {
				s.log.Error("restore canceled and rolled back", "space", intent.Space, "error", e)
				continue
			}
			return e
		}
	}
	return nil
}

func (s *Server) stageRestoreCurrent(ctx context.Context, space, root string) (*spaceCheckpoint, error) {
	if _, err := s.store.GetFileSpace(ctx, space); err == nil {
		return s.stageSpaceCheckpoint(ctx, space, "Before restore", root)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	for _, kind := range []string{"files", "state"} {
		base := "files"
		if kind == "state" {
			base = "file-state"
		}
		source := filepath.Join(s.ms.Root, base, space)
		if _, err := os.Stat(source); err == nil {
			if err = cloneRecoveryTree(ctx, source, filepath.Join(root, kind), kind == "state"); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(root, kind), 0700); err != nil {
			return nil, err
		}
	}
	entries, err := recoveryEntries(ctx, root)
	if err != nil {
		return nil, err
	}
	point := &spaceCheckpoint{Format: 1, Info: spaceCheckpointInfo{ID: storage.NewID(), SpaceID: space, At: time.Now().UTC(), Label: "Before space recreation"}, Files: entries}
	return point, nil
}

func recoverySnapshotUsage(root string) (int64, error) {
	total, err := fileTreeBytes(filepath.Join(root, "files"))
	if err != nil {
		return 0, err
	}
	for _, kind := range []string{"versions", "trash"} {
		dirs, e := os.ReadDir(filepath.Join(root, "state", kind))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return 0, e
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			n, e := fileTreeBytes(filepath.Join(root, "state", kind, dir.Name(), "content"))
			if e != nil {
				return 0, e
			}
			total += n
		}
	}
	return total, nil
}
