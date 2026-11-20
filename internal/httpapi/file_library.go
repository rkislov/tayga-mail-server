package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

var errFileConflict = errors.New("file changed; reload before saving")
var errFileLocked = errors.New("file is being edited; try again after the editor releases it")
var errFileTooLarge = errors.New("file exceeds 10 GiB")
var errFileIncomplete = errors.New("incomplete upload")

type fileRevision struct {
	ActivityIDs map[string]string `json:"activity_ids,omitempty"`
	Protected   bool              `json:"protected"`
	ID          string            `json:"id"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	Size        int64             `json:"size"`
	IsDir       bool              `json:"is_dir"`
	At          time.Time         `json:"at"`
}

func fileETag(info os.FileInfo) string {
	if info == nil {
		return ""
	}
	return fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
}

// Metadata and upload staging stay outside the user-visible tree and all DAV,
// shared-link and ZIP views. Back up file-state together with files.
func (s *Server) fileStateRoot(owner, kind string) (string, error) {
	if owner == "" || strings.ContainsAny(owner, `/\`) {
		return "", errPathEscape
	}
	root := filepath.Join(s.ms.Root, "file-state", owner, kind)
	return root, os.MkdirAll(root, 0700)
}

// Caller holds fileMu. Retain old content without copying multi-GB files.
// Persist metadata first; an incomplete entry without content is never listed.
func (s *Server) retainFile(owner, rel, kind, abs string) (fileRevision, string, error) {
	var rev fileRevision
	info, err := os.Stat(abs)
	if err != nil {
		return rev, "", err
	}
	root, err := s.fileStateRoot(owner, kind)
	if err != nil {
		return rev, "", err
	}
	rev = fileRevision{ID: storage.NewID(), Path: cleanRel(rel), Name: filepath.Base(abs), Size: info.Size(), IsDir: info.IsDir(), At: time.Now().UTC()}
	if kind == "trash" {
		rev.ActivityIDs = s.fileActivityBindings(owner, rel)
	} else if kind == "versions" {
		if state, e := s.fileActivity(owner); e == nil && state.Paths[cleanRel(rel)] != "" {
			rev.ActivityIDs = map[string]string{cleanRel(rel): state.Paths[cleanRel(rel)]}
		}
	}
	dir := filepath.Join(root, rev.ID)
	if err = os.Mkdir(dir, 0700); err != nil {
		return rev, "", err
	}
	data, _ := json.Marshal(rev)
	if err = os.WriteFile(filepath.Join(dir, "metadata.json"), data, 0600); err == nil {
		if kind == "versions" {
			// All managed writers replace atomically. A hard link keeps the prior
			// immutable inode without a second multi-GB copy or a missing-file gap.
			err = os.Link(abs, filepath.Join(dir, "content"))
		} else {
			err = os.Rename(abs, filepath.Join(dir, "content"))
		}
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return rev, "", err
	}
	return rev, dir, nil
}

func (s *Server) fileRevisions(owner, kind, rel string) ([]fileRevision, error) {
	root, err := s.fileStateRoot(owner, kind)
	if err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := []fileRevision{}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, dir.Name(), "metadata.json"))
		var rev fileRevision
		if err != nil || json.Unmarshal(data, &rev) != nil || rev.ID != dir.Name() {
			continue
		}
		if _, err = os.Stat(filepath.Join(root, dir.Name(), "content")); err != nil {
			continue
		}
		if rel != "" && rev.Path != cleanRel(rel) {
			continue
		}
		out = append(out, rev)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return out[i].ID > out[j].ID
		}
		return out[i].At.After(out[j].At)
	})
	return out, nil
}

func (s *Server) rewriteRevision(owner string, rev fileRevision) error {
	root, err := s.fileStateRoot(owner, "versions")
	if err != nil {
		return err
	}
	dir := filepath.Join(root, rev.ID)
	return writeFileStateJSON(filepath.Join(dir, "metadata.json"), rev)
}

func (s *Server) revision(owner, kind, id string) (fileRevision, string, error) {
	var rev fileRevision
	if len(id) != 36 || strings.ContainsAny(id, `/\.`) {
		return rev, "", os.ErrNotExist
	}
	root, err := s.fileStateRoot(owner, kind)
	if err != nil {
		return rev, "", err
	}
	dir := filepath.Join(root, id)
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return rev, "", err
	}
	if json.Unmarshal(data, &rev) != nil || rev.ID != id {
		return rev, "", os.ErrNotExist
	}
	return rev, dir, nil
}

// commitFile is shared by web, public upload links, WebDAV and WOPI. The
// expected ETag prevents a slow uploader from overwriting a concurrent edit.
// Caller holds fileMu and, if applicable, the corresponding DAV/WOPI lock.
func (s *Server) commitFile(owner, rel, temp, expected string, actors ...fileActor) (string, error) {
	if err := s.checkStageGeneration(temp, owner); err != nil {
		return "", err
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		return "", err
	}
	abs, err := safeJoin(root, rel)
	if err != nil || abs == root {
		return "", errPathEscape
	}
	info, err := os.Stat(abs)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if info != nil && !info.Mode().IsRegular() {
		return "", errPathEscape
	}
	if expected != "*" && fileETag(info) != expected {
		return "", errFileConflict
	}
	newInfo, err := os.Stat(temp)
	if err != nil {
		return "", err
	}
	if err = s.checkFileQuota(owner, newInfo.Size()); err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		return "", err
	}
	backup := ""
	if info != nil {
		_, backup, err = s.retainFile(owner, rel, "versions", abs)
		if err != nil {
			return "", err
		}
	}
	if err = os.Rename(temp, abs); err != nil {
		if backup != "" {
			_ = os.RemoveAll(backup)
		}
		return "", err
	}
	action := "updated"
	if info == nil {
		action = "created"
	}
	s.recordFileActivity(owner, rel, action, actors)
	info, err = os.Stat(abs)
	return fileETag(info), err
}

func (s *Server) stageFile(ctx context.Context, owner string, src io.Reader, size int64) (string, int64, error) {
	if size > maxWebUploadBytes {
		return "", 0, errFileTooLarge
	}
	if size > 0 {
		s.fileMu.Lock()
		err := s.checkFileQuota(owner, size)
		s.fileMu.Unlock()
		if err != nil {
			return "", 0, err
		}
	}
	root, err := s.fileStateRoot(owner, "uploads")
	if err != nil {
		return "", 0, err
	}
	f, err := os.CreateTemp(root, "upload-")
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	if err = s.saveStageGeneration(f.Name(), owner); err != nil {
		s.removeFileStage(f.Name())
		return "", 0, err
	}
	n, err := io.Copy(f, io.LimitReader(src, maxWebUploadBytes+1))
	if err == nil && n > maxWebUploadBytes {
		err = errFileTooLarge
	}
	if err == nil && size >= 0 && n != size {
		err = errFileIncomplete
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = f.Chmod(0640)
	}
	if err == nil {
		err = f.Sync()
	}
	if err == nil {
		err = f.Close()
	}
	if err != nil {
		s.removeFileStage(f.Name())
		return "", n, err
	}
	return f.Name(), n, nil
}

func (s *Server) saveFile(w http.ResponseWriter, r *http.Request, owner, rel string) (int64, error) {
	root, err := s.filesRoot(owner)
	if err != nil {
		return 0, err
	}
	abs, err := safeJoin(root, rel)
	if err != nil || abs == root {
		return 0, errPathEscape
	}
	info, err := os.Stat(abs)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	expected := fileETag(info)
	if match := r.Header.Get("If-Match"); match != "" && match != expected && !(match == "*" && info != nil) {
		return 0, errFileConflict
	}
	if r.Header.Get("If-None-Match") == "*" && info != nil {
		return 0, errFileConflict
	}
	actor := s.requestFileActor(r)
	temp, n, err := s.stageFile(r.Context(), owner, r.Body, r.ContentLength)
	if err != nil {
		return n, err
	}
	defer s.removeFileStage(temp)
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if actor.ID != "" && actor.ID != owner && !s.fileAccessAllowed(r, owner, actor.ID, rel, true) {
		return n, os.ErrPermission
	}
	if actor.Source == "public_link" {
		token := strings.TrimPrefix(r.URL.Path, "/s/")
		sh, e := s.store.GetFileShareByToken(r.Context(), token)
		if e != nil || sh.UserID != owner || sh.Rights != "write" || !pathAccessApplies(sh.Path, cleanRel(rel)) || (sh.ExpiresAt != nil && !time.Now().Before(*sh.ExpiresAt)) || (sh.MaxDownloads > 0 && sh.DownloadCount >= sh.MaxDownloads) {
			return n, os.ErrPermission
		}
	}
	release, err := s.claimFileWrite(owner, rel, "")
	if err != nil {
		return n, err
	}
	defer release()
	etag, err := s.commitFile(owner, rel, temp, expected, actor)
	if err == nil {
		w.Header().Set("ETag", etag)
	}
	return n, err
}

func (s *Server) trashFile(owner, rel string, actors ...fileActor) (fileRevision, error) {
	var rev fileRevision
	root, err := s.filesRoot(owner)
	if err != nil {
		return rev, err
	}
	abs, err := safeJoin(root, rel)
	if err != nil || abs == root {
		return rev, errPathEscape
	}
	var dir string
	rev, dir, err = s.retainFile(owner, rel, "trash", abs)
	if err == nil {
		err = s.store.RetargetFileReferences(context.Background(), owner, cleanRel(rel), "")
		if err != nil {
			if os.Rename(filepath.Join(dir, "content"), abs) == nil {
				_ = os.RemoveAll(dir)
			}
		} else {
			s.invalidateOfficePath(owner, rel)
			actor := fileActor{Source: "system"}
			if len(actors) > 0 {
				actor = actors[0]
			}
			bindings := map[string]string{}
			affected := []string{}
			for p, id := range rev.ActivityIDs {
				bindings[p] = ""
				affected = append(affected, id)
			}
			s.recordFileEvent(owner, fileActivityEvent{Path: rel, Object: rev.ActivityIDs[cleanRel(rel)], Affected: affected, Action: "trashed", Actor: actor, Bindings: bindings})
		}
	}
	return rev, err
}

func (s *Server) invalidateOfficePath(owner, rel string) {
	rel = cleanRel(rel)
	for token, session := range s.officeSessions {
		if session.Owner == owner && (session.Path == rel || strings.HasPrefix(session.Path, rel+"/")) {
			delete(s.officeSessions, token)
		}
	}
}

func fileError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, errSpaceRecoveryPending):
		status = 503
	case errors.Is(err, errFileConflict), errors.Is(err, os.ErrExist):
		status = 409
	case errors.Is(err, errFileLocked), errors.Is(err, errFileProtected):
		status = 423
	case errors.Is(err, errFileTooLarge):
		status = 413
	case errors.Is(err, errFileQuota):
		status = http.StatusInsufficientStorage
	case errors.Is(err, errFileIncomplete):
		status = 400
	case errors.Is(err, os.ErrPermission):
		status = 403
	case errors.Is(err, os.ErrNotExist):
		status = 404
	case errors.Is(err, errPathEscape):
		status = 400
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleFileLibrary(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	owner := au.ID
	op := strings.TrimPrefix(r.URL.Path, "/api/v1/files/library/")
	if requested := r.URL.Query().Get("owner_id"); requested != "" && requested != au.ID {
		permitted := s.fileCanManage(r.Context(), requested, au.ID)
		if op == "usage" || op == "search" {
			if _, e := s.store.GetFileSpace(r.Context(), requested); e == nil {
				permitted = s.fileAccessAllowed(r, requested, au.ID, "", false)
			}
		}
		if !permitted {
			writeJSON(w, 403, map[string]string{"error": "owner or space management required"})
			return
		}
		owner = requested
	}

	if op == "retention" || op == "protection" || strings.HasPrefix(op, "cleanup/") {
		s.handleFileRetention(w, r, owner, op)
		return
	}
	if op == "compare" {
		s.handleFileComparison(w, r, owner)
		return
	}
	if op == "usage" && r.Method == http.MethodGet {
		s.fileMu.Lock()
		usage, err := s.fileUsage(owner)
		s.fileMu.Unlock()
		if err != nil {
			fileError(w, err)
			return
		}
		writeJSON(w, 200, usage)
		return
	}
	if op == "search" && r.Method == http.MethodGet {
		if mode := r.URL.Query().Get("mode"); mode != "" {
			s.searchFileContents(w, r, au.ID)
		} else {
			s.searchFiles(w, r, owner)
		}
		return
	}
	if (op == "trash" || op == "versions") && r.Method == http.MethodGet {
		s.fileMu.Lock()
		if !s.requireFileManagement(w, r, owner) {
			s.fileMu.Unlock()
			return
		}
		items, err := s.fileRevisions(owner, op, r.URL.Query().Get("path"))
		s.fileMu.Unlock()
		if err != nil {
			fileError(w, err)
			return
		}
		provider := s.cfgLive().Files.OfficeProvider
		writeJSON(w, 200, map[string]any{"entries": items, "office_compare_enabled": (provider == "onlyoffice" || provider == "r7") && s.cfgLive().Files.OfficeURL != "", "visual_compare_enabled": s.cfgLive().Files.OfficeURL != ""})
		return
	}
	if (op == "restore" && r.Method == http.MethodPost) || ((op == "trash" || op == "versions") && r.Method == http.MethodDelete) {
		var req struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid request"})
			return
		}
		if r.Method == http.MethodDelete {
			req.Kind = op
		}
		if req.Kind != "trash" && req.Kind != "versions" {
			writeJSON(w, 400, map[string]string{"error": "invalid kind"})
			return
		}
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		if !s.requireFileManagement(w, r, owner) {
			return
		}
		if err := s.recoverFileCleanup(r.Context(), owner); err != nil {
			fileError(w, err)
			return
		}
		rev, dir, err := s.revision(owner, req.Kind, req.ID)
		if err != nil {
			fileError(w, err)
			return
		}
		if r.Method == http.MethodDelete {
			if rev.Protected {
				fileError(w, errFileProtected)
				return
			}
			if err = os.RemoveAll(dir); err != nil {
				fileError(w, err)
				return
			}
			s.recordRevisionActivity(owner, rev, req.Kind+"_deleted", fileActor{ID: au.ID, Source: "web"}, "")
			writeJSON(w, 200, map[string]string{"status": "deleted"})
			return
		}
		root, err := s.filesRoot(owner)
		if err != nil {
			fileError(w, err)
			return
		}
		abs, err := safeJoin(root, rev.Path)
		if err != nil || abs == root {
			fileError(w, errPathEscape)
			return
		}
		release, err := s.claimFileWrite(owner, rev.Path, "")
		if err != nil {
			fileError(w, err)
			return
		}
		defer release()
		if req.Kind == "trash" {
			if _, err = os.Lstat(abs); err == nil {
				fileError(w, os.ErrExist)
				return
			}
			if err = os.MkdirAll(filepath.Dir(abs), 0750); err == nil {
				err = os.Rename(filepath.Join(dir, "content"), abs)
			}
			if err == nil {
				_ = os.RemoveAll(dir)
			}
		} else {
			// Versions are immutable. Link into staging rather than holding the
			// mutation mutex while copying a potentially multi-GB document.
			var uploads string
			uploads, err = s.fileStateRoot(owner, "uploads")
			if err == nil {
				temp := filepath.Join(uploads, "restore-"+storage.NewID())
				if err = os.Link(filepath.Join(dir, "content"), temp); err == nil {
					_, err = s.commitFile(owner, rev.Path, temp, "*", fileActor{ID: au.ID, Source: "web"})
					_ = os.Remove(temp)
				}
			}
		}
		if err != nil {
			fileError(w, err)
			return
		}
		if req.Kind == "trash" && len(rev.ActivityIDs) > 0 {
			s.recordFileEvent(owner, fileActivityEvent{Path: rev.Path, Object: rev.ActivityIDs[rev.Path], Bindings: rev.ActivityIDs, Action: "restored", Actor: fileActor{ID: au.ID, Source: "web"}})
		} else {
			s.recordFileActivity(owner, rev.Path, "restored", []fileActor{{ID: au.ID, Source: "web"}})
		}
		writeJSON(w, 200, map[string]string{"path": rev.Path, "status": "restored"})
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}

func (s *Server) searchFiles(w http.ResponseWriter, r *http.Request, owner string) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" || len(query) > 256 {
		writeJSON(w, 400, map[string]string{"error": "search query required"})
		return
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	base, err := safeJoin(root, r.URL.Query().Get("path"))
	if err != nil {
		fileError(w, err)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 1000 {
		page = 1000
	}
	limit := 50
	offset := (page - 1) * limit
	total, visited := 0, 0
	items := []fileRevision{}
	truncated := false
	err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := r.Context().Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		visited++
		if visited > 100000 {
			truncated = true
			return fs.SkipAll
		}
		if entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".upload-") {
			return nil
		}
		if path == base || !strings.Contains(strings.ToLower(entry.Name()), query) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		if total >= offset && len(items) < limit {
			rel, _ := filepath.Rel(root, path)
			items = append(items, fileRevision{Path: filepath.ToSlash(rel), Name: info.Name(), Size: info.Size(), IsDir: info.IsDir(), At: info.ModTime()})
		}
		total++
		return nil
	})
	if err != nil {
		fileError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"entries": items, "total": total, "page": page, "page_size": limit, "truncated": truncated})
}
