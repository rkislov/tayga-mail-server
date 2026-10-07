package storage_test

import (
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestRolesHelpers(t *testing.T) {
	if storage.JoinRoles([]string{"Admin", "admin", " user "}) != "global_admin,user" {
		t.Fatal(storage.JoinRoles([]string{"Admin", "admin", " user "}))
	}
	if !storage.HasRole("admin,user", storage.RoleAdmin) {
		t.Fatal("has admin")
	}
	if !storage.IsGlobalAdmin("admin,user") {
		t.Fatal("is global")
	}
	got := storage.WithRole("user", storage.RoleAdmin)
	if got != "user,global_admin" && got != "global_admin,user" {
		t.Fatal(got)
	}
	if storage.WithoutRole("admin,user", storage.RoleAdmin) != "user" {
		t.Fatal(storage.WithoutRole("admin,user", storage.RoleAdmin))
	}
	if storage.AdminScope("domain_admin") != "domain" {
		t.Fatal(storage.AdminScope("domain_admin"))
	}
}
