package xmpp

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestRoomArchivePostgres(t *testing.T) {
	raw := os.Getenv("TAYGA_TEST_ROOM_POSTGRES_DSN")
	if raw == "" {
		t.Skip("set a disposable PostgreSQL DSN")
	}
	target, err := url.Parse(raw)
	if err != nil || (target.Scheme != "postgres" && target.Scheme != "postgresql") {
		t.Fatal("invalid fixture DSN")
	}
	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	schema := "room_test_" + strings.ReplaceAll(storage.NewID(), "-", "")
	if _, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	q := target.Query()
	q.Set("search_path", schema)
	target.RawQuery = q.Encode()
	cfg := &config.Config{Server: config.ServerConfig{Hostname: "example.com"}, Storage: config.StorageConfig{Driver: "postgres", Postgres: config.PostgresConfig{DSN: target.String()}}}
	st, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	tenant, err := st.CreateTenant(ctx, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := st.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "alice@example.com", LocalPart: "alice", AuthSource: "local", Enabled: true, PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	server := New(cfg, nil, st, nil, nil)
	testRoomArchive(t, server, st.(*storage.Store), user)
	testGroupOMEMO(t, server, st.(*storage.Store), user)
}
