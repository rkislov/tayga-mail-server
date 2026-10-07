package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

var (
	ErrWebAuthnDisabled = errors.New("webauthn disabled")
	ErrWebAuthnSession  = errors.New("invalid webauthn session")
)

// WebAuthnService wraps go-webauthn with UUID-backed credential/session storage.
type WebAuthnService struct {
	Store  storage.Driver
	Tokens *TokenService
	WA     *webauthn.WebAuthn
	Cfg    config.WebAuthnConfig
	TTL    time.Duration
}

// NewWebAuthnService builds a WebAuthn relying party from config (no-op WA if disabled).
func NewWebAuthnService(store storage.Driver, tokens *TokenService, mfaCfg config.MFAConfig) (*WebAuthnService, error) {
	svc := &WebAuthnService{
		Store:  store,
		Tokens: tokens,
		Cfg:    mfaCfg.WebAuthn,
		TTL:    mfaCfg.ChallengeTTL,
	}
	if svc.TTL <= 0 {
		svc.TTL = 5 * time.Minute
	}
	if !mfaCfg.WebAuthn.Enabled {
		return svc, nil
	}
	if mfaCfg.WebAuthn.RPID == "" || len(mfaCfg.WebAuthn.RPOrigins) == 0 {
		return nil, errors.New("webauthn: rp_id and rp_origins required when enabled")
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: mfaCfg.WebAuthn.RPDisplayName,
		RPID:          mfaCfg.WebAuthn.RPID,
		RPOrigins:     mfaCfg.WebAuthn.RPOrigins,
	})
	if err != nil {
		return nil, err
	}
	svc.WA = wa
	return svc, nil
}

func (s *WebAuthnService) Enabled() bool {
	return s != nil && s.Cfg.Enabled && s.WA != nil
}

type waUser struct {
	id    []byte
	name  string
	disp  string
	creds []webauthn.Credential
}

func (u *waUser) WebAuthnID() []byte                         { return u.id }
func (u *waUser) WebAuthnName() string                       { return u.name }
func (u *waUser) WebAuthnDisplayName() string                { return u.disp }
func (u *waUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func userHandle(userID string) ([]byte, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	b := id[:]
	out := make([]byte, 16)
	copy(out, b[:])
	return out, nil
}

func (s *WebAuthnService) loadUser(ctx context.Context, u *storage.User) (*waUser, error) {
	handle, err := userHandle(u.ID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListWebAuthnCredentials(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(row.CredentialJSON), &c); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	disp := u.DisplayName
	if disp == "" {
		disp = u.Email
	}
	return &waUser{id: handle, name: u.Email, disp: disp, creds: creds}, nil
}

func (s *WebAuthnService) saveSession(ctx context.Context, userID, kind string, sd *webauthn.SessionData) (string, error) {
	raw, err := json.Marshal(sd)
	if err != nil {
		return "", err
	}
	exp := sd.Expires
	if exp.IsZero() {
		exp = time.Now().UTC().Add(s.TTL)
	}
	sess, err := s.Store.CreateWebAuthnSession(ctx, &storage.WebAuthnSession{
		UserID:      userID,
		Kind:        kind,
		SessionJSON: string(raw),
		ExpiresAt:   exp,
	})
	if err != nil {
		return "", err
	}
	return sess.ID, nil
}

func (s *WebAuthnService) loadSession(ctx context.Context, sessionID, kind, userID string) (*webauthn.SessionData, error) {
	sess, err := s.Store.GetWebAuthnSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrWebAuthnSession
		}
		return nil, err
	}
	if sess.Kind != kind || sess.UserID != userID {
		return nil, ErrWebAuthnSession
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.Store.DeleteWebAuthnSession(ctx, sessionID)
		return nil, ErrTokenExpired
	}
	var sd webauthn.SessionData
	if err := json.Unmarshal([]byte(sess.SessionJSON), &sd); err != nil {
		return nil, err
	}
	return &sd, nil
}

// BeginRegistration starts a WebAuthn registration ceremony for an authenticated user.
func (s *WebAuthnService) BeginRegistration(ctx context.Context, u *storage.User) (options any, sessionID string, err error) {
	if !s.Enabled() {
		return nil, "", ErrWebAuthnDisabled
	}
	wu, err := s.loadUser(ctx, u)
	if err != nil {
		return nil, "", err
	}
	creation, sd, err := s.WA.BeginRegistration(wu,
		webauthn.WithExclusions(webauthn.Credentials(wu.creds).CredentialDescriptors()),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
	)
	if err != nil {
		return nil, "", err
	}
	sid, err := s.saveSession(ctx, u.ID, "register", sd)
	if err != nil {
		return nil, "", err
	}
	return creation, sid, nil
}

// FinishRegistration completes registration and stores the credential under a UUID PK.
func (s *WebAuthnService) FinishRegistration(ctx context.Context, u *storage.User, sessionID, name string, credentialJSON []byte) (*storage.WebAuthnCredential, error) {
	if !s.Enabled() {
		return nil, ErrWebAuthnDisabled
	}
	sd, err := s.loadSession(ctx, sessionID, "register", u.ID)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credentialJSON)
	if err != nil {
		return nil, err
	}
	wu, err := s.loadUser(ctx, u)
	if err != nil {
		return nil, err
	}
	cred, err := s.WA.CreateCredential(wu, *sd, parsed)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = "Passkey"
	}
	row, err := s.Store.CreateWebAuthnCredential(ctx, &storage.WebAuthnCredential{
		UserID:         u.ID,
		CredentialID:   storage.EncodeCredentialID(cred.ID),
		Name:           name,
		CredentialJSON: string(raw),
		SignCount:      cred.Authenticator.SignCount,
	})
	if err != nil {
		return nil, err
	}
	_ = s.Store.DeleteWebAuthnSession(ctx, sessionID)
	return row, nil
}

// BeginLogin starts assertion for MFA after a password challenge.
func (s *WebAuthnService) BeginLogin(ctx context.Context, challenge string) (options any, sessionID string, err error) {
	if !s.Enabled() {
		return nil, "", ErrWebAuthnDisabled
	}
	u, err := s.userFromMFAChallenge(ctx, challenge)
	if err != nil {
		return nil, "", err
	}
	wu, err := s.loadUser(ctx, u)
	if err != nil {
		return nil, "", err
	}
	if len(wu.creds) == 0 {
		return nil, "", ErrInvalidCredentials
	}
	assertion, sd, err := s.WA.BeginLogin(wu)
	if err != nil {
		return nil, "", err
	}
	sid, err := s.saveSession(ctx, u.ID, "login", sd)
	if err != nil {
		return nil, "", err
	}
	return assertion, sid, nil
}

// FinishLogin verifies assertion against the MFA challenge and issues tokens.
func (s *WebAuthnService) FinishLogin(ctx context.Context, challenge, sessionID string, credentialJSON []byte) (*TokenPair, error) {
	if !s.Enabled() {
		return nil, ErrWebAuthnDisabled
	}
	u, err := s.userFromMFAChallenge(ctx, challenge)
	if err != nil {
		return nil, err
	}
	sd, err := s.loadSession(ctx, sessionID, "login", u.ID)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credentialJSON)
	if err != nil {
		return nil, err
	}
	wu, err := s.loadUser(ctx, u)
	if err != nil {
		return nil, err
	}
	cred, err := s.WA.ValidateLogin(wu, *sd, parsed)
	if err != nil {
		return nil, err
	}
	if err := s.persistCredentialUpdate(ctx, u.ID, cred); err != nil {
		return nil, err
	}
	_ = s.Store.DeleteWebAuthnSession(ctx, sessionID)
	_ = s.Store.DeleteMFAChallenge(ctx, challenge)
	return s.Tokens.IssueTokens(ctx, u.ID)
}

func (s *WebAuthnService) persistCredentialUpdate(ctx context.Context, userID string, cred *webauthn.Credential) error {
	row, err := s.Store.GetWebAuthnCredentialByCredID(ctx, storage.EncodeCredentialID(cred.ID))
	if err != nil {
		return err
	}
	if row.UserID != userID {
		return ErrInvalidCredentials
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	row.CredentialJSON = string(raw)
	row.SignCount = cred.Authenticator.SignCount
	return s.Store.UpdateWebAuthnCredential(ctx, row)
}

func (s *WebAuthnService) userFromMFAChallenge(ctx context.Context, challenge string) (*storage.User, error) {
	ch, err := s.Store.GetMFAChallenge(ctx, challenge)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if time.Now().UTC().After(ch.ExpiresAt) {
		_ = s.Store.DeleteMFAChallenge(ctx, challenge)
		return nil, ErrTokenExpired
	}
	return s.Store.GetUserByID(ctx, ch.UserID)
}

// ListCredentials returns UUID-keyed credential metadata for the user.
func (s *WebAuthnService) ListCredentials(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.Store.ListWebAuthnCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":         r.ID,
			"name":       r.Name,
			"created_at": r.CreatedAt,
		})
	}
	return out, nil
}

// DeleteCredential removes a credential by its UUID primary key.
func (s *WebAuthnService) DeleteCredential(ctx context.Context, userID, id string) error {
	return s.Store.DeleteWebAuthnCredential(ctx, userID, id)
}

// DeriveRPIDFromPublicURL is a helper for tests/config.
func DeriveRPIDFromPublicURL(publicURL string) (rpID string, origin string, err error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return "", "", err
	}
	return u.Hostname(), u.Scheme + "://" + u.Host, nil
}
