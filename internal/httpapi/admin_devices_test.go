package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestDeviceRetirementAdministrationIsScopedAndSeparateFromWipe(t *testing.T) {
	cfg := config.Default()
	cfg.FlowSync.Enabled = false
	cfg.HTTP.Admins = nil
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "devices.db")
	ctx := t.Context()
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant, _ := store.CreateTenant(ctx, "devices")
	one, _ := store.CreateDomain(ctx, tenant.ID, "one.test")
	two, _ := store.CreateDomain(ctx, tenant.ID, "two.test")
	admin, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: one.ID, Email: "admin@one.test", LocalPart: "admin", Enabled: true, Roles: storage.RoleDomainAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetDomainAdminDomains(ctx, admin.ID, []string{one.ID}); err != nil {
		t.Fatal(err)
	}
	foreign, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: two.ID, Email: "user@two.test", LocalPart: "user", Enabled: true, Roles: storage.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	ownDevice, _ := store.EnsureFlowSyncDevice(ctx, admin.ID, "old-android", "Android")
	otherDevice, _ := store.EnsureFlowSyncDevice(ctx, foreign.ID, "other-android", "Android")
	layer := auth.NewLayer(store, cfg.LDAP, cfg.MFA, cfg.OIDC)
	pair, err := layer.Tokens.IssueTokens(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	server := New(cfg, nil, store, layer, nil, nil, nil).Handler()
	request := func(method, path, body string, want int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: %d want %d: %s", path, w.Code, want, w.Body)
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := request("GET", "/api/v1/admin/devices", "", 200); len(got["devices"].([]any)) != 1 {
		t.Fatal("foreign device visible")
	}
	request("POST", "/api/v1/admin/devices/"+otherDevice.ID, `{"action":"retire","confirm_device_id":"other-android"}`, 404)
	request("POST", "/api/v1/admin/devices/"+ownDevice.ID, `{"action":"retire"}`, 400)
	result := request("POST", "/api/v1/admin/devices/"+ownDevice.ID, `{"action":"retire","confirm_device_id":"old-android"}`, 200)
	if result["remote_wipe_sent"] != false || result["blocked"] != true {
		t.Fatal("retirement sent wipe or left access")
	}
	if got := request("GET", "/api/v1/admin/devices", "", 200); len(got["devices"].([]any)) != 0 {
		t.Fatal("retired device remains in current list")
	}
	if got := request("GET", "/api/v1/admin/devices?include_retired=true", "", 200); len(got["devices"].([]any)) != 1 {
		t.Fatal("retired deny record missing")
	}
	request("POST", "/api/v1/admin/devices/"+ownDevice.ID, `{"action":"wipe_account","confirm_device_id":"old-android"}`, 409)
	request("POST", "/api/v1/admin/devices/"+ownDevice.ID, `{"action":"restore"}`, 400)
	restored := request("POST", "/api/v1/admin/devices/"+ownDevice.ID, `{"action":"restore","confirm_device_id":"old-android"}`, 200)
	if restored["blocked"] != true || restored["requires_reprovision"] != true {
		t.Fatal("restoration granted access")
	}
	request("POST", "/api/v1/admin/devices/"+ownDevice.ID, `{"action":"unblock"}`, 200)

}
