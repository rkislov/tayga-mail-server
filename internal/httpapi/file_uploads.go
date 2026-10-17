package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const uploadChunkBytes = 8 << 20
const uploadLifetime = 24 * time.Hour

// A session belongs to its initiating user, even when writing a shared folder.
// Only durable offsets are acknowledged. A partial last chunk is discarded on
// retry, so losing a response or restarting the server cannot duplicate bytes.
type resumableUpload struct {
	ID       string    `json:"id"`
	Owner    string    `json:"owner_id"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Offset   int64     `json:"offset"`
	Expires  time.Time `json:"expires"`
	Complete bool      `json:"complete"`
}
type uploadChunk struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type uploadRecord struct {
	Chunks []uploadChunk `json:"chunks"`
	resumableUpload
	ExpectedETag string `json:"expected_etag"`
}

func saveUploadRecord(dir string, u uploadRecord) error {
	raw, err := json.Marshal(u)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "metadata.new"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(filepath.Join(dir, "metadata.new"), filepath.Join(dir, "metadata.json"))
}
func readUploadRecord(dir string) (uploadRecord, error) {
	var u uploadRecord
	raw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return u, err
	}
	err = json.Unmarshal(raw, &u)
	return u, err
}
func (s *Server) handleFileUploads(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	// Fixed stripes avoid an unbounded lock map. Never hold fileMu while receiving
	// network data. Operations on other accounts continue during a slow upload.
	hash := sha256.Sum256([]byte(au.ID))
	lock := &s.uploadLocks[int(hash[0])%len(s.uploadLocks)]
	lock.Lock()
	defer lock.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	root, err := s.fileStateRoot(au.ID, "resumable")
	if err != nil {
		fileError(w, err)
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/files/uploads")
	if suffix == "" || suffix == "/" {
		items := []resumableUpload{}
		var reserved int64
		dirs, err := os.ReadDir(root)
		if err != nil {
			fileError(w, err)
			return
		}
		for _, entry := range dirs {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			u, e := readUploadRecord(dir)
			if e != nil { // Incomplete session creation, never a committed user file.
				if info, e := entry.Info(); e == nil && time.Since(info.ModTime()) > uploadLifetime {
					_ = os.RemoveAll(dir)
				}
				continue
			}
			if time.Now().After(u.Expires) {
				_ = os.RemoveAll(dir)
				continue
			}
			if !u.Complete {
				items = append(items, u.resumableUpload)
				reserved += u.Size
			}
		}
		if r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]any{"entries": items, "chunk_size": uploadChunkBytes})
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		var req struct {
			Path  string `json:"path"`
			Owner string `json:"owner_id"`
			Size  int64  `json:"size"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || req.Size < 0 {
			writeJSON(w, 400, map[string]string{"error": "invalid upload"})
			return
		}
		if req.Size > maxWebUploadBytes {
			fileError(w, errFileTooLarge)
			return
		}
		if len(items) >= 4 || reserved+req.Size > 2*maxWebUploadBytes {
			writeJSON(w, 429, map[string]string{"error": "Too many unfinished uploads. Resume or cancel an upload first."})
			return
		}
		owner := req.Owner
		if owner == "" {
			owner = au.ID
		}
		if owner != au.ID && !s.fileAccessAllowed(r, owner, au.ID, req.Path, true) {
			writeJSON(w, 403, map[string]string{"error": "file access denied"})
			return
		}
		targetRoot, e := s.filesRoot(owner)
		if e != nil {
			fileError(w, e)
			return
		}
		abs, e := safeJoin(targetRoot, req.Path)
		if e != nil || abs == targetRoot {
			fileError(w, errPathEscape)
			return
		}
		info, e := os.Stat(abs)
		if e != nil && !os.IsNotExist(e) {
			fileError(w, e)
			return
		}
		if info != nil && !info.Mode().IsRegular() {
			fileError(w, errPathEscape)
			return
		}
		u := uploadRecord{resumableUpload: resumableUpload{ID: storage.NewID(), Owner: owner, Path: cleanRel(req.Path), Size: req.Size, Expires: time.Now().UTC().Add(uploadLifetime)}, ExpectedETag: fileETag(info)}
		dir := filepath.Join(root, u.ID)
		if e = os.Mkdir(dir, 0700); e == nil {
			e = os.WriteFile(filepath.Join(dir, "content"), nil, 0640)
		}
		if e == nil {
			e = saveUploadRecord(dir, u)
		}
		if e != nil {
			_ = os.RemoveAll(dir)
			fileError(w, e)
			return
		}
		writeJSON(w, 201, u.resumableUpload)
		return
	}
	parts := strings.Split(strings.TrimPrefix(suffix, "/"), "/")
	id := parts[0]
	if len(id) != 36 || strings.ContainsAny(id, `/\.`) || len(parts) > 2 {
		fileError(w, os.ErrNotExist)
		return
	}
	dir := filepath.Join(root, id)
	u, err := readUploadRecord(dir)
	if err != nil || u.ID != id {
		fileError(w, os.ErrNotExist)
		return
	}
	if time.Now().After(u.Expires) {
		_ = os.RemoveAll(dir)
		writeJSON(w, 410, map[string]string{"error": "upload expired"})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if err := os.RemoveAll(dir); err != nil {
			fileError(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if u.Owner != au.ID && !s.fileAccessAllowed(r, u.Owner, au.ID, u.Path, true) {
		writeJSON(w, 403, map[string]string{"error": "file access denied"})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"upload": u.resumableUpload, "chunks": u.Chunks})
		return
	}
	if len(parts) == 2 && parts[1] == "complete" && r.Method == http.MethodPost {
		if u.Complete {
			writeJSON(w, 200, u.resumableUpload)
			return
		}
		if u.Offset != u.Size {
			fileError(w, errFileIncomplete)
			return
		}
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		// Revalidate after acquiring the same lock used by ACL/move/trash changes.
		if u.Owner != au.ID && !s.fileAccessAllowed(r, u.Owner, au.ID, u.Path, true) {
			writeJSON(w, 403, map[string]string{"error": "file access denied"})
			return
		}
		release, e := s.claimFileWrite(u.Owner, u.Path, "")
		if e != nil {
			fileError(w, e)
			return
		}
		defer release()
		targetRoot, e := s.filesRoot(u.Owner)
		if e != nil {
			fileError(w, e)
			return
		}
		abs, e := safeJoin(targetRoot, u.Path)
		if e != nil {
			fileError(w, e)
			return
		}
		content := filepath.Join(dir, "content")
		staged, e := os.Stat(content)
		if e != nil {
			fileError(w, e)
			return
		}
		if staged.Size() != u.Size {
			fileError(w, errFileIncomplete)
			return
		}
		current, _ := os.Stat(abs)
		// Keeping a hard link until metadata is committed makes a crash immediately
		// after rename recoverable without saving an extra version or overwriting edits.
		if current == nil || !os.SameFile(staged, current) {
			link := filepath.Join(dir, "commit")
			_ = os.Remove(link)
			if e = os.Link(content, link); e == nil {
				_, e = s.commitFile(u.Owner, u.Path, link, u.ExpectedETag)
			}
			_ = os.Remove(link)
			if e != nil {
				fileError(w, e)
				return
			}
		}
		u.Complete = true
		if e = saveUploadRecord(dir, u); e != nil {
			fileError(w, e)
			return
		}
		_ = os.Remove(content)
		writeJSON(w, 200, u.resumableUpload)
		return
	}
	if len(parts) != 1 || r.Method != http.MethodPatch {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	offset, e := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if e != nil || offset < 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid Upload-Offset"})
		return
	}
	w.Header().Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	if u.Complete || offset != u.Offset {
		writeJSON(w, 409, map[string]string{"error": "upload offset mismatch"})
		return
	}
	if len(u.Chunks) >= 2048 || r.ContentLength < 0 || r.ContentLength > uploadChunkBytes || r.ContentLength == 0 || r.ContentLength > u.Size-u.Offset {
		writeJSON(w, 400, map[string]string{"error": "invalid chunk size"})
		return
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(2 * time.Minute))
	defer controller.SetReadDeadline(time.Time{})
	f, e := os.OpenFile(filepath.Join(dir, "content"), os.O_WRONLY, 0640)
	if e != nil {
		fileError(w, e)
		return
	}
	defer f.Close()
	if e = f.Truncate(u.Offset); e == nil {
		_, e = f.Seek(u.Offset, io.SeekStart)
	}
	var n int64
	digest := sha256.New()
	if e == nil {
		n, e = io.Copy(io.MultiWriter(f, digest), io.LimitReader(r.Body, r.ContentLength+1))
	}
	if e == nil && (n != r.ContentLength || r.Context().Err() != nil) {
		e = errFileIncomplete
	}
	if e == nil {
		e = f.Sync()
	}
	if e != nil {
		_ = f.Truncate(u.Offset)
		fileError(w, fmt.Errorf("chunk not saved: %w", e))
		return
	}
	u.Offset += n
	u.Chunks = append(u.Chunks, uploadChunk{Size: n, SHA256: hex.EncodeToString(digest.Sum(nil))})
	if e = saveUploadRecord(dir, u); e != nil {
		fileError(w, e)
		return
	}
	w.Header().Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	writeJSON(w, 200, u.resumableUpload)
}

// Expired staging content is not a user backup. Only session directories created
// here are removed; current files, versions and trash are never traversed.
func (s *Server) cleanExpiredUploads(ctx context.Context) {
	root := filepath.Join(s.ms.Root, "file-state")
	owners, err := os.ReadDir(root)
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
		sum := sha256.Sum256([]byte(owner.Name()))
		lock := &s.uploadLocks[int(sum[0])%len(s.uploadLocks)]
		lock.Lock()
		folder := filepath.Join(root, owner.Name(), "resumable")
		entries, _ := os.ReadDir(folder)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(folder, entry.Name())
			record, e := readUploadRecord(dir)
			expired := e == nil && time.Now().After(record.Expires)
			if e != nil {
				if info, e := entry.Info(); e == nil {
					expired = time.Since(info.ModTime()) > uploadLifetime
				}
			}
			if expired {
				_ = os.RemoveAll(dir)
			}
		}
		lock.Unlock()
	}
}
func (s *Server) runUploadCleanup(ctx context.Context) {
	s.cleanExpiredUploads(ctx)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanExpiredUploads(ctx)
		}
	}
}
