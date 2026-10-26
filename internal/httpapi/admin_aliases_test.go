package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"net/mail"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestAdminAliasesAPIAndDelivery(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Mailstore.Root = filepath.Join(dir, "mail")
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
	foreignTenant, _ := st.CreateTenant(ctx, "Foreign")
	foreignDomain, _ := st.CreateDomain(ctx, foreignTenant.ID, "foreign.example")
	newUser := func(d *storage.Domain, local string) (*storage.User, string) {
		t.Helper()
		u, e := st.CreateUser(ctx, &storage.User{DomainID: d.ID, TenantID: d.TenantID, LocalPart: local, Email: local + "@" + d.Name, PasswordHash: hash, Enabled: true})
		if e != nil {
			t.Fatal(e)
		}
		login, e := layer.LoginWithPassword(ctx, u.Email, "secret")
		if e != nil {
			t.Fatal(e)
		}
		return u, login.Tokens.AccessToken
	}
	_, adminToken := newUser(domain, "admin")
	member, memberToken := newUser(domain, "member")
	manager, managerToken := newUser(domain, "manager")
	foreignUser, _ := newUser(foreignDomain, "user")
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
	domains := "/api/v1/admin/domains/" + domain.ID + "/aliases"
	users := "/api/v1/admin/users/" + member.ID + "/aliases"
	payload := map[string]string{"name": "alias.example"}
	call("", "GET", domains, nil, 401)
	call(memberToken, "GET", domains, nil, 403)
	call(managerToken, "POST", domains, payload, 403)
	var d storage.DomainAlias
	if err = json.Unmarshal(call(adminToken, "POST", domains, payload, 201), &d); err != nil {
		t.Fatal(err)
	}
	call(adminToken, "POST", domains, payload, 409)
	call(adminToken, "POST", domains, map[string]string{"name": "bad domain"}, 400)
	call(adminToken, "POST", domains, map[string]string{"name": "example.com"}, 409)
	call(managerToken, "GET", domains, nil, 200)
	dkim := call(managerToken, "GET", "/api/v1/admin/dkim", nil, 200)
	if !bytes.Contains(dkim, []byte("alias.example")) {
		t.Fatal("missing alias DKIM", string(dkim))
	}
	call(managerToken, "POST", "/api/v1/admin/dkim", map[string]any{"domain": "alias.example", "selector": "mail", "generate": true, "enabled": true}, 200)
	binding, err := storage.ReadDKIMBinding(ctx, st, "alias.example")
	if err != nil || binding == nil || binding.Domain != "alias.example" {
		t.Fatal("alias DKIM binding", binding, err)
	}

	call(managerToken, "GET", "/api/v1/admin/domains/"+other.ID+"/aliases", nil, 403)
	call(adminToken, "DELETE", "/api/v1/admin/domains/"+other.ID+"/aliases/"+d.ID, nil, 404)
	var a storage.Alias
	if err = json.Unmarshal(call(managerToken, "POST", users, map[string]string{"local_part": "sales"}, 201), &a); err != nil {
		t.Fatal(err)
	}
	call(managerToken, "GET", users, nil, 200)
	call(memberToken, "GET", users, nil, 403)
	call(adminToken, "POST", users, map[string]string{"local_part": "member"}, 409)
	call(adminToken, "POST", users, map[string]string{"local_part": "a@b"}, 400)
	call(managerToken, "POST", users, map[string]string{"local_part": "cross", "domain_id": other.ID}, 403)
	call(adminToken, "POST", users, map[string]string{"local_part": "cross", "domain_id": foreignDomain.ID}, 403)
	call(adminToken, "POST", users, map[string]string{"local_part": "cross", "domain_id": other.ID}, 201)
	visible := call(managerToken, "GET", users, nil, 200)
	if bytes.Contains(visible, []byte("cross@other.example")) {
		t.Fatal("exposed unmanaged source domain", string(visible))
	}
	call(managerToken, "GET", "/api/v1/admin/users/"+foreignUser.ID+"/aliases", nil, 403)
	call(adminToken, "PATCH", users+"/"+a.ID, map[string]string{"local_part": "support"}, 200)
	call(adminToken, "PATCH", users+"/"+a.ID, map[string]string{"local_part": "support", "domain_id": other.ID}, 400)
	call(adminToken, "DELETE", "/api/v1/admin/users/"+manager.ID+"/aliases/"+a.ID, nil, 404)
	call(adminToken, "POST", "/api/v1/mail/send", map[string]any{"to": []string{"support@alias.example", "member@alias.example", member.Email}, "text": "Alias message", "subject": "Aliases"}, 200)
	box, err := st.GetMailbox(ctx, member.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := st.ListMessages(ctx, box.ID)
	if err != nil || len(messages) != 1 {
		t.Fatal("alias deduplication", messages, err)
	}

	var incoming map[string]any
	if err = json.Unmarshal(call(memberToken, "GET", "/api/v1/mail/messages/"+messages[0].ID, nil, 200), &incoming); err != nil {
		t.Fatal(err)
	}
	if incoming["reply_from"] != "support@alias.example" {
		t.Fatal("reply lost alias identity", incoming["reply_from"])
	}
	for _, value := range incoming["reply_all_to"].([]any) {
		if value == "support@alias.example" || value == member.Email || value == "member@alias.example" {
			t.Fatal("reply all includes own alias", value)
		}
	}
	call(adminToken, "POST", "/api/v1/mail/send", map[string]any{"to": []string{"missing@alias.example"}, "text": "Unknown"}, 400)

	addresses := call(memberToken, "GET", "/api/v1/mail/senders", nil, 200)
	for _, address := range []string{member.Email, "member@alias.example", "support@example.com", "support@alias.example", "cross@other.example"} {
		if !bytes.Contains(addresses, []byte(address)) {
			t.Fatal("missing sender", address, string(addresses))
		}
	}
	call(memberToken, "POST", "/api/v1/mail/send", map[string]any{"from": "support@alias.example", "to": []string{"manager@example.com", "external@outside.example"}, "text": "From alias", "subject": "Send as"}, 200)
	queue, err := st.ListOutbound(ctx, 10)
	if err != nil || len(queue) != 1 || queue[0].EnvelopeFrom != "support@alias.example" {
		t.Fatal("alias envelope", queue, err)
	}
	queuedMessage, err := mail.ReadMessage(bytes.NewReader(queue[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	queuedFrom, err := queuedMessage.Header.AddressList("From")
	if err != nil || len(queuedFrom) != 1 || queuedFrom[0].Address != "support@alias.example" {
		t.Fatal("alias From", queuedFrom, err)
	}
	targetBox, err := st.GetMailbox(ctx, manager.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	received, err := st.ListMessages(ctx, targetBox.ID)
	if err != nil || len(received) != 1 || !bytes.Contains([]byte(received[0].FromAddr), []byte("support@alias.example")) {
		t.Fatal("alias From", received, err)
	}
	message := call(managerToken, "GET", "/api/v1/mail/messages/"+received[0].ID, nil, 200)
	if !bytes.Contains(message, []byte("support@alias.example")) {
		t.Fatal("reply sender", string(message))
	}
	for _, address := range []string{"admin@example.com", "forged@outside.example", "user@foreign.example"} {
		call(memberToken, "POST", "/api/v1/mail/send", map[string]any{"from": address, "to": []string{"external@outside.example"}, "text": "forged"}, 403)
	}
	call(memberToken, "POST", "/api/v1/mail/send", map[string]any{"from": "member@example.com\r\nBcc: attacker@outside.example", "to": []string{member.Email}, "text": "forged"}, 400)
	call(adminToken, "PATCH", domains+"/"+d.ID, map[string]string{"name": "renamed.example"}, 200)
	if binding, err := storage.ReadDKIMBinding(ctx, st, "alias.example"); err != nil || binding != nil {
		t.Fatal("renamed alias retained signing grant", binding, err)
	}

	call(adminToken, "GET", domains+"/extra/path", nil, 404)
	call(adminToken, "DELETE", users+"/"+a.ID, nil, 200)
	call(adminToken, "DELETE", users+"/"+a.ID, nil, 404)
	call(memberToken, "POST", "/api/v1/mail/send", map[string]any{"from": "support@example.com", "to": []string{member.Email}, "text": "revoked"}, 403)

	call(adminToken, "DELETE", domains+"/"+d.ID, nil, 200)
	if _, err = st.ResolveRecipient(ctx, "member@renamed.example"); err != storage.ErrNotFound {
		t.Fatal(err)
	}
}
