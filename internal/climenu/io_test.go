package climenu

import "testing"

func TestRedactDSN(t *testing.T) {
	in := "postgres://tayga:secret@localhost:5432/tayga?sslmode=disable"
	got := redactDSN(in)
	if got == in || got == "" {
		t.Fatalf("expected redacted, got %q", got)
	}
	if redactDSN("sqlite") != "sqlite" {
		t.Fatal("passthrough")
	}
}
