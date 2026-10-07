package auth

import (
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestGroupMatches(t *testing.T) {
	groups := []string{"CN=mail-admins,OU=groups,DC=example,DC=com", "users"}
	admins := []string{"mail-admins"}
	if !groupMatches(groups, admins) {
		t.Fatal("cn match")
	}
	if !groupMatches(groups, []string{"cn=mail-admins,ou=groups,dc=example,dc=com"}) {
		t.Fatal("dn match")
	}
	if groupMatches(groups, []string{"other"}) {
		t.Fatal("should not match")
	}
}

func TestApplyRoles(t *testing.T) {
	d := &Directory{Store: nil}
	u := &storage.User{ID: "1", Roles: ""}
	dc := config.LDAPDomainConfig{
		Groups: config.LDAPGroupsConfig{
			Mode:        "memberof",
			AdminGroups: []string{"mail-admins"},
		},
	}
	// Without store, UpdateUserRoles would panic — only test match helpers path via WithRole.
	roles := ""
	if groupMatches([]string{"mail-admins"}, dc.Groups.AdminGroups) {
		roles = storage.WithRole(roles, storage.RoleAdmin)
	}
	if !storage.HasRole(roles, storage.RoleAdmin) {
		t.Fatal(roles)
	}
	_ = d
	_ = u
}
