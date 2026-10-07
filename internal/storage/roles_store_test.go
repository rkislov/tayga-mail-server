package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestUserRolesPersist(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "a@ex.com", LocalPart: "a", AuthSource: "ldap", Enabled: true,
		Roles: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetUserByID(ctx, u.ID)
	if err != nil || !storage.HasRole(got.Roles, storage.RoleAdmin) {
		t.Fatalf("%+v err=%v", got, err)
	}
	if err := store.UpdateUserRoles(ctx, u.ID, ""); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetUserByID(ctx, u.ID)
	if storage.HasRole(got.Roles, storage.RoleAdmin) {
		t.Fatal("role should be cleared")
	}
}
