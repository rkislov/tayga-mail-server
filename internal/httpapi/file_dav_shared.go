package httpapi

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// Ancestors are visible only for navigation; their contents are filtered and
// they never confer write access to siblings of an explicit grant.
func (f *personalDAV) allowed(ctx context.Context, name string, write, ancestors bool) bool {
	if f.grantee == "" {
		return true
	}
	abs, err := safeJoin(f.root, name)
	if err != nil {
		return false
	}
	if f.srv.fileAccessAllowedContext(ctx, f.owner, f.grantee, name, write) {
		return true
	}
	if write || !ancestors {
		return false
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return false
	}
	rel := cleanRel(name)
	grants, err := f.srv.store.ListFileAccess(ctx, f.owner, f.grantee)
	if err != nil {
		return false
	}
	for _, grant := range grants {
		if rel != "" && !strings.HasPrefix(grant.Path, rel+"/") {
			continue
		}
		target, e := safeJoin(f.root, grant.Path)
		if e != nil {
			continue
		}
		if _, e = os.Stat(target); e == nil {
			return true
		}
	}
	return false
}

type sharedDAVReadFile struct {
	*os.File
	fs  *personalDAV
	ctx context.Context
	rel string
}

func (f *sharedDAVReadFile) Read(p []byte) (int, error) {
	if !f.fs.allowed(f.ctx, f.rel, false, false) {
		return 0, os.ErrPermission
	}
	return f.File.Read(p)
}

// Avoid os.File.WriteTo bypassing the access check during io.Copy.
func (f *sharedDAVReadFile) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, struct{ io.Reader }{f})
}
func (f *sharedDAVReadFile) Readdir(count int) ([]os.FileInfo, error) {
	var out []os.FileInfo
	for count <= 0 || len(out) < count {
		if !f.fs.allowed(f.ctx, f.rel, false, true) {
			return nil, os.ErrPermission
		}
		entries, err := f.File.Readdir(1)
		for _, entry := range entries {
			name := filepath.ToSlash(filepath.Join(f.rel, entry.Name()))
			if entry.Mode()&os.ModeSymlink == 0 && !strings.HasPrefix(entry.Name(), ".upload-") && f.fs.allowed(f.ctx, name, false, true) {
				out = append(out, entry)
			}
		}
		if err != nil {
			if err == io.EOF && (count <= 0 || len(out) > 0) {
				return out, nil
			}
			return out, err
		}
	}
	return out, nil
}

func (s *Server) serveSharedDAVIndex(w http.ResponseWriter, r *http.Request, actor string) {
	if r.Method != "PROPFIND" && r.Method != "OPTIONS" {
		http.Error(w, "read-only collection", 403)
		return
	}
	if r.Method == "PROPFIND" && r.Header.Get("Depth") != "0" && r.Header.Get("Depth") != "1" {
		http.Error(w, "use Depth 0 or 1 for shared discovery", 403)
		return
	}
	grants, err := s.store.ListFileAccess(r.Context(), "", actor)
	if err != nil {
		fileError(w, err)
		return
	}
	owners := map[string]bool{}
	for _, grant := range grants {
		owner, e := s.store.GetUserByID(r.Context(), grant.OwnerID)
		if e != nil || !owner.Enabled {
			continue
		}
		root, e := s.filesRoot(owner.ID)
		if e != nil {
			continue
		}
		abs, e := safeJoin(root, grant.Path)
		if e != nil {
			continue
		}
		if _, e = os.Stat(abs); e == nil {
			owners[owner.Email] = true
		}
	}
	entries := []os.FileInfo{}
	for email := range owners {
		entries = append(entries, davSharedInfo{name: email})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	h := webdav.Handler{Prefix: "/dav/shared", FileSystem: &sharedDAVIndex{entries: entries}, LockSystem: webdav.NewMemLS()}
	w.Header().Set("Cache-Control", "private, no-cache")
	h.ServeHTTP(w, r)
}

type davSharedInfo struct{ name string }

func (i davSharedInfo) Name() string       { return i.name }
func (i davSharedInfo) Size() int64        { return 0 }
func (i davSharedInfo) Mode() os.FileMode  { return os.ModeDir | 0550 }
func (i davSharedInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (i davSharedInfo) IsDir() bool        { return true }
func (i davSharedInfo) Sys() any           { return nil }

type sharedDAVIndex struct{ entries []os.FileInfo }

func (f *sharedDAVIndex) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	rel := cleanRel(name)
	if rel == "" {
		return davSharedInfo{name: "shared"}, nil
	}
	for _, entry := range f.entries {
		if rel == entry.Name() {
			return entry, nil
		}
	}
	return nil, os.ErrNotExist
}
func (f *sharedDAVIndex) OpenFile(ctx context.Context, name string, flags int, perm os.FileMode) (webdav.File, error) {
	if flags&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) != 0 {
		return nil, os.ErrPermission
	}
	info, err := f.Stat(ctx, name)
	if err != nil {
		return nil, err
	}
	entries := []os.FileInfo{}
	if cleanRel(name) == "" {
		entries = f.entries
	}
	return &sharedDAVIndexFile{info: info, entries: entries}, nil
}
func (f *sharedDAVIndex) Mkdir(context.Context, string, os.FileMode) error { return os.ErrPermission }
func (f *sharedDAVIndex) RemoveAll(context.Context, string) error          { return os.ErrPermission }
func (f *sharedDAVIndex) Rename(context.Context, string, string) error     { return os.ErrPermission }

type sharedDAVIndexFile struct {
	info    os.FileInfo
	entries []os.FileInfo
	pos     int
}

func (f *sharedDAVIndexFile) Stat() (os.FileInfo, error)     { return f.info, nil }
func (f *sharedDAVIndexFile) Close() error                   { return nil }
func (f *sharedDAVIndexFile) Read([]byte) (int, error)       { return 0, io.EOF }
func (f *sharedDAVIndexFile) Write([]byte) (int, error)      { return 0, os.ErrPermission }
func (f *sharedDAVIndexFile) Seek(int64, int) (int64, error) { return 0, os.ErrPermission }
func (f *sharedDAVIndexFile) Readdir(n int) ([]os.FileInfo, error) {
	if f.pos >= len(f.entries) && n > 0 {
		return nil, io.EOF
	}
	end := len(f.entries)
	if n > 0 && f.pos+n < end {
		end = f.pos + n
	}
	out := f.entries[f.pos:end]
	f.pos = end
	return out, nil
}
