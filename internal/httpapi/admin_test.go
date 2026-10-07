package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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

func TestMeAndAdminQuota(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer os.RemoveAll(dir)

	cfg := config.Default()
	cfg.HTTP.Admins = []string{"admin@ex.com"}
	cfg.FlowSync.Enabled = false
	layer := auth.NewLayer(store, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	_, err = store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "admin@ex.com", LocalPart: "admin", AuthSource: "local",
		PasswordHash: hash, Enabled: true, QuotaBytes: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: hash, Enabled: true, QuotaBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), store, layer, ms).Handler()

	res, err := layer.LoginWithPassword(ctx, "admin@ex.com", "secret")
	if err != nil || res.Tokens == nil {
		t.Fatalf("login: %v %+v", err, res)
	}
	token := res.Tokens.AccessToken

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me %d %s", w.Code, w.Body.String())
	}
	var me map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me["email"] != "admin@ex.com" || me["is_admin"] != true {
		t.Fatalf("me: %s", w.Body.String())
	}
	if me["quota_bytes"].(float64) != 1000 {
		t.Fatalf("quota: %v", me["quota_bytes"])
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || !bytes.Contains(w2.Body.Bytes(), []byte("u@ex.com")) {
		t.Fatalf("users %d %s", w2.Code, w2.Body.String())
	}

	req3 := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+user.ID+"/quota", bytes.NewReader([]byte(`{"quota_bytes":42}`)))
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("quota %d %s", w3.Code, w3.Body.String())
	}
	got, _ := store.GetUserByID(ctx, user.ID)
	if got.QuotaBytes != 42 {
		t.Fatalf("quota=%d", got.QuotaBytes)
	}

	reqDis := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+user.ID, bytes.NewReader([]byte(`{"enabled":false}`)))
	reqDis.Header.Set("Authorization", "Bearer "+token)
	reqDis.Header.Set("Content-Type", "application/json")
	wDis := httptest.NewRecorder()
	h.ServeHTTP(wDis, reqDis)
	if wDis.Code != http.StatusOK {
		t.Fatalf("disable %d %s", wDis.Code, wDis.Body.String())
	}
	got, _ = store.GetUserByID(ctx, user.ID)
	if got.Enabled {
		t.Fatal("expected disabled")
	}

	reqPw := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+user.ID+"/password", bytes.NewReader([]byte(`{"password":"newsecret1"}`)))
	reqPw.Header.Set("Authorization", "Bearer "+token)
	reqPw.Header.Set("Content-Type", "application/json")
	wPw := httptest.NewRecorder()
	h.ServeHTTP(wPw, reqPw)
	if wPw.Code != http.StatusOK {
		t.Fatalf("password %d %s", wPw.Code, wPw.Body.String())
	}
	_ = store.UpdateUserEnabled(ctx, user.ID, true)
	if _, err := layer.LoginWithPassword(ctx, "u@ex.com", "newsecret1"); err != nil {
		t.Fatalf("login after reset: %v", err)
	}

	// non-admin forbidden
	ures, err := layer.LoginWithPassword(ctx, "u@ex.com", "newsecret1")
	if err != nil {
		t.Fatal(err)
	}
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req4.Header.Set("Authorization", "Bearer "+ures.Tokens.AccessToken)
	w4 := httptest.NewRecorder()
	h.ServeHTTP(w4, req4)
	if w4.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w4.Code)
	}
}
