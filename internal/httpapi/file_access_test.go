package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestFileSharingPermissionsAndPassword(t *testing.T) {
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
	h := httpapi.New(cfg, slog.Default(), st, layer, ms, nil, nil).Handler()
	login, err := layer.LoginWithPassword(ctx, u.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, code int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}

	call("PUT", "/api/v1/files/content?path=team/report.txt", "report", 200)
	var share struct {
		URL string `json:"url"`
	}
	json.Unmarshal(call("POST", "/api/v1/files/shares", `{"path":"team","password":"protected","rights":"read","ttl_hours":1}`, 201), &share)
	public := func(method, path, password, body string, code int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if password != "" {
			r.SetBasicAuth("", password)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("public %s: %d %s", path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	public("GET", share.URL, "", "", 401)
	public("GET", share.URL, "wrong", "", 401)
	archive := public("GET", share.URL, "protected", "", 200)
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "report.txt") {
			f, _ := file.Open()
			raw, _ := io.ReadAll(f)
			f.Close()
			found = string(raw) == "report"
		}
	}
	if !found {
		t.Fatal("shared folder archive lost file")
	}
	public("PUT", share.URL+"?path=report.txt", "protected", "overwrite", 403)
	public("GET", share.URL+"?path=../outside", "protected", "", 400)
	other, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "v@ex.com", LocalPart: "v", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := layer.LoginWithPassword(ctx, other.Email, "secret")
	sharedCall := func(method, path, body string, code int) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+second.Tokens.AccessToken)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("shared %s %s: %d %s", method, path, w.Code, w.Body.String())
		}
	}
	ownerQuery := "&owner_id=" + u.ID
	sharedCall("GET", "/api/v1/files/content?path=team/report.txt"+ownerQuery, "", 403)
	call("POST", "/api/v1/files/access", `{"path":"team","email":"v@ex.com","rights":"read"}`, 200)
	sharedCall("GET", "/api/v1/files/content?path=team/report.txt"+ownerQuery, "", 200)
	sharedCall("PUT", "/api/v1/files/content?path=team/report.txt"+ownerQuery, "bad", 403)
	sharedCall("GET", "/api/v1/files/content?path=team2/report.txt"+ownerQuery, "", 403)
	call("POST", "/api/v1/files/access", `{"path":"team","email":"v@ex.com","rights":"write"}`, 200)
	sharedCall("PUT", "/api/v1/files/content?path=team/new.txt"+ownerQuery, "new", 200)
	payload, _ := json.Marshal(map[string]string{"path": "team", "grantee_id": other.ID})
	call("DELETE", "/api/v1/files/access", string(payload), 200)
	sharedCall("GET", "/api/v1/files/content?path=team/new.txt"+ownerQuery, "", 403)
}
