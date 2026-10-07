package mailstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliverGzipAndRead(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	raw := []byte("From: a@b.c\r\nSubject: archived\r\n\r\nbody text\r\n")
	rel, size, err := s.DeliverGzip("user@example.com", "Archive", raw)
	if err != nil {
		t.Fatal(err)
	}
	if size <= 0 || !strings.HasSuffix(rel, ".gz") {
		t.Fatalf("rel=%q size=%d", rel, size)
	}
	got, err := s.Read(rel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
	// on-disk bytes should be compressed
	disk, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	if len(disk) >= len(raw) && disk[0] != 0x1f {
		t.Fatalf("expected gzip on disk")
	}
}

func TestMoveToArchive(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	raw := []byte("From: a@b.c\r\nSubject: move me\r\n\r\nx\r\n")
	rel, _, err := s.Deliver("user@example.com", "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	newRel, _, out, err := s.MoveToArchive("user@example.com", rel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(newRel, ".gz") {
		t.Fatalf("newRel=%q", newRel)
	}
	if string(out) != string(raw) {
		t.Fatalf("raw mismatch")
	}
	if _, err := os.Stat(s.Abs(rel)); !os.IsNotExist(err) {
		t.Fatalf("source should be gone")
	}
}
