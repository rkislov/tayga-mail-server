package httpapi_test

import (
	"bytes"
	"context"
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

func TestSieveScriptsAPI(t *testing.T) {
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
	layer := auth.NewLayer(store, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	_, err = store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), store, layer, ms).Handler()
	res, err := layer.LoginWithPassword(ctx, "u@ex.com", "secret")
	if err != nil || res.Tokens == nil {
		t.Fatalf("login: %v", err)
	}
	token := res.Tokens.AccessToken

	script := `require ["fileinto"];\nkeep;`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sieve/scripts/default", bytes.NewReader([]byte(`{"script":"require [\"fileinto\"];\nkeep;","active":true}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put %d %s (script len hint %d)", w.Code, w.Body.String(), len(script))
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/sieve/scripts", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || !bytes.Contains(w2.Body.Bytes(), []byte(`"active":true`)) {
		t.Fatalf("list %d %s", w2.Code, w2.Body.String())
	}

	u, _ := store.GetUserByEmail(ctx, "u@ex.com")
	got, err := store.GetActiveSieveScript(ctx, u.ID)
	if err != nil || !got.Active || got.Name != "default" {
		t.Fatalf("active: %+v %v", got, err)
	}
}
