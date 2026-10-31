package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
)

// These are authentication verifiers, never API response fields.
type XMPPSCRAMKey struct {
	CredentialID string `json:"-"`
	SourceHash   string `json:"-"`
	StoredKey    string `json:"-"`
	ServerKey    string `json:"-"`
}

// Authentication must see every active credential, including old devices that
// may be outside the paginated/recent history shown by the management API.
func (s *Store) ActiveXMPPDevicePasswords(ctx context.Context, userID string) ([]XMPPDevicePassword, error) {
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT id,user_id,name,secret_hash,created_at,last_used_at,revoked_at FROM xmpp_device_passwords WHERE user_id=? AND revoked_at IS NULL ORDER BY id`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []XMPPDevicePassword
	for rows.Next() {
		d, err := scanXMPPDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *d)
		if len(devices) > 50 {
			return nil, ErrXMPPDeviceLimit
		}
	}
	return devices, rows.Err()
}

// A single random salt per account allows independent device credentials to
// use the same XMPP login name. It survives primary password changes.
func (s *Store) EnsureXMPPSCRAMSalt(ctx context.Context, userID string) (string, error) {
	salt := make([]byte, 18)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	_, err := s.db.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_scram_accounts(user_id,salt) VALUES(?,?) ON CONFLICT(user_id) DO NOTHING`), userID, base64.StdEncoding.EncodeToString(salt))
	if err != nil {
		return "", err
	}
	var encoded string
	err = s.db.QueryRowContext(ctx, s.rebind(`SELECT salt FROM xmpp_scram_accounts WHERE user_id=?`), userID).Scan(&encoded)
	return encoded, err
}

func (s *Store) XMPPSCRAMKeys(ctx context.Context, userID string) (string, []XMPPSCRAMKey, error) {
	var salt string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT salt FROM xmpp_scram_accounts WHERE user_id=?`), userID).Scan(&salt)
	if err != nil {
		return "", nil, mapErr(err)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(`SELECT credential_id,source_hash,stored_key,server_key FROM xmpp_scram_keys WHERE user_id=? ORDER BY credential_id LIMIT 51`), userID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var keys []XMPPSCRAMKey
	for rows.Next() {
		var k XMPPSCRAMKey
		if err = rows.Scan(&k.CredentialID, &k.SourceHash, &k.StoredKey, &k.ServerKey); err != nil {
			return "", nil, err
		}
		keys = append(keys, k)
	}
	return salt, keys, rows.Err()
}

func (s *Store) putXMPPSCRAMKey(ctx context.Context, tx *sql.Tx, userID string, k XMPPSCRAMKey) error {
	_, err := tx.ExecContext(ctx, s.rebind(`INSERT INTO xmpp_scram_keys(user_id,credential_id,source_hash,stored_key,server_key) VALUES(?,?,?,?,?) ON CONFLICT(user_id,credential_id) DO UPDATE SET source_hash=excluded.source_hash,stored_key=excluded.stored_key,server_key=excluded.server_key`), userID, k.CredentialID, k.SourceHash, k.StoredKey, k.ServerKey)
	return err
}

// Conditional enrollment cannot resurrect a revoked credential or overwrite a
// newer password with a verifier computed by an earlier login.
func (s *Store) SaveXMPPSCRAMKey(ctx context.Context, userID string, k XMPPSCRAMKey) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockXMPPUser(ctx, tx, userID); err != nil {
		return err
	}
	var current string
	if k.CredentialID == "" {
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT password_hash FROM users WHERE id=? AND (auth_source='' OR auth_source='local')`), userID).Scan(&current)
	} else {
		err = tx.QueryRowContext(ctx, s.rebind(`SELECT secret_hash FROM xmpp_device_passwords WHERE user_id=? AND id=? AND revoked_at IS NULL`), userID, k.CredentialID).Scan(&current)
	}
	if err != nil {
		return mapErr(err)
	}
	if current != k.SourceHash {
		return ErrUnauthorized
	}
	if err = s.putXMPPSCRAMKey(ctx, tx, userID, k); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateUserPasswordWithSCRAM(ctx context.Context, userID, passwordHash string, k XMPPSCRAMKey) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = s.lockXMPPUser(ctx, tx, userID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, s.rebind(`UPDATE users SET password_hash=? WHERE id=?`), passwordHash, userID); err != nil {
		return err
	}
	k.CredentialID = ""
	k.SourceHash = passwordHash
	if err = s.putXMPPSCRAMKey(ctx, tx, userID, k); err != nil {
		return err
	}
	return tx.Commit()
}
