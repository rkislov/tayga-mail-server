package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestContactEditPreservesIdentityAndOwnership(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := st.CreateTenant(ctx, "t")
	dom, _ := st.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "u@ex.com", LocalPart: "u", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	ms := mailstore.New(filepath.Join(dir, "mail"))
	h := httpapi.New(cfg, slog.Default(), st, layer, ms, nil, nil).Handler()
	login, err := layer.LoginWithPassword(ctx, u.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, code int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+login.Tokens.AccessToken)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}

	var books struct {
		Books []struct {
			ID string `json:"id"`
		} `json:"books"`
	}
	json.Unmarshal(call("GET", "/api/v1/contacts/books", "", 200), &books)
	path := "/api/v1/contacts/books/" + books.Books[0].ID + "/cards"
	var created struct {
		ID  string `json:"id"`
		UID string `json:"uid"`
	}
	json.Unmarshal(call("POST", path, `{"fn":"Original","email":"a@ex.com"}`, 200), &created)
	payload, _ := json.Marshal(map[string]string{"id": created.ID, "fn": "Edited", "email": "b@ex.com"})
	call("PUT", path, string(payload), 200)
	if got := call("GET", "/api/v1/contacts/cards/"+created.ID, "", 200); !bytes.Contains(got, []byte(`"fn":"Edited"`)) || !bytes.Contains(got, []byte(created.UID)) {
		t.Fatal("contact identity or edits lost")
	}
	other, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: "other@ex.com", LocalPart: "other", Enabled: true, AuthSource: "local", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	st.EnsureDAVDefaults(ctx, other.ID)
	otherBooks, _ := st.ListAddressBooks(ctx, other.ID)
	otherLogin, err := layer.LoginWithPassword(ctx, other.Email, "secret")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPut, "/api/v1/contacts/books/"+otherBooks[0].ID+"/cards", strings.NewReader(string(payload)))
	r.Header.Set("Authorization", "Bearer "+otherLogin.Tokens.AccessToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("foreign contact edit allowed: %d", w.Code)
	}
}
