package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

// MFAService handles TOTP enrollment and challenge verification.
type MFAService struct {
	Store  storage.Driver
	Tokens *TokenService
	Issuer string
	TTL    time.Duration
}

// BeginSetup creates (or replaces) a pending TOTP secret. Does not enable MFA yet.
func (m *MFAService) BeginSetup(ctx context.Context, userID, email string) (secret, uri string, backupPlain []string, err error) {
	secret, err = GenerateTOTPSecret()
	if err != nil {
		return "", "", nil, err
	}
	plain, hashes, err := GenerateBackupCodes(8)
	if err != nil {
		return "", "", nil, err
	}
	raw, err := storage.EncodeBackupCodes(hashes)
	if err != nil {
		return "", "", nil, err
	}
	if err := m.Store.UpsertUserMFA(ctx, &storage.UserMFA{
		UserID:      userID,
		TOTPSecret:  secret,
		TOTPEnabled: false,
		BackupCodes: raw,
	}); err != nil {
		return "", "", nil, err
	}
	return secret, TOTPURI(m.Issuer, email, secret), plain, nil
}

// ConfirmSetup enables TOTP after the user proves they can generate a valid code.
func (m *MFAService) ConfirmSetup(ctx context.Context, userID, code string) error {
	mfa, err := m.Store.GetUserMFA(ctx, userID)
	if err != nil {
		return err
	}
	if mfa.TOTPSecret == "" {
		return ErrInvalidMFACode
	}
	if !ValidateTOTP(mfa.TOTPSecret, code) {
		return ErrInvalidMFACode
	}
	mfa.TOTPEnabled = true
	return m.Store.UpsertUserMFA(ctx, mfa)
}

// Disable turns off TOTP for the user (requires a valid code or backup).
func (m *MFAService) Disable(ctx context.Context, userID, code string) error {
	mfa, err := m.Store.GetUserMFA(ctx, userID)
	if err != nil {
		return err
	}
	if !m.consumeCode(ctx, mfa, code) {
		return ErrInvalidMFACode
	}
	mfa.TOTPEnabled = false
	mfa.TOTPSecret = ""
	mfa.BackupCodes = "[]"
	return m.Store.UpsertUserMFA(ctx, mfa)
}

// IsEnabled reports whether the user has TOTP and/or WebAuthn credentials.
func (m *MFAService) IsEnabled(ctx context.Context, userID string) bool {
	if mfa, err := m.Store.GetUserMFA(ctx, userID); err == nil && mfa.TOTPEnabled {
		return true
	}
	n, err := m.Store.CountWebAuthnCredentials(ctx, userID)
	return err == nil && n > 0
}

// MFAMethods returns which second factors the user can use.
func (m *MFAService) MFAMethods(ctx context.Context, userID string) []string {
	var methods []string
	if mfa, err := m.Store.GetUserMFA(ctx, userID); err == nil && mfa.TOTPEnabled {
		methods = append(methods, "totp")
	}
	if n, err := m.Store.CountWebAuthnCredentials(ctx, userID); err == nil && n > 0 {
		methods = append(methods, "webauthn")
	}
	return methods
}

// LoginWithPassword verifies password; if MFA is on, returns a challenge.
func (l *Layer) LoginWithPassword(ctx context.Context, username, password string) (*LoginResult, error) {
	u, err := l.verifyPassword(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if l.MFA != nil && l.MFA.IsEnabled(ctx, u.ID) {
		ch, err := randomToken(24)
		if err != nil {
			return nil, err
		}
		ttl := l.Tokens.Cfg.ChallengeTTL
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}
		if err := l.Store.CreateMFAChallenge(ctx, &storage.MFAChallenge{
			Token:     ch,
			UserID:    u.ID,
			ExpiresAt: time.Now().UTC().Add(ttl),
		}); err != nil {
			return nil, err
		}
		return &LoginResult{
			MFARequired: true,
			Challenge:   ch,
			UserEmail:   u.Email,
			Methods:     l.MFA.MFAMethods(ctx, u.ID),
		}, nil
	}
	pair, err := l.Tokens.IssueTokens(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Tokens: pair, UserEmail: u.Email}, nil
}

// CompleteMFA verifies a TOTP/backup code against a challenge and issues tokens.
func (l *Layer) CompleteMFA(ctx context.Context, challenge, code string) (*TokenPair, error) {
	ch, err := l.Store.GetMFAChallenge(ctx, challenge)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if time.Now().UTC().After(ch.ExpiresAt) {
		_ = l.Store.DeleteMFAChallenge(ctx, challenge)
		return nil, ErrTokenExpired
	}
	mfa, err := l.Store.GetUserMFA(ctx, ch.UserID)
	if err != nil || !mfa.TOTPEnabled {
		return nil, ErrInvalidMFACode
	}
	if l.MFA == nil || !l.MFA.consumeCode(ctx, mfa, code) {
		return nil, ErrInvalidMFACode
	}
	_ = l.Store.DeleteMFAChallenge(ctx, challenge)
	return l.Tokens.IssueTokens(ctx, ch.UserID)
}

func (m *MFAService) consumeCode(ctx context.Context, mfa *storage.UserMFA, code string) bool {
	if ValidateTOTP(mfa.TOTPSecret, code) {
		return true
	}
	hashes, err := storage.DecodeBackupCodes(mfa.BackupCodes)
	if err != nil {
		return false
	}
	remaining, ok := ConsumeBackupCode(hashes, code)
	if !ok {
		return false
	}
	raw, err := storage.EncodeBackupCodes(remaining)
	if err != nil {
		return false
	}
	mfa.BackupCodes = raw
	_ = m.Store.UpsertUserMFA(ctx, mfa)
	return true
}

// verifyPassword checks credentials without MFA gating (used by web login).
func (l *Layer) verifyPassword(ctx context.Context, username, password string) (*storage.User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	domain := ""
	if at := strings.LastIndex(username, "@"); at >= 0 {
		domain = username[at+1:]
	}

	u, err := l.Store.GetUserByEmail(ctx, username)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		if l.dir != nil && l.dir.EnabledFor(domain) {
			return l.LDAP.Authenticate(ctx, domain, username, password)
		}
		return nil, ErrInvalidCredentials
	}
	if !u.Enabled {
		return nil, ErrUserDisabled
	}
	switch u.AuthSource {
	case "", "local":
		ok, err := l.Hasher.Verify(u.PasswordHash, password)
		if err != nil || !ok {
			return nil, ErrInvalidCredentials
		}
		return u, nil
	case "ldap":
		return l.LDAP.Authenticate(ctx, domain, username, password)
	case "oidc", "resource":
		return nil, ErrInvalidCredentials
	default:
		return nil, ErrUnsupportedSource
	}
}
