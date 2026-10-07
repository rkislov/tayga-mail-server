package httpapi_test

import (
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
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

func TestAdminQuarantine(t *testing.T) {
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
	user, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ms := mailstore.New(filepath.Join(dir, "mail"))
	eng := sieve.New(store, ms, nil)
	raw := []byte("From: spammer@evil.test\r\nSubject: buy pills\r\n\r\nxx\r\n")
	if err := eng.FileInto(ctx, user, "Quarantine", nil, raw, "<q@ex.com>"); err != nil {
		t.Fatal(err)
	}

	h := httpapi.New(cfg, slog.Default(), store, layer, ms, nil).Handler()
	res, err := layer.LoginWithPassword(ctx, "admin@ex.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	token := res.Tokens.AccessToken

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/quarantine", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
			Folder  string `json:"folder"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Subject != "buy pills" || body.Items[0].Folder != "Quarantine" {
		t.Fatalf("%+v", body.Items)
	}
	id := body.Items[0].ID

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/quarantine/"+id+"/release", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("release %d %s", w2.Code, w2.Body.String())
	}
	inbox, err := store.GetMailbox(ctx, user.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := store.ListMessages(ctx, inbox.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("inbox msgs=%d err=%v", len(msgs), err)
	}

	if err := eng.FileInto(ctx, user, "Junk", nil, raw, "<j@ex.com>"); err != nil {
		t.Fatal(err)
	}
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/quarantine?folder=Junk", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, req3)
	_ = json.Unmarshal(w3.Body.Bytes(), &body)
	if len(body.Items) != 1 {
		t.Fatalf("junk list %+v", body.Items)
	}
	req4 := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/quarantine/"+body.Items[0].ID, nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	h.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("delete %d %s", w4.Code, w4.Body.String())
	}
}
