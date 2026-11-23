// SPDX-License-Identifier: Apache-2.0
package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/storage"
)

func TestMattermostFederationAdminScopeAndRedaction(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := config.Default()
	cfg.FlowSync.Enabled = false
	cfg.Mattermost.Enabled = true
	cfg.Mattermost.Federation.Enabled = true
	cfg.Mailstore.Root = filepath.Join(root, "mail")
	if e := os.MkdirAll(cfg.Mailstore.Root, 0700); e != nil {
		t.Fatal(e)
	}
	cfg.Storage.SQLite.Path = filepath.Join(root, "db.sqlite")
	cfg.HTTP.PublicURL = "https://mail.example.test"
	st, e := storage.Open(ctx, cfg.Storage)
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	layer := auth.NewLayer(st, cfg.LDAP, cfg.MFA, cfg.OIDC)
	tenant, _ := st.CreateTenant(ctx, "A")
	otherTenant, _ := st.CreateTenant(ctx, "B")
	one, _ := st.CreateDomain(ctx, tenant.ID, "one.test")
	sibling, _ := st.CreateDomain(ctx, tenant.ID, "sibling.test")
	foreign, _ := st.CreateDomain(ctx, otherTenant.ID, "foreign.test")
	makeUser := func(name, role string, d *storage.Domain) (*storage.User, string) {
		u, e := st.CreateUser(storage.WithoutWelcomeMail(ctx), &storage.User{TenantID: d.TenantID, DomainID: d.ID, Email: name + "@" + d.Name, LocalPart: name, AuthSource: "local", Roles: role, Enabled: true})
		if e != nil {
			t.Fatal(e)
		}
		if role == storage.RoleDomainAdmin {
			if e = st.SetDomainAdminDomains(ctx, u.ID, []string{d.ID}); e != nil {
				t.Fatal(e)
			}
		}
		pair, e := layer.Tokens.IssueTokens(ctx, u.ID)
		if e != nil {
			t.Fatal(e)
		}
		return u, pair.AccessToken
	}
	_, global := makeUser("root", storage.RoleGlobalAdmin, one)
	owner, admin := makeUser("domain", storage.RoleDomainAdmin, one)
	_, plain := makeUser("plain", storage.RoleUser, one)
	handler := httpapi.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), st, layer, nil, nil, nil).Handler()
	request := func(method, path, token, body string, status int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s => %d expected %d: %s", path, w.Code, status, w.Body)
		}
		if strings.Contains(w.Body.String(), "private-upstream-token") {
			t.Fatal("upstream credential returned")
		}
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	base := "/api/v1/admin/mattermost/federation"
	scope := "?tenant_id=" + one.TenantID + "&domain_id=" + one.ID
	request("GET", base+scope, plain, "", 403)
	request("GET", base+"?tenant_id="+sibling.TenantID+"&domain_id="+sibling.ID, admin, "", 403)
	request("GET", base+"?tenant_id="+foreign.TenantID+"&domain_id="+foreign.ID, admin, "", 403)
	created := request("POST", base+scope, admin, `{"name":"counterpart","mode":"rest","site_url":"https://chat.example.test","access_token":"private-upstream-token"}`, 201)
	peer := created["peer"].(map[string]any)
	id := peer["id"].(string)
	if peer["owner_id"] != owner.ID || peer["domain_id"] != one.ID {
		t.Fatal("incorrect scope", peer)
	}
	listed := request("GET", base+scope, admin, "", 200)
	if len(listed["peers"].([]any)) != 1 {
		t.Fatal(listed)
	}
	request("GET", base+"/"+id+"/links?tenant_id="+sibling.TenantID+"&domain_id="+sibling.ID, global, "", 404)
	request("POST", base+scope, admin, `{"name":"smuggled","mode":"rest","site_url":"https://chat.example.test","access_token":"private-upstream-token","domain_id":"`+sibling.ID+`"}`, 400)
	request("DELETE", base+"/"+id+scope, admin, "", 200)
	var secret string
	if e = st.DB().QueryRow(`SELECT secret FROM chat_federation_peers WHERE id=?`, id).Scan(&secret); e != nil || secret != "" {
		t.Fatal("disabled connection retained its secret", e)
	}
	request("POST", base+scope, admin, `{"name":"counterpart","mode":"rest","site_url":"https://chat.example.test","access_token":"private-upstream-token"}`, 201)
}
