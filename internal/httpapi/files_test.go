package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestFilesArchiveMoveAndIsolation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := st.CreateTenant(ctx, "t")
	dom, _ := st.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "u@ex.com", LocalPart: "u", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	handler := httpapi.New(cfg, slog.Default(), st, layer, ms, nil, nil).Handler()
	login, err := layer.LoginWithPassword(ctx, u.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	call("POST", "/api/v1/files/mkdir", `{"path":"folder/empty"}`, 200)
	call("PUT", "/api/v1/files/content?path=folder/nested/doc.txt", "archived body", 200)
	response := call("GET", "/api/v1/files/archive?path=folder", "", 200)
	if response.Header().Get("Content-Type") != "application/zip" {
		t.Fatal("not a zip")
	}
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	foundEmpty, foundDoc := false, false
	for _, file := range archive.File {
		if file.Name == "folder/empty/" {
			foundEmpty = true
		}
		if file.Name == "folder/nested/doc.txt" {
			foundDoc = true
			r, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			data, e := io.ReadAll(r)
			r.Close()
			if e != nil || string(data) != "archived body" {
				t.Fatalf("bad archive body %v", e)
			}
		}
	}
	if !foundEmpty || !foundDoc {
		t.Fatal("nested file or empty directory missing")
	}
	call("POST", "/api/v1/files/move", `{"from":"folder","to":"renamed"}`, 200)
	if call("GET", "/api/v1/files/content?path=renamed/nested/doc.txt", "", 200).Body.String() != "archived body" {
		t.Fatal("move lost contents")
	}
	call("PUT", "/api/v1/files/content?path=other.txt", "keep", 200)
	call("POST", "/api/v1/files/move", `{"from":"renamed/nested/doc.txt","to":"other.txt"}`, 409)
	if call("GET", "/api/v1/files/content?path=other.txt", "", 200).Body.String() != "keep" {
		t.Fatal("move overwrote destination")
	}
	call("POST", "/api/v1/files/move", `{"from":"renamed","to":"renamed/inside"}`, 400)
	call("DELETE", "/api/v1/files?path=..", "", 400)
	call("GET", "/api/v1/files/content?path=../other-user/file.txt", "", 400)
	outside := filepath.Join(dir, "outside")
	if err = os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(ms.Root, "files", u.ID, "link")); err == nil {
		call("GET", "/api/v1/files/content?path=link", "", 400)
	}
	call("DELETE", "/api/v1/files?path=renamed", "", 200)
	call("GET", "/api/v1/files/content?path=renamed/nested/doc.txt", "", 404)
}
