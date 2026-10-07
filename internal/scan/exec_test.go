package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestExecScanner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "scan.sh")
	// Exit 1 if stdin contains VIRUS
	content := "#!/bin/sh\ndata=$(cat)\necho \"$data\" | grep -q VIRUS && exit 1\nexit 0\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewExec([]string{script}, 5*time.Second)
	ctx := context.Background()
	res, err := s.Scan(ctx, []byte("hello clean"))
	if err != nil || !res.Clean {
		t.Fatalf("clean %#v %v", res, err)
	}
	res, err = s.Scan(ctx, []byte("has VIRUS inside"))
	if err != nil || res.Clean {
		t.Fatalf("infected %#v %v", res, err)
	}
}
