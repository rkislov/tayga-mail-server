package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tayga/tms/internal/auth"
	"golang.org/x/net/webdav"
)

type officeFileLock struct {
	Value, Token string
	Expires      time.Time
}
type fileLockSystem struct {
	webdav.LockSystem
	office map[string]officeFileLock // protected by Server.fileMu
}

func (s *Server) locksFor(owner string) *fileLockSystem {
	if s.fileDAVLocks == nil {
		s.fileDAVLocks = map[string]*fileLockSystem{}
	}
	if s.fileDAVLocks[owner] == nil {
		s.fileDAVLocks[owner] = &fileLockSystem{LockSystem: webdav.NewMemLS(), office: map[string]officeFileLock{}}
	}
	return s.fileDAVLocks[owner]
}

// fileMu protects mutation/office state; DAV locks also cover web/API writes.
func (s *Server) claimFileWrite(owner, rel, officeValue string) (func(), error) {
	ls := s.locksFor(owner)
	now := time.Now()
	rel = "/" + cleanRel(rel)
	if lock, ok := ls.office[rel]; ok {
		if now.Before(lock.Expires) {
			if officeValue == "" || officeValue != lock.Value {
				return nil, errFileLocked
			}
			release, err := ls.Confirm(now, rel, "", webdav.Condition{Token: lock.Token})
			if err != nil {
				return nil, errFileLocked
			}
			return release, nil
		}
		delete(ls.office, rel)
	}
	token, err := ls.Create(now, webdav.LockDetails{Root: rel, Duration: time.Minute})
	if err != nil {
		return nil, errFileLocked
	}
	return func() { _ = ls.Unlock(time.Now(), token) }, nil
}

type davUploadState struct {
	quotaExceeded atomic.Bool
	failed        atomic.Bool
	bytes         atomic.Int64
	expected      int64
}
type davQuotaResponse struct {
	http.ResponseWriter
	state *davUploadState
}

func (w davQuotaResponse) WriteHeader(status int) {
	if w.state.quotaExceeded.Load() && status >= 400 {
		status = http.StatusInsufficientStorage
	}
	w.ResponseWriter.WriteHeader(status)
}

type davBody struct {
	io.ReadCloser
	state *davUploadState
}

func (b *davBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.state.bytes.Add(int64(n))
	if err != nil && err != io.EOF {
		b.state.failed.Store(true)
	}
	return n, err
}

func (s *Server) handleFileDAV(w http.ResponseWriter, r *http.Request) {
	var owner string
	if token := auth.ParseBearerToken(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(r.Header.Get("Authorization")), "bearer ") && token != "" {
		if u, err := s.authn.AuthenticateToken(r.Context(), "", token); err == nil && u != nil {
			owner = u.ID
		}
	} else if name, password, ok := r.BasicAuth(); ok {
		if u, err := s.authn.Authenticate(r.Context(), name, password); err == nil && u != nil {
			owner = u.ID
		}
	}
	if owner == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Tayga Files", charset="UTF-8"`)
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.ContentLength > maxWebUploadBytes {
		http.Error(w, "file exceeds 10 GiB", 413)
		return
	}
	root, err := s.filesRoot(owner)
	if err != nil {
		fileError(w, err)
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == "MOVE" {
		abs, pathErr := safeJoin(root, strings.TrimPrefix(r.URL.Path, "/dav/files"))
		if pathErr != nil {
			http.Error(w, "invalid path", 400)
			return
		}
		info, statErr := os.Stat(abs)
		if statErr != nil && !os.IsNotExist(statErr) {
			fileError(w, statErr)
			return
		}
		match := r.Header.Get("If-Match")
		if (match != "" && match != fileETag(info) && !(match == "*" && info != nil)) || (r.Header.Get("If-None-Match") == "*" && info != nil) {
			http.Error(w, "precondition failed", 412)
			return
		}
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		w.Header().Set("Content-Disposition", "attachment")
		w.Header().Set("Content-Security-Policy", "sandbox")
		w.Header().Set("X-Content-Type-Options", "nosniff")
	}
	state := &davUploadState{expected: r.ContentLength}
	if r.Body != nil {
		r.Body = &davBody{ReadCloser: http.MaxBytesReader(w, r.Body, maxWebUploadBytes), state: state}
	}
	s.fileMu.Lock()
	locks := s.locksFor(owner)
	s.fileMu.Unlock()
	h := webdav.Handler{Prefix: "/dav/files", FileSystem: &personalDAV{srv: s, owner: owner, root: root, upload: state}, LockSystem: locks}
	w.Header().Set("Cache-Control", "private, no-cache")
	h.ServeHTTP(davQuotaResponse{w, state}, r)
}

type personalDAV struct {
	srv         *Server
	owner, root string
	upload      *davUploadState
}

func (f *personalDAV) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	abs, err := safeJoin(f.root, name)
	if err != nil {
		return nil, os.ErrPermission
	}
	return os.Stat(abs)
}
func (f *personalDAV) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	abs, err := safeJoin(f.root, name)
	if err != nil {
		return os.ErrPermission
	}
	f.srv.fileMu.Lock()
	defer f.srv.fileMu.Unlock()
	return os.Mkdir(abs, 0750)
}
func (f *personalDAV) RemoveAll(ctx context.Context, name string) error {
	f.srv.fileMu.Lock()
	defer f.srv.fileMu.Unlock()
	_, err := f.srv.trashFile(f.owner, name)
	return err
}
func (f *personalDAV) Rename(ctx context.Context, from, to string) error {
	f.srv.fileMu.Lock()
	defer f.srv.fileMu.Unlock()
	return f.srv.moveFile(f.owner, from, to)
}
func (s *Server) moveFile(owner, from, to string) error {
	root, err := s.filesRoot(owner)
	if err != nil {
		return err
	}
	src, err := safeJoin(root, from)
	if err != nil {
		return err
	}
	dst, err := safeJoin(root, to)
	if err != nil {
		return err
	}
	if src == root || dst == root || src == dst || strings.HasPrefix(dst, src+string(os.PathSeparator)) {
		return errPathEscape
	}
	if _, err = os.Stat(dst); err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	if err = os.Rename(src, dst); err != nil {
		return err
	}
	if err = s.store.RetargetFileReferences(context.Background(), owner, cleanRel(from), cleanRel(to)); err != nil {
		_ = os.Rename(dst, src)
		return err
	}
	s.invalidateOfficePath(owner, from)
	// Histories follow moved files/folders. A failed metadata update must not
	// pretend a failed move: the original path remains visible in version history.
	revs, e := s.fileRevisions(owner, "versions", "")
	if e == nil {
		for _, rev := range revs {
			old := cleanRel(from)
			if rev.Path != old && !strings.HasPrefix(rev.Path, old+"/") {
				continue
			}
			rev.Path = cleanRel(to) + strings.TrimPrefix(rev.Path, old)
			rev.Name = filepath.Base(rev.Path)
			_ = s.rewriteRevision(owner, rev)
		}
	}
	return nil
}

func (f *personalDAV) OpenFile(ctx context.Context, name string, flags int, perm os.FileMode) (webdav.File, error) {
	abs, err := safeJoin(f.root, name)
	if err != nil {
		return nil, os.ErrPermission
	}
	if flags&(os.O_WRONLY|os.O_RDWR) == 0 {
		return os.Open(abs)
	}
	if abs == f.root {
		return nil, os.ErrPermission
	}
	info, err := os.Stat(abs)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if info != nil && !info.Mode().IsRegular() {
		return nil, os.ErrPermission
	}
	if info == nil && flags&os.O_CREATE == 0 {
		return nil, os.ErrNotExist
	}
	if info != nil && flags&os.O_EXCL != 0 {
		return nil, os.ErrExist
	}
	// DAV requires an existing parent; do not silently create collections on PUT.
	if parent, err := os.Stat(filepath.Dir(abs)); err != nil || !parent.IsDir() {
		return nil, os.ErrNotExist
	}
	root, err := f.srv.fileStateRoot(f.owner, "uploads")
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(root, "dav-")
	if err != nil {
		return nil, err
	}
	if info != nil && flags&os.O_TRUNC == 0 {
		old, e := os.Open(abs)
		if e == nil {
			_, e = io.Copy(temp, old)
			old.Close()
		}
		if e != nil {
			temp.Close()
			os.Remove(temp.Name())
			return nil, e
		}
		if flags&os.O_APPEND == 0 {
			_, _ = temp.Seek(0, io.SeekStart)
		}
	}
	return &davFile{File: temp, fs: f, ctx: ctx, rel: cleanRel(name), expected: fileETag(info)}, nil
}

type davFile struct {
	*os.File
	fs            *personalDAV
	ctx           context.Context
	rel, expected string
	failed        bool
	closed        bool
}

// Do not inherit os.File.ReadFrom: io.Copy must go through our size/error checks.
func (f *davFile) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.Copy(struct{ io.Writer }{f}, r)
	if err != nil {
		f.failed = true
	}
	return n, err
}

func (f *davFile) Write(p []byte) (int, error) {
	pos, err := f.File.Seek(0, io.SeekCurrent)
	if err != nil || pos+int64(len(p)) > maxWebUploadBytes {
		f.failed = true
		return 0, errors.New("file exceeds 10 GiB")
	}
	n, err := f.File.Write(p)
	if err != nil {
		f.failed = true
	}
	return n, err
}
func (f *davFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	defer os.Remove(f.File.Name())
	if err := f.File.Chmod(0640); err != nil {
		f.File.Close()
		return err
	}
	if err := f.File.Sync(); err != nil {
		f.File.Close()
		return err
	}
	if err := f.File.Close(); err != nil {
		return err
	}
	state := f.fs.upload
	if f.failed || f.ctx.Err() != nil || state.failed.Load() || (state.expected > 0 && state.bytes.Load() != state.expected) {
		return errors.New("incomplete upload")
	}
	f.fs.srv.fileMu.Lock()
	defer f.fs.srv.fileMu.Unlock()
	_, err := f.fs.srv.commitFile(f.fs.owner, f.rel, f.File.Name(), f.expected)
	if errors.Is(err, errFileQuota) {
		f.fs.upload.quotaExceeded.Store(true)
	}
	return err
}
