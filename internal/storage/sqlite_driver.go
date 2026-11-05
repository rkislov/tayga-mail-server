package storage

import (
	"context"
	"database/sql"
	"fmt"

	"modernc.org/sqlite"
)

const sqliteDriverName = "tayga-sqlite"

func init() {
	// PRAGMAs belong to a physical connection, not the database/sql pool. An
	// idle connection can be replaced after cancellation, errors or recycling.
	d := &sqlite.Driver{}
	d.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		for _, statement := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000"} {
			if _, err := conn.ExecContext(context.Background(), statement, nil); err != nil {
				return fmt.Errorf("configure SQLite connection: %w", err)
			}
		}
		return nil
	})
	sql.Register(sqliteDriverName, d)
}
