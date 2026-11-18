package mmsettings

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestTenantDomainInheritanceAndSecretIsolation(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(t.TempDir(), "policy.db")
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant, _ := store.CreateTenant(ctx, "A")
	other, _ := store.CreateTenant(ctx, "B")
	first, _ := store.CreateDomain(ctx, tenant.ID, "a.test")
	second, _ := store.CreateDomain(ctx, tenant.ID, "b.test")
	foreign, _ := store.CreateDomain(ctx, other.ID, "c.test")
	service := &Service{Store: store, Defaults: Defaults{Enabled: true, SiteName: "Tayga"}}
	on, off := true, false
	tenantName, domainName := "Tenant A", "Domain A"
	if err = service.Save(ctx, Scope{}, Overrides{PluginsEnabled: &on}); err != nil {
		t.Fatal(err)
	}
	if err = service.Save(ctx, Scope{TenantID: tenant.ID}, Overrides{SiteName: &tenantName, Plugins: map[string]Plugin{"gitlab": {Enabled: &on, Configuration: map[string]any{"URL": "https://gitlab.a.test", "ClientSecret": "tenant-secret", "nested": map[string]any{"ApiToken": "nested-secret"}}}}}); err != nil {
		t.Fatal(err)
	}
	scope := Scope{tenant.ID, first.ID}
	if err = service.Save(ctx, scope, Overrides{SiteName: &domainName, Enabled: &off, Plugins: map[string]Plugin{"gitlab": {Configuration: map[string]any{"ClientSecret": "domain-secret"}}}}); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.Resolve(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Enabled || !resolved.PluginsEnabled || resolved.SiteName != domainName || resolved.Plugins["gitlab"].Configuration["URL"] != "https://gitlab.a.test" || resolved.Plugins["gitlab"].Configuration["ClientSecret"] != "domain-secret" {
		t.Fatal(resolved)
	}
	sibling, err := service.Resolve(ctx, Scope{tenant.ID, second.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !sibling.Enabled || sibling.SiteName != tenantName || sibling.Plugins["gitlab"].Configuration["ClientSecret"] != "tenant-secret" {
		t.Fatal(sibling)
	}
	foreignPolicy, err := service.Resolve(ctx, Scope{other.ID, foreign.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(foreignPolicy.Plugins) != 0 || foreignPolicy.SiteName != "Tayga" {
		t.Fatal("tenant settings leaked", foreignPolicy)
	}
	if err = service.Validate(ctx, Scope{other.ID, first.ID}); err == nil {
		t.Fatal("foreign domain accepted")
	}
	safe, _ := json.Marshal(Redact(resolved))
	if strings.Contains(string(safe), "domain-secret") || strings.Contains(string(safe), "nested-secret") {
		t.Fatalf("secret exposed: %s", safe)
	}
	own, err := service.Read(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(Redact(own))
	var masked Overrides
	if err = json.Unmarshal(raw, &masked); err != nil {
		t.Fatal(err)
	}
	if err = service.Save(ctx, scope, masked); err != nil {
		t.Fatal(err)
	}
	after, _ := service.Resolve(ctx, scope)
	if after.Plugins["gitlab"].Configuration["ClientSecret"] != "domain-secret" {
		t.Fatal("masked secret was overwritten")
	}
	if err = service.Save(ctx, scope, Overrides{}); err != nil {
		t.Fatal(err)
	}
	after, _ = service.Resolve(ctx, scope)
	if !after.Enabled || after.SiteName != tenantName {
		t.Fatal("inheritance reset failed", after)
	}
	// Saving a masked inherited value must not copy a parent's secret into a child.
	if err = service.Save(ctx, scope, Overrides{Plugins: map[string]Plugin{"gitlab": {Configuration: map[string]any{"ClientSecret": "***"}}}}); err != nil {
		t.Fatal(err)
	}
	own, _ = service.Read(ctx, scope)
	if _, ok := own.Plugins["gitlab"].Configuration["ClientSecret"]; ok {
		t.Fatal("inherited secret copied into child")
	}
}
