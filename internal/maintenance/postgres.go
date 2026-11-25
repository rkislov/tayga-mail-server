package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"
)

type postgresLock struct {
	connection *sql.Conn
	exclusive  bool
}

// Advisory locks are database-local. Keep a dedicated connection for the whole
// storage lifetime so an offline maintenance process cannot race a new server.
func Postgres(ctx context.Context, db *sql.DB, exclusive bool) (io.Closer, error) {
	connection, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	query := "SELECT pg_try_advisory_lock_shared(83172811)"
	if exclusive {
		query = "SELECT pg_try_advisory_lock(83172811)"
	}
	var acquired bool
	if err = connection.QueryRowContext(ctx, query).Scan(&acquired); err != nil || !acquired {
		connection.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("PostgreSQL is in use by Tayga or offline maintenance; stop the other process first")
	}
	return &postgresLock{connection: connection, exclusive: exclusive}, nil
}
func (l *postgresLock) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := "SELECT pg_advisory_unlock_shared(83172811)"
	if l.exclusive {
		query = "SELECT pg_advisory_unlock(83172811)"
	}
	_, err := l.connection.ExecContext(ctx, query)
	return errors.Join(err, l.connection.Close())
}
