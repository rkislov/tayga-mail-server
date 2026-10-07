package storage

import (
	"context"
	"database/sql"
	"time"
)

// TryAcquireUserWriter acquires or renews the writer lease for userID on nodeID.
func (s *Store) TryAcquireUserWriter(ctx context.Context, userID, nodeID string, ttl time.Duration) (bool, string, error) {
	if userID == "" || nodeID == "" {
		return false, "", nil
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	now := time.Now().UTC()
	exp := now.Add(ttl)

	if s.dialect == DialectPostgres {
		return s.tryAcquireUserWriterPG(ctx, userID, nodeID, now, exp)
	}
	return s.tryAcquireUserWriterSQLite(ctx, userID, nodeID, now, exp)
}

func (s *Store) tryAcquireUserWriterPG(ctx context.Context, userID, nodeID string, now, exp time.Time) (bool, string, error) {
	// Upsert when missing, owned by us, or expired.
	q := `
		INSERT INTO user_writer_leases(user_id, node_id, expires_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET node_id = EXCLUDED.node_id,
		    expires_at = EXCLUDED.expires_at,
		    updated_at = EXCLUDED.updated_at
		WHERE user_writer_leases.node_id = EXCLUDED.node_id
		   OR user_writer_leases.expires_at < $4
		RETURNING node_id`
	var holder string
	err := s.db.QueryRowContext(ctx, q, userID, nodeID, exp, now).Scan(&holder)
	if err == nil {
		return holder == nodeID, holder, nil
	}
	if err != sql.ErrNoRows {
		return false, "", err
	}
	// Conflict: someone else holds a live lease.
	err = s.db.QueryRowContext(ctx, `SELECT node_id FROM user_writer_leases WHERE user_id = $1`, userID).Scan(&holder)
	if err != nil {
		return false, "", err
	}
	return false, holder, nil
}

func (s *Store) tryAcquireUserWriterSQLite(ctx context.Context, userID, nodeID string, now, exp time.Time) (bool, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = tx.Rollback() }()

	var curNode string
	var curExp time.Time
	err = tx.QueryRowContext(ctx, `SELECT node_id, expires_at FROM user_writer_leases WHERE user_id = ?`, userID).
		Scan(&curNode, &curExp)
	switch {
	case err == sql.ErrNoRows:
		_, err = tx.ExecContext(ctx, `INSERT INTO user_writer_leases(user_id, node_id, expires_at, updated_at) VALUES (?, ?, ?, ?)`,
			userID, nodeID, exp, now)
		if err != nil {
			return false, "", err
		}
		if err := tx.Commit(); err != nil {
			return false, "", err
		}
		return true, nodeID, nil
	case err != nil:
		return false, "", err
	}

	if curNode == nodeID || !curExp.After(now) {
		_, err = tx.ExecContext(ctx, `UPDATE user_writer_leases SET node_id = ?, expires_at = ?, updated_at = ? WHERE user_id = ?`,
			nodeID, exp, now, userID)
		if err != nil {
			return false, "", err
		}
		if err := tx.Commit(); err != nil {
			return false, "", err
		}
		return true, nodeID, nil
	}
	_ = tx.Rollback()
	return false, curNode, nil
}

func (s *Store) ReleaseUserWriter(ctx context.Context, userID, nodeID string) error {
	q := s.rebind(`DELETE FROM user_writer_leases WHERE user_id = ? AND node_id = ?`)
	_, err := s.db.ExecContext(ctx, q, userID, nodeID)
	return err
}
