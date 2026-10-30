package climenu

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
	"gopkg.in/yaml.v3"
)

func TestOfficeMenuPersistsAndPreservesSecret(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Storage.SQLite.Path = filepath.Join(dir, "settings.db")
	cfgPath := filepath.Join(dir, "config.yaml")
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(cfgPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	m := &Menu{ui: &UI{out: output}, cfgPath: cfgPath}
	secret := "menu-office-secret-at-least-32-bytes"
	for _, tc := range []struct{ input, provider, url string }{
		{"7\n3\nhttps://office.example.com\n" + secret + "\ny\n", "onlyoffice", "https://office.example.com"},
		{"7\n4\nhttps://r7.example.com\n\ny\n", "r7", "https://r7.example.com"},
		{"7\n2\nhttps://collabora.example.com\ny\n", "collabora", "https://collabora.example.com"},
		{"7\n1\ny\n", "collabora", ""},
		{"7\n3\nhttps://unsaved.example.com\n\nn\n", "collabora", ""},
	} {
		m.ui.in = bufio.NewReader(strings.NewReader(tc.input))
		if err = m.editSettingsSection(); err != nil {
			t.Fatal(err)
		}
		if err = m.withHub(func(_ context.Context, _ *settings.Hub, current *config.Config, _ storage.Driver) error {
			if current.Files.OfficeProvider != tc.provider || current.Files.OfficeURL != tc.url || current.Files.OfficeJWTSecret != secret {
				t.Fatal("office settings were not saved or secret was lost")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	log, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), secret) {
		t.Fatal("menu output leaked JWT secret")
	}
}
