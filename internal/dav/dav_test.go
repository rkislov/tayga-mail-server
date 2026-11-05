package dav_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/dav"
	"github.com/tayga/tms/internal/storage"
)

func TestCalDAVPropFindPrincipal(t *testing.T) {
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

	layer := auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{
		Issuer: "Test", AccessTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour, ChallengeTTL: 5 * time.Minute,
	}, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureDAVDefaults(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	dav.Mount(mux, store, layer)

	body := `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`
	req := httptest.NewRequest("PROPFIND", "/dav/cal/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", "0")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("u@ex.com:secret")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusMultiStatus && w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("/dav/cal/u@ex.com/")) {
		t.Fatalf("missing principal in response: %s", w.Body.String())
	}
	discovery := `<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:schedule-inbox-URL/><c:schedule-outbox-URL/><c:calendar-user-address-set/></d:prop></d:propfind>`
	req = httptest.NewRequest("PROPFIND", "/dav/cal/u@ex.com/", strings.NewReader(discovery))
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Depth", "0")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("u@ex.com:secret")))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 207 || !strings.Contains(w.Body.String(), "/dav/cal/u@ex.com/inbox/") || strings.Contains(w.Body.String(), "404") {
		t.Fatal(w.Code, w.Body.String())
	}
	req = httptest.NewRequest("PROPFIND", "/dav/cal/u@ex.com/inbox/", strings.NewReader(discovery))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("unauthenticated inbox", w.Code)
	}

}

func TestWellKnownRedirect(t *testing.T) {
	mux := http.NewServeMux()
	dav.Mount(mux, nil, nil) // handlers still register well-known
	req := httptest.NewRequest(http.MethodGet, "/.well-known/caldav", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusPermanentRedirect {
		t.Fatalf("status %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/dav/cal/" {
		t.Fatalf("location %q", loc)
	}
}
