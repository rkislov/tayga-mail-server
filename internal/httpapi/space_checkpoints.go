package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const recoveryMaxEntries = 100000
const recoveryMaxBytes int64 = 100 << 30
const recoveryManifestLimit = 64 << 20

var errSpaceRecoveryPending = errors.New("space recovery is pending; restart or retry recovery before accessing files")

type spaceCheckpointInfo struct {
	ID        string    `json:"id"`
	SpaceID   string    `json:"space_id"`
	At        time.Time `json:"at"`
	Label     string    `json:"label"`
	Automatic bool      `json:"automatic"`
	Entries   int       `json:"entries"`
	Bytes     int64     `json:"logical_bytes"`
}
type recoveryEntry struct {
	Path     string    `json:"path"`
	Dir      bool      `json:"dir,omitempty"`
	Size     int64     `json:"size"`
	SHA256   string    `json:"sha256,omitempty"`
	Modified time.Time `json:"modified"`
}
type spaceCheckpoint struct {
	Format int                        `json:"format"`
	Info   spaceCheckpointInfo        `json:"info"`
	State  *storage.FileSpaceSnapshot `json:"state"`
	Files  []recoveryEntry            `json:"files"`
}

func (s *Server) checkpointRoot(space string) string {
	return filepath.Join(s.ms.Root, "space-recovery", "checkpoints", space)
}
func checkpointPathAllowed(path string) bool {
	if path == "" || path == "." || strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\\\x00") || filepath.ToSlash(filepath.Clean(path)) != path {
		return false
	}
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if p == ".." || p == "" {
			return false
		}
	}
	if parts[0] == "files" {
		return true
	}
	if parts[0] != "state" {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	switch parts[1] {
	case "versions", "trash", "activity", "cleanup", "retention":
		return true
	}
	return false
}
func copyRecoveryFile(ctx context.Context, source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	buffer := make([]byte, 128<<10)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		var n int
		n, err = in.Read(buffer)
		if n > 0 {
			if _, e := out.Write(buffer[:n]); e != nil {
				err = e
				break
			}
		}
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		if info, e := in.Stat(); e == nil {
			err = os.Chtimes(target, info.ModTime(), info.ModTime())
		} else {
			err = e
		}
	}
	return err
}
func cloneRecoveryTree(ctx context.Context, source, target string, state bool) error {
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if state && !checkpointPathAllowed("state/"+filepath.ToSlash(rel)) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errPathEscape
		}
		targetPath := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errPathEscape
		}
		immutable := !state || (len(parts) >= 3 && (parts[0] == "versions" || parts[0] == "trash") && parts[2] == "content")
		if immutable {
			return os.Link(path, targetPath)
		}
		return copyRecoveryFile(ctx, path, targetPath)
	})
}
func recoveryEntries(ctx context.Context, root string) ([]recoveryEntry, error) {
	entries := []recoveryEntry{}
	var bytes int64
	for _, kind := range []string{"files", "state"} {
		base := filepath.Join(root, kind)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, e error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if !checkpointPathAllowed(rel) || d.Type()&os.ModeSymlink != 0 {
				return errPathEscape
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return errPathEscape
			}
			size := int64(0)
			if !d.IsDir() {
				size = info.Size()
				if strings.HasPrefix(rel, "state/") && !recoveryContentPath(rel) && size > 64<<20 {
					return errFileTooLarge
				}
				bytes += size
			}
			if len(entries) >= recoveryMaxEntries || bytes > recoveryMaxBytes || size > maxWebUploadBytes {
				return errors.New("space recovery size limit exceeded")
			}
			entries = append(entries, recoveryEntry{Path: rel, Dir: d.IsDir(), Size: size, Modified: info.ModTime()})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// Called with fileMu held. Managed content is immutable; mutable journals are
// copied while writers are stopped. Network streaming never holds fileMu.
func (s *Server) captureSpaceCheckpoint(ctx context.Context, space, label string, automatic bool) (*spaceCheckpoint, error) {
	state, err := s.store.SnapshotFileSpace(ctx, space)
	if err != nil {
		return nil, err
	}
	if err = s.ensureRecoveryOwner(space, state.Space.TenantID, state.Space.Generation); err != nil {
		return nil, err
	}
	id := storage.NewID()
	root := s.checkpointRoot(space)
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	temp := filepath.Join(root, ".new-"+id)
	if err = os.Mkdir(temp, 0700); err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	files := filepath.Join(s.ms.Root, "files", space)
	if err = os.MkdirAll(files, 0750); err != nil {
		return nil, err
	}
	if err = cloneRecoveryTree(ctx, files, filepath.Join(temp, "files"), false); err != nil {
		return nil, err
	}
	source := filepath.Join(s.ms.Root, "file-state", space)
	if _, e := os.Stat(source); e == nil {
		if err = cloneRecoveryTree(ctx, source, filepath.Join(temp, "state"), true); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if err = os.MkdirAll(filepath.Join(temp, "state"), 0700); err != nil {
		return nil, err
	}
	entries, err := recoveryEntries(ctx, temp)
	if err != nil {
		return nil, err
	}
	point := &spaceCheckpoint{Format: 1, State: state, Files: entries, Info: spaceCheckpointInfo{ID: id, SpaceID: space, At: time.Now().UTC(), Label: label, Automatic: automatic, Entries: len(entries)}}
	for _, e := range entries {
		point.Info.Bytes += e.Size
	}
	if err = writeFileStateJSON(filepath.Join(temp, "checkpoint.json"), point); err != nil {
		return nil, err
	}
	if err = syncRecoveryTree(temp); err != nil {
		return nil, err
	}
	if err = os.Rename(temp, filepath.Join(root, id)); err != nil {
		return nil, err
	}
	if err = syncRecoveryDir(root); err != nil {
		return nil, err
	}
	s.pruneAutomaticCheckpoints(space)
	return point, nil
}
func syncRecoveryDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func (s *Server) readSpaceCheckpoint(space, id string) (*spaceCheckpoint, error) {
	if !storage.ValidFileSpaceID(space) || !storage.ValidFileSpaceID(id) {
		return nil, errPathEscape
	}
	point, err := readCheckpointFile(filepath.Join(s.checkpointRoot(space), id, "checkpoint.json"))
	if err == nil && (point.Info.ID != id || point.Info.SpaceID != space) {
		return nil, storage.ErrFileSpaceInvalid
	}
	return point, err
}
func readCheckpointFile(path string) (*spaceCheckpoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, recoveryManifestLimit+1))
	if err != nil || len(data) > recoveryManifestLimit {
		return nil, errFileTooLarge
	}
	var point spaceCheckpoint
	if err = json.Unmarshal(data, &point); err != nil {
		return nil, err
	}
	if point.Format != 1 || point.State == nil || point.State.Space == nil || !storage.ValidFileSpaceID(point.Info.ID) || point.Info.SpaceID != point.State.Space.ID || !storage.ValidFileSpaceID(point.Info.SpaceID) {
		return nil, storage.ErrFileSpaceInvalid
	}
	return &point, nil
}
func (s *Server) listSpaceCheckpoints(space string) ([]spaceCheckpointInfo, error) {
	dirs, err := os.ReadDir(s.checkpointRoot(space))
	if os.IsNotExist(err) {
		return []spaceCheckpointInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []spaceCheckpointInfo{}
	for _, dir := range dirs {
		if !dir.IsDir() || !storage.ValidFileSpaceID(dir.Name()) {
			continue
		}
		point, e := s.readSpaceCheckpoint(space, dir.Name())
		if e != nil {
			return nil, e
		}
		out = append(out, point.Info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
func (s *Server) pruneAutomaticCheckpoints(space string) {
	points, err := s.listSpaceCheckpoints(space)
	if err != nil {
		return
	}
	kept := 0
	for _, point := range points {
		if !point.Automatic {
			continue
		}
		kept++
		if kept > 50 && s.spaceCheckpointReaders[space+"/"+point.ID] == 0 {
			if err = os.RemoveAll(filepath.Join(s.checkpointRoot(space), point.ID)); err != nil {
				s.log.Error("checkpoint pruning failed", "space", space, "error", err)
			}
		}
	}
}
func (s *Server) autoSpaceCheckpoint(space, action string) {
	if s.spaceRecoverySuppressed[space] || s.spaceIsBlocked(space) {
		return
	}
	if _, err := s.store.GetFileSpace(context.Background(), space); errors.Is(err, storage.ErrNotFound) {
		return
	} else if err != nil {
		s.log.Error("checkpoint metadata unavailable", "error", err)
		return
	}
	if _, err := s.captureSpaceCheckpoint(context.Background(), space, action, true); err != nil {
		s.log.Error("automatic space checkpoint failed", "space", space, "error", err)
		if s.spaceCheckpointErrors == nil {
			s.spaceCheckpointErrors = map[string]string{}
		}
		s.spaceCheckpointErrors[space] = "Automatic checkpoint failed; changes may have no recovery point"
	} else {
		delete(s.spaceCheckpointErrors, space)
	}
}

func syncRecoveryTree(root string) error {
	dirs := []string{}
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := syncRecoveryDir(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}
