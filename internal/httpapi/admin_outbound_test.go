package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestAdminOutboundQueue(t *testing.T) {
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
	cfg.Mailstore.Root = filepath.Join(dir, "mail")
	cfg.HTTP.Admins = []string{"admin@ex.com"}
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
	it, err := store.EnqueueOutbound(ctx, &storage.OutboundItem{
		EnvelopeFrom: "admin@ex.com",
		EnvelopeTo:   "x@remote.test",
		Data:         []byte("hi"),
		NextAttempt:  time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), store, layer, ms, nil, nil).Handler()
	res, err := layer.LoginWithPassword(ctx, "admin@ex.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	token := res.Tokens.AccessToken

	// A domain administrator must neither list nor mutate another domain's queue.
	otherDom, _ := store.CreateDomain(ctx, tenant.ID, "other.example")
	scopedUser, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: otherDom.ID, Email: "scoped@other.example", LocalPart: "scoped", AuthSource: "local", PasswordHash: hash, Enabled: true, Roles: "domain_admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetDomainAdminDomains(ctx, scopedUser.ID, []string{otherDom.ID}); err != nil {
		t.Fatal(err)
	}
	scopedLogin, err := layer.LoginWithPassword(ctx, scopedUser.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		method, path string
		code         int
	}{{"GET", "/api/v1/admin/outbound", 200}, {"POST", "/api/v1/admin/outbound/" + it.ID + "/retry", 404}, {"DELETE", "/api/v1/admin/outbound/" + it.ID, 404}} {
		req := httptest.NewRequest(check.method, check.path, nil)
		req.Header.Set("Authorization", "Bearer "+scopedLogin.Tokens.AccessToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != check.code {
			t.Fatalf("scoped %s: %d %s", check.path, rec.Code, rec.Body.String())
		}
		if check.method == "GET" {
			var result struct {
				Total int
				Items []any
			}
			if json.Unmarshal(rec.Body.Bytes(), &result) != nil || result.Total != 0 || len(result.Items) != 0 {
				t.Fatal("queue leaked outside domain")
			}
		}
	}

	for _, check := range []struct {
		path, body string
		code       int
	}{{"/api/v1/admin/dkim", `{"domain":"ex.com","selector":"mail","generate":true}`, 404}, {"/api/v1/admin/dkim", `{"domain":"other.example","selector":"mail","generate":true}`, 200}} {
		req := httptest.NewRequest("POST", check.path, strings.NewReader(check.body))
		req.Header.Set("Authorization", "Bearer "+scopedLogin.Tokens.AccessToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != check.code {
			t.Fatalf("DKIM scope %d %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "PRIVATE KEY") {
			t.Fatal("private key leaked in metadata")
		}
	}
	binding, err := storage.ReadDKIMBinding(ctx, store, "other.example")
	if err != nil || binding == nil {
		t.Fatal("binding missing")
	}
	info, err := os.Stat(binding.KeyFile)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private key permissions")
	}
	for _, check := range []struct {
		path string
		code int
	}{{"/api/v1/admin/dkim?domain=ex.com", 404}, {"/api/v1/admin/dkim?domain=other.example", 200}, {"/api/v1/admin/dkim", 200}} {
		req := httptest.NewRequest("GET", check.path, nil)
		req.Header.Set("Authorization", "Bearer "+scopedLogin.Tokens.AccessToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != check.code {
			t.Fatalf("DKIM GET scope %d %s", rec.Code, rec.Body.String())
		}
		if check.path == "/api/v1/admin/dkim" && (strings.Contains(rec.Body.String(), "ex.com") || strings.Contains(rec.Body.String(), "PRIVATE KEY")) {
			t.Fatal("DKIM metadata scope leak")
		}
	}
	// A tenant administrator gains domains added to that tenant, never another tenant.
	if err = store.UpdateUserRoles(ctx, scopedUser.ID, "tenant_admin"); err != nil {
		t.Fatal(err)
	}
	otherTenant, _ := store.CreateTenant(ctx, "outside")
	outsideDomain, _ := store.CreateDomain(ctx, otherTenant.ID, "outside.example")
	if allowed, err := store.UserAdministersDomain(ctx, scopedUser.ID, dom.ID); err != nil || !allowed {
		t.Fatal("tenant domain missing")
	}
	if allowed, err := store.UserAdministersDomain(ctx, scopedUser.ID, outsideDomain.ID); err != nil || allowed {
		t.Fatal("cross-tenant authorization")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/outbound", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var body struct {
		Total int `json:"total"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].ID != it.ID {
		t.Fatalf("%+v", body)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/outbound/"+it.ID+"/retry", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("retry %d %s", w2.Code, w2.Body.String())
	}

	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/outbound/"+it.ID, nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("delete %d %s", w3.Code, w3.Body.String())
	}
	n, _ := store.CountOutbound(ctx)
	if n != 0 {
		t.Fatal(n)
	}
}
