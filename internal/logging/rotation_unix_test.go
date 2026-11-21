//go:build !windows

package logging

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSIGHUPReopensWithoutStoppingLogger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tayga.log")
	log, closeLog, err := New("info", "json", path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeLog()
	log.Info("before-rotation")
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SIGHUP did not create the replacement log")
		}
		time.Sleep(10 * time.Millisecond)
	}
	log.Info("after-rotation")
	if err := closeLog(); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(path + ".1")
	current, _ := os.ReadFile(path)
	if !strings.Contains(string(old), "before-rotation") || strings.Contains(string(old), "after-rotation") || !strings.Contains(string(current), "after-rotation") {
		t.Fatalf("unexpected log placement: old=%q current=%q", old, current)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("log permissions: %v %v", info, err)
	}
}
