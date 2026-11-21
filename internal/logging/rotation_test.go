package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRotationKeepsConcurrentRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tayga.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	writer := &rotatingFile{file: file, path: path}
	defer writer.Close()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for record := 0; record < 100; record++ {
				if _, err := fmt.Fprintf(writer, "%d:%d\n", worker, record); err != nil {
					t.Error(err)
					return
				}
			}
		}(worker)
	}
	for rotation := 0; rotation < 8; rotation++ {
		if err := os.Rename(path, fmt.Sprintf("%s.%d", path, rotation)); err != nil {
			t.Fatal(err)
		}
		if err := writer.reopen(); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(path + "*")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Fields(string(data)) {
			if seen[line] {
				t.Fatalf("duplicate record %q", line)
			}
			seen[line] = true
		}
	}
	if len(seen) != 800 {
		t.Fatalf("records: got %d want 800", len(seen))
	}
}

func TestFailedReopenKeepsOriginalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tayga.log")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := &rotatingFile{file: file, path: path}
	defer writer.Close()
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	// A directory in place of the new log makes reopening fail even as root.
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writer.reopen(); err == nil {
		t.Fatal("expected reopen failure")
	}
	if _, err := writer.Write([]byte("retained\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path + ".1")
	if err != nil || string(data) != "retained\n" {
		t.Fatalf("old log: %q %v", data, err)
	}
}
