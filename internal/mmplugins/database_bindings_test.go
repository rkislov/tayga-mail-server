package mmplugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateProductDatabaseBindings(t *testing.T) {
	dir := t.TempDir()
	scope := "domain:tenant:domain"
	name := ProductDatabaseName(scope, "playbooks")
	path := filepath.Join(dir, ".databases.json")
	good := []databaseBinding{{Scope: scope, PluginID: "playbooks", DSN: "postgres://" + name + ":fixture@127.0.0.1:5432/" + name + "?sslmode=disable"}}
	save := func(rows []databaseBinding, mode os.FileMode) {
		raw, _ := json.Marshal(rows)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	save(good, 0600)
	bindings, err := LoadDatabaseBindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if bindings.DataSource(scope, "playbooks") == "" || bindings.DataSource("domain:tenant:sibling", "playbooks") != "" || bindings.DataSource(scope, "focalboard") != "" {
		t.Fatal("binding escaped its scope/plugin")
	}
	save(good, 0644)
	if _, err := LoadDatabaseBindings(dir); err == nil {
		t.Fatal("public credential file accepted")
	}
	bad := append([]databaseBinding(nil), good...)
	bad[0].DSN = "postgres://postgres:fixture@localhost:5432/tayga"
	save(bad, 0600)
	if _, err := LoadDatabaseBindings(dir); err == nil {
		t.Fatal("mail database/admin role accepted")
	}
	bad[0].DSN = "postgres://" + name + ":fixture@remote.example:5432/" + name + "?sslmode=disable"
	save(bad, 0600)
	if _, err := LoadDatabaseBindings(dir); err == nil {
		t.Fatal("unverified remote TLS accepted")
	}
	bad[0].DSN = "postgres://" + name + ":fixture@127.0.0.1:5432/" + name + "?sslmode=disable&host=remote.example"
	save(bad, 0600)
	if _, err := LoadDatabaseBindings(dir); err == nil {
		t.Fatal("DSN host override bypassed TLS policy")
	}
	save(append(good, good...), 0600)
	if _, err := LoadDatabaseBindings(dir); err == nil {
		t.Fatal("duplicate binding accepted")
	}
}
