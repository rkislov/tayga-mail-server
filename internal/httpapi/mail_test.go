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

func TestMailSendListRead(t *testing.T) {
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
	cfg.HTTP.Admins = nil
	cfg.FlowSync.Enabled = false
	layer := auth.NewLayer(store, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	_, err = store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "a@ex.com", LocalPart: "a", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "b@ex.com", LocalPart: "b", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), store, layer, ms, nil, nil).Handler()

	res, err := layer.LoginWithPassword(ctx, "a@ex.com", "secret")
	if err != nil || res.Tokens == nil {
		t.Fatalf("login: %v", err)
	}
	token := res.Tokens.AccessToken
	authz := "Bearer " + token

	body, _ := json.Marshal(map[string]any{
		"to": []string{"b@ex.com"}, "subject": "Hello", "text": "world",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mail/send", bytes.NewReader(body))
	req.Header.Set("Authorization", authz)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send %d %s", w.Code, w.Body.String())
	}

	// login as b, list INBOX
	resB, err := layer.LoginWithPassword(ctx, "b@ex.com", "secret")
	if err != nil || resB.Tokens == nil {
		t.Fatalf("login b: %v", err)
	}
	tokenB := resB.Tokens.AccessToken

	req = httptest.NewRequest(http.MethodGet, "/api/v1/mail/mailboxes", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mailboxes %d %s", w.Code, w.Body.String())
	}
	var mbOut struct {
		Mailboxes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"mailboxes"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &mbOut)
	var inboxID string
	for _, mb := range mbOut.Mailboxes {
		if mb.Name == "INBOX" {
			inboxID = mb.ID
		}
	}
	if inboxID == "" {
		t.Fatalf("no inbox: %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/mail/mailboxes/"+inboxID+"/messages", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list %d %s", w.Code, w.Body.String())
	}
	var list struct {
		Messages []struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Messages) < 1 || list.Messages[0].Subject != "Hello" {
		t.Fatalf("messages: %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/mail/messages/"+list.Messages[0].ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["text"] != "world" {
		t.Fatalf("body: %s", w.Body.String())
	}
}
