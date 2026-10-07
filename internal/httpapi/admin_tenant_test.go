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

func TestAdminTenantDomains(t *testing.T) {
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
	cfg.FlowSync.Enabled = false
	cfg.HTTP.Admins = []string{"admin@ex.com"}
	layer := auth.NewLayer(store, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "acme")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	_, err = store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "admin@ex.com", LocalPart: "admin", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), store, layer, ms, nil).Handler()
	res, err := layer.LoginWithPassword(ctx, "admin@ex.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	token := res.Tokens.AccessToken

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenant", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"name":"acme"`)) {
		t.Fatalf("tenant %d %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/domains", bytes.NewReader([]byte(`{"name":"new.ex.com"}`)))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("create domain %d %s", w2.Code, w2.Body.String())
	}

	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/domains/new.ex.com", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("delete %d %s", w3.Code, w3.Body.String())
	}

	// cannot delete domain with users
	req4 := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/domains/ex.com", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	h.ServeHTTP(w4, req4)
	if w4.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d %s", w4.Code, w4.Body.String())
	}

	var tenantResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &tenantResp)
	if tenantResp["tenant"] == nil {
		t.Fatal("missing tenant")
	}
}
