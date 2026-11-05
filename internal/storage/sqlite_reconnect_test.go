package storage

import (
	"context"
	"testing"
)

func TestSQLiteReconnectPreservesForeignKeysAndMessageCascades(t *testing.T) {
	f := newBookingFixture(t)
	if f.s.dialect != DialectSQLite {
		t.Skip("SQLite connection lifecycle")
	}
	ctx := context.Background()
	if err := f.s.SetTravelMode(ctx, f.user.ID, "suggest"); err != nil {
		t.Fatal(err)
	}
	m := travelMessage(t, f, "INBOX")
	if err := f.s.UpdateEWSMailMetadata(ctx, m.ID, "", `{"importance":1}`); err != nil {
		t.Fatal(err)
	}
	// Force replacement rather than testing the connection configured by Open.
	f.s.DB().SetMaxIdleConns(0)
	var enabled int
	if err := f.s.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil || enabled != 1 {
		t.Fatalf("replacement foreign_keys=%d: %v", enabled, err)
	}
	if err := f.s.DeleteMessage(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"travel_jobs", "ews_mail_metadata"} {
		var n int
		if err := f.s.DB().QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE message_id=?", m.ID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("orphan %s rows=%d: %v", table, n, err)
		}
	}
	if _, err := f.s.DB().ExecContext(ctx, `INSERT INTO ews_mail_metadata(message_id,data) VALUES('missing','{}')`); err == nil {
		t.Fatal("replacement connection accepted an orphan")
	}
	var timeout int
	if err := f.s.DB().QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatalf("replacement busy_timeout=%d: %v", timeout, err)
	}
}
