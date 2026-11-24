package full

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mmplugins"
)

// ProductBackup describes logical dumps captured while the installation's
// exclusive maintenance lock fences all TMS/plugin writers. Restoration is
// deliberately staged: credentials from a restored copy never contact the
// original PostgreSQL server automatically.
type ProductBackup struct {
	Bindings  string        `json:"bindings"`
	Directory string        `json:"directory"`
	Databases []ProductDump `json:"databases"`
}
type ProductDump struct {
	Scope    string `json:"scope"`
	PluginID string `json:"plugin_id"`
	File     string `json:"file"`
}
type productBinding struct {
	Scope    string `json:"scope"`
	PluginID string `json:"plugin_id"`
	DSN      string `json:"dsn"`
}

func snapshotProducts(ctx context.Context, root string, cfg *config.Config) (*ProductBackup, func(), error) {
	folder := cfg.Mattermost.PluginDirectory
	if folder == "" {
		folder = filepath.Join(cfg.Mailstore.Root, "mattermost-plugins")
	}
	path := filepath.Join(folder, ".databases.json")
	if _, err := os.Lstat(path + ".restore-pending"); err == nil {
		return nil, nil, errors.New("product database restoration is pending; refusing incomplete snapshot")
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil, func() {}, nil
	} else if err != nil {
		return nil, nil, err
	}
	if _, err := mmplugins.LoadDatabaseBindings(folder); err != nil {
		return nil, nil, err
	}
	rel, err := relative(root, path)
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var bindings []productBinding
	if err = json.Unmarshal(raw, &bindings); err != nil {
		return nil, nil, err
	}
	if len(bindings) == 0 {
		return nil, func() {}, nil
	}
	dumpDir, err := os.MkdirTemp(root, ".product-dumps-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dumpDir) }
	dirRel, _ := relative(root, dumpDir)
	result := &ProductBackup{Bindings: rel, Directory: dirRel}
	for _, b := range bindings {
		name := mmplugins.ProductDatabaseName(b.Scope, b.PluginID) + ".dump"
		cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--file="+filepath.Join(dumpDir, name))
		// No credentials in argv,
		// output, the manifest or error messages. Ignore inherited libpq overrides.
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "PG") {
				cmd.Env = append(cmd.Env, e)
			}
		}
		u, _ := url.Parse(b.DSN)
		port := u.Port()
		if port == "" {
			port = "5432"
		}
		password, _ := u.User.Password()
		cmd.Env = append(cmd.Env, "PGHOST="+u.Hostname(), "PGPORT="+port, "PGDATABASE="+strings.TrimPrefix(u.Path, "/"), "PGUSER="+u.User.Username(), "PGPASSWORD="+password, "PGCONNECT_TIMEOUT=10")
		for key, env := range map[string]string{"sslmode": "PGSSLMODE", "sslrootcert": "PGSSLROOTCERT", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY"} {
			if v := u.Query().Get(key); v != "" {
				cmd.Env = append(cmd.Env, env+"="+v)
			}
		}
		if err = cmd.Run(); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("PostgreSQL product dump failed for %s (matching pg_dump and database access required)", b.PluginID)
		}
		if err = os.Chmod(filepath.Join(dumpDir, name), 0600); err != nil {
			cleanup()
			return nil, nil, err
		}
		result.Databases = append(result.Databases, ProductDump{Scope: b.Scope, PluginID: b.PluginID, File: dirRel + "/" + name})
	}
	index, _ := json.Marshal(result)
	if err = os.WriteFile(filepath.Join(dumpDir, "restore.json"), index, 0600); err != nil {
		cleanup()
		return nil, nil, err
	}
	return result, cleanup, nil
}
func validateProductBackup(p *ProductBackup) error {
	if p == nil {
		return nil
	}
	if !safeName(p.Bindings) || !strings.HasSuffix(p.Bindings, "/.databases.json") || !safeName(p.Directory) || len(p.Databases) == 0 {
		return errors.New("invalid product backup layout")
	}
	seen := map[string]bool{}
	for _, d := range p.Databases {
		expected := p.Directory + "/" + mmplugins.ProductDatabaseName(d.Scope, d.PluginID) + ".dump"
		if !regexp.MustCompile(`^domain:[A-Za-z0-9_-]{1,128}:[A-Za-z0-9_-]{1,128}$`).MatchString(d.Scope) || (d.PluginID != "playbooks" && d.PluginID != "focalboard") || d.File != expected || !safeName(d.File) || seen[d.File] {
			return errors.New("invalid product database dump")
		}
		seen[d.File] = true
	}
	return nil
}
func verifyProductBackup(root string, m *Manifest) error {
	p := m.Header.Products
	for _, e := range m.Entries {
		if filepath.Base(e.Path) == ".databases.json" && (p == nil || e.Path != p.Bindings) {
			return errors.New("plugin database bindings without matching logical backup")
		}
	}
	if err := validateProductBackup(p); err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(root, p.Directory, "restore.json"))
	if err != nil {
		return err
	}
	var index ProductBackup
	if err = json.Unmarshal(raw, &index); err != nil || !reflect.DeepEqual(&index, p) {
		return errors.New("product restore index mismatch")
	}
	for _, d := range p.Databases {
		f, err := os.Open(filepath.Join(root, d.File))
		if err != nil {
			return err
		}
		var signature [5]byte
		n, err := f.Read(signature[:])
		f.Close()
		if err != nil || n != 5 || string(signature[:]) != "PGDMP" {
			return errors.New("invalid PostgreSQL custom dump signature")
		}
	}
	if _, err := mmplugins.LoadDatabaseBindings(filepath.Dir(filepath.Join(root, p.Bindings))); err != nil {
		return err
	}
	return nil
}
