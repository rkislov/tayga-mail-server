package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRequiresFile(t *testing.T) {
	if _, _, err := New("info", "json", ""); err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestNewWritesFileAndStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tayga.log")
	log, closeFn, err := New("info", "json", path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	log.Info("hello-file-log", "k", "v")
	_ = closeFn()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello-file-log") {
		t.Fatalf("log file missing message: %s", data)
	}
}
