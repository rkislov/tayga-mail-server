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
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestOrganizationDirectoryAPIAndWebDelivery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.HTTP.Admins = []string{"admin@example.com"}
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	hash, _ := layer.Hasher.Hash("secret")
	tenant, _ := st.CreateTenant(ctx, "Tenant")
	domain, _ := st.CreateDomain(ctx, tenant.ID, "example.com")
	other, _ := st.CreateDomain(ctx, tenant.ID, "other.example")
	newUser := func(domain *storage.Domain, local string) (*storage.User, string) {
		t.Helper()
		u, err := st.CreateUser(ctx, &storage.User{DomainID: domain.ID, TenantID: tenant.ID, LocalPart: local, Email: local + "@" + domain.Name, PasswordHash: hash, AuthSource: "local", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		result, err := layer.LoginWithPassword(ctx, u.Email, "secret")
		if err != nil {
			t.Fatal(err)
		}
		return u, result.Tokens.AccessToken
	}
	admin, adminToken := newUser(domain, "admin")
	member, memberToken := newUser(domain, "member")
	_, otherToken := newUser(other, "other")
	_, domainToken := newUser(domain, "manager")
	manager, _ := st.GetUserByEmail(ctx, "manager@example.com")
	if err = st.UpdateUserRoles(ctx, manager.ID, "domain_admin"); err != nil {
		t.Fatal(err)
	}
	if err = st.SetDomainAdminDomains(ctx, manager.ID, []string{domain.ID}); err != nil {
		t.Fatal(err)
	}
	h := httpapi.New(cfg, slog.Default(), st, layer, mailstore.New(filepath.Join(dir, "mail")), nil, nil).Handler()
	call := func(token, method, path string, value any, code int) []byte {
		t.Helper()
		body, _ := json.Marshal(value)
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != code {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	base := "/api/v1/admin/directory"
	query := "?domain_id=" + domain.ID
	call("", "GET", "/api/v1/directory", nil, 401)
	call(memberToken, "GET", "/api/v1/directory", nil, 404)
	call(memberToken, "GET", base+query, nil, 403)
	call(domainToken, "GET", base+"?domain_id="+other.ID, nil, 403)
	if err = st.PutSetting(ctx, "domain.gal."+domain.ID, "true"); err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting(ctx, "domain.gal."+other.ID, "true"); err != nil {
		t.Fatal(err)
	}
	var group storage.DirectoryGroup
	payload := map[string]any{"name": "Отдел разработки", "local_part": "engineering", "member_ids": []string{admin.ID, member.ID}}
	if err = json.Unmarshal(call(domainToken, "POST", base+query, payload, 201), &group); err != nil {
		t.Fatal(err)
	}
	own := call(memberToken, "GET", "/api/v1/directory", nil, 200)
	if !bytes.Contains(own, []byte(group.Email)) || bytes.Contains(own, []byte(hash)) || bytes.Contains(own, []byte(`"editable":true`)) {
		t.Fatal(string(own))
	}
	foreign := call(otherToken, "GET", "/api/v1/directory?domain_id="+domain.ID, nil, 200)
	if bytes.Contains(foreign, []byte(group.ID)) || bytes.Contains(foreign, []byte(member.Email)) {
		t.Fatal("directory crossed domains", string(foreign))
	}
	call(memberToken, "DELETE", base+"/"+group.ID+query+"&revision=1", nil, 403)
	call(adminToken, "PUT", base+"/"+group.ID+"?domain_id="+other.ID, group, 404)
	gal := call(memberToken, "GET", "/api/v1/contacts/books/gal/cards", nil, 200)
	if !bytes.Contains(gal, []byte(group.Email)) {
		t.Fatal(string(gal))
	}
	call(memberToken, "GET", "/api/v1/contacts/cards/gal-group-"+group.ID, nil, 200)
	call(otherToken, "GET", "/api/v1/contacts/cards/gal-group-"+group.ID, nil, 404)
	call(adminToken, "POST", "/api/v1/mail/send", map[string]any{"to": []string{group.Email, member.Email}, "text": "Hello team", "subject": "Department"}, 200)
	for _, user := range []*storage.User{admin, member} {
		box, err := st.GetMailbox(ctx, user.ID, "INBOX")
		if err != nil {
			t.Fatal(err)
		}
		messages, err := st.ListMessages(ctx, box.ID)
		if err != nil || len(messages) != 1 {
			t.Fatal(user.Email, messages, err)
		}
	}
	group.Name = "Development"
	call(domainToken, "PUT", base+"/"+group.ID+query, group, 200)
	call(domainToken, "PUT", base+"/"+group.ID+query, group, 409)
	call(domainToken, "DELETE", base+"/"+group.ID+query+"&revision=1", nil, 409)
	call(domainToken, "DELETE", base+"/"+group.ID+query+"&revision=2", nil, 200)
	if got := call(memberToken, "GET", "/api/v1/directory", nil, 200); strings.Contains(string(got), group.ID) {
		t.Fatal("deleted group retained")
	}
	source := storage.DefaultDirectorySource()
	source.Mode, source.BaseDN = "ldap", "ou=Groups,dc=example,dc=com"
	call(domainToken, "PUT", base+"/source"+query, source, 400)
	layer.LDAP = auth.NewDirectory(st, config.LDAPConfig{Domains: map[string]config.LDAPDomainConfig{
		"example.com": {Enabled: true, URL: "ldap://127.0.0.1:1", BaseDN: "ou=People,dc=example,dc=com", OrganizationBaseDN: "ou=Groups,dc=example,dc=com", BindPassword: "test-secret"},
	}})
	call(domainToken, "PUT", base+"/source?domain_id="+other.ID, source, 403)
	call(memberToken, "PUT", base+"/source"+query, source, 403)
	bad := source
	bad.BaseDN = "dc=example,dc=com"
	call(domainToken, "PUT", base+"/source"+query, bad, 400)
	// Status timestamps are server-owned, even if a client supplies them.
	source.LastError = "forged"
	saved := call(domainToken, "PUT", base+"/source"+query, source, 200)
	if bytes.Contains(saved, []byte("forged")) {
		t.Fatal("client supplied sync status", string(saved))
	}
	call(domainToken, "PUT", base+"/source"+query, source, 409)
	for _, action := range []string{"preview", "sync"} {
		call(memberToken, "POST", base+"/"+action+query, nil, 403)
		call(domainToken, "POST", base+"/"+action+"?domain_id="+other.ID, nil, 403)
	}
	call(domainToken, "POST", base+query, payload, 409)
	adminView := call(domainToken, "GET", base+query, nil, 200)
	if !bytes.Contains(adminView, []byte(`"ldap_available":true`)) || !bytes.Contains(adminView, []byte(`"editable":false`)) || bytes.Contains(adminView, []byte("test-secret")) {
		t.Fatal(string(adminView))
	}
	own = call(memberToken, "GET", "/api/v1/directory", nil, 200)
	for _, private := range []string{"source", "ldap_root", "base_dn", "test-secret"} {
		if bytes.Contains(own, []byte(private)) {
			t.Fatal("LDAP configuration exposed to normal user", string(own))
		}
	}
}
