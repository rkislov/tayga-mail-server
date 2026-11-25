package mattermost

import (
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
)

func TestProductBootstrapScopeAndAuthentication(t *testing.T) {
	f := setup(t)
	if err := f.api.EnablePlugins(filepath.Join(t.TempDir(), "plugins"), nil); err != nil {
		t.Fatal(err)
	}
	defer f.api.Close()
	scope := userScope(f.users[0])
	if _, err := f.api.pluginRuntimeForScope(t.Context(), scope); err != nil {
		t.Fatal(err)
	}
	path := "/mm/" + chat.ID(scope.Key()) + "/products/bootstrap"
	call := func(token, url string, want int) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", url, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("Mattermost-User-Id", chat.ID(f.users[0].ID))
		w := httptest.NewRecorder()
		f.api.ServeScopedPluginHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: %d != %d: %s", url, w.Code, want, w.Body.String())
		}
		return w
	}
	call("", path, 401)
	call(f.tokens[3], path, 403)
	w := call(f.tokens[0], path, 200)
	var bootstrap map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &bootstrap); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.Body.String(), f.users[3].Email) || strings.Contains(w.Body.String(), chat.ID(f.users[3].ID)) {
		t.Fatal("foreign tenant bootstrap data leaked")
	}
	if bootstrap["user"].(map[string]any)["id"] != chat.ID(f.users[0].ID) {
		t.Fatal("identity spoofed")
	}
	english := call(f.tokens[0], path+"?lang=en", 200)
	if !strings.Contains(english.Body.String(), `"locale":"en"`) ||
		!strings.Contains(english.Body.String(), `"DefaultClientLocale":"en"`) {
		t.Fatal("product locale was not applied consistently", english.Body.String())
	}
	call(f.tokens[0], "/mm/"+chat.ID(scope.Key())+"/api/v4/users/"+chat.ID(f.users[3].ID), 403)
	call(f.tokens[0], "/mm/"+chat.ID(scope.Key())+"/static/plugins/playbooks/.databases.json", 404)
}

func TestProductDomainBulkQueriesAndReadOnlyCookie(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	siblingDomain, err := f.store.CreateDomain(ctx, f.users[0].TenantID, "sibling.test")
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := f.store.CreateUser(ctx, &storage.User{TenantID: f.users[0].TenantID, DomainID: siblingDomain.ID, Email: "hidden@sibling.test", LocalPart: "hidden", DisplayName: "Private sibling name", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	room, err := f.api.chat.CreateChannel(ctx, sibling, "sibling-board-channel", "Private sibling channel", "O")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a legacy global chat membership. Product scope must still filter it.
	if err = f.api.chat.JoinPublicChannel(ctx, f.users[0], room.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.api.EnablePlugins(filepath.Join(t.TempDir(), "plugins"), nil); err != nil {
		t.Fatal(err)
	}
	defer f.api.Close()
	scope := userScope(f.users[0])
	if _, err = f.api.pluginRuntimeForScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	base := "/mm/" + chat.ID(scope.Key())
	request := func(method, path, body string, token string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, base+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		f.api.ServeScopedPluginHTTP(w, r)
		return w
	}
	boot := request("GET", "/products/bootstrap", "", f.tokens[0], nil)
	if boot.Code != 200 {
		t.Fatal(boot.Code, boot.Body.String())
	}
	cookies := boot.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != base+"/" || cookies[0].MaxAge != 120 {
		t.Fatal("unsafe product session cookie")
	}
	for _, path := range []string{"/api/v4/users/ids", "/api/v4/users/status/ids"} {
		w := request("POST", path, `["`+chat.ID(sibling.ID)+`"]`, f.tokens[0], nil)
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatal("foreign domain bulk lookup", path, w.Code, w.Body.String())
		}
	}
	w := request("GET", "/api/v4/users/me/channels", "", f.tokens[0], nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), room.ID) {
		t.Fatal("foreign channel metadata leaked", w.Code, w.Body.String())
	}
	if w = request("GET", "/products/bootstrap", "", "", cookies[0]); w.Code != 200 {
		t.Fatal("read-only cookie failed", w.Code)
	}
	if w = request("POST", "/api/v4/users/me/preferences", `[]`, "", cookies[0]); w.Code != 401 {
		t.Fatal("cookie authorized a mutation", w.Code)
	}
	if err = f.api.auth.Tokens.RevokeAccess(ctx, f.tokens[0]); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/products/bootstrap", "", "", cookies[0]); w.Code != 401 {
		t.Fatal("revoked product cookie accepted", w.Code)
	}
}
