// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"github.com/tayga/tms/internal/storage/migrations"
	"strings"
	"testing"
)

func TestCalendarEmbeddedMigrationKeepsAttachedMail(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	object, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, f.event(t, "legacy-attached-mail", 19, 0))
	if err != nil {
		t.Fatal(err)
	}
	file, err := f.s.CreateCalendarAttachment(ctx, &CalendarAttachment{CalendarObjectID: object.ID, UserID: f.user.ID, Filename: "old.eml", ContentType: "message/rfc822", Size: 10, StoragePath: "old-mail", EWSItemType: "Message"})
	if err != nil {
		t.Fatal(err)
	}
	filename := "075_calendar_embedded_items." + f.s.DialectName() + ".sql"
	raw, err := migrations.FS.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err = execScript(ctx, f.s.db, strings.SplitN(string(raw), "-- +goose Down", 2)[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.ExecContext(ctx, f.s.rebind(`DELETE FROM schema_migrations WHERE version=?`), strings.TrimSuffix(filename, ".sql")); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetCalendarAttachment(ctx, file.ID)
	if err != nil || got.EWSItemXML != "" || got.EWSItemType != "Message" || got.StoragePath != "old-mail" {
		t.Fatal("legacy attached message changed", got, err)
	}
}
