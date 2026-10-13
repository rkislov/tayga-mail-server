package storage_test

import (
	"context"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"path/filepath"
	"testing"
)

func TestDeleteHierarchyRequiresEmptyParents(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "test.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant, err := store.CreateTenant(ctx, "empty-check")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := store.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "user@example.com", LocalPart: "user", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteDomain(ctx, tenant.ID, domain.ID); err == nil {
		t.Fatal("deleted populated domain")
	}
	if err := store.DeleteTenant(ctx, tenant.ID); err == nil {
		t.Fatal("deleted populated tenant")
	}
	if _, err := store.GetUserByID(ctx, user.ID); err != nil {
		t.Fatal("user lost after rejected deletion")
	}
	if err := store.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTenant(ctx, tenant.ID); err == nil {
		t.Fatal("tenant still contains an empty domain")
	}
	if err := store.DeleteDomain(ctx, tenant.ID, domain.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatal(err)
	}
}
