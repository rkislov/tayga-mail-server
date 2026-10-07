package auth

import (
	"strings"
	"testing"
)

func TestExpandGroupClosure(t *testing.T) {
	// user ∈ team ⊂ staff ⊂ mail-admins
	parents := map[string][]string{
		"cn=team,ou=groups,dc=ex,dc=com":        {"cn=staff,ou=groups,dc=ex,dc=com"},
		"cn=staff,ou=groups,dc=ex,dc=com":       {"cn=mail-admins,ou=groups,dc=ex,dc=com"},
		"cn=mail-admins,ou=groups,dc=ex,dc=com": nil,
	}
	got := expandGroupClosure(
		[]string{"cn=team,ou=groups,dc=ex,dc=com"},
		func(dn string) ([]string, error) {
			return parents[strings.ToLower(dn)], nil
		},
		8,
	)
	if !containsFold(got, "cn=mail-admins,ou=groups,dc=ex,dc=com") {
		t.Fatalf("missing nested admin group: %v", got)
	}
	if !containsFold(got, "mail-admins") {
		t.Fatalf("missing CN alias: %v", got)
	}
	if !groupMatches(got, []string{"mail-admins"}) {
		t.Fatal("admin match via nested")
	}
}

func TestExpandGroupClosureMaxDepth(t *testing.T) {
	parents := map[string][]string{
		"cn=a,dc=ex": {"cn=b,dc=ex"},
		"cn=b,dc=ex": {"cn=c,dc=ex"},
		"cn=c,dc=ex": {"cn=d,dc=ex"},
	}
	got := expandGroupClosure([]string{"cn=a,dc=ex"}, func(dn string) ([]string, error) {
		return parents[strings.ToLower(dn)], nil
	}, 1)
	if containsFold(got, "cn=c,dc=ex") {
		t.Fatalf("depth 1 should stop at b: %v", got)
	}
	if !containsFold(got, "cn=b,dc=ex") {
		t.Fatalf("expected b: %v", got)
	}
}

func TestExpandGroupClosureCycle(t *testing.T) {
	parents := map[string][]string{
		"cn=a,dc=ex": {"cn=b,dc=ex"},
		"cn=b,dc=ex": {"cn=a,dc=ex"},
	}
	got := expandGroupClosure([]string{"cn=a,dc=ex"}, func(dn string) ([]string, error) {
		return parents[strings.ToLower(dn)], nil
	}, 20)
	n := 0
	for _, g := range got {
		if strings.Contains(g, "=") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected 2 DNs in cycle, got %d: %v", n, got)
	}
}

func containsFold(list []string, want string) bool {
	want = strings.ToLower(want)
	for _, g := range list {
		if strings.EqualFold(g, want) {
			return true
		}
	}
	return false
}
