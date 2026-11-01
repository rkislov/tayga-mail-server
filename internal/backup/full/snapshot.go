// Package full implements encrypted offline snapshots of a self-contained
// SQLite installation. It intentionally refuses PostgreSQL, S3 and external
// paths: those require a separately coordinated database/object-store snapshot.
package full

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/maintenance"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/version"
)

const format = "tayga-full-sqlite-v1"
const maxManifest = 64 << 20

type Header struct {
	Format   string    `json:"format"`
	Version  string    `json:"version"`
	Created  time.Time `json:"created"`
	Root     string    `json:"root"`
	Config   string    `json:"config"`
	Database string    `json:"database"`
}

type Entry struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256,omitempty"`
	Directory bool   `json:"directory,omitempty"`
	Mode      int64  `json:"mode"`
}

type Manifest struct {
	Header  Header           `json:"header"`
	Entries []Entry          `json:"entries"`
	Counts  map[string]int64 `json:"counts"`
}

type Options struct{ Root, Config, Output, Recipient string }

func relative(root, name string) (string, error) {
	if !filepath.IsAbs(name) {
		return "", fmt.Errorf("full backup requires absolute paths: %s", name)
	}
	rel, err := filepath.Rel(root, filepath.Clean(name))
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", fmt.Errorf("path outside installation root: %s", name)
	}
	return filepath.ToSlash(rel), nil
}

func Snapshot(ctx context.Context, opts Options) (*Manifest, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	if canonicalRoot == string(filepath.Separator) {
		return nil, errors.New("installation root cannot be filesystem root")
	}
	configRel, err := relative(root, opts.Config)
	if err != nil {
		return nil, err
	}
	output, err := filepath.Abs(opts.Output)
	if err != nil {
		return nil, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return nil, err
	}
	output = filepath.Join(parent, filepath.Base(output))
	if output == canonicalRoot {
		return nil, errors.New("backup output must be outside installation root")
	}
	if _, err := relative(canonicalRoot, output); err == nil {
		return nil, errors.New("backup output must be outside installation root")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return nil, errors.New("output already exists or is inaccessible")
	}
	recipient, err := age.ParseX25519Recipient(opts.Recipient)
	if err != nil {
		return nil, fmt.Errorf("recipient: %w", err)
	}
	cfg, err := config.Load(opts.Config)
	if err != nil {
		return nil, err
	}
	if cfg.Storage.Driver != "sqlite" {
		return nil, errors.New("full snapshot currently requires local SQLite")
	}
	dbRel, err := relative(root, cfg.Storage.SQLite.Path)
	if err != nil {
		return nil, err
	}
	lock, err := maintenance.SQLite(cfg.Storage.SQLite.Path, true)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	// No storage.Open here: a backup must never run migrations on its source.
	db, err := openDB(cfg.Storage.SQLite.Path, false)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var busy, pages, checkpointed int
	if err = db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &pages, &checkpointed); err != nil || busy != 0 {
		return nil, fmt.Errorf("cannot checkpoint database (busy=%d): %v", busy, err)
	}
	// Also fence external SQL writers for the duration of the physical copy.
	if _, err = db.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, err
	}
	defer db.Exec("ROLLBACK")
	if err = checkDB(ctx, db); err != nil {
		return nil, err
	}
	st := storage.NewSQLStore(db, storage.DialectSQLite)
	hub := settings.NewHub(st, cfg)
	if err = hub.Load(ctx); err != nil {
		return nil, err
	}
	if err = validateLayout(ctx, root, root, hub.Config(), st); err != nil {
		return nil, err
	}
	counts, err := countTables(ctx, db)
	if err != nil {
		return nil, err
	}
	m := &Manifest{Header: Header{Format: format, Version: version.String(), Created: time.Now().UTC(), Root: root, Config: configRel, Database: dbRel}, Counts: counts}
	// No plaintext staging archive. Publish only after every stream is closed
	// successfully; a failed run removes its own incomplete ciphertext.
	f, err := os.CreateTemp(parent, ".tayga-full-*.partial")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	encrypted, err := age.Encrypt(f, recipient)
	if err != nil {
		return nil, err
	}
	gz := gzip.NewWriter(encrypted)
	tw := tar.NewWriter(gz)
	if err = writeJSON(tw, "header.json", &m.Header); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	err = fs.WalkDir(r.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." || name == dbRel+maintenance.LockSuffix || name == dbRel+"-wal" || name == dbRel+"-shm" || name == dbRel+"-journal" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("unsupported file type (including symlink): %s", name)
		}
		h := &tar.Header{Name: "data/" + name, Mode: int64(info.Mode().Perm()), ModTime: info.ModTime(), Typeflag: tar.TypeReg, Size: info.Size()}
		e := Entry{Path: name, Size: info.Size(), Directory: info.IsDir(), Mode: h.Mode}
		if info.IsDir() {
			h.Typeflag = tar.TypeDir
			h.Size = 0
			e.Size = 0
		}
		if err = tw.WriteHeader(h); err != nil {
			return err
		}
		if !info.IsDir() {
			f, err := r.Open(name)
			if err != nil {
				return err
			}
			hash := sha256.New()
			n, copyErr := io.Copy(io.MultiWriter(tw, hash), &contextReader{ctx: ctx, r: f})
			after, statErr := f.Stat()
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if statErr != nil {
				return statErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) || !os.SameFile(info, after) {
				return fmt.Errorf("file changed during offline backup: %s", name)
			}
			e.SHA256 = hex.EncodeToString(hash.Sum(nil))
		}
		m.Entries = append(m.Entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = writeJSON(tw, "manifest.json", m); err != nil {
		return nil, err
	}
	for _, closeStream := range []func() error{tw.Close, gz.Close, encrypted.Close, f.Sync, f.Close} {
		if err = closeStream(); err != nil {
			return nil, err
		}
	}
	// Link is an atomic no-replace publication on the same filesystem.
	if err = os.Link(f.Name(), output); err != nil {
		return nil, err
	}
	if err = syncPath(parent); err != nil {
		return nil, err
	}
	return m, nil
}

func writeJSON(tw *tar.Writer, name string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(b) > maxManifest {
		return errors.New("manifest exceeds 64 MiB")
	}
	if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(b))}); err != nil {
		return err
	}
	_, err = tw.Write(b)
	return err
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func validateLayout(ctx context.Context, root, original string, cfg *config.Config, st *storage.Store) error {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if cfg.Mailstore.ObjectStore.Enabled {
		return errors.New("S3/object storage requires a coordinated external snapshot; refusing incomplete backup")
	}
	if _, err := relative(root, cfg.Log.File); err != nil {
		return err
	}
	required := []string{cfg.Storage.SQLite.Path, cfg.Mailstore.Root, cfg.TLS.CertFile, cfg.TLS.KeyFile, cfg.TLS.CertsDir, cfg.SMTP.DKIM.PrivateKeyFile, cfg.ARC.PrivateKeyFile}
	if cfg.Server.SecretsKey == "" {
		required = append(required, filepath.Join(filepath.Dir(cfg.Mailstore.Root), "secrets.key"))
	}
	values, err := st.ListSettings(ctx)
	if err != nil {
		return err
	}
	for key, value := range values {
		if strings.HasPrefix(key, "dkim.domain.") {
			var binding storage.DKIMBinding
			if err := json.Unmarshal([]byte(value), &binding); err != nil {
				return err
			}
			required = append(required, binding.KeyFile)
		}
	}
	for _, name := range required {
		name = rebase(name, original, root)
		if name == "" {
			continue
		}
		rel, err := relative(root, name)
		if err != nil {
			return err
		}
		resolved, err := filepath.EvalSymlinks(name)
		if err != nil {
			return fmt.Errorf("required path missing: %s: %w", name, err)
		}
		if resolved != filepath.Join(canonicalRoot, rel) {
			return fmt.Errorf("required path contains a symlink: %s", name)
		}
	}
	for _, ref := range []struct{ query, base string }{
		{"SELECT file_path FROM messages", cfg.Mailstore.Root},
		{"SELECT storage_path FROM calendar_attachments", filepath.Join(cfg.Mailstore.Root, "calendar-attachments")},
		{"SELECT storage_path FROM note_attachments", filepath.Join(cfg.Mailstore.Root, "notes")},
	} {
		rows, err := st.DB().QueryContext(ctx, ref.query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var name string
			if err = rows.Scan(&name); err != nil {
				break
			}
			name = rebase(name, original, root)
			if !filepath.IsAbs(name) {
				if !filepath.IsLocal(name) {
					err = errors.New("invalid content reference")
					break
				}
				name = filepath.Join(ref.base, filepath.FromSlash(name))
			}
			if _, err = relative(root, name); err != nil {
				break
			}
			var info os.FileInfo
			info, err = os.Lstat(name)
			if err != nil {
				err = fmt.Errorf("referenced content missing: %s: %w", name, err)
				break
			}
			if !info.Mode().IsRegular() {
				err = fmt.Errorf("referenced content is not a regular file: %s", name)
				break
			}
		}
		rowErr := rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if rowErr != nil {
			return rowErr
		}
	}
	return nil
}
