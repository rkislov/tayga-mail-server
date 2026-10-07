package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage/migrations"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Open creates a Driver for the configured backend and runs migrations.
func Open(ctx context.Context, cfg config.StorageConfig) (Driver, error) {
	var (
		db      *sql.DB
		dialect Dialect
		err     error
	)

	switch cfg.Driver {
	case "sqlite":
		db, err = sql.Open("sqlite", cfg.SQLite.Path)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}
		if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("enable foreign_keys: %w", err)
		}
		db.SetMaxOpenConns(1)
		dialect = DialectSQLite
	case "postgres":
		db, err = sql.Open("postgres", cfg.Postgres.DSN)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		dialect = DialectPostgres
	default:
		return nil, fmt.Errorf("unknown storage driver %q", cfg.Driver)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(ctx, db, dialect); err != nil {
		_ = db.Close()
		return nil, err
	}
	return NewSQLStore(db, dialect), nil
}

func migrate(ctx context.Context, db *sql.DB, dialect Dialect) error {
	var files []string
	switch dialect {
	case DialectSQLite:
		files = []string{
			"001_init.sqlite.sql",
			"002_mfa_oauth.sqlite.sql",
			"003_dav.sqlite.sql",
			"004_flowsync.sqlite.sql",
			"005_webauthn.sqlite.sql",
			"006_outbound.sqlite.sql",
			"007_sticky.sqlite.sql",
			"008_user_roles.sqlite.sql",
			"009_dmarc_reports.sqlite.sql",
			"010_greylist.sqlite.sql",
		}
	case DialectPostgres:
		files = []string{
			"001_init.postgres.sql",
			"002_mfa_oauth.postgres.sql",
			"003_dav.postgres.sql",
			"004_flowsync.postgres.sql",
			"005_webauthn.postgres.sql",
			"006_outbound.postgres.sql",
			"007_sticky.postgres.sql",
			"008_user_roles.postgres.sql",
			"009_dmarc_reports.postgres.sql",
			"010_greylist.postgres.sql",
		}
	default:
		return fmt.Errorf("unsupported dialect")
	}
	for _, filename := range files {
		if err := applyMigration(ctx, db, dialect, filename); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, dialect Dialect, filename string) error {
	raw, err := migrations.FS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", filename, err)
	}
	up := extractGooseUp(string(raw))
	version := strings.TrimSuffix(filename, ".sql")

	applied, err := isApplied(ctx, db, dialect, version)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := execScript(ctx, tx, up); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}

	var insert string
	if dialect == DialectSQLite {
		insert = `INSERT OR IGNORE INTO schema_migrations(version) VALUES (?)`
	} else {
		insert = `INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING`
	}
	if _, err := tx.ExecContext(ctx, insert, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	return tx.Commit()
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func execScript(ctx context.Context, db execer, script string) error {
	for _, stmt := range splitSQL(script) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %q: %w", truncate(stmt, 80), err)
		}
	}
	return nil
}

func splitSQL(script string) []string {
	parts := strings.Split(script, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "--") {
			continue
		}
		// skip pure comment blocks
		lines := strings.Split(p, "\n")
		var useful []string
		for _, line := range lines {
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "--") {
				continue
			}
			useful = append(useful, line)
		}
		if len(useful) == 0 {
			continue
		}
		out = append(out, strings.Join(useful, "\n"))
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func isApplied(ctx context.Context, db *sql.DB, dialect Dialect, version string) (bool, error) {
	var q string
	if dialect == DialectSQLite {
		q = `SELECT 1 FROM schema_migrations WHERE version = ?`
	} else {
		q = `SELECT 1 FROM schema_migrations WHERE version = $1`
	}
	var one int
	err := db.QueryRowContext(ctx, q, version).Scan(&one)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	msg := err.Error()
	if strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") {
		return false, nil
	}
	return false, err
}

func extractGooseUp(src string) string {
	const upMarker = "-- +goose Up"
	const downMarker = "-- +goose Down"
	start := strings.Index(src, upMarker)
	if start < 0 {
		return src
	}
	start += len(upMarker)
	end := strings.Index(src[start:], downMarker)
	if end < 0 {
		return strings.TrimSpace(src[start:])
	}
	return strings.TrimSpace(src[start : start+end])
}
