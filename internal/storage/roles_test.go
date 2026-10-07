package storage_test

import (
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestRolesHelpers(t *testing.T) {
	if storage.JoinRoles([]string{"Admin", "admin", " user "}) != "admin,user" {
		t.Fatal(storage.JoinRoles([]string{"Admin", "admin", " user "}))
	}
	if !storage.HasRole("admin,user", storage.RoleAdmin) {
		t.Fatal("has admin")
	}
	got := storage.WithRole("user", storage.RoleAdmin)
	if got != "user,admin" && got != "admin,user" {
		t.Fatal(got)
	}
	if storage.WithoutRole("admin,user", storage.RoleAdmin) != "user" {
		t.Fatal(storage.WithoutRole("admin,user", storage.RoleAdmin))
	}
}
