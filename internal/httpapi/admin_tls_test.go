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
	"github.com/tayga/tms/internal/tlsutil"
)

func TestAdminTLSGenerateAndStatus(t *testing.T) {
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
	cfg.TLS.CertFile = filepath.Join(dir, "server.crt")
	cfg.TLS.KeyFile = filepath.Join(dir, "server.key")
	cfg.TLS.AutoGenerate = false
	cfg.Server.Hostname = "mail.test"

	tlsMgr, err := tlsutil.NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	layer := auth.NewLayer(store, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "t")
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
	h := httpapi.New(cfg, slog.Default(), store, layer, ms, tlsMgr).Handler()

	res, err := layer.LoginWithPassword(ctx, "admin@ex.com", "secret")
	if err != nil || res.Tokens == nil {
		t.Fatal(err)
	}
	token := res.Tokens.AccessToken

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tls", bytes.NewReader([]byte(`{"hosts":["mail.test","127.0.0.1"],"days":30}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("generate %d %s", w.Code, w.Body.String())
	}
	var info map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &info)
	if info["configured"] != true {
		t.Fatalf("info: %s", w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tls", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK || !bytes.Contains(w2.Body.Bytes(), []byte("fingerprint_sha256")) {
		t.Fatalf("status %d %s", w2.Code, w2.Body.String())
	}
}
