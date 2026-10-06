package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

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
	Store  storage.Driver
	Hasher PasswordHasher
	LDAP   LDAPProvider
	OIDC   OIDCProvider
}

// LDAPProvider is a per-domain LDAP authenticator stub/interface.
type LDAPProvider interface {
	Authenticate(ctx context.Context, domain, username, password string) (*storage.User, error)
}

// OIDCProvider is reserved for XOAUTH2 / token validation (stub).
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

func NewLayer(store storage.Driver) *Layer {
	return &Layer{
		Store:  store,
		Hasher: Argon2id{},
		LDAP:   StubLDAP{},
		OIDC:   StubOIDC{},
	}
}

func (l *Layer) Authenticate(ctx context.Context, username, password string) (*storage.User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	u, err := l.Store.GetUserByEmail(ctx, username)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
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
		at := strings.LastIndex(username, "@")
		domain := ""
		if at >= 0 {
			domain = username[at+1:]
		}
		return l.LDAP.Authenticate(ctx, domain, username, password)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSource, u.AuthSource)
	}
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
