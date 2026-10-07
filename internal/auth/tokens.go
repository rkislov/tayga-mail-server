package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

var (
	ErrMFARequired    = errors.New("mfa required")
	ErrInvalidToken   = errors.New("invalid token")
	ErrTokenExpired   = errors.New("token expired")
	ErrInvalidMFACode = errors.New("invalid mfa code")
)

// TokenPair is issued after successful (password+MFA or OIDC) login.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// LoginResult is returned by web login.
type LoginResult struct {
	Tokens      *TokenPair `json:"tokens,omitempty"`
	MFARequired bool       `json:"mfa_required"`
	Challenge   string     `json:"challenge,omitempty"`
	Methods     []string   `json:"methods,omitempty"` // totp, webauthn
	UserEmail   string     `json:"email,omitempty"`
}

// TokenService issues and validates opaque OAuth tokens for XOAUTH2/OAUTHBEARER.
type TokenService struct {
	Store storage.Driver
	Cfg   config.MFAConfig
}

// IssueTokens creates a new access/refresh pair for userID.
func (t *TokenService) IssueTokens(ctx context.Context, userID string) (*TokenPair, error) {
	access, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = t.Store.CreateOAuthToken(ctx, &storage.OAuthToken{
		UserID:           userID,
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresAt:        now.Add(t.Cfg.AccessTokenTTL),
		RefreshExpiresAt: now.Add(t.Cfg.RefreshTokenTTL),
	})
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(t.Cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// ValidateAccessToken implements OIDCProvider for protocol auth.
func (t *TokenService) ValidateAccessToken(ctx context.Context, token string) (*storage.User, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	ot, err := t.Store.GetOAuthTokenByAccess(ctx, token)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if time.Now().UTC().After(ot.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	u, err := t.Store.GetUserByID(ctx, ot.UserID)
	if err != nil {
		return nil, err
	}
	if !u.Enabled {
		return nil, ErrUserDisabled
	}
	return u, nil
}

// Refresh exchanges a refresh token for a new pair (rotating).
func (t *TokenService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	ot, err := t.Store.GetOAuthTokenByRefresh(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if time.Now().UTC().After(ot.RefreshExpiresAt) {
		_ = t.Store.DeleteOAuthToken(ctx, ot.ID)
		return nil, ErrTokenExpired
	}
	_ = t.Store.DeleteOAuthToken(ctx, ot.ID)
	return t.IssueTokens(ctx, ot.UserID)
}

// RevokeAccess deletes the token row for the given access token.
func (t *TokenService) RevokeAccess(ctx context.Context, accessToken string) error {
	ot, err := t.Store.GetOAuthTokenByAccess(ctx, accessToken)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return err
	}
	return t.Store.DeleteOAuthToken(ctx, ot.ID)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
