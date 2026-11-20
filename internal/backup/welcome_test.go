package backup_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestWelcomeBackupHistoryAndScopeRemapping(t *testing.T) {
	ctx := context.Background()
	open := func(name string) storage.Driver {
		t.Helper()
		s, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), name)}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	}
	src, dst := open("source.db"), open("dest.db")
	tenant, err := src.CreateTenant(ctx, "welcome-backup")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := src.CreateDomain(ctx, tenant.ID, "welcome-backup.example")
	if err != nil {
		t.Fatal(err)
	}
	other, err := src.CreateTenant(ctx, "private-other")
	if err != nil {
		t.Fatal(err)
	}
	var photo bytes.Buffer
	if err := png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	save := storage.WelcomeChange{Action: "save", Content: storage.WelcomeRevision{Subject: "Hello", HTML: `<p>First</p><img src="data:image/png;base64,` + base64.StdEncoding.EncodeToString(photo.Bytes()) + `">`, Text: "First", Comment: "first"}}
	if _, err = src.ChangeWelcomeTemplate(ctx, "tenants", other.ID, "other-admin", save); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []struct{ kind, id string }{{"tenants", tenant.ID}, {"domains", domain.ID}} {
		if _, err = src.ChangeWelcomeTemplate(ctx, scope.kind, scope.id, "admin@example.com", save); err != nil {
			t.Fatal(err)
		}
		if _, err = src.ChangeWelcomeTemplate(ctx, scope.kind, scope.id, "admin@example.com", storage.WelcomeChange{Action: "publish", Expected: 1, Revision: 1}); err != nil {
			t.Fatal(err)
		}
		if _, err = src.ChangeWelcomeTemplate(ctx, scope.kind, scope.id, "admin@example.com", storage.WelcomeChange{Action: "restore", Expected: 2, Revision: 1}); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	if err = backup.WriteTarGz(ctx, src, backup.Options{TenantID: tenant.ID}, &archive); err != nil {
		t.Fatal(err)
	}
	// Existing names with different UUIDs exercise the restore mapping.
	mappedTenant, err := dst.CreateTenant(ctx, tenant.Name)
	if err != nil {
		t.Fatal(err)
	}
	mappedDomain, err := dst.CreateDomain(ctx, mappedTenant.ID, domain.Name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = backup.RestoreTarGz(ctx, dst, bytes.NewReader(archive.Bytes()), backup.RestoreOptions{SkipExisting: true}); err != nil {
		t.Fatal(err)
	}
	all, err := dst.ListWelcomeTemplates(ctx, "")
	if err != nil || len(all) != 2 {
		t.Fatal("scope leak", len(all), err)
	}
	for _, scope := range []struct{ kind, oldID, newID string }{{"tenants", tenant.ID, mappedTenant.ID}, {"domains", domain.ID, mappedDomain.ID}} {
		before, e := src.GetWelcomeTemplate(ctx, scope.kind, scope.oldID)
		if e != nil {
			t.Fatal(e)
		}
		after, e := dst.GetWelcomeTemplate(ctx, scope.kind, scope.newID)
		if e != nil {
			t.Fatal(e)
		}
		before.ScopeID = scope.newID
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("history lost: before=%+v after=%+v", before, after)
		}
	}
	// A repeated restore must not erase newer local versions or audit events.
	if _, err = dst.ChangeWelcomeTemplate(ctx, "tenants", mappedTenant.ID, "new-admin", storage.WelcomeChange{Action: "save", Expected: 3, Content: save.Content}); err != nil {
		t.Fatal(err)
	}
	if _, err = backup.RestoreTarGz(ctx, dst, bytes.NewReader(archive.Bytes()), backup.RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	current, err := dst.GetWelcomeTemplate(ctx, "tenants", mappedTenant.ID)
	if err != nil || current.Generation != 4 || len(current.Revisions) != 3 {
		t.Fatal("restore overwrote live history", current, err)
	}
}

func TestWelcomeRestoreDoesNotOnboardExistingAccounts(t *testing.T) {
	ctx := context.Background()
	open := func() storage.Driver {
		t.Helper()
		s, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "restore.db")}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	}
	src, dst := open(), open()
	tenant, err := src.CreateTenant(ctx, "onboarding-restore")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := src.CreateDomain(ctx, tenant.ID, "onboarding-restore.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = src.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "existing", Email: "existing@" + domain.Name, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = src.ChangeWelcomeTemplate(ctx, "tenants", tenant.ID, "admin", storage.WelcomeChange{Action: "save", Content: storage.WelcomeRevision{Subject: "Welcome", HTML: "<p>Welcome</p>", Text: "Welcome"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = src.ChangeWelcomeTemplate(ctx, "tenants", tenant.ID, "admin", storage.WelcomeChange{Action: "publish", Expected: 1, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err = backup.WriteTarGz(ctx, src, backup.Options{}, &archive); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = backup.RestoreTarGz(ctx, dst, bytes.NewReader(archive.Bytes()), backup.RestoreOptions{}); err != nil {
			t.Fatal(err)
		}
		stats, err := dst.WelcomeDeliveryStats(ctx, "tenants", tenant.ID)
		if err != nil || stats.Pending != 0 || stats.Delivered != 0 {
			t.Fatal("restore queued an old account", stats, err)
		}
	}
	if _, err = dst.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, LocalPart: "new", Email: "new@" + domain.Name, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	stats, err := dst.WelcomeDeliveryStats(ctx, "tenants", tenant.ID)
	if err != nil || stats.Pending != 1 {
		t.Fatal("new account not queued", stats, err)
	}
}
