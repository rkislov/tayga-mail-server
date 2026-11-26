package mmplugins

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DatabaseBindings is a private operator-managed map, never plugin configuration.
// A scope's modules may share a database, with distinct non-inheriting login
// roles and schema privileges. The legacy per-module format is still accepted.
type DatabaseBindings struct{ entries map[string]string }
type databaseBinding struct {
	Scope    string `json:"scope"`
	PluginID string `json:"plugin_id"`
	DSN      string `json:"dsn"`
}

func ProductDatabaseName(scope, id string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + id))
	return "tmsplug_" + hex.EncodeToString(sum[:12])
}

func SharedProductDatabaseName(scope string) string {
	sum := sha256.Sum256([]byte(scope))
	return "tmsprod_" + hex.EncodeToString(sum[:12])
}

// ProductDatabaseSchema is empty for a legacy database containing one module.
func ProductDatabaseSchema(database, scope, id string) string {
	if database != SharedProductDatabaseName(scope) {
		return ""
	}
	if id == "focalboard" {
		return "boards"
	}
	return "public"
}

func LoadDatabaseBindings(directory string) (*DatabaseBindings, error) {
	path := filepath.Join(directory, ".databases.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return &DatabaseBindings{entries: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1<<20 {
		return nil, errors.New("plugin database bindings must be a private regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []databaseBinding
	if err = json.Unmarshal(data, &rows); err != nil {
		return nil, errors.New("invalid plugin database bindings JSON")
	}
	bindings := &DatabaseBindings{entries: map[string]string{}}
	for _, row := range rows {
		if !regexp.MustCompile(`^domain:[A-Za-z0-9_-]{1,128}:[A-Za-z0-9_-]{1,128}$`).MatchString(row.Scope) || (row.PluginID != "playbooks" && row.PluginID != "focalboard") {
			return nil, errors.New("invalid plugin database scope")
		}
		parsed, e := url.Parse(row.DSN)
		if e != nil || parsed.Scheme != "postgres" || parsed.User == nil {
			return nil, errors.New("plugin database DSN must be PostgreSQL URL")
		}
		for key, values := range parsed.Query() {
			switch key {
			case "sslmode", "sslrootcert", "sslcert", "sslkey", "connect_timeout", "application_name":
				if len(values) != 1 {
					return nil, errors.New("duplicate plugin database connection option")
				}
			default:
				return nil, errors.New("unsupported plugin database connection option")
			}
		}
		expected := ProductDatabaseName(row.Scope, row.PluginID)
		database := strings.TrimPrefix(parsed.Path, "/")
		if (database != expected && database != SharedProductDatabaseName(row.Scope)) || parsed.User.Username() != expected {
			return nil, errors.New("plugin database and login must match their scope binding")
		}
		if parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" && parsed.Query().Get("sslmode") != "verify-full" {
			return nil, errors.New("remote plugin database requires verified TLS")
		}
		key := row.Scope + "\x00" + row.PluginID
		if _, ok := bindings.entries[key]; ok {
			return nil, errors.New("duplicate plugin database binding")
		}
		bindings.entries[key] = row.DSN
	}
	return bindings, nil
}
func (b *DatabaseBindings) DataSource(scope, id string) string {
	if b == nil {
		return ""
	}
	return b.entries[scope+"\x00"+id]
}
func (b *DatabaseBindings) Configure(options *Options) {
	scope := options.Scope
	options.SQLDriverName = "postgres"
	options.SQLDataSource = func(id string) string { return b.DataSource(scope, id) }
	options.SQLDatabase = func(id string) (*sql.DB, error) {
		dsn := b.DataSource(scope, id)
		if dsn == "" {
			return nil, errors.New("dedicated plugin database is not provisioned for this scope")
		}
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, errors.New("plugin database configuration invalid")
		}
		db.SetMaxOpenConns(16)
		db.SetMaxIdleConns(4)
		ctx, cancel := sqlTimeout()
		defer cancel()
		parsed, _ := url.Parse(dsn) // LoadDatabaseBindings has validated this URL.
		expected := strings.TrimPrefix(parsed.Path, "/")
		schema := ProductDatabaseSchema(expected, scope, id)
		var database, user, currentSchema, searchPath string
		var privileged, crossedSchema bool
		err = db.QueryRowContext(ctx, `SELECT current_database(),current_user,coalesce(current_schema(),''),current_setting('search_path'),
			rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=pg_roles.oid),
			CASE WHEN $1 = '' THEN false ELSE has_database_privilege(current_database(),'CREATE') OR
			EXISTS(SELECT 1 FROM pg_namespace WHERE nspname IN ('public','boards') AND nspname <> $1
				AND (has_schema_privilege(oid,'USAGE') OR has_schema_privilege(oid,'CREATE'))) END
			FROM pg_roles WHERE rolname=current_user`, schema).Scan(&database, &user, &currentSchema, &searchPath, &privileged, &crossedSchema)
		if err != nil || privileged || crossedSchema || database != expected || user != ProductDatabaseName(scope, id) ||
			(schema != "" && (currentSchema != schema || searchPath != schema)) {
			db.Close()
			return nil, errors.New("plugin database role validation failed")
		}
		if err = prepareProductMetadata(ctx, db, *options); err != nil {
			db.Close()
			return nil, fmt.Errorf("prepare scoped plugin metadata: %w", err)
		}
		return db, nil
	}
	options.RefreshSQL = func(db *sql.DB) error {
		ctx, cancel := sqlTimeout()
		defer cancel()
		return syncProductMetadata(ctx, db, *options)
	}
}
