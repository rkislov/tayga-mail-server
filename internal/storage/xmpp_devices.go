package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrXMPPDeviceLimit = errors.New("maximum of 50 active XMPP device passwords")
var ErrXMPPDeviceNotTested = errors.New("connect at least one client using a device password first")

type XMPPDevicePassword struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	SecretHash string     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func (s *Store) CreateXMPPDevicePassword(ctx context.Context, d *XMPPDevicePassword) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockXMPPUser(ctx, tx, d.UserID); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM xmpp_device_passwords WHERE user_id=? AND revoked_at IS NULL`), d.UserID).Scan(&count); err != nil {
		return err
	}
	if count >= 50 {
		return ErrXMPPDeviceLimit
	}
	_, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_device_passwords(id,user_id,name,secret_hash,created_at) VALUES(?,?,?,?,?)`), d.ID, d.UserID, d.Name, d.SecretHash, d.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) lockXMPPUser(ctx context.Context, tx *sql.Tx, userID string) error {
	result, err := tx.ExecContext(ctx, s.rebind(`UPDATE users SET id=id WHERE id=?`), userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func scanXMPPDevice(row interface{ Scan(...any) error }) (*XMPPDevicePassword, error) {
	var d XMPPDevicePassword
	err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.SecretHash, &d.CreatedAt, &d.LastUsedAt, &d.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}
func (s *Store) GetXMPPDevicePassword(ctx context.Context, userID, id string) (*XMPPDevicePassword, error) {
	return scanXMPPDevice(s.db.QueryRowContext(ctx, s.rebind(`SELECT id,user_id,name,secret_hash,created_at,last_used_at,revoked_at FROM xmpp_device_passwords WHERE user_id=? AND id=?`), userID, id))
}
func (s *Store) ListXMPPDevicePasswords(ctx context.Context, userID string) ([]XMPPDevicePassword, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,user_id,name,secret_hash,created_at,last_used_at,revoked_at FROM xmpp_device_passwords WHERE user_id=? ORDER BY CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END,created_at DESC,id LIMIT 100`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []XMPPDevicePassword{}
	for rows.Next() {
		d, err := scanXMPPDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}
func (s *Store) TouchXMPPDevicePassword(ctx context.Context, userID, id string) error {
	result, err := s.db.ExecContext(ctx, s.rebind(`UPDATE xmpp_device_passwords SET last_used_at=? WHERE user_id=? AND id=? AND revoked_at IS NULL`), time.Now().UTC(), userID, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) RevokeXMPPDevicePassword(ctx context.Context, userID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockXMPPUser(ctx, tx, userID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, s.rebind(`UPDATE xmpp_device_passwords SET revoked_at=COALESCE(revoked_at,?),secret_hash='' WHERE user_id=? AND id=?`), time.Now().UTC(), userID, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_push_registrations WHERE user_id=? AND credential_id=?`), userID, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) XMPPDevicesOnly(ctx context.Context, userID string) (bool, error) {
	var only bool
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT devices_only FROM xmpp_auth_policy WHERE user_id=?`), userID).Scan(&only)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return only, err
}
func (s *Store) SetXMPPDevicesOnly(ctx context.Context, userID string, only bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockXMPPUser(ctx, tx, userID); err != nil {
		return err
	}
	var current bool
	err = tx.QueryRowContext(ctx, s.rebind(`SELECT devices_only FROM xmpp_auth_policy WHERE user_id=?`), userID).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if only && !current {
		var count int
		if err = tx.QueryRowContext(ctx, s.rebind(`SELECT COUNT(*) FROM xmpp_device_passwords WHERE user_id=? AND revoked_at IS NULL AND last_used_at IS NOT NULL`), userID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return ErrXMPPDeviceNotTested
		}
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_auth_policy(user_id,devices_only) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET devices_only=excluded.devices_only`), userID, only); err != nil {
		return err
	}
	if only {
		if _, err = tx.ExecContext(ctx, s.rebind(`DELETE FROM xmpp_push_registrations WHERE user_id=? AND credential_id=''`), userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
