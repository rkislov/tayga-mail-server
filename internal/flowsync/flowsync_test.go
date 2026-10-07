package flowsync_test

import (
	"context"
	"encoding/base64"
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
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestFlowSyncFolderSyncUsesUUIDs(t *testing.T) {
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

	ms := mailstore.New(filepath.Join(dir, "mail"))
	layer := auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{
		Issuer: "T", AccessTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour, ChallengeTTL: 5 * time.Minute,
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
	if _, err := store.EnsureMailbox(ctx, u.ID, "INBOX", "/tmp"); err != nil {
		t.Fatal(err)
	}
	mb, err := store.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	if len(mb.ID) < 32 {
		t.Fatalf("expected UUID mailbox id, got %q", mb.ID)
	}

	cfg := &config.Config{
		Server:   config.ServerConfig{Hostname: "mail.ex.com"},
		HTTP:     config.HTTPConfig{Listen: ":8080", PublicURL: "http://127.0.0.1:8080"},
		FlowSync: config.FlowSyncConfig{Enabled: true},
	}
	mux := http.NewServeMux()
	flowsync.Mount(mux, cfg, slog.Default(), store, layer, ms)

	req := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=FolderSync&DeviceId=dev1&DeviceType=Test", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("u@ex.com:secret")))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "<ServerId>"+mb.ID+"</ServerId>") {
		t.Fatalf("expected mailbox UUID in ServerId, body=%s", body)
	}
	if w.Header().Get("X-FlowSync") != "Tayga-Proprietary" {
		t.Fatal("missing proprietary header")
	}
}

func TestAutodiscoverPOX(t *testing.T) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Hostname: "mail.ex.com"},
		HTTP:     config.HTTPConfig{PublicURL: "http://127.0.0.1:8080"},
		FlowSync: config.FlowSyncConfig{Enabled: true},
	}
	mux := http.NewServeMux()
	flowsync.Mount(mux, cfg, slog.Default(), nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/Autodiscover/Autodiscover.xml", strings.NewReader(`<EMailAddress>a@ex.com</EMailAddress>`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "/Microsoft-Server-ActiveSync") {
		t.Fatalf("missing AS url: %s", w.Body.String())
	}
}
