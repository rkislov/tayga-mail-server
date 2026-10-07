package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GreylistTouch inserts or updates a greylist triplet.
// On first sight returns allowed=false. After delay (or if already passed) returns allowed=true.
func (s *Store) GreylistTouch(ctx context.Context, clientIP, envelopeFrom, rcpt string, delay time.Duration) (bool, error) {
	if delay <= 0 {
		delay = 5 * time.Minute
	}
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var firstSeen time.Time
	var passed bool
	sel := s.rebind(`SELECT first_seen, passed FROM greylist WHERE client_ip = ? AND envelope_from = ? AND rcpt = ?`)
	err = tx.QueryRowContext(ctx, sel, clientIP, envelopeFrom, rcpt).Scan(&firstSeen, &passed)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		ins := s.rebind(`INSERT INTO greylist(id, client_ip, envelope_from, rcpt, first_seen, last_seen, passed)
			VALUES (?, ?, ?, ?, ?, ?, ?)`)
		if _, err := tx.ExecContext(ctx, ins, NewID(), clientIP, envelopeFrom, rcpt, now, now, false); err != nil {
			// Concurrent insert — load winner
			if err2 := tx.QueryRowContext(ctx, sel, clientIP, envelopeFrom, rcpt).Scan(&firstSeen, &passed); err2 != nil {
				return false, err
			}
		} else {
			if err := tx.Commit(); err != nil {
				return false, err
			}
			return false, nil
		}
	case err != nil:
		return false, err
	}

	allowed := passed || now.Sub(firstSeen) >= delay
	upd := s.rebind(`UPDATE greylist SET last_seen = ?, passed = ? WHERE client_ip = ? AND envelope_from = ? AND rcpt = ?`)
	if _, err := tx.ExecContext(ctx, upd, now, allowed, clientIP, envelopeFrom, rcpt); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return allowed, nil
}

func (s *Store) DeleteExpiredGreylist(ctx context.Context, olderThan time.Time) (int64, error) {
	q := s.rebind(`DELETE FROM greylist WHERE last_seen < ?`)
	res, err := s.db.ExecContext(ctx, q, olderThan.UTC())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
