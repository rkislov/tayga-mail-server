package scan

import (
	"strings"
	"testing"
)

func TestInjectHeader(t *testing.T) {
	msg := []byte("From: a@ex.com\r\nSubject: hi\r\n\r\nbody\r\n")
	out := InjectHeader(msg, "X-Virus-Status", "Quarantined")
	s := string(out)
	if !strings.Contains(s, "X-Virus-Status: Quarantined\r\n") {
		t.Fatalf("%q", s)
	}
	if !strings.Contains(s, "\r\n\r\nbody") {
		t.Fatalf("body broken: %q", s)
	}
	// Header should be before blank line.
	i := strings.Index(s, "X-Virus-Status")
	j := strings.Index(s, "\r\n\r\n")
	if i < 0 || j < 0 || i > j {
		t.Fatal("header not in header section")
	}
}
