package auth_test

import (
	"testing"

	"github.com/tayga/tms/internal/auth"
)

func TestExpandFilter(t *testing.T) {
	got := auth.ExpandFilter("(&(objectClass=inetOrgPerson)(mail={email})(uid={user}))", "Admin@Example.COM")
	want := "(&(objectClass=inetOrgPerson)(mail=admin@example.com)(uid=admin))"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExpandFilterEscapesSpecialChars(t *testing.T) {
	got := auth.ExpandFilter("(mail={email})", `evil*)(uid=*)@ex.com`)
	if got == `(mail=evil*)(uid=*)@ex.com)` {
		t.Fatal("filter injection not escaped")
	}
	// Escaped form must not contain raw * from the local-part unescaped in a way that breaks the filter.
	if !contains(got, `\2a`) {
		t.Fatalf("expected escaped asterisk in %q", got)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
