package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user disabled")
	ErrUnsupportedSource  = errors.New("unsupported auth source")
)

// Authenticator verifies credentials for protocol and web login.
type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (*storage.User, error)
}

// PasswordHasher hashes and verifies local passwords (Argon2id).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encodedHash, password string) (bool, error)
}

// Layer composes local auth with optional LDAP/OIDC providers per domain.
type Layer struct {
	sessions SessionRegistry
	Store    storage.Driver
	Hasher   PasswordHasher
	LDAP     LDAPProvider
	OIDC     OIDCProvider
	Tokens   *TokenService
	MFA      *MFAService
	WebAuthn *WebAuthnService
	OIDCCl   *OIDCClient
	mfaCfg   config.MFAConfig
	dir      *Directory // optional; used for EnabledFor checks
}

// LDAPProvider is a per-domain LDAP authenticator.
type LDAPProvider interface {
	Authenticate(ctx context.Context, domain, username, password string) (*storage.User, error)
}

// OIDCProvider validates opaque access tokens (XOAUTH2 / OAUTHBEARER).
type OIDCProvider interface {
	ValidateAccessToken(ctx context.Context, token string) (*storage.User, error)
}

// StubLDAP always returns ErrUnsupportedSource.
type StubLDAP struct{}

func (StubLDAP) Authenticate(context.Context, string, string, string) (*storage.User, error) {
	return nil, ErrUnsupportedSource
}

// StubOIDC always returns ErrUnsupportedSource.
type StubOIDC struct{}

func (StubOIDC) ValidateAccessToken(context.Context, string) (*storage.User, error) {
	return nil, ErrUnsupportedSource
}

// NewLayer builds an auth layer with LDAP, MFA, WebAuthn, and token services.
func NewLayer(store storage.Driver, ldapCfg config.LDAPConfig, mfaCfg config.MFAConfig, oidcCfg config.OIDCConfig) *Layer {
	tokens := &TokenService{Store: store, Cfg: mfaCfg}
	mfa := &MFAService{
		Store:  store,
		Tokens: tokens,
		Issuer: mfaCfg.Issuer,
		TTL:    mfaCfg.ChallengeTTL,
	}
	wa, err := NewWebAuthnService(store, tokens, mfaCfg)
	if err != nil {
		// Misconfiguration: leave WebAuthn nil; callers see ErrWebAuthnDisabled.
		wa = &WebAuthnService{Store: store, Tokens: tokens, Cfg: mfaCfg.WebAuthn, TTL: mfaCfg.ChallengeTTL}
	}
	l := &Layer{
		Store:    store,
		Hasher:   Argon2id{},
		OIDC:     tokens,
		Tokens:   tokens,
		MFA:      mfa,
		WebAuthn: wa,
		mfaCfg:   mfaCfg,
	}
	l.OIDCCl = NewOIDCClient(store, tokens, oidcCfg)
	if len(ldapCfg.Domains) > 0 {
		dir := NewDirectory(store, ldapCfg)
		l.LDAP = dir
		l.dir = dir
	} else {
		l.LDAP = StubLDAP{}
	}
	return l
}

// Authenticate verifies username/password for IMAP/SMTP/POP3/ManageSieve.
// Users with TOTP enabled are rejected when RequireTokenForMFAUsers is set
// (they must use XOAUTH2/OAUTHBEARER with an access token from the HTTP API).
func (l *Layer) Authenticate(ctx context.Context, username, password string) (*storage.User, error) {
	u, err := l.verifyPassword(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if l.mfaCfg.RequireTokenForMFAUsers && l.MFA != nil && l.MFA.IsEnabled(ctx, u.ID) {
		return nil, ErrMFARequired
	}
	return u, nil
}

// AuthenticateToken validates an OAuth access token for protocol SASL.
func (l *Layer) AuthenticateToken(ctx context.Context, username, token string) (*storage.User, error) {
	u, err := l.OIDC.ValidateAccessToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if username != "" && !strings.EqualFold(username, u.Email) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// Argon2id implements PasswordHasher using the encoded PHC string format.
type Argon2id struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

func (a Argon2id) params() (time, memory uint32, threads uint8, keyLen uint32) {
	time = a.Time
	if time == 0 {
		time = 1
	}
	memory = a.Memory
	if memory == 0 {
		memory = 64 * 1024
	}
	threads = a.Threads
	if threads == 0 {
		threads = 4
	}
	keyLen = a.KeyLen
	if keyLen == 0 {
		keyLen = 32
	}
	return
}

func (a Argon2id) Hash(password string) (string, error) {
	time, memory, threads, keyLen := a.params()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, time, threads, b64Salt, b64Hash), nil
}

func (a Argon2id) Verify(encodedHash, password string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	// "", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid argon2id hash format")
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
