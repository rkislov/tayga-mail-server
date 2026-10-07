package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	q := s.rebind(`SELECT value FROM settings WHERE key = ?`)
	var value string
	err := s.db.QueryRowContext(ctx, q, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) ListSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *Store) PutSetting(ctx context.Context, key, value string) error {
	now := time.Now().UTC()
	var q string
	if s.dialect == DialectPostgres {
		q = `INSERT INTO settings(key, value, updated_at) VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`
	} else {
		q = `INSERT INTO settings(key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	}
	_, err := s.db.ExecContext(ctx, q, key, value, now)
	return err
}
