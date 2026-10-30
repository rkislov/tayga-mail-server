package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tayga/tms/internal/storage"
)

const XMPPDevicePasswordPrefix = "tayga-xmpp."

var ErrInvalidDeviceName = errors.New("device name must contain 1 to 80 characters")

type xmppAccessLock struct {
	mu   sync.RWMutex
	refs int
}

func (a *Layer) lockXMPPAccess(userID string, write bool) func() {
	a.xmppMu.Lock()
	if a.xmppLocks == nil {
		a.xmppLocks = map[string]*xmppAccessLock{}
	}
	l := a.xmppLocks[userID]
	if l == nil {
		l = &xmppAccessLock{}
		a.xmppLocks[userID] = l
	}
	l.refs++
	a.xmppMu.Unlock()
	if write {
		l.mu.Lock()
	} else {
		l.mu.RLock()
	}
	return func() {
		if write {
			l.mu.Unlock()
		} else {
			l.mu.RUnlock()
		}
		a.xmppMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(a.xmppLocks, userID)
		}
		a.xmppMu.Unlock()
	}
}

// The secret has 256 bits of CSPRNG entropy. Only its SHA-256 digest is stored;
// it is not a user-chosen password and never participates in ordinary login.
func (a *Layer) CreateXMPPDevicePassword(ctx context.Context, userID, name string) (*storage.XMPPDevicePassword, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return nil, "", ErrInvalidDeviceName
	}
	user, err := a.Store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if !user.Enabled {
		return nil, "", ErrUserDisabled
	}
	secret, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	id := storage.NewID()
	password := XMPPDevicePasswordPrefix + id + "." + secret
	sum := sha256.Sum256([]byte(password))
	device := &storage.XMPPDevicePassword{ID: id, UserID: userID, Name: name, SecretHash: hex.EncodeToString(sum[:]), CreatedAt: time.Now().UTC()}
	if err = a.Store.CreateXMPPDevicePassword(ctx, device); err != nil {
		return nil, "", err
	}
	return device, password, nil
}

// AuthenticateXMPP permits scoped credentials only at the XMPP SASL boundary.
// The final check when registering the connection closes an auth/revoke race.
func (a *Layer) AuthenticateXMPP(ctx context.Context, username, password string) (*storage.User, string, error) {
	if !strings.HasPrefix(password, XMPPDevicePasswordPrefix) {
		u, err := a.Authenticate(ctx, username, password)
		return u, "", err
	}
	parts := strings.Split(password, ".")
	if len(parts) != 3 || len(parts[1]) != 36 || len(parts[2]) != 43 {
		return nil, "", ErrInvalidCredentials
	}
	u, err := a.Store.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil || !u.Enabled {
		return nil, "", ErrInvalidCredentials
	}
	d, err := a.Store.GetXMPPDevicePassword(ctx, u.ID, parts[1])
	if err != nil || d.RevokedAt != nil {
		return nil, "", ErrInvalidCredentials
	}
	sum := sha256.Sum256([]byte(password))
	want, err := hex.DecodeString(d.SecretHash)
	if err != nil || subtle.ConstantTimeCompare(want, sum[:]) != 1 {
		return nil, "", ErrInvalidCredentials
	}
	return u, d.ID, nil
}

func (a *Layer) checkXMPPAccess(ctx context.Context, userID, credentialID string) error {
	u, err := a.Store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !u.Enabled {
		return ErrUserDisabled
	}
	if credentialID != "" {
		d, err := a.Store.GetXMPPDevicePassword(ctx, userID, credentialID)
		if err != nil || d.RevokedAt != nil {
			return ErrInvalidCredentials
		}
		return nil
	}
	only, err := a.Store.XMPPDevicesOnly(ctx, userID)
	if err != nil {
		return err
	}
	if only {
		return ErrInvalidCredentials
	}
	return nil
}

// Used around a short authenticated state transition, not around read loops.
// Revocation commits before closing captured connections outside this lock.
func (a *Layer) WithXMPPAccess(ctx context.Context, userID, credentialID string, fn func() error) error {
	unlock := a.lockXMPPAccess(userID, false)
	defer unlock()
	if err := a.checkXMPPAccess(ctx, userID, credentialID); err != nil {
		return err
	}
	return fn()
}
func (a *Layer) TrackXMPPSession(ctx context.Context, user *storage.User, credentialID, address, agent, key string, close func() error) (string, error) {
	var id string
	err := a.WithXMPPAccess(ctx, user.ID, credentialID, func() error {
		if credentialID != "" {
			if err := a.Store.TouchXMPPDevicePassword(ctx, user.ID, credentialID); err != nil {
				return err
			}
		}
		id = a.TrackSession(user, "XMPP", address, agent, key, true, close)
		a.sessions.mu.Lock()
		entry := a.sessions.items[id]
		entry.CredentialID = credentialID
		a.sessions.items[id] = entry
		a.sessions.mu.Unlock()
		return nil
	})
	return id, err
}
func (a *Layer) matchingXMPPSessions(userID, credentialID string) []string {
	a.sessions.mu.Lock()
	defer a.sessions.mu.Unlock()
	var ids []string
	for id, e := range a.sessions.items {
		if e.UserID == userID && e.Protocol == "XMPP" && e.CredentialID == credentialID {
			ids = append(ids, id)
		}
	}
	return ids
}
func (a *Layer) closeXMPPSessions(ids []string) error {
	var failures []error
	for _, id := range ids {
		if err := a.CloseSession(id); err != nil && !errors.Is(err, storage.ErrNotFound) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
func (a *Layer) RevokeXMPPDevicePassword(ctx context.Context, userID, id string) error {
	unlock := a.lockXMPPAccess(userID, true)
	if err := a.Store.RevokeXMPPDevicePassword(ctx, userID, id); err != nil {
		unlock()
		return err
	}
	sessions := a.matchingXMPPSessions(userID, id)
	unlock()
	return a.closeXMPPSessions(sessions)
}
func (a *Layer) SetXMPPDevicesOnly(ctx context.Context, userID string, only bool) error {
	unlock := a.lockXMPPAccess(userID, true)
	if err := a.Store.SetXMPPDevicesOnly(ctx, userID, only); err != nil {
		unlock()
		return err
	}
	var sessions []string
	if only {
		sessions = a.matchingXMPPSessions(userID, "")
	}
	unlock()
	return a.closeXMPPSessions(sessions)
}
func (a *Layer) CloseOwnXMPPSession(userID, id string) error {
	a.sessions.mu.Lock()
	e, ok := a.sessions.items[id]
	a.sessions.mu.Unlock()
	if !ok || e.UserID != userID || e.Protocol != "XMPP" {
		return storage.ErrNotFound
	}
	return a.CloseSession(id)
}
