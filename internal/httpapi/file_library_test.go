package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

type fileFixture struct {
	s             *Server
	tokens, users []string
	t             *testing.T
}

func newFileFixture(t *testing.T) *fileFixture {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.HTTP.PublicURL = "https://files.example.com"
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, err := layer.Hasher.Hash("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	tenant, _ := st.CreateTenant(ctx, "office-test")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	f := &fileFixture{s: New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), st, layer, mailstore.New(filepath.Join(dir, "data")), nil, nil), t: t}
	for _, name := range []string{"owner", "reader", "editor"} {
		u, e := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: name + "@example.com", LocalPart: name, DisplayName: name, Enabled: true, AuthSource: "local", PasswordHash: hash})
		if e != nil {
			t.Fatal(e)
		}
		login, e := layer.LoginWithPassword(ctx, u.Email, "test-secret")
		if e != nil {
			t.Fatal(e)
		}
		f.users = append(f.users, u.ID)
		f.tokens = append(f.tokens, login.Tokens.AccessToken)
	}
	return f
}
func (f *fileFixture) call(user int, method, path, body string, code int, headers ...string) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if user >= 0 {
		r.Header.Set("Authorization", "Bearer "+f.tokens[user])
	}
	for i := 0; i < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, r)
	if w.Code != code {
		f.t.Fatalf("%s %s: expected %d, got %d %s", method, strings.Split(path, "?")[0], code, w.Code, w.Body.String())
	}
	return w
}
func revisions(t *testing.T, w *httptest.ResponseRecorder) []fileRevision {
	t.Helper()
	var v struct{ Entries []fileRevision }
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v.Entries
}
func TestFileLibraryVersionTrashAndSharing(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=team/report.txt", "v1", 200)
	first := f.call(0, "GET", "/api/v1/files/content?path=team/report.txt", "", 200).Header().Get("ETag")
	f.call(0, "PUT", "/api/v1/files/content?path=team/report.txt", "v2", 200, "If-Match", first)
	f.call(0, "PUT", "/api/v1/files/content?path=team/report.txt", "stale", 409, "If-Match", first)
	v := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=team/report.txt", "", 200))
	if len(v) != 1 {
		t.Fatal(v)
	}
	f.call(1, "GET", "/api/v1/files/library/versions?owner_id="+f.users[0], "", 403)
	f.call(1, "POST", "/api/v1/files/library/restore", `{"id":"`+v[0].ID+`","kind":"versions"}`, 404)
	f.call(0, "POST", "/api/v1/files/library/restore", `{"id":"`+v[0].ID+`","kind":"versions"}`, 200)
	if f.call(0, "GET", "/api/v1/files/content?path=team/report.txt", "", 200).Body.String() != "v1" {
		t.Fatal("restore")
	}
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"reader@example.com","rights":"read"}`, 200)
	var link struct{ URL string }
	json.Unmarshal(f.call(0, "POST", "/api/v1/files/shares", `{"path":"team/report.txt","rights":"read"}`, 201).Body.Bytes(), &link)
	f.call(0, "POST", "/api/v1/files/move", `{"from":"team","to":"moved"}`, 200)
	f.call(1, "GET", "/api/v1/files/content?path=moved/report.txt&owner_id="+f.users[0], "", 200)
	f.call(-1, "GET", link.URL, "", 200)
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=moved/report.txt", "", 200))) != 2 {
		t.Fatal("history did not follow move")
	}
	f.call(0, "DELETE", "/api/v1/files?path=moved", "", 200)
	trash := revisions(t, f.call(0, "GET", "/api/v1/files/library/trash", "", 200))
	if len(trash) != 1 || !trash[0].IsDir {
		t.Fatal(trash)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=moved/report.txt", "private replacement", 200)
	f.call(-1, "GET", link.URL, "", 404)
	f.call(1, "GET", "/api/v1/files/content?path=moved/report.txt&owner_id="+f.users[0], "", 403)
	f.call(0, "POST", "/api/v1/files/library/restore", `{"id":"`+trash[0].ID+`","kind":"trash"}`, 409)
	f.call(0, "DELETE", "/api/v1/files?path=moved", "", 200)
	f.call(0, "POST", "/api/v1/files/library/restore", `{"id":"`+trash[0].ID+`","kind":"trash"}`, 200)
	if f.call(0, "GET", "/api/v1/files/content?path=moved/report.txt", "", 200).Body.String() != "v1" {
		t.Fatal("trash contents")
	}
	f.call(0, "DELETE", "/api/v1/files/library/trash", `{"id":"`+trash[0].ID+`"}`, 404)
	search := f.call(0, "GET", "/api/v1/files/library/search?q=report", "", 200)
	if !bytes.Contains(search.Body.Bytes(), []byte("moved/report.txt")) {
		t.Fatal(search.Body.String())
	}
	root, _ := f.s.filesRoot(f.users[0])
	outside := filepath.Join(t.TempDir(), "private.txt")
	os.WriteFile(outside, []byte("private"), 0600)
	os.Symlink(outside, filepath.Join(root, "linked.txt"))
	f.call(0, "GET", "/api/v1/files/content?path=linked.txt", "", 400)
	if bytes.Contains(f.call(0, "GET", "/api/v1/files/library/search?q=linked", "", 200).Body.Bytes(), []byte("linked.txt")) {
		t.Fatal("symlink leaked")
	}
}
func TestFileDAVRoundTripLocksAndAtomicUpload(t *testing.T) {
	f := newFileFixture(t)
	f.call(-1, "PROPFIND", "/dav/files/", "", 401)
	f.call(-1, "PROPFIND", "/dav/files/", "", 207, "Authorization", "Basic b3duZXJAZXhhbXBsZS5jb206dGVzdC1zZWNyZXQ=", "Depth", "1")
	f.call(0, "MKCOL", "/dav/files/docs/", "", 201)
	f.call(0, "PUT", "/dav/files/docs/a.txt", "one", 201, "If-None-Match", "*")
	f.call(0, "PUT", "/dav/files/docs/a.txt", "bad", 412, "If-None-Match", "*")
	etag := f.call(0, "GET", "/dav/files/docs/a.txt", "", 200).Header().Get("ETag")
	f.call(0, "PUT", "/dav/files/docs/a.txt", "two", 201, "If-Match", etag)
	f.call(1, "GET", "/dav/files/docs/a.txt", "", 404)
	lock := f.call(0, "LOCK", "/dav/files/docs/a.txt", `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`, 200, "Depth", "0").Header().Get("Lock-Token")
	if lock == "" {
		t.Fatal("missing lock token")
	}
	f.call(0, "PUT", "/api/v1/files/content?path=docs/a.txt", "blocked", 423)
	f.call(0, "PUT", "/dav/files/docs/a.txt", "blocked", 423)
	f.call(0, "PUT", "/dav/files/docs/a.txt", "three", 201, "If", "("+lock+")")
	f.call(0, "UNLOCK", "/dav/files/docs/a.txt", "", 204, "Lock-Token", lock)
	r := httptest.NewRequest("PUT", "/dav/files/docs/a.txt", strings.NewReader("short"))
	r.ContentLength = 100
	r.Header.Set("Authorization", "Bearer "+f.tokens[0])
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, r)
	if w.Code < 400 {
		t.Fatal("partial upload accepted")
	}
	if f.call(0, "GET", "/dav/files/docs/a.txt", "", 200).Body.String() != "three" {
		t.Fatal("partial upload damaged file")
	}
	f.call(0, "COPY", "/dav/files/docs/a.txt", "", 201, "Destination", "https://example.com/dav/files/docs/b.txt")
	f.call(0, "MOVE", "/dav/files/docs/b.txt", "", 201, "Destination", "https://example.com/dav/files/docs/c.txt")
	f.call(0, "DELETE", "/dav/files/docs/c.txt", "", 204)
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/trash", "", 200))) != 1 {
		t.Fatal("DAV bypassed trash")
	}
	f.call(0, "PUT", "/dav/files/%2e%2e/%2e%2e/outside.txt", "escape", 400)
}
func TestFileConcurrentUploadDoesNotOverwrite(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "before", 200)
	started, resume := make(chan struct{}), make(chan struct{})
	reader := &pausedReader{started: started, resume: resume, data: strings.NewReader("slow upload")}
	r := httptest.NewRequest("PUT", "/api/v1/files/content?path=a.txt", reader)
	r.Header.Set("Authorization", "Bearer "+f.tokens[0])
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { f.s.Handler().ServeHTTP(w, r); close(done) }()
	<-started
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "newer", 200)
	close(resume)
	<-done
	if w.Code != 409 {
		t.Fatalf("concurrent overwrite: %d", w.Code)
	}
	if f.call(0, "GET", "/api/v1/files/content?path=a.txt", "", 200).Body.String() != "newer" {
		t.Fatal("lost update")
	}
}

type pausedReader struct {
	started, resume chan struct{}
	data            io.Reader
	begun           bool
}

func (p *pausedReader) Read(b []byte) (int, error) {
	if !p.begun {
		p.begun = true
		close(p.started)
		<-p.resume
	}
	return p.data.Read(b)
}
