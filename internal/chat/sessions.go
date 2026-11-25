package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type Session struct {
	ID, UserID, TenantID, DomainID       string
	CreatedAt, ExpiresAt, LastActivityAt int64
	ObservedID                           string `json:"-"`
}

// OAuth expiry is left untouched. This record authorizes only the Mattermost
// adapter, and deleting the backing token revokes it atomically via the FK.
func (s *Service) RegisterSession(ctx context.Context, id, tenant, domain string, expires int64) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, s.q(`INSERT INTO chat_sessions(session_id,tenant_id,domain_id,expires_at,last_activity_at) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`), id, tenant, domain, expires, time.Now().UnixMilli())
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, s.q(`UPDATE chat_push_devices SET expires_at=(SELECT expires_at FROM chat_sessions WHERE session_id=?) WHERE session_id=?`), id, id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) Session(ctx context.Context, id string) (Session, error) {
	var v Session
	var token string
	e := s.db.QueryRowContext(ctx, s.q(`SELECT s.session_id,t.user_id,s.tenant_id,s.domain_id,s.expires_at,s.last_activity_at,t.access_token FROM chat_sessions s JOIN oauth_tokens t ON t.id=s.session_id WHERE s.session_id=?`), id).Scan(&v.ID, &v.UserID, &v.TenantID, &v.DomainID, &v.ExpiresAt, &v.LastActivityAt, &token)
	if e == nil {
		v.ObservedID = observedSessionID(token)
	}
	return v, e
}

func observedSessionID(token string) string {
	hash := sha256.Sum256([]byte("Web\x00" + token))
	return hex.EncodeToString(hash[:])
}
func (s *Service) TouchSession(ctx context.Context, id string, now int64) error {
	_, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_sessions SET last_activity_at=? WHERE session_id=? AND last_activity_at<? AND expires_at>?`), now, id, now-60000, now)
	return e
}

func (s *Service) Sessions(ctx context.Context, userID, tenant, domain string) ([]Session, error) {
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT s.session_id,t.user_id,s.tenant_id,s.domain_id,t.created_at,s.expires_at,s.last_activity_at FROM chat_sessions s JOIN oauth_tokens t ON t.id=s.session_id WHERE t.user_id=? AND s.tenant_id=? AND s.domain_id=? AND s.expires_at>? ORDER BY s.last_activity_at DESC`), userID, tenant, domain, time.Now().UnixMilli())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var v Session
		var created time.Time
		if e = rows.Scan(&v.ID, &v.UserID, &v.TenantID, &v.DomainID, &created, &v.ExpiresAt, &v.LastActivityAt); e != nil {
			return nil, e
		}
		v.CreatedAt = created.UnixMilli()
		out = append(out, v)
	}
	return out, rows.Err()
}

// ActiveSessions lists persistent Mattermost authorizations for administrative
// filtering by the caller's domain rights. It never returns access tokens.
func (s *Service) ActiveSessions(ctx context.Context) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT s.session_id,t.user_id,s.tenant_id,s.domain_id,t.created_at,s.expires_at,s.last_activity_at,t.access_token
  FROM chat_sessions s JOIN oauth_tokens t ON t.id=s.session_id
  WHERE s.expires_at>? ORDER BY s.last_activity_at DESC`), time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var v Session
		var created time.Time
		var token string
		if err := rows.Scan(&v.ID, &v.UserID, &v.TenantID, &v.DomainID, &created, &v.ExpiresAt, &v.LastActivityAt, &token); err != nil {
			return nil, err
		}
		v.CreatedAt = created.UnixMilli()
		v.ObservedID = observedSessionID(token)
		out = append(out, v)
	}
	return out, rows.Err()
}
