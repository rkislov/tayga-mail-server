package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// UserMFA holds TOTP enrollment state for a user.
type UserMFA struct {
	UserID      string
	TOTPSecret  string
	TOTPEnabled bool
	BackupCodes string // JSON array of hashed codes
	UpdatedAt   time.Time
}

// OAuthToken is an opaque access/refresh token pair.
type OAuthToken struct {
	ID               string
	UserID           string
	AccessToken      string
	RefreshToken     string
	ExpiresAt        time.Time
	RefreshExpiresAt time.Time
	CreatedAt        time.Time
}

// MFAChallenge is a short-lived token issued after password verification
// when TOTP is required.
type MFAChallenge struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s *Store) GetUserMFA(ctx context.Context, userID string) (*UserMFA, error) {
	q := s.rebind(`SELECT user_id, totp_secret, totp_enabled, backup_codes, updated_at FROM user_mfa WHERE user_id = ?`)
	m := &UserMFA{}
	var enabled any
	err := s.db.QueryRowContext(ctx, q, userID).Scan(&m.UserID, &m.TOTPSecret, &enabled, &m.BackupCodes, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.TOTPEnabled = asBool(enabled)
	return m, nil
}

func (s *Store) UpsertUserMFA(ctx context.Context, m *UserMFA) error {
	now := time.Now().UTC()
	m.UpdatedAt = now
	enabled := 0
	if m.TOTPEnabled {
		enabled = 1
	}
	if s.dialect == DialectPostgres {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO user_mfa (user_id, totp_secret, totp_enabled, backup_codes, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id) DO UPDATE SET
				totp_secret = EXCLUDED.totp_secret,
				totp_enabled = EXCLUDED.totp_enabled,
				backup_codes = EXCLUDED.backup_codes,
				updated_at = EXCLUDED.updated_at`,
			m.UserID, m.TOTPSecret, m.TOTPEnabled, m.BackupCodes, now)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_mfa (user_id, totp_secret, totp_enabled, backup_codes, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			totp_secret = excluded.totp_secret,
			totp_enabled = excluded.totp_enabled,
			backup_codes = excluded.backup_codes,
			updated_at = excluded.updated_at`,
		m.UserID, m.TOTPSecret, enabled, m.BackupCodes, now)
	return err
}

func (s *Store) CreateOAuthToken(ctx context.Context, t *OAuthToken) (*OAuthToken, error) {
	now := time.Now().UTC()
	t.CreatedAt = now
	if t.ID == "" {
		t.ID = NewID()
	}
	q := s.rebind(`INSERT INTO oauth_tokens (id, user_id, access_token, refresh_token, expires_at, refresh_expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, t.ID, t.UserID, t.AccessToken, t.RefreshToken, t.ExpiresAt, t.RefreshExpiresAt, now)
	return t, err
}

func (s *Store) GetOAuthTokenByAccess(ctx context.Context, accessToken string) (*OAuthToken, error) {
	q := s.rebind(`SELECT id, user_id, access_token, refresh_token, expires_at, refresh_expires_at, created_at
		FROM oauth_tokens WHERE access_token = ?`)
	return s.scanOAuthToken(s.db.QueryRowContext(ctx, q, accessToken))
}

func (s *Store) GetOAuthTokenByRefresh(ctx context.Context, refreshToken string) (*OAuthToken, error) {
	q := s.rebind(`SELECT id, user_id, access_token, refresh_token, expires_at, refresh_expires_at, created_at
		FROM oauth_tokens WHERE refresh_token = ?`)
	return s.scanOAuthToken(s.db.QueryRowContext(ctx, q, refreshToken))
}

func (s *Store) DeleteOAuthToken(ctx context.Context, id string) error {
	q := s.rebind(`DELETE FROM oauth_tokens WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

func (s *Store) DeleteOAuthTokensByUser(ctx context.Context, userID string) error {
	q := s.rebind(`DELETE FROM oauth_tokens WHERE user_id = ?`)
	_, err := s.db.ExecContext(ctx, q, userID)
	return err
}

func (s *Store) scanOAuthToken(row interface{ Scan(dest ...any) error }) (*OAuthToken, error) {
	t := &OAuthToken{}
	err := row.Scan(&t.ID, &t.UserID, &t.AccessToken, &t.RefreshToken, &t.ExpiresAt, &t.RefreshExpiresAt, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) CreateMFAChallenge(ctx context.Context, c *MFAChallenge) error {
	c.CreatedAt = time.Now().UTC()
	q := s.rebind(`INSERT INTO mfa_challenges (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, q, c.Token, c.UserID, c.ExpiresAt, c.CreatedAt)
	return err
}

func (s *Store) GetMFAChallenge(ctx context.Context, token string) (*MFAChallenge, error) {
	q := s.rebind(`SELECT token, user_id, expires_at, created_at FROM mfa_challenges WHERE token = ?`)
	c := &MFAChallenge{}
	err := s.db.QueryRowContext(ctx, q, token).Scan(&c.Token, &c.UserID, &c.ExpiresAt, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) DeleteMFAChallenge(ctx context.Context, token string) error {
	q := s.rebind(`DELETE FROM mfa_challenges WHERE token = ?`)
	_, err := s.db.ExecContext(ctx, q, token)
	return err
}

func (s *Store) DeleteExpiredMFAChallenges(ctx context.Context, before time.Time) error {
	q := s.rebind(`DELETE FROM mfa_challenges WHERE expires_at < ?`)
	_, err := s.db.ExecContext(ctx, q, before)
	return err
}

// EncodeBackupCodes serializes hashed backup codes as JSON.
func EncodeBackupCodes(hashes []string) (string, error) {
	b, err := json.Marshal(hashes)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeBackupCodes parses hashed backup codes from JSON.
func DecodeBackupCodes(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
