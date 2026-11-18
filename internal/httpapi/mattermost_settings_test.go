package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/storage"
)

func TestMattermostScopedAdministration(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.FlowSync.Enabled = false
	cfg.Mattermost.Enabled = true
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "scopes.db")
	st, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, cfg.LDAP, cfg.MFA, cfg.OIDC)
	a, _ := st.CreateTenant(ctx, "A")
	b, _ := st.CreateTenant(ctx, "B")
	one, _ := st.CreateDomain(ctx, a.ID, "one.test")
	two, _ := st.CreateDomain(ctx, a.ID, "two.test")
	foreign, _ := st.CreateDomain(ctx, b.ID, "other.test")
	makeUser := func(name, role string, domain *storage.Domain) string {
		u, e := st.CreateUser(ctx, &storage.User{TenantID: domain.TenantID, DomainID: domain.ID, Email: name + "@" + domain.Name, LocalPart: name, AuthSource: "local", Roles: role, Enabled: true})
		if e != nil {
			t.Fatal(e)
		}
		if role == storage.RoleDomainAdmin {
			if e = st.SetDomainAdminDomains(ctx, u.ID, []string{domain.ID}); e != nil {
				t.Fatal(e)
			}
		}
		p, e := layer.Tokens.IssueTokens(ctx, u.ID)
		if e != nil {
			t.Fatal(e)
		}
		return p.AccessToken
	}
	global := makeUser("global", storage.RoleGlobalAdmin, one)
	tenantAdmin := makeUser("tenant", storage.RoleTenantAdmin, one)
	domainAdmin := makeUser("domain", storage.RoleDomainAdmin, one)
	user := makeUser("user", storage.RoleUser, one)
	h := httpapi.New(cfg, slog.Default(), st, layer, nil, nil, nil).Handler()
	request := func(method, path, token, body string, status int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s => %d want %d: %s", method, path, w.Code, status, w.Body)
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	base := "/api/v1/admin/mattermost"
	own := base + "?tenant_id=" + a.ID + "&domain_id=" + one.ID
	sibling := base + "?tenant_id=" + a.ID + "&domain_id=" + two.ID
	other := base + "?tenant_id=" + b.ID + "&domain_id=" + foreign.ID
	request("GET", base, user, "", 403)
	request("GET", base, domainAdmin, "", 403)
	request("PUT", base, global, `{"plugins_enabled":true}`, 200)
	request("PUT", base+"?tenant_id="+a.ID, tenantAdmin, `{"site_name":"Tenant A"}`, 200)
	result := request("PUT", own, domainAdmin, `{"plugins":{"gitlab":{"configuration":{"ClientSecret":"private-a"}}}}`, 200)
	raw, _ := json.Marshal(result)
	if bytes.Contains(raw, []byte("private-a")) {
		t.Fatal("secret leaked")
	}
	request("GET", sibling, domainAdmin, "", 403)
	request("PUT", other, tenantAdmin, `{"enabled":false}`, 403)
	request("GET", base+"?tenant_id="+b.ID+"&domain_id="+one.ID, global, "", 404)
	request("PUT", own, domainAdmin, `{"enabled":false}{}`, 400)
	request("PUT", own, domainAdmin, `{"unknown_field":true}`, 400)
	scopes := request("GET", base+"/scopes", domainAdmin, "", 200)
	items := scopes["scopes"].([]any)
	if len(items) != 1 {
		t.Fatal(scopes)
	}
	read := request("GET", own, domainAdmin, "", 200)
	if read["effective"].(map[string]any)["site_name"] != "Tenant A" {
		t.Fatal(read)
	}
	request("PUT", own, domainAdmin, `{"enabled":false}`, 200)
	request("GET", "/api/v4/users/me", domainAdmin, "", 403)
	request("GET", "/api/v4/users/me", tenantAdmin, "", 403)
	request("PUT", own, domainAdmin, `{}`, 200)
	draft := request("PUT", own, domainAdmin, `{"email":{"incoming_address":"inbox@one.test","outgoing_from":"chat@one.test","subject_prefix":"[TMS chat]"}}`, 200)
	email := draft["effective"].(map[string]any)["email"].(map[string]any)
	if email["incoming_enabled"] != false || email["outgoing_enabled"] != false || email["user_opt_in_required"] != true {
		t.Fatal(email)
	}
	request("PUT", own, domainAdmin, `{"email":{"incoming_enabled":true}}`, 400)
	request("PUT", own, domainAdmin, `{"email":{"outgoing_from":"chat@two.test"}}`, 400)
	request("PUT", sibling, domainAdmin, `{"email":{"subject_prefix":"[wrong]"}}`, 403)
	request("PUT", own, user, `{"email":{"subject_prefix":"[wrong]"}}`, 403)
	request("PUT", own, domainAdmin, `{}`, 200)
	request("GET", "/api/v4/users/me", domainAdmin, "", 200)
}
