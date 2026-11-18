package chat

import (
	"context"
	"database/sql"
	"time"
)

type PushDevice struct {
	SessionID, UserID, TenantID, DomainID, DeviceID string
	Disabled                                        bool
	ExpiresAt                                       int64
}
type PushJob struct {
	PostID, SessionID string
	Attempts          int
	NextAt            int64
}

func (s *Service) RegisterPush(ctx context.Context, d PushDevice) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	// A device shared by multiple logins belongs to the latest authenticated
	// registration, preventing delivery for a previous account on that device.
	if _, e = tx.ExecContext(ctx, s.q(`DELETE FROM chat_push_devices WHERE device_id=? AND session_id<>?`), d.DeviceID, d.SessionID); e != nil {
		return e
	}
	disabled := 0
	if d.Disabled {
		disabled = 1
	}
	_, e = tx.ExecContext(ctx, s.q(`INSERT INTO chat_push_devices(session_id,user_id,tenant_id,domain_id,device_id,disabled,expires_at,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(session_id) DO UPDATE SET tenant_id=excluded.tenant_id,domain_id=excluded.domain_id,device_id=excluded.device_id,disabled=excluded.disabled,expires_at=excluded.expires_at,updated_at=excluded.updated_at`), d.SessionID, d.UserID, d.TenantID, d.DomainID, d.DeviceID, disabled, d.ExpiresAt, time.Now().UnixMilli())
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) RemovePush(ctx context.Context, sessionID string) error {
	_, e := s.db.ExecContext(ctx, s.q(`DELETE FROM chat_push_devices WHERE session_id=?`), sessionID)
	return e
}
func (s *Service) PushDevice(ctx context.Context, sessionID string) (PushDevice, error) {
	var d PushDevice
	var disabled int
	e := s.db.QueryRowContext(ctx, s.q(`SELECT d.session_id,d.user_id,d.tenant_id,d.domain_id,d.device_id,d.disabled,CASE WHEN s.expires_at<d.expires_at THEN s.expires_at ELSE d.expires_at END FROM chat_push_devices d JOIN oauth_tokens t ON t.id=d.session_id LEFT JOIN chat_sessions s ON s.session_id=d.session_id WHERE d.session_id=?`), sessionID).Scan(&d.SessionID, &d.UserID, &d.TenantID, &d.DomainID, &d.DeviceID, &disabled, &d.ExpiresAt)
	if e == sql.ErrNoRows {
		return d, ErrForbidden
	}
	d.Disabled = disabled != 0
	return d, e
}
func (s *Service) DuePush(ctx context.Context, now int64) ([]PushJob, error) {
	rows, e := s.db.QueryContext(ctx, s.q(`SELECT post_id,session_id,attempts,next_at FROM chat_push_jobs WHERE state='pending' AND next_at<=? ORDER BY next_at LIMIT 5`), now)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	list := []PushJob{}
	for rows.Next() {
		var j PushJob
		if e = rows.Scan(&j.PostID, &j.SessionID, &j.Attempts, &j.NextAt); e != nil {
			return nil, e
		}
		list = append(list, j)
	}
	return list, rows.Err()
}
func (s *Service) ClaimPush(ctx context.Context, j PushJob, now int64) (bool, error) {
	result, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_push_jobs SET next_at=? WHERE post_id=? AND session_id=? AND state='pending' AND next_at=?`), now+60000, j.PostID, j.SessionID, j.NextAt)
	if e != nil {
		return false, e
	}
	n, e := result.RowsAffected()
	return n == 1, e
}
func (s *Service) FinishPush(ctx context.Context, j PushJob, state, result string, next int64) error {
	finished := int64(0)
	if state != "pending" {
		finished = time.Now().UnixMilli()
	}
	_, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_push_jobs SET attempts=attempts+1,state=?,result=?,next_at=?,finished_at=? WHERE post_id=? AND session_id=?`), state, result, next, finished, j.PostID, j.SessionID)
	return e
}
func (s *Service) PushUnread(ctx context.Context, userID, channelID string, created int64) (bool, error) {
	var viewed int64
	e := s.db.QueryRowContext(ctx, s.q(`SELECT last_viewed_at FROM chat_members WHERE channel_id=? AND user_id=?`), channelID, userID).Scan(&viewed)
	if e == sql.ErrNoRows {
		return false, nil
	}
	return viewed < created, e
}
func (s *Service) CleanupPush(ctx context.Context, now int64) error {
	if _, e := s.db.ExecContext(ctx, s.q(`DELETE FROM chat_push_receipts WHERE created_at<?`), now-int64(7*24*time.Hour/time.Millisecond)); e != nil {
		return e
	}
	if _, e := s.db.ExecContext(ctx, s.q(`DELETE FROM chat_push_jobs WHERE (finished_at>0 AND finished_at<?) OR (state='pending' AND post_id IN (SELECT id FROM chat_posts WHERE created_at<?))`), now-int64(7*24*time.Hour/time.Millisecond), now-int64(24*time.Hour/time.Millisecond)); e != nil {
		return e
	}
	_, e := s.db.ExecContext(ctx, s.q(`DELETE FROM chat_push_devices WHERE expires_at<? OR NOT EXISTS(SELECT 1 FROM oauth_tokens t WHERE t.id=chat_push_devices.session_id)`), now)
	return e
}

// LegacyPushSessions exposes fingerprints instead of raw access tokens while
// migrating the previously staged registry into revocable session-bound rows.
func (s *Service) LegacyPushSessions(ctx context.Context) (map[string]PushDevice, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT t.id,t.user_id,t.access_token,t.expires_at,u.tenant_id,u.domain_id FROM oauth_tokens t JOIN users u ON u.id=t.user_id WHERE u.enabled=TRUE`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := map[string]PushDevice{}
	for rows.Next() {
		var d PushDevice
		var access string
		var expires time.Time
		if e = rows.Scan(&d.SessionID, &d.UserID, &access, &expires, &d.TenantID, &d.DomainID); e != nil {
			return nil, e
		}
		d.ExpiresAt = expires.UnixMilli()
		if d.ExpiresAt > time.Now().UnixMilli() {
			result["mattermost.push.device:"+ID(access)] = d
		}
	}
	return result, rows.Err()
}
func (s *Service) ClearLegacyPush(ctx context.Context) error {
	_, e := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key LIKE 'mattermost.push.device:%'`)
	return e
}

func (s *Service) InitSetting(ctx context.Context, key, value string) (string, error) {
	if existing, ok, e := s.store.GetSetting(ctx, key); e != nil {
		return "", e
	} else if ok {
		return existing, nil
	}
	if _, e := s.db.ExecContext(ctx, s.q(`INSERT INTO settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO NOTHING`), key, value, time.Now().UTC()); e != nil {
		return "", e
	}
	existing, _, e := s.store.GetSetting(ctx, key)
	return existing, e
}
func (s *Service) PushReceipt(ctx context.Context, id, sessionID, postID string) error {
	var post any
	if postID != "" {
		post = postID
	}
	_, e := s.db.ExecContext(ctx, s.q(`INSERT INTO chat_push_receipts(id,session_id,post_id,created_at) VALUES(?,?,?,?) ON CONFLICT(id) DO NOTHING`), id, sessionID, post, time.Now().UnixMilli())
	return e
}
func (s *Service) AckPushReceipt(ctx context.Context, id, postID string) (bool, error) {
	now := time.Now().UnixMilli()
	result, e := s.db.ExecContext(ctx, s.q(`UPDATE chat_push_receipts SET received_at=? WHERE id=? AND COALESCE(post_id,'')=? AND received_at=0 AND created_at>? AND session_id IN (SELECT d.session_id FROM chat_push_devices d JOIN oauth_tokens t ON t.id=d.session_id LEFT JOIN chat_sessions s ON s.session_id=d.session_id WHERE d.disabled=0 AND d.expires_at>? AND (s.session_id IS NULL OR s.expires_at>?))`), now, id, postID, now-int64(7*24*time.Hour/time.Millisecond), now, now)
	if e != nil {
		return false, e
	}
	n, e := result.RowsAffected()
	return n == 1, e
}
