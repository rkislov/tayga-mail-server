package climenu

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
	"gopkg.in/yaml.v3"
)

func TestMattermostMenuScopeAndInheritance(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(dir, "menu.db")
	path := filepath.Join(dir, "config.yaml")
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	menu := &Menu{cfgPath: path, ui: &UI{out: output}}
	var tenantID, domainID string
	if err = menu.withHub(func(ctx context.Context, _ *settings.Hub, _ *config.Config, store storage.Driver) error {
		tenant, e := store.CreateTenant(ctx, "Tenant")
		if e != nil {
			return e
		}
		domain, e := store.CreateDomain(ctx, tenant.ID, "example.test")
		if e != nil {
			return e
		}
		tenantID, domainID = tenant.ID, domain.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Settings 8 -> Mattermost 4 -> tenant 1 -> domain 1 -> access enabled,
	// plugins inherited, name inherited, plugin JSON unchanged, save.
	menu.ui.in = bufio.NewReader(strings.NewReader("8\n4\n1\n1\n2\n1\ny\n\ny\n"))
	if err = menu.editSettingsSection(); err != nil {
		t.Fatal(err)
	}
	if err = menu.withHub(func(ctx context.Context, _ *settings.Hub, _ *config.Config, store storage.Driver) error {
		service := &mmsettings.Service{Store: store}
		own, e := service.Read(ctx, mmsettings.Scope{TenantID: tenantID, DomainID: domainID})
		if e != nil {
			return e
		}
		if own.Enabled == nil || !*own.Enabled || own.PluginsEnabled != nil {
			t.Fatal("wrong scope or inheritance", own)
		}
		parent, e := service.Read(ctx, mmsettings.Scope{TenantID: tenantID})
		if e != nil {
			return e
		}
		if parent.Enabled != nil {
			t.Fatal("parent was modified")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
