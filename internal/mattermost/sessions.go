package mattermost

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

// Native Mattermost clients retain a session token, not an OAuth refresh pair.
// Its persisted authorization is specific to this adapter. General TMS OAuth
// validation keeps the original access expiry and never accepts this fallback.
func (s *API) authenticate(ctx context.Context, token string) (*storage.User, error) {
	if strings.HasPrefix(token, "tmsbot_") {
		return s.authenticateBot(ctx, token)
	}
	if token == "" {
		return nil, auth.ErrInvalidToken
	}
	backing, e := s.store.GetOAuthTokenByAccess(ctx, token)
	if e != nil {
		return nil, e
	}
	session, e := s.chat.Session(ctx, backing.ID)
	if errors.Is(e, sql.ErrNoRows) {
		u, err := s.auth.AuthenticateToken(ctx, "", token)
		if err != nil {
			return nil, err
		}
		// Only live, successfully authenticated credentials may acquire an adapter
		// session. Already expired, unregistered tokens can never be revived.
		expiry := backing.RefreshExpiresAt
		if cap := backing.CreatedAt.Add(30 * 24 * time.Hour); expiry.After(cap) {
			expiry = cap
		}
		if !expiry.After(time.Now()) {
			return nil, auth.ErrTokenExpired
		}
		if err = s.chat.RegisterSession(ctx, backing.ID, u.TenantID, u.DomainID, expiry.UnixMilli()); err != nil {
			return nil, err
		}
		session, e = s.chat.Session(ctx, backing.ID)
	}
	if e != nil {
		return nil, e
	}
	if session.ExpiresAt <= time.Now().UnixMilli() {
		return nil, auth.ErrTokenExpired
	}
	u, e := s.store.GetUserByID(ctx, backing.UserID)
	if e != nil {
		return nil, e
	}
	if !u.Enabled || u.TenantID != session.TenantID || u.DomainID != session.DomainID {
		return nil, auth.ErrInvalidToken
	}
	if e = s.chat.TouchSession(ctx, backing.ID, time.Now().UnixMilli()); e != nil {
		return nil, e
	}
	return u, nil
}

func (s *API) sessionModels(ctx context.Context, u *storage.User, accessToken string) ([]any, error) {
	current, err := s.store.GetOAuthTokenByAccess(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	list, e := s.chat.Sessions(ctx, u.ID, u.TenantID, u.DomainID)
	if e != nil {
		return nil, e
	}
	out := []any{}
	for _, v := range list {
		out = append(out, map[string]any{"id": chat.ID(v.ID), "is_current": v.ID == current.ID, "user_id": chat.ID(u.ID), "create_at": v.CreatedAt, "last_activity_at": v.LastActivityAt, "expires_at": v.ExpiresAt, "device_id": "", "roles": "system_user", "is_oauth": false, "props": map[string]string{}})
	}
	return out, nil
}
func (s *API) revokeSessionHTTP(w http.ResponseWriter, r *http.Request, u *storage.User) {
	if r.Method != http.MethodPost {
		wrongMethod(w)
		return
	}
	var body struct {
		ID string `json:"session_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	list, e := s.chat.Sessions(r.Context(), u.ID, u.TenantID, u.DomainID)
	if e != nil {
		s.failure(w, e)
		return
	}
	for _, v := range list {
		if chat.ID(v.ID) == body.ID {
			if e = s.store.DeleteOAuthToken(r.Context(), v.ID); e != nil {
				s.failure(w, e)
				return
			}
			write(w, 200, map[string]string{"status": "OK"})
			return
		}
	}
	s.failure(w, storage.ErrNotFound)
}
