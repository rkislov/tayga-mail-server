package mailstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store manages Maildir++ layouts on disk, optionally mirrored to an object store.
type Store struct {
	Root string
	blob Blob
	log  *slog.Logger
}

func New(root string) *Store {
	return &Store{Root: root}
}

// SetObjectStore attaches a durable blob backend. Local maildir remains the hot cache;
// Deliver/Delete/MoveToCur sync keys, and Read falls back to the blob on cache miss.
func (s *Store) SetObjectStore(b Blob, log *slog.Logger) {
	if s == nil {
		return
	}
	s.blob = b
	if log == nil {
		log = slog.Default()
	}
	s.log = log
}

// UserRoot returns the Maildir++ base path for a user email.
func (s *Store) UserRoot(email string) string {
	email = strings.ToLower(email)
	safe := strings.ReplaceAll(email, "/", "_")
	return filepath.Join(s.Root, safe)
}

// Abs resolves a path relative to the mailstore root.
func (s *Store) Abs(relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(s.Root, relPath)
}

// EnsureUser creates Maildir++ INBOX (cur/new/tmp) for the user.
func (s *Store) EnsureUser(email string) (string, error) {
	root := s.UserRoot(email)
	for _, sub := range []string{"cur", "new", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o750); err != nil {
			return "", err
		}
	}
	return root, nil
}

// EnsureFolder creates a Maildir++ folder (.Sent, .Drafts, …) under the user root.
func (s *Store) EnsureFolder(email, folder string) (string, error) {
	root, err := s.EnsureUser(email)
	if err != nil {
		return "", err
	}
	folder = strings.TrimSpace(folder)
	if folder == "" || strings.EqualFold(folder, "INBOX") {
		return root, nil
	}
	name := folder
	if !strings.HasPrefix(name, ".") {
		name = "." + name
	}
	path := filepath.Join(root, name)
	for _, sub := range []string{"cur", "new", "tmp"} {
		if err := os.MkdirAll(filepath.Join(path, sub), 0o750); err != nil {
			return "", err
		}
	}
	return path, nil
}

// Deliver writes a message into the new/ subdirectory and returns the relative file path.
func (s *Store) Deliver(email, folder string, data []byte) (relPath string, size int64, err error) {
	folderPath, err := s.EnsureFolder(email, folder)
	if err != nil {
		return "", 0, err
	}

	tmpName := fmt.Sprintf("%d.%d.%s", time.Now().UnixNano(), os.Getpid(), hostname())
	tmpPath := filepath.Join(folderPath, "tmp", tmpName)
	if err := os.WriteFile(tmpPath, data, 0o640); err != nil {
		return "", 0, err
	}

	finalName := tmpName + ":2,"
	newPath := filepath.Join(folderPath, "new", finalName)
	if err := os.Rename(tmpPath, newPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}

	rel, err := filepath.Rel(s.Root, newPath)
	if err != nil {
		rel = newPath
	}
	s.blobPut(rel, data)
	return rel, int64(len(data)), nil
}

// Read returns the raw message bytes for a relative path under the root.
// On local miss, tries the object store and repopulates the cache.
func (s *Store) Read(relPath string) ([]byte, error) {
	data, err := os.ReadFile(s.Abs(relPath))
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) || s.blob == nil {
		return nil, err
	}
	data, berr := s.blob.Get(context.Background(), blobKey(relPath))
	if berr != nil {
		if errors.Is(berr, errBlobNotFound) {
			return nil, err
		}
		return nil, berr
	}
	abs := s.Abs(relPath)
	if mkErr := os.MkdirAll(filepath.Dir(abs), 0o750); mkErr == nil {
		_ = os.WriteFile(abs, data, 0o640)
	}
	return data, nil
}

// Delete removes a message file. Missing files are ignored.
func (s *Store) Delete(relPath string) error {
	err := os.Remove(s.Abs(relPath))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	s.blobDelete(relPath)
	return nil
}

// MoveToCur moves a message from new/ to cur/ with Maildir flag suffix.
func (s *Store) MoveToCur(relPath string, imapFlags []string) (string, error) {
	abs := s.Abs(relPath)
	dir, name := filepath.Split(abs)
	cleanDir := filepath.Clean(dir)

	oldRel := relPath
	if filepath.Base(cleanDir) == "cur" {
		newName := rewriteMaildirName(name, imapFlags)
		newAbs := filepath.Join(cleanDir, newName)
		if newAbs != abs {
			if err := os.Rename(abs, newAbs); err != nil {
				return "", err
			}
			abs = newAbs
		}
		return s.finishMove(oldRel, abs)
	}

	if filepath.Base(cleanDir) != "new" {
		newName := rewriteMaildirName(name, imapFlags)
		newAbs := filepath.Join(cleanDir, newName)
		if newAbs != abs {
			if err := os.Rename(abs, newAbs); err != nil {
				return "", err
			}
			abs = newAbs
		}
		return s.finishMove(oldRel, abs)
	}

	parent := filepath.Dir(cleanDir)
	curDir := filepath.Join(parent, "cur")
	if err := os.MkdirAll(curDir, 0o750); err != nil {
		return "", err
	}
	newName := rewriteMaildirName(stripInfo(name), imapFlags)
	newAbs := filepath.Join(curDir, newName)
	if err := os.Rename(abs, newAbs); err != nil {
		return "", err
	}
	return s.finishMove(oldRel, newAbs)
}

func (s *Store) finishMove(oldRel, newAbs string) (string, error) {
	newRel := relFromRoot(s.Root, newAbs)
	if s.blob != nil && newRel != oldRel {
		if data, err := os.ReadFile(newAbs); err == nil {
			s.blobPut(newRel, data)
			s.blobDelete(oldRel)
		}
	}
	return newRel, nil
}

func blobKey(relPath string) string {
	return filepath.ToSlash(relPath)
}

func (s *Store) blobPut(relPath string, data []byte) {
	if s == nil || s.blob == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.blob.Put(ctx, blobKey(relPath), data); err != nil && s.log != nil {
		s.log.Warn("object store put failed", "key", relPath, "err", err)
	}
}

func (s *Store) blobDelete(relPath string) {
	if s == nil || s.blob == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.blob.Delete(ctx, blobKey(relPath)); err != nil && s.log != nil {
		s.log.Warn("object store delete failed", "key", relPath, "err", err)
	}
}

// ListFolders returns IMAP mailbox names under a user (INBOX + .Folder → Folder).
func (s *Store) ListFolders(email string) ([]string, error) {
	root := s.UserRoot(email)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"INBOX"}, nil
		}
		return nil, err
	}
	out := []string{"INBOX"}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, strings.TrimPrefix(e.Name(), "."))
	}
	return out, nil
}

// RenameFolder renames a Maildir++ folder on disk (not INBOX).
func (s *Store) RenameFolder(email, oldName, newName string) error {
	if strings.EqualFold(oldName, "INBOX") || strings.EqualFold(newName, "INBOX") {
		return fmt.Errorf("cannot rename INBOX")
	}
	root := s.UserRoot(email)
	return os.Rename(filepath.Join(root, "."+oldName), filepath.Join(root, "."+newName))
}

// RemoveFolder deletes a Maildir++ folder (not INBOX).
func (s *Store) RemoveFolder(email, name string) error {
	if strings.EqualFold(name, "INBOX") {
		return fmt.Errorf("cannot delete INBOX")
	}
	return os.RemoveAll(filepath.Join(s.UserRoot(email), "."+name))
}

func relFromRoot(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return rel
}

func stripInfo(name string) string {
	if i := strings.Index(name, ":2,"); i >= 0 {
		return name[:i]
	}
	if i := strings.Index(name, ":"); i >= 0 {
		return name[:i]
	}
	return name
}

func rewriteMaildirName(name string, imapFlags []string) string {
	base := stripInfo(name)
	return base + ":2," + maildirInfo(imapFlags)
}

func maildirInfo(imapFlags []string) string {
	seen := map[byte]bool{}
	var chars []byte
	add := func(c byte) {
		if !seen[c] {
			seen[c] = true
			chars = append(chars, c)
		}
	}
	for _, f := range imapFlags {
		switch strings.ToLower(f) {
		case `\seen`:
			add('S')
		case `\flagged`:
			add('F')
		case `\answered`:
			add('R')
		case `\deleted`:
			add('T')
		case `\draft`:
			add('D')
		}
	}
	for i := 0; i < len(chars); i++ {
		for j := i + 1; j < len(chars); j++ {
			if chars[j] < chars[i] {
				chars[i], chars[j] = chars[j], chars[i]
			}
		}
	}
	return string(chars)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "localhost"
	}
	return h
}
