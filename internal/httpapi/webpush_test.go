package httpapi_test

import (
	"context"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/webpush"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushRequiresBearerAndHidesPrivateKey(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.FlowSync.Enabled = false
	driver, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	layer := auth.NewLayer(driver, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("test-password")
	tenant, _ := driver.CreateTenant(ctx, "test")
	domain, _ := driver.CreateDomain(ctx, tenant.ID, "example.com")
	_, err = driver.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "alice@example.com", LocalPart: "alice", AuthSource: "local", PasswordHash: hash, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	login, err := layer.LoginWithPassword(ctx, "alice@example.com", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	push, err := webpush.New(ctx, driver.(*storage.Store), "mail.example.com")
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.New(cfg, slog.Default(), driver, layer, mailstore.New(filepath.Join(dir, "mail")), nil, nil)
	server.SetWebPush(push)
	handler := server.Handler()
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/v1/push?access_token="+login.Tokens.AccessToken, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("query token accepted for %s: %d", method, w.Code)
		}
	}
	req := httptest.NewRequest("GET", "/api/v1/push", nil)
	req.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), push.PublicKey()) || strings.Contains(w.Body.String(), "private") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
}
