package full

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
	"gopkg.in/yaml.v3"
)

func openDB(name string, readonly bool) (*sql.DB, error) {
	if info, err := os.Lstat(name); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("database must be an existing regular file: %s", name)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(name)}
	q := url.Values{"mode": {"rw"}}
	if readonly {
		q.Set("mode", "ro")
		q.Set("immutable", "1")
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func checkDB(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errors.New("SQLite quick_check failed")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return errors.New("SQLite foreign_key_check failed")
	}
	return rows.Err()
}

func countTables(ctx context.Context, db *sql.DB) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(names))
	for _, name := range names {
		var count int64
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM "`+strings.ReplaceAll(name, `"`, `""`)+`"`).Scan(&count); err != nil {
			return nil, err
		}
		counts[name] = count
	}
	return counts, nil
}

func verifyRestored(ctx context.Context, root string, m *Manifest) error {
	db, err := openDB(filepath.Join(root, m.Header.Database), true)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = checkDB(ctx, db); err != nil {
		return err
	}
	counts, err := countTables(ctx, db)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(counts, m.Counts) {
		return errors.New("database row counts differ from snapshot")
	}
	// Configuration may reference the original absolute root. Rebase only in
	// memory for validation; the verified archive remains byte-for-byte intact.
	cfg, err := config.Load(filepath.Join(root, m.Header.Config))
	if err != nil {
		return err
	}
	st := storage.NewSQLStore(db, storage.DialectSQLite)
	hub := settings.NewHub(st, cfg)
	if err = hub.Load(ctx); err != nil {
		return err
	}
	cfg = hub.Config()
	rebaseConfig(cfg, m.Header.Root, root)
	// Domain signing bindings also contain absolute paths. Validate a temporary
	// facade, without modifying the restored DB before checksum acceptance.
	if cfg.Storage.SQLite.Path != filepath.Join(root, m.Header.Database) {
		return errors.New("config/database manifest mismatch")
	}
	return validateLayout(ctx, root, m.Header.Root, cfg, st)
}

func rebase(value, from, to string) string {
	if value == from {
		return to
	}
	if strings.HasPrefix(value, from+string(filepath.Separator)) {
		return to + strings.TrimPrefix(value, from)
	}
	return value
}

func rebaseConfig(c *config.Config, from, to string) {
	for _, p := range []*string{&c.Storage.SQLite.Path, &c.Mailstore.Root, &c.TLS.CertFile, &c.TLS.KeyFile, &c.TLS.CertsDir, &c.SMTP.DKIM.PrivateKeyFile, &c.ARC.PrivateKeyFile, &c.Log.File} {
		*p = rebase(*p, from, to)
	}
}

func rebaseJSON(v any, from, to string) any {
	switch v := v.(type) {
	case string:
		return rebase(v, from, to)
	case []any:
		for i := range v {
			v[i] = rebaseJSON(v[i], from, to)
		}
		return v
	case map[string]any:
		for k, value := range v {
			v[k] = rebaseJSON(value, from, to)
		}
		return v
	default:
		return v
	}
}

func relocate(ctx context.Context, stage, target string, m *Manifest) error {
	cfgFile := filepath.Join(stage, m.Header.Config)
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return err
	}
	rebaseConfig(cfg, m.Header.Root, target)
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err = os.WriteFile(cfgFile, b, 0600); err != nil {
		return err
	}
	db, err := openDB(filepath.Join(stage, m.Header.Database), false)
	if err != nil {
		return err
	}
	defer db.Close()
	st := storage.NewSQLStore(db, storage.DialectSQLite)
	values, err := st.ListSettings(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, raw := range values {
		var value any
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			continue
		}
		value = rebaseJSON(value, m.Header.Root, target)
		rebased, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE settings SET value=? WHERE key=?", string(rebased), key); err != nil {
			return err
		}
	}
	for _, column := range [][2]string{{"mailboxes", "path"}, {"messages", "file_path"}, {"calendar_attachments", "storage_path"}, {"note_attachments", "storage_path"}} {
		query := fmt.Sprintf("UPDATE %s SET %s=? || substr(%s, ?) WHERE substr(%s, 1, ?)=?", column[0], column[1], column[1], column[1])
		prefix := m.Header.Root + string(filepath.Separator)
		if _, err = tx.ExecContext(ctx, query, target+string(filepath.Separator), len([]rune(prefix))+1, len([]rune(prefix)), prefix); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	var busy, pages, checkpointed int
	if err = db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &pages, &checkpointed); err != nil || busy != 0 {
		return fmt.Errorf("restore checkpoint failed: %v", err)
	}
	return checkDB(ctx, db)
}
