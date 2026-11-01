package full

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"filippo.io/age"
	"github.com/tayga/tms/internal/maintenance"
)

type RestoreOptions struct {
	Input, Identity, Target string
	MaxBytes                int64
	// VerifyOnly removes the isolated extraction after checking the database.
	VerifyOnly bool
}

// Restore verifies authenticated ciphertext, every file digest, the SQLite
// database and application content references before publishing a new tree.
// It never overwrites an installation or starts its services.
func Restore(ctx context.Context, opts RestoreOptions) (*Manifest, error) {
	target, err := filepath.Abs(opts.Target)
	if err != nil {
		return nil, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	target = filepath.Join(parent, filepath.Base(target))
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return nil, errors.New("restore target must not exist")
	}
	if !opts.VerifyOnly && !publishSupported {
		return nil, errors.New("full restore publication is supported on Linux and macOS")
	}
	stage, err := os.MkdirTemp(parent, ".tayga-restore-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	key, err := os.Open(opts.Identity)
	if err != nil {
		return nil, err
	}
	identities, keyErr := age.ParseIdentities(io.LimitReader(key, 64<<10))
	key.Close()
	if keyErr != nil {
		return nil, errors.New("invalid age identity file")
	}
	f, err := os.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	plain, err := age.Decrypt(&contextReader{ctx: ctx, r: f}, identities...)
	if err != nil {
		return nil, fmt.Errorf("decrypt backup: %w", err)
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 100 << 30
	}
	m, err := extract(ctx, plain, stage, opts.MaxBytes)
	if err != nil {
		return nil, err
	}
	if err = verifyRestored(ctx, stage, m); err != nil {
		return nil, err
	}
	if opts.VerifyOnly {
		return m, nil
	}
	if err = relocate(ctx, stage, target, m); err != nil {
		return nil, err
	}
	for _, name := range []string{m.Header.Config, m.Header.Database} {
		if err = syncPath(filepath.Join(stage, name)); err != nil {
			return nil, err
		}
	}
	// Directory entries must reach disk as well as their already-synced files.
	for i := len(m.Entries) - 1; i >= 0; i-- {
		if m.Entries[i].Directory {
			if err = syncPath(filepath.Join(stage, m.Entries[i].Path)); err != nil {
				return nil, err
			}
		}
	}
	if err = syncPath(stage); err != nil {
		return nil, err
	}
	// Atomic no-replace publication also protects against a target appearing
	// while we were validating the archive.
	if err = publishDirectory(stage, target); err != nil {
		return nil, err
	}
	if err = syncPath(parent); err != nil {
		return nil, err
	}
	return m, nil
}

func syncPath(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func safeName(s string) bool {
	return s != "." && fsValidPath(s) && !strings.ContainsAny(s, "\\:\x00")
}

func fsValidPath(s string) bool {
	return s != "" && !strings.HasPrefix(s, "/") && path.Clean(s) == s && s != ".." && !strings.HasPrefix(s, "../")
}

func extract(ctx context.Context, reader io.Reader, stage string, maxBytes int64) (*Manifest, error) {
	br := bufio.NewReader(reader)
	gz, err := gzip.NewReader(br)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	gz.Multistream(false)
	// Includes tar headers/padding as well as file data, protecting against
	// archives with huge padding or unbounded tiny entries.
	limited := &io.LimitedReader{R: &contextReader{ctx: ctx, r: gz}, N: maxBytes + maxManifest + (64 << 20)}
	tr := tar.NewReader(limited)
	root, err := os.OpenRoot(stage)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var header Header
	var manifest *Manifest
	var entries []Entry
	seen := make(map[string]bool)
	var total int64
	for index := 0; ; index++ {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if index > 1_000_000 {
			return nil, errors.New("too many archive entries")
		}
		if manifest != nil {
			return nil, errors.New("entry after manifest")
		}
		if index == 0 {
			if h.Name != "header.json" || h.Typeflag != tar.TypeReg || h.Size > maxManifest {
				return nil, errors.New("invalid snapshot header")
			}
			if err = json.NewDecoder(tr).Decode(&header); err != nil {
				return nil, err
			}
			if header.Format != format || !safeName(header.Config) || !safeName(header.Database) || !filepath.IsAbs(header.Root) {
				return nil, errors.New("unsupported snapshot format or layout")
			}
			continue
		}
		if h.Name == "manifest.json" {
			if h.Typeflag != tar.TypeReg || h.Size > maxManifest {
				return nil, errors.New("invalid manifest")
			}
			if err = json.NewDecoder(tr).Decode(&manifest); err != nil {
				return nil, err
			}
			continue
		}
		if !strings.HasPrefix(h.Name, "data/") {
			return nil, errors.New("unknown archive entry")
		}
		name := strings.TrimPrefix(h.Name, "data/")
		for _, suffix := range []string{"-wal", "-shm", "-journal", maintenance.LockSuffix} {
			if name == header.Database+suffix {
				return nil, errors.New("database sidecars must not be restored")
			}
		}
		if !safeName(name) || seen[name] {
			return nil, fmt.Errorf("unsafe or duplicate archive path: %q", name)
		}
		seen[name] = true
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return nil, errors.New("links and special files are not supported")
		}
		if h.Size < 0 || h.Size > maxBytes-total {
			return nil, errors.New("backup exceeds restore byte limit")
		}
		if h.Typeflag == tar.TypeDir && h.Size != 0 {
			return nil, errors.New("invalid directory size")
		}
		total += h.Size
		e := Entry{Path: name, Size: h.Size, Directory: h.Typeflag == tar.TypeDir, Mode: h.Mode}
		if e.Directory {
			if err = root.MkdirAll(name, 0700); err != nil {
				return nil, err
			}
		} else {
			if err = root.MkdirAll(path.Dir(name), 0700); err != nil {
				return nil, err
			}
			f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600|os.FileMode(h.Mode)&0100)
			if err != nil {
				return nil, err
			}
			hash := sha256.New()
			n, copyErr := io.Copy(io.MultiWriter(f, hash), tr)
			syncErr := f.Sync()
			closeErr := f.Close()
			if err = errors.Join(copyErr, syncErr, closeErr); err != nil {
				return nil, err
			}
			if n != h.Size {
				return nil, errors.New("truncated file")
			}
			e.SHA256 = hex.EncodeToString(hash.Sum(nil))
		}
		entries = append(entries, e)
	}
	// Read to authenticated EOF. Checking tar EOF alone misses a truncated age
	// final chunk, corrupt gzip trailer, or appended second compressed stream.
	if _, err = io.Copy(io.Discard, limited); err != nil {
		return nil, err
	}
	if limited.N == 0 {
		return nil, errors.New("archive exceeds expanded byte limit")
	}
	if _, err = br.ReadByte(); err != io.EOF {
		if err == nil {
			return nil, errors.New("trailing data after archive")
		}
		return nil, err
	}
	if manifest == nil || !reflect.DeepEqual(manifest.Header, header) || !reflect.DeepEqual(manifest.Entries, entries) {
		return nil, errors.New("snapshot manifest/checksum mismatch")
	}
	if !seen[header.Config] || !seen[header.Database] {
		return nil, errors.New("snapshot lacks config or database")
	}
	return manifest, nil
}
