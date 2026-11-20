package full

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSnapshotRestoresMaildirFlagNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot represent Maildir colon filenames")
	}
	f := newFullFixture(t)
	names := []string{"maildir/owner@example.test/cur/1791519862991166478.mail.example.test:2,S", "maildir/owner@example.test/.Archive/2023/new/1791519862991166478.mail.example.test:2,"}
	for _, name := range names {
		f.put(t, name, "From: sender@example.test\r\nSubject: Maildir flags\r\n\r\nPreserve flags.\r\n")
	}
	_, err := f.snapshot(context.Background())
	must(t, err)
	target := filepath.Join(f.parent, "recovered-maildir")
	_, err = Restore(context.Background(), RestoreOptions{Input: f.output, Identity: f.keyFile, Target: target})
	must(t, err)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(target, name))
		must(t, err)
		if string(data) != f.files[name] {
			t.Fatalf("Maildir content changed: %s", name)
		}
	}
}

func TestSnapshotNamesRejectTraversalAndWindowsEscapes(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escape", "/absolute", "dir/../../escape", "dir//file", "dir/./file", `dir\file`, "file\x00suffix", "C:/outside", "C:outside", `\\server\share`} {
		if safeName(name) {
			t.Errorf("accepted unsafe name %q", name)
		}
	}
	if runtime.GOOS != "windows" && !safeName("maildir/user/cur/mail:2,S") {
		t.Fatal("rejected Unix Maildir flags")
	}
	if runtime.GOOS == "windows" && safeName("maildir/user/cur/mail:stream") {
		t.Fatal("accepted Windows alternate data stream")
	}
}
