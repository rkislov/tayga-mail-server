package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"time"
)

// WebAuthnCredential is a registered passkey / security key for a user.
// Row primary key and user_id are UUIDs; credential_id is the authenticator's ID (base64url).
type WebAuthnCredential struct {
	ID             string
	UserID         string
	CredentialID   string // base64.RawURLEncoding of authenticator credential ID
	Name           string
	CredentialJSON string
	SignCount      uint32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WebAuthnSession stores ceremony SessionData between begin and finish.
type WebAuthnSession struct {
	ID          string
	UserID      string
	Kind        string // register | login
	SessionJSON string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

func EncodeCredentialID(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}

func DecodeCredentialID(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func (s *Store) CreateWebAuthnCredential(ctx context.Context, c *WebAuthnCredential) (*WebAuthnCredential, error) {
	now := time.Now().UTC()
	if c.ID == "" {
		c.ID = NewID()
	}
	c.CreatedAt = now
	c.UpdatedAt = now
	q := s.rebind(`INSERT INTO webauthn_credentials
		(id, user_id, credential_id, name, credential_json, sign_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, c.ID, c.UserID, c.CredentialID, c.Name, c.CredentialJSON, c.SignCount, now, now)
	return c, err
}

func (s *Store) ListWebAuthnCredentials(ctx context.Context, userID string) ([]*WebAuthnCredential, error) {
	q := s.rebind(`SELECT id, user_id, credential_id, name, credential_json, sign_count, created_at, updated_at
		FROM webauthn_credentials WHERE user_id = ? ORDER BY created_at`)
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*WebAuthnCredential
	for rows.Next() {
		c := &WebAuthnCredential{}
		var sc int64
		if err := rows.Scan(&c.ID, &c.UserID, &c.CredentialID, &c.Name, &c.CredentialJSON, &sc, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.SignCount = uint32(sc)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetWebAuthnCredentialByCredID(ctx context.Context, credentialID string) (*WebAuthnCredential, error) {
	q := s.rebind(`SELECT id, user_id, credential_id, name, credential_json, sign_count, created_at, updated_at
		FROM webauthn_credentials WHERE credential_id = ?`)
	c := &WebAuthnCredential{}
	var sc int64
	err := s.db.QueryRowContext(ctx, q, credentialID).Scan(
		&c.ID, &c.UserID, &c.CredentialID, &c.Name, &c.CredentialJSON, &sc, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.SignCount = uint32(sc)
	return c, nil
}

func (s *Store) GetWebAuthnCredential(ctx context.Context, id string) (*WebAuthnCredential, error) {
	q := s.rebind(`SELECT id, user_id, credential_id, name, credential_json, sign_count, created_at, updated_at
		FROM webauthn_credentials WHERE id = ?`)
	c := &WebAuthnCredential{}
	var sc int64
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.UserID, &c.CredentialID, &c.Name, &c.CredentialJSON, &sc, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.SignCount = uint32(sc)
	return c, nil
}

func (s *Store) UpdateWebAuthnCredential(ctx context.Context, c *WebAuthnCredential) error {
	now := time.Now().UTC()
	q := s.rebind(`UPDATE webauthn_credentials SET credential_json = ?, sign_count = ?, updated_at = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, c.CredentialJSON, c.SignCount, now, c.ID)
	return err
}

func (s *Store) DeleteWebAuthnCredential(ctx context.Context, userID, id string) error {
	q := s.rebind(`DELETE FROM webauthn_credentials WHERE id = ? AND user_id = ?`)
	res, err := s.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountWebAuthnCredentials(ctx context.Context, userID string) (int, error) {
	q := s.rebind(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`)
	var n int
	err := s.db.QueryRowContext(ctx, q, userID).Scan(&n)
	return n, err
}

func (s *Store) CreateWebAuthnSession(ctx context.Context, sess *WebAuthnSession) (*WebAuthnSession, error) {
	now := time.Now().UTC()
	if sess.ID == "" {
		sess.ID = NewID()
	}
	sess.CreatedAt = now
	q := s.rebind(`INSERT INTO webauthn_sessions (id, user_id, kind, session_json, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, sess.ID, sess.UserID, sess.Kind, sess.SessionJSON, sess.ExpiresAt, now)
	return sess, err
}

func (s *Store) GetWebAuthnSession(ctx context.Context, id string) (*WebAuthnSession, error) {
	q := s.rebind(`SELECT id, user_id, kind, session_json, expires_at, created_at FROM webauthn_sessions WHERE id = ?`)
	sess := &WebAuthnSession{}
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&sess.ID, &sess.UserID, &sess.Kind, &sess.SessionJSON, &sess.ExpiresAt, &sess.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return sess, err
}

func (s *Store) DeleteWebAuthnSession(ctx context.Context, id string) error {
	q := s.rebind(`DELETE FROM webauthn_sessions WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}
