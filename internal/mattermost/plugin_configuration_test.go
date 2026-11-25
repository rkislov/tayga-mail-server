package mattermost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/mmsettings"
)

func TestPluginSettingsPatchOnlyManagedDomain(t *testing.T) {
	f := setup(t)
	dir := t.TempDir()
	const plugin = "test-monitor"
	if err := os.MkdirAll(filepath.Join(dir, plugin), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, plugin, "plugin.json"), []byte(`{"id":"test-monitor","name":"Monitoring","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.api.EnablePlugins(dir, []string{f.users[0].Email}); err != nil {
		t.Fatal(err)
	}
	defer f.api.Close()
	scope := userScope(f.users[0])
	path := "/plugins/" + plugin + "/configuration"
	f.call(t, "PATCH", path, f.tokens[1], map[string]any{"set": map[string]string{"URL": "https://example.test"}}, 403)
	f.call(t, "PATCH", path, f.tokens[0], map[string]any{"set": map[string]string{"URL": "https://domain.test", "CredentialValue": "scope-secret"}}, 200)
	f.call(t, "PATCH", path, f.tokens[0], map[string]any{"set": map[string]string{"CredentialValue": "***"}, "inherit": []string{"URL"}}, 200)
	saved, err := f.api.policy.Read(t.Context(), scope)
	if err != nil {
		t.Fatal(err)
	}
	fields := saved.Plugins[plugin].Configuration
	if len(fields) != 1 || fields["CredentialValue"] != "scope-secret" {
		t.Fatal("masked value or field inheritance was lost")
	}
	safe := mmsettings.RedactPluginFields(saved, map[string]map[string]bool{plugin: {"credentialvalue": true}})
	if strings.Contains(safe.(map[string]any)["plugins"].(map[string]any)[plugin].(map[string]any)["configuration"].(map[string]any)["CredentialValue"].(string), "scope-secret") {
		t.Fatal("manifest secret was exposed")
	}
	sibling, err := f.api.policy.Read(t.Context(), userScope(f.users[3]))
	if err != nil || len(sibling.Plugins) != 0 {
		t.Fatal("settings crossed the domain", err)
	}
}
