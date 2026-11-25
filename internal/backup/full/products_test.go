package full

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tayga/tms/internal/mmplugins"
	"gopkg.in/yaml.v3"
)

func TestFullProductDumpQuarantinesRestoredConnections(t *testing.T) {
	dsn := os.Getenv("TMS_TEST_PLUGIN_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable localhost PostgreSQL and pg_dump")
	}
	u, err := url.Parse(dsn)
	must(t, err)
	if u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" {
		t.Fatal("localhost fixture only")
	}
	admin, err := sql.Open("postgres", dsn)
	must(t, err)
	defer admin.Close()
	f := newFullFixture(t)
	f.cfg.Mattermost.PluginDirectory = filepath.Join(f.root, "plugins")
	must(t, os.MkdirAll(f.cfg.Mattermost.PluginDirectory, 0700))
	raw, err := yaml.Marshal(f.cfg)
	must(t, err)
	must(t, os.WriteFile(f.cfgFile, raw, 0600))
	scope := "domain:backup:" + strings.ReplaceAll(uuid.NewString(), "-", "")
	name := mmplugins.ProductDatabaseName(scope, "playbooks")
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(`CREATE ROLE "` + name + `" LOGIN PASSWORD '` + password + `' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`)
	must(t, err)
	defer admin.Exec(`DROP ROLE "` + name + `"`)
	_, err = admin.Exec(`CREATE DATABASE "` + name + `" OWNER "` + name + `"`)
	must(t, err)
	defer admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`)
	u.User = url.UserPassword(name, password)
	u.Path = "/" + name
	product, err := sql.Open("postgres", u.String())
	must(t, err)
	_, err = product.Exec(`CREATE TABLE saved_checklist(id text primary key,title text); INSERT INTO saved_checklist VALUES('fixture','Checklist survives backup')`)
	must(t, err)
	must(t, product.Close())
	rows := []productBinding{{Scope: scope, PluginID: "playbooks", DSN: u.String()}}
	raw, err = json.Marshal(rows)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(f.cfg.Mattermost.PluginDirectory, ".databases.json"), raw, 0600))
	m, err := f.snapshot(t.Context())
	must(t, err)
	if m.Header.Format != productFormat || m.Header.Products == nil || len(m.Header.Products.Databases) != 1 {
		t.Fatal("product dump omitted")
	}
	if strings.Contains(string(mustJSON(t, m)), password) {
		t.Fatal("credential leaked to manifest")
	}
	matches, err := filepath.Glob(filepath.Join(f.root, ".product-dumps-*"))
	must(t, err)
	if len(matches) != 0 {
		t.Fatal("plaintext staging dump left behind")
	}
	target := filepath.Join(f.parent, "restored-products")
	_, err = Restore(context.Background(), RestoreOptions{Input: f.output, Identity: f.keyFile, Target: target})
	must(t, err)
	live := filepath.Join(target, m.Header.Products.Bindings)
	if _, err = os.Stat(live); !os.IsNotExist(err) {
		t.Fatal("restored installation can connect to source PostgreSQL")
	}
	if _, err = os.Stat(live + ".restore-pending"); err != nil {
		t.Fatal(err)
	}
	for _, d := range m.Header.Products.Databases {
		data, err := os.ReadFile(filepath.Join(target, d.File))
		must(t, err)
		if len(data) < 5 || string(data[:5]) != "PGDMP" {
			t.Fatal("custom dump not restored")
		}
	}
}
func mustJSON(t *testing.T, v any) []byte { t.Helper(); b, e := json.Marshal(v); must(t, e); return b }
func TestProductBackupRejectsUnsafeLayout(t *testing.T) {
	for _, p := range []*ProductBackup{{Bindings: "../.databases.json", Directory: "d", Databases: []ProductDump{{}}}, {Bindings: "plugins/.databases.json", Directory: "../d", Databases: []ProductDump{{}}}, {Bindings: "plugins/.databases.json", Directory: "d", Databases: []ProductDump{{File: "../outside"}}}} {
		if validateProductBackup(p) == nil {
			t.Fatal("unsafe product backup accepted")
		}
	}
}
