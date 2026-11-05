package storage

import (
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/storage/migrations"
	"strings"
	"testing"
)

func TestMessageMetadataQuarantinePreservesRowsAndRollsBack(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "archive", true: "rollback"}[rollback], func(t *testing.T) {
			f := newBookingFixture(t)
			if f.s.dialect != DialectSQLite {
				t.Skip("injected legacy SQLite corruption")
			}
			ctx := context.Background()
			db := f.s.DB()
			msg := travelMessage(t, f, "INBOX")
			if err := f.s.UpdateEWSMailMetadata(ctx, msg.ID, "", `{"keep":"healthy"}`); err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{`PRAGMA foreign_keys=OFF`, `INSERT INTO travel_jobs(message_id,user_id,state,attempts,warning) VALUES('orphan-travel','` + f.user.ID + `','done',4,'original warning')`, `INSERT INTO ews_mail_metadata(message_id,data) VALUES('orphan-ews','{"category":"retain"}')`, `PRAGMA foreign_keys=ON`} {
				if _, err := db.ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}
			if rollback {
				if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_quarantine BEFORE DELETE ON ews_mail_metadata BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := migrations.FS.ReadFile("065_message_metadata_quarantine.sqlite.sql")
			if err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, q := range splitSQL(extractGooseUp(string(raw))) {
				if _, err = tx.ExecContext(ctx, q); err != nil {
					break
				}
			}
			if err != nil {
				tx.Rollback()
			} else {
				err = tx.Commit()
			}
			if rollback {
				if err == nil {
					t.Fatal("injected failure was ignored")
				}
				var count int
				db.QueryRow(`SELECT count(*) FROM storage_orphaned_message_metadata`).Scan(&count)
				if count != 0 {
					t.Fatal("partial quarantine committed")
				}
				db.QueryRow(`SELECT count(*) FROM travel_jobs WHERE message_id='orphan-travel'`).Scan(&count)
				if count != 1 {
					t.Fatal("partial cleanup committed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			rows, err := db.Query(`SELECT source_table,row_data FROM storage_orphaned_message_metadata`)
			if err != nil {
				t.Fatal(err)
			}
			archived := map[string]map[string]any{}
			for rows.Next() {
				var table, data string
				rows.Scan(&table, &data)
				var v map[string]any
				if err = json.Unmarshal([]byte(data), &v); err != nil {
					t.Fatal(err)
				}
				archived[table] = v
			}
			rows.Close()
			if len(archived) != 2 || archived["travel_jobs"]["warning"] != "original warning" || archived["travel_jobs"]["attempts"] != float64(4) || archived["ews_mail_metadata"]["data"] != `{"category":"retain"}` {
				t.Fatalf("quarantine lost values: %+v", archived)
			}
			value, err := f.s.EWSMailMetadata(ctx, msg.ID)
			if err != nil || !strings.Contains(value, "healthy") {
				t.Fatal("healthy metadata changed", value, err)
			}
			check, err := db.Query(`PRAGMA foreign_key_check`)
			if err != nil {
				t.Fatal(err)
			}
			defer check.Close()
			if check.Next() {
				t.Fatal("foreign-key violations remain")
			}
			current, err := f.s.GetMessageByID(ctx, msg.ID)
			if err != nil || current.FilePath != msg.FilePath {
				t.Fatal("mail changed", err)
			}
		})
	}
}
