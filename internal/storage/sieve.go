package storage

import (
	"context"
	"time"
)

func (s *Store) ListSieveScripts(ctx context.Context, userID string) ([]*SieveScript, error) {
	q := s.rebind(`SELECT id, user_id, name, script, active, created_at FROM sieve_scripts WHERE user_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*SieveScript
	for rows.Next() {
		sc, err := s.scanSieve(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) GetSieveScript(ctx context.Context, userID, name string) (*SieveScript, error) {
	q := s.rebind(`SELECT id, user_id, name, script, active, created_at FROM sieve_scripts WHERE user_id = ? AND name = ?`)
	return s.scanSieve(s.db.QueryRowContext(ctx, q, userID, name))
}

func (s *Store) GetActiveSieveScript(ctx context.Context, userID string) (*SieveScript, error) {
	q := s.rebind(`SELECT id, user_id, name, script, active, created_at FROM sieve_scripts WHERE user_id = ? AND active = ?`)
	active := 1
	if s.dialect == DialectPostgres {
		return s.scanSieve(s.db.QueryRowContext(ctx, q, userID, true))
	}
	return s.scanSieve(s.db.QueryRowContext(ctx, q, userID, active))
}

func (s *Store) PutSieveScript(ctx context.Context, userID, name, script string) (*SieveScript, error) {
	now := time.Now().UTC()
	existing, err := s.GetSieveScript(ctx, userID, name)
	if err == nil {
		q := s.rebind(`UPDATE sieve_scripts SET script = ? WHERE id = ?`)
		if _, err := s.db.ExecContext(ctx, q, script, existing.ID); err != nil {
			return nil, err
		}
		existing.Script = script
		return existing, nil
	}
	if err != ErrNotFound {
		return nil, err
	}

	sc := &SieveScript{ID: NewID(), UserID: userID, Name: name, Script: script, Active: false, CreatedAt: now}
	q := s.rebind(`INSERT INTO sieve_scripts(id, user_id, name, script, active, created_at) VALUES (?, ?, ?, ?, ?, ?)`)
	active := 0
	if s.dialect == DialectPostgres {
		if _, err := s.db.ExecContext(ctx, q, sc.ID, userID, name, script, false, now); err != nil {
			return nil, mapErr(err)
		}
		return sc, nil
	}
	if _, err := s.db.ExecContext(ctx, q, sc.ID, userID, name, script, active, now); err != nil {
		return nil, mapErr(err)
	}
	return sc, nil
}

func (s *Store) DeleteSieveScript(ctx context.Context, userID, name string) error {
	q := s.rebind(`DELETE FROM sieve_scripts WHERE user_id = ? AND name = ?`)
	res, err := s.db.ExecContext(ctx, q, userID, name)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetActiveSieveScript(ctx context.Context, userID, name string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var deactivate string
	if s.dialect == DialectPostgres {
		deactivate = `UPDATE sieve_scripts SET active = FALSE WHERE user_id = $1`
	} else {
		deactivate = `UPDATE sieve_scripts SET active = 0 WHERE user_id = ?`
	}
	if _, err := tx.ExecContext(ctx, deactivate, userID); err != nil {
		return err
	}
	if name == "" {
		return tx.Commit()
	}

	var activate string
	if s.dialect == DialectPostgres {
		activate = `UPDATE sieve_scripts SET active = TRUE WHERE user_id = $1 AND name = $2`
	} else {
		activate = `UPDATE sieve_scripts SET active = 1 WHERE user_id = ? AND name = ?`
	}
	res, err := tx.ExecContext(ctx, activate, userID, name)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) scanSieve(row interface{ Scan(dest ...any) error }) (*SieveScript, error) {
	sc := &SieveScript{}
	var active any
	if err := row.Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.Script, &active, &sc.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	sc.Active = asBool(active)
	return sc, nil
}
