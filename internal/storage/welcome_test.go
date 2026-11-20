package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage/migrations"
)

func TestWelcomeStoragePersistenceCASAndMigration(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	other, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	kind, id := "tenants", f.user.TenantID
	change := WelcomeChange{Action: "save", Content: WelcomeRevision{Subject: "Hello", HTML: "<p>Hello</p>", Text: "Hello"}}
	for _, expected := range []int64{0, 1} {
		gate := make(chan struct{})
		results := make(chan error, 2)
		change.Expected = expected
		for _, driver := range []Driver{f.s, other} {
			go func(d Driver) {
				<-gate
				_, e := d.ChangeWelcomeTemplate(ctx, kind, id, "admin@example.com", change)
				results <- e
			}(driver)
		}
		close(gate)
		success := 0
		for range 2 {
			if e := <-results; e == nil {
				success++
			} else if !errors.Is(e, ErrWelcomeConflict) {
				t.Fatal(e)
			}
		}
		if success != 1 {
			t.Fatalf("%d writers committed", success)
		}
	}
	v, err := other.GetWelcomeTemplate(ctx, kind, id)
	if err != nil || v.Generation != 2 || len(v.Revisions) != 2 {
		t.Fatal("persistence", v, err)
	}
	if _, err = other.ChangeWelcomeTemplate(ctx, kind, id, "publisher@example.com", WelcomeChange{Action: "publish", Expected: 2, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	v, err = f.s.ChangeWelcomeTemplate(ctx, kind, id, "restorer@example.com", WelcomeChange{Action: "restore", Expected: 3, Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if v.Published != 1 || v.Generation != 4 || v.Revisions[2].RestoredFrom != 1 || v.Revisions[2].Author != "restorer@example.com" {
		t.Fatal("restore", v)
	}
	if err = other.ImportWelcomeTemplate(ctx, *v); err != nil {
		t.Fatal(err)
	}
	all, err := other.ListWelcomeTemplates(ctx, id)
	if err != nil || len(all) != 1 || len(all[0].Events) != 4 {
		t.Fatal("export", all, err)
	}
	// Verify rollback/reapply on the disposable fixture, with both SQL dialects.
	suffix := "sqlite"
	if f.s.dialect == DialectPostgres {
		suffix = "postgres"
	}
	raw, err := migrations.FS.ReadFile("059_welcome_templates." + suffix + ".sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("missing down migration")
	}
	tx, err := f.s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = execScript(ctx, tx, parts[1]); err != nil {
		t.Fatal(err)
	}
	if err = execScript(ctx, tx, strings.TrimPrefix(parts[0], "-- +goose Up")); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = f.s.ImportWelcomeTemplate(ctx, *v); err != nil {
		t.Fatal(err)
	}
	loaded, err := other.GetWelcomeTemplate(ctx, kind, id)
	if err != nil || loaded.Generation != v.Generation || len(loaded.Revisions) != 3 {
		t.Fatal("import", loaded, err)
	}
}
