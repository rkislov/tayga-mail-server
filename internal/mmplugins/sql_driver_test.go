package mmplugins

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin"
	rpcdriver "github.com/mattermost/mattermost/server/public/shared/driver"
	_ "modernc.org/sqlite"
)

type boundSQLDriver struct {
	plugin.Driver
	bridge *sqlBridge
	id     string
}

func (b boundSQLDriver) Conn(master bool) (string, error) {
	return b.bridge.ConnWithPluginID(master, b.id)
}

func TestPluginSQLBridgeIsolationAndLifecycle(t *testing.T) {
	root := t.TempDir()
	bridge := newSQLBridge(func(id string) (*sql.DB, error) { return sql.Open("sqlite", filepath.Join(root, id+".db")) })
	defer bridge.Close()
	open := func(id string) *sql.DB {
		db := sql.OpenDB(rpcdriver.NewConnector(boundSQLDriver{bridge, bridge, id}, true))
		t.Cleanup(func() { db.Close() })
		return db
	}
	a, b := open("boards"), open("playbooks")
	for _, db := range []*sql.DB{a, b} {
		if _, e := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT)"); e != nil {
			t.Fatal(e)
		}
	}
	tx, e := a.Begin()
	if e != nil {
		t.Fatal(e)
	}
	stmt, e := tx.Prepare("INSERT INTO items(value) VALUES(?)")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = stmt.Exec("private board"); e != nil {
		t.Fatal(e)
	}
	stmt.Close()
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var value string
	if e = a.QueryRow("SELECT value FROM items").Scan(&value); e != nil || value != "private board" {
		t.Fatal(value, e)
	}
	var count int
	if e = b.QueryRow("SELECT count(*) FROM items").Scan(&count); e != nil || count != 0 {
		t.Fatal("cross-plugin data", count, e)
	}
	tx, e = a.BeginTx(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec("INSERT INTO items(value) VALUES(?)", "rolled back"); e != nil {
		t.Fatal(e)
	}
	tx.Rollback()
	if e = a.QueryRow("SELECT count(*) FROM items").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	if _, e = bridge.Conn(true); e == nil {
		t.Fatal("unbound SQL connection granted")
	}
	if _, e = bridge.ConnQuery("foreign-handle", "SELECT 1", []driver.NamedValue{}); e == nil {
		t.Fatal("unknown handle accepted")
	}
	bridge.ShutdownConns("boards")
	for _, conn := range bridge.conns {
		if conn.owner == "boards" {
			t.Fatal("deactivated plugin connection survived")
		}
	}
	if e = b.QueryRow("SELECT count(*) FROM items").Scan(&count); e != nil {
		t.Fatal("unrelated plugin connection closed", e)
	}
}
