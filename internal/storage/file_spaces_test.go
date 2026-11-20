package storage

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage/migrations"
)

func TestFileSpaceStorageGroupsCASAndPrincipals(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	if err := f.s.UpdateUserRoles(ctx, f.user.ID, RoleTenantAdmin); err != nil {
		t.Fatal(err)
	}
	group, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: f.user.DomainID, LocalPart: "space-test-managers", Name: "Space managers", MemberIDs: []string{f.user.ID}})
	if err != nil {
		t.Fatal(err)
	}
	space, err := f.s.SaveFileSpace(ctx, f.user.ID, &FileSpace{TenantID: f.user.TenantID, Name: "Team workspace", Kind: "department", QuotaBytes: 100, VersionDays: 30, Groups: []FileSpaceGroup{{GroupID: group.ID, Rights: "manage"}}})
	if err != nil {
		t.Fatal(err)
	}
	var owner string
	if err = f.s.db.QueryRowContext(ctx, f.s.rebind("SELECT space_id FROM file_owners WHERE id=?"), space.ID).Scan(&owner); err != nil || owner != space.ID {
		t.Fatalf("owner %s %v", owner, err)
	}
	list, err := f.s.ListFileSpaces(ctx, f.user.ID)
	if err != nil || len(list) != 1 || list[0].Rights != "manage" {
		t.Fatalf("list %+v %v", list, err)
	}
	if _, err = f.s.GetUserByID(ctx, space.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("synthetic user")
	}
	// Rights derive from membership, not the creator's identity.
	if err = f.s.UpdateUserRoles(ctx, f.user.ID, ""); err != nil {
		t.Fatal(err)
	}
	if rights, err := f.s.FileSpaceRights(ctx, space.ID, f.user.ID); err != nil || rights != "manage" {
		t.Fatalf("group rights %q %v", rights, err)
	}
	copy := *space
	copy.Name = "Updated"
	updated, err := f.s.SaveFileSpace(ctx, f.user.ID, &copy)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("save %+v %v", updated, err)
	}
	if _, err = f.s.SaveFileSpace(ctx, f.user.ID, &copy); !errors.Is(err, ErrFileSpaceConflict) {
		t.Fatalf("stale save %v", err)
	}
	group.MemberIDs = []string{}
	if _, err = f.s.SaveDirectoryGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	if rights, err := f.s.FileSpaceRights(ctx, space.ID, f.user.ID); err != nil || rights != "" {
		t.Fatalf("revoked %q %v", rights, err)
	}
	if _, err = f.s.SaveFileSpace(ctx, f.user.ID, updated); !errors.Is(err, ErrFileSpaceForbidden) {
		t.Fatal("former manager changed space")
	}
	// Tenant administrators can recover a space with no remaining manager group.
	if err = f.s.UpdateUserRoles(ctx, f.user.ID, RoleTenantAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.SaveFileSpace(ctx, f.user.ID, updated); err != nil {
		t.Fatal(err)
	}
}
func TestFileSpaceMigrationPreservesPersonalShares(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err = db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	files, err := fs.Glob(migrations.FS, "*.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if name >= "062_file_spaces.sqlite.sql" {
			continue
		}
		if err = applyMigration(ctx, db, DialectSQLite, name); err != nil {
			t.Fatal(err)
		}
	}
	store := NewSQLStore(db, DialectSQLite)
	tenant, err := store.CreateTenant(ctx, "Upgrade")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := store.CreateDomain(ctx, tenant.ID, "upgrade.example")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "owner", Email: "owner@upgrade.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "reader", Email: "reader@upgrade.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	share, err := store.CreateFileShare(ctx, &FileShare{UserID: owner.ID, Path: "file.txt", Rights: "write", PasswordHash: "retained-hash"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SetFileAccess(ctx, owner.ID, "file.txt", reader.ID, "read"); err != nil {
		t.Fatal(err)
	}
	if err = applyMigration(ctx, db, DialectSQLite, "062_file_spaces.sqlite.sql"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetFileShareByToken(ctx, share.Token)
	if err != nil || got.PasswordHash != "retained-hash" || got.Rights != "write" {
		t.Fatalf("share %+v %v", got, err)
	}
	grants, err := store.ListFileAccess(ctx, owner.ID, reader.ID)
	if err != nil || len(grants) != 1 {
		t.Fatal("lost grant")
	}
	// New user insertions also get a file principal via the migration trigger.
	newer, err := store.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "new", Email: "new@upgrade.example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateFileShare(ctx, &FileShare{UserID: newer.ID, Path: "new.txt", Rights: "read"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteUser(ctx, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetFileShareByToken(ctx, share.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("personal cascade %v", err)
	}
	// Reopening runs all migrations idempotently and retains the principals.
	driver, err := Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	driver.Close()
}
func TestFileSpaceStatementBlocks(t *testing.T) {
	script := "CREATE TABLE before_trigger(id TEXT);\n-- +goose StatementBegin\nCREATE TRIGGER example AFTER INSERT ON before_trigger BEGIN SELECT 1; SELECT 2; END;\n-- +goose StatementEnd\nCREATE TABLE after_trigger(id TEXT);"
	statements := splitSQL(script)
	if len(statements) != 3 || !strings.Contains(statements[1], "SELECT 1; SELECT 2;") {
		t.Fatalf("blocks %#v", statements)
	}
}

func TestFileSpaceRecoveryStorageAtomicAndIdempotent(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	if err := f.s.UpdateUserRoles(ctx, f.user.ID, RoleTenantAdmin); err != nil {
		t.Fatal(err)
	}
	group, err := f.s.SaveDirectoryGroup(ctx, &DirectoryGroup{DomainID: f.user.DomainID, LocalPart: "recovery-managers", Name: "Recovery", MemberIDs: []string{f.user.ID}})
	if err != nil {
		t.Fatal(err)
	}
	space, err := f.s.SaveFileSpace(ctx, f.user.ID, &FileSpace{TenantID: f.user.TenantID, Name: "Recoverable", Kind: "project", QuotaBytes: 100, VersionDays: 7, Groups: []FileSpaceGroup{{GroupID: group.ID, Rights: "manage"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.SetFileAccess(ctx, space.ID, "document.txt", f.user.ID, "write"); err != nil {
		t.Fatal(err)
	}
	link, err := f.s.CreateFileShare(ctx, &FileShare{UserID: space.ID, Path: "document.txt", Rights: "read", PasswordHash: "test-hash"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := f.s.SnapshotFileSpace(ctx, space.ID)
	if err != nil {
		t.Fatal(err)
	}
	space.Name = "Changed"
	space.QuotaBytes = 500
	if _, err = f.s.SaveFileSpace(ctx, f.user.ID, space); err != nil {
		t.Fatal(err)
	}
	f.s.DeleteFileShare(ctx, space.ID, link.ID)
	f.s.DeleteFileAccess(ctx, space.ID, "document.txt", f.user.ID)
	operation := NewID()
	if err = f.s.RestoreFileSpaceSnapshot(ctx, snapshot, operation); err != nil {
		t.Fatal(err)
	}
	restored, err := f.s.GetFileSpace(ctx, space.ID)
	if err != nil || restored.Name != "Recoverable" || restored.QuotaBytes != 100 || restored.Generation != 1 {
		t.Fatalf("state %+v %v", restored, err)
	}
	if err = f.s.RestoreFileSpaceSnapshot(ctx, snapshot, operation); err != nil {
		t.Fatal(err)
	}
	again, _ := f.s.GetFileSpace(ctx, space.ID)
	if again.Generation != 1 {
		t.Fatal("restore repeated")
	}
	got, err := f.s.GetFileShareByToken(ctx, link.Token)
	if err != nil || got.PasswordHash != "test-hash" {
		t.Fatal("share not restored")
	}
	acl, _ := f.s.ListFileAccess(ctx, space.ID, f.user.ID)
	if len(acl) != 1 {
		t.Fatal("access not restored")
	}
	// The dependency identity, rather than just a reused UUID, must match.
	broken, err := CloneFileSpaceSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	broken.Users[0].Email = "different@example.com"
	if err = f.s.ValidateFileSpaceSnapshot(ctx, broken); !errors.Is(err, ErrFileSpaceInvalid) {
		t.Fatalf("identity accepted %v", err)
	}
	if err = f.s.RestoreFileSpaceSnapshot(ctx, broken, NewID()); !errors.Is(err, ErrFileSpaceInvalid) {
		t.Fatal("invalid snapshot applied")
	}
	after, _ := f.s.GetFileSpace(ctx, space.ID)
	if after.Generation != 1 {
		t.Fatal("failed restore changed generation")
	}
}
