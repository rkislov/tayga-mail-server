package mailstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store manages Maildir++ layouts on disk.
type Store struct {
	Root string
}

func New(root string) *Store {
	return &Store{Root: root}
}

// UserRoot returns the Maildir++ base path for a user email.
func (s *Store) UserRoot(email string) string {
	email = strings.ToLower(email)
	safe := strings.ReplaceAll(email, "/", "_")
	return filepath.Join(s.Root, safe)
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
	return rel, int64(len(data)), nil
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "localhost"
	}
	return h
}
