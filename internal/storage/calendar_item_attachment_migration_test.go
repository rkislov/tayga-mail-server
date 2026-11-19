// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage/migrations"
)

func TestCalendarItemAttachmentMigrationKeepsLegacyFiles(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	object, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, f.event(t, "legacy-mail-attachment", 19, 0))
	if err != nil {
		t.Fatal(err)
	}
	filename := "074_calendar_item_attachments." + f.s.DialectName() + ".sql"
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
	id := NewID()
	_, err = f.s.db.ExecContext(ctx, f.s.rebind(`INSERT INTO calendar_attachments(id,calendar_object_id,user_id,filename,content_type,size,storage_path,created_at) VALUES(?,?,?,?,?,?,?,?)`), id, object.ID, f.user.ID, "old.eml", "message/rfc822", 10, "old-file", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	file, err := reopened.GetCalendarAttachment(ctx, id)
	if err != nil || file.EWSItemType != "" || file.Filename != "old.eml" || file.StoragePath != "old-file" || file.Size != 10 {
		t.Fatal("legacy file changed type or metadata", file, err)
	}
	if _, err = reopened.DB().ExecContext(ctx, f.s.rebind(`UPDATE calendar_attachments SET ews_item_type=? WHERE id=?`), "Unknown", id); err == nil {
		t.Fatal("unknown attachment kind accepted")
	}
}
