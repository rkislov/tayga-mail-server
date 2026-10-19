package storage

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
)

// Opt in with a disposable PostgreSQL URL. Each fixture has its own schema;
// no existing application tables are migrated or removed.
func bookingPostgresConfig(t *testing.T) *config.StorageConfig {
	t.Helper()
	raw := os.Getenv("TAYGA_TEST_BOOKING_POSTGRES_DSN")
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("test PostgreSQL DSN must be a URL")
	}
	db, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal(err)
	}
	schema := "calendar_test_" + strings.ReplaceAll(NewID(), "-", "")
	if _, err = db.ExecContext(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer db.Close()
		if _, err := db.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return &config.StorageConfig{Driver: "postgres", Postgres: config.PostgresConfig{DSN: u.String()}}
}

// All identifiers/conditions are fixed by these tests, never user input.
func failCalendarWrite(t *testing.T, s *Store, table, operation, condition string) func() {
	t.Helper()
	q := `CREATE TRIGGER fail_calendar_write BEFORE ` + operation + ` ON ` + table
	if s.dialect == DialectPostgres {
		if _, err := s.db.Exec(`CREATE FUNCTION fail_calendar_write_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected calendar failure'; END $$`); err != nil {
			t.Fatal(err)
		}
		q += ` FOR EACH ROW`
	}
	if condition != "" {
		q += ` WHEN (` + condition + `)`
	}
	if s.dialect == DialectPostgres {
		q += ` EXECUTE FUNCTION fail_calendar_write_fn()`
	} else {
		q += ` BEGIN SELECT RAISE(ABORT,'injected calendar failure'); END`
	}
	if _, err := s.db.Exec(q); err != nil {
		t.Fatal(err)
	}
	return func() {
		q := `DROP TRIGGER fail_calendar_write`
		if s.dialect == DialectPostgres {
			q += ` ON ` + table
		}
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
		if s.dialect == DialectPostgres {
			if _, err := s.db.Exec(`DROP FUNCTION fail_calendar_write_fn()`); err != nil {
				t.Fatal(err)
			}
		}
	}
}
