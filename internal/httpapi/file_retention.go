package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

var errFileProtected = errors.New("version is protected; remove protection before deleting")

type fileRetentionPolicy struct {
	Automatic    bool `json:"automatic"`
	VersionDays  int  `json:"version_days"`
	TrashDays    int  `json:"trash_days"`
	KeepVersions int  `json:"keep_versions"`
}
type fileCleanupEntry struct {
	ID        string       `json:"id"`
	Kind      string       `json:"kind"`
	Revision  fileRevision `json:"revision"`
	Bytes     int64        `json:"bytes"`
	At        time.Time    `json:"at"`
	Completed bool         `json:"completed"`
}
type fileCleanupPlan struct {
	Entries []fileCleanupEntry `json:"entries"`
	Bytes   int64              `json:"quota_bytes"`
}

// All retention state and mutations are serialized with managed file writers.
// Sync the replacement and its directory so an intent survives a process crash.
func writeFileStateJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s *Server) retentionPolicy(owner string) (fileRetentionPolicy, error) {
	var p fileRetentionPolicy
	if space, err := s.store.GetFileSpace(context.Background(), owner); err == nil {
		return fileRetentionPolicy{Automatic: space.Automatic, VersionDays: space.VersionDays, TrashDays: space.TrashDays, KeepVersions: space.KeepVersions}, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return p, err
	}
	root, err := s.fileStateRoot(owner, "retention")
	if err != nil {
		return p, err
	}
	data, err := os.ReadFile(filepath.Join(root, "policy.json"))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(data, &p)
	if err == nil && !p.valid() {
		err = errors.New("invalid retention policy")
	}
	return p, err
}
func (p fileRetentionPolicy) valid() bool {
	return p.VersionDays >= 0 && p.VersionDays <= 36500 && p.TrashDays >= 0 && p.TrashDays <= 36500 && p.KeepVersions >= 0 && p.KeepVersions <= 10000
}
func (s *Server) cleanupPlan(ctx context.Context, owner string, p fileRetentionPolicy, now time.Time) (fileCleanupPlan, error) {
	plan := fileCleanupPlan{Entries: []fileCleanupEntry{}}
	for _, kind := range []string{"versions", "trash"} {
		days := p.VersionDays
		if kind == "trash" {
			days = p.TrashDays
		}
		if days == 0 {
			continue
		}
		revisions, err := s.fileRevisions(owner, kind, "")
		if err != nil {
			return plan, err
		}
		counts := map[string]int{}
		for _, rev := range revisions {
			if err := ctx.Err(); err != nil {
				return plan, err
			}
			counts[rev.Path]++
			if rev.Protected || (kind == "versions" && counts[rev.Path] <= p.KeepVersions) || !rev.At.Before(now.Add(-time.Duration(days)*24*time.Hour)) {
				continue
			}
			_, dir, err := s.revision(owner, kind, rev.ID)
			if err != nil {
				return plan, err
			}
			n, err := fileTreeBytes(filepath.Join(dir, "content"))
			if err != nil {
				return plan, err
			}
			plan.Entries = append(plan.Entries, fileCleanupEntry{Kind: kind, Revision: rev, Bytes: n})
			plan.Bytes += n
		}
	}
	return plan, nil
}
func (s *Server) cleanupJournal(owner string) ([]fileCleanupEntry, error) {
	root, err := s.fileStateRoot(owner, "cleanup")
	if err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	entries := []fileCleanupEntry{}
	for _, d := range dirs {
		if !strings.HasSuffix(d.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, d.Name()))
		if err != nil {
			return nil, err
		}
		var e fileCleanupEntry
		if err = json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		if e.ID+".json" != d.Name() || (e.Kind != "versions" && e.Kind != "trash") || len(e.Revision.ID) != 36 || strings.ContainsAny(e.Revision.ID, `/\.`) {
			return nil, errors.New("invalid cleanup journal")
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].At.After(entries[j].At) })
	return entries, nil
}
func (s *Server) finishCleanup(owner string, e fileCleanupEntry) error {
	root, err := s.fileStateRoot(owner, e.Kind)
	if err != nil {
		return err
	}
	if err = os.RemoveAll(filepath.Join(root, e.Revision.ID)); err != nil {
		return err
	}
	// Persist removal before marking the journal complete.
	dir, err := os.Open(root)
	if err != nil {
		return err
	}
	err = dir.Sync()
	dir.Close()
	if err != nil {
		return err
	}
	s.recordRevisionActivity(owner, e.Revision, e.Kind+"_cleaned", fileActor{Source: "retention"}, e.ID)
	e.Completed = true
	journal, err := s.fileStateRoot(owner, "cleanup")
	if err != nil {
		return err
	}
	return writeFileStateJSON(filepath.Join(journal, e.ID+".json"), e)
}
func (s *Server) recoverFileCleanup(ctx context.Context, owner string) error {
	entries, err := s.cleanupJournal(owner)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !e.Completed {
			if err = s.finishCleanup(owner, e); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Server) executeFileCleanup(ctx context.Context, owner string, p fileRetentionPolicy) (fileCleanupPlan, error) {
	if err := s.recoverFileCleanup(ctx, owner); err != nil {
		return fileCleanupPlan{}, err
	}
	plan, err := s.cleanupPlan(ctx, owner, p, time.Now().UTC())
	if err != nil {
		return plan, err
	}
	root, err := s.fileStateRoot(owner, "cleanup")
	if err != nil {
		return plan, err
	}
	for i := range plan.Entries {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		e := &plan.Entries[i]
		e.ID = storage.NewID()
		e.At = time.Now().UTC()
		if err = writeFileStateJSON(filepath.Join(root, e.ID+".json"), e); err != nil {
			return plan, err
		}
		if err = s.finishCleanup(owner, *e); err != nil {
			return plan, err
		}
		e.Completed = true
	}
	return plan, nil
}
func (s *Server) cleanFileRetention(ctx context.Context) {
	owners, err := os.ReadDir(filepath.Join(s.ms.Root, "file-state"))
	if err != nil {
		return
	}
	for _, owner := range owners {
		if ctx.Err() != nil {
			return
		}
		if !owner.IsDir() {
			continue
		}
		s.fileMu.Lock()
		if s.spaceIsBlocked(owner.Name()) {
			s.fileMu.Unlock()
			continue
		}
		err = s.recoverFileCleanup(ctx, owner.Name())
		if err == nil {
			var p fileRetentionPolicy
			p, err = s.retentionPolicy(owner.Name())
			if err == nil && p.Automatic {
				_, err = s.executeFileCleanup(ctx, owner.Name(), p)
			}
		}
		s.fileMu.Unlock()
		if err != nil && ctx.Err() == nil {
			s.log.Error("file retention cleanup failed", "owner", owner.Name(), "error", err)
		}
	}
}
func (s *Server) handleFileRetention(w http.ResponseWriter, r *http.Request, owner, op string) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if !s.requireFileManagement(w, r, owner) {
		return
	}
	// Finish durable intents before policy/protection changes or a new preview.
	if err := s.recoverFileCleanup(r.Context(), owner); err != nil {
		fileError(w, err)
		return
	}
	if op == "cleanup/journal" && r.Method == http.MethodGet {
		entries, err := s.cleanupJournal(owner)
		if err != nil {
			fileError(w, err)
			return
		}
		// Return bounded pages; retain the full journal on disk.
		before := r.URL.Query().Get("before")
		var beforeTime time.Time
		if before != "" {
			var parseErr error
			beforeTime, parseErr = time.Parse(time.RFC3339Nano, before)
			if parseErr != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid before timestamp"})
				return
			}
		}
		out := []fileCleanupEntry{}
		for _, e := range entries {
			if before != "" && !e.At.Before(beforeTime) {
				continue
			}
			out = append(out, e)
			if len(out) == 100 {
				break
			}
		}
		writeJSON(w, 200, map[string]any{"entries": out})
		return
	}
	if op == "protection" && r.Method == http.MethodPatch {
		var req struct {
			ID        string `json:"id"`
			Protected bool   `json:"protected"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid request"})
			return
		}
		rev, _, err := s.revision(owner, "versions", req.ID)
		if err == nil {
			rev.Protected = req.Protected
			err = s.rewriteRevision(owner, rev)
		}
		if err != nil {
			fileError(w, err)
			return
		}
		s.recordRevisionActivity(owner, rev, "protection_changed", s.requestFileActor(r), "")
		writeJSON(w, 200, rev)
		return
	}
	p, err := s.retentionPolicy(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	if op == "retention" {
		if r.Method == http.MethodPut {
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p) != nil || !p.valid() {
				writeJSON(w, 400, map[string]string{"error": "invalid retention policy"})
				return
			}
			var e error
			if space, err := s.store.GetFileSpace(r.Context(), owner); err == nil {
				space.Automatic = p.Automatic
				space.VersionDays = p.VersionDays
				space.TrashDays = p.TrashDays
				space.KeepVersions = p.KeepVersions
				au, authErr := s.userFromBearer(r)
				if authErr != nil {
					fileError(w, os.ErrPermission)
					return
				}
				_, e = s.store.SaveFileSpace(r.Context(), au.ID, space)
			} else {
				var root string
				root, e = s.fileStateRoot(owner, "retention")
				if e == nil {
					e = writeFileStateJSON(filepath.Join(root, "policy.json"), p)
				}
			}
			if e != nil {
				fileError(w, e)
				return
			}
		} else if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, 200, p)
		return
	}
	if (op == "cleanup/preview" || op == "cleanup/run") && r.Method == http.MethodPost {
		var plan fileCleanupPlan
		if op == "cleanup/run" {
			plan, err = s.executeFileCleanup(r.Context(), owner, p)
		} else {
			plan, err = s.cleanupPlan(r.Context(), owner, p, time.Now().UTC())
		}
		if err != nil {
			fileError(w, err)
			return
		}
		writeJSON(w, 200, plan)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
