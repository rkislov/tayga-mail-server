package siem

import (
	"strings"
	"testing"
)

func TestFormatCEF(t *testing.T) {
	line := FormatCEF("Tayga", "TaygaMail", "1.0", "auth:login", "Login success", 3, map[string]string{
		"src":   "1.2.3.4",
		"suser": "a@b.c",
		"msg":   "ok=yes",
	})
	wantPrefix := "CEF:0|Tayga|TaygaMail|1.0|auth:login|Login success|3|"
	if !strings.HasPrefix(line, wantPrefix) {
		t.Fatalf("prefix: got %q want %q…", line, wantPrefix)
	}
	for _, part := range []string{"src=1.2.3.4", "suser=a@b.c", `msg=ok\=yes`} {
		if !strings.Contains(line, part) {
			t.Fatalf("missing %q in %s", part, line)
		}
	}
}

func TestFormatCEFEscapesHeader(t *testing.T) {
	line := FormatCEF("A|B", "P", "v", "sig", "n", 1, nil)
	if line != "CEF:0|A\\|B|P|v|sig|n|1|" {
		t.Fatalf("got %q", line)
	}
}
