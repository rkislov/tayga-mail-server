package auth

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/tayga/tms/internal/storage"
	"github.com/xdg-go/scram"
)

const XMPPSCRAMIterations = 32768

func (a *Layer) newXMPPSCRAMKey(ctx context.Context, userID, credentialID, source, password string) (storage.XMPPSCRAMKey, error) {
	k := storage.XMPPSCRAMKey{CredentialID: credentialID, SourceHash: source}
	salt, err := a.Store.EnsureXMPPSCRAMSalt(ctx, userID)
	if err != nil {
		return k, err
	}
	raw, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		return k, err
	}
	client, err := scram.SHA256.NewClient("", password, "")
	if err != nil {
		// RFC 5802: use the original password if SASLprep fails.
		client, err = scram.SHA256.NewClientUnprepped("", password, "")
		if err != nil {
			return k, err
		}
	}
	c, err := client.GetStoredCredentialsWithError(scram.KeyFactors{Salt: string(raw), Iters: XMPPSCRAMIterations})
	if err != nil {
		return k, err
	}
	k.StoredKey = base64.StdEncoding.EncodeToString(c.StoredKey)
	k.ServerKey = base64.StdEncoding.EncodeToString(c.ServerKey)
	return k, nil
}

// Called only after a successful TLS password login. LDAP passwords are never
// cached: doing so would bypass directory disablement/password changes.
func (a *Layer) EnrollXMPPSCRAM(ctx context.Context, u *storage.User, credentialID, password string) error {
	source := u.PasswordHash
	if credentialID == "" {
		if u.AuthSource != "" && u.AuthSource != "local" {
			return nil
		}
	} else {
		d, err := a.Store.GetXMPPDevicePassword(ctx, u.ID, credentialID)
		if err != nil {
			return err
		}
		if d.RevokedAt != nil {
			return ErrInvalidCredentials
		}
		source = d.SecretHash
	}
	_, keys, err := a.Store.XMPPSCRAMKeys(ctx, u.ID)
	if err == nil {
		for _, k := range keys {
			if k.CredentialID == credentialID && k.SourceHash == source {
				return nil
			}
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	k, err := a.newXMPPSCRAMKey(ctx, u.ID, credentialID, source, password)
	if err != nil {
		return err
	}
	return a.Store.SaveXMPPSCRAMKey(ctx, u.ID, k)
}

// Password and verifier change in one transaction, preserving existing device
// salts and Monal's SCRAM pin. Hash-only restore/import remains fail-closed.
func (a *Layer) SetLocalPassword(ctx context.Context, userID, password string) error {
	hash, err := a.Hasher.Hash(password)
	if err != nil {
		return err
	}
	k, err := a.newXMPPSCRAMKey(ctx, userID, "", hash, password)
	if err != nil {
		return err
	}
	unlock := a.lockXMPPAccess(userID, true)
	err = a.Store.UpdateUserPasswordWithSCRAM(ctx, userID, hash, k)
	unlock()
	return err
}

func (a *Layer) xmppPrimaryAllowed(ctx context.Context, u *storage.User) (bool, error) {
	if u.AuthSource != "" && u.AuthSource != "local" {
		return false, nil
	}
	only, err := a.Store.XMPPDevicesOnly(ctx, u.ID)
	if err != nil || only {
		return false, err
	}
	if a.mfaCfg.RequireTokenForMFAUsers && a.MFA != nil {
		m, err := a.MFA.Store.GetUserMFA(ctx, u.ID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return false, err
		}
		if err == nil && m.TOTPEnabled {
			return false, nil
		}
	}
	return true, nil
}

// Advertise SCRAM only if EVERY allowed password for this account is ready.
// Otherwise a client could select SCRAM for an unenrolled device password and
// permanently pin a mechanism that cannot authenticate it.
func (a *Layer) XMPPSCRAMCredentials(ctx context.Context, email string) (*storage.User, string, []storage.XMPPSCRAMKey, error) {
	u, err := a.Store.GetUserByEmail(ctx, email)
	if err != nil || !u.Enabled {
		return nil, "", nil, ErrInvalidCredentials
	}
	primary, err := a.xmppPrimaryAllowed(ctx, u)
	if err != nil {
		return nil, "", nil, err
	}
	only, err := a.Store.XMPPDevicesOnly(ctx, u.ID)
	if err != nil {
		return nil, "", nil, err
	}
	if !only && u.AuthSource != "" && u.AuthSource != "local" {
		return nil, "", nil, ErrInvalidCredentials
	}
	salt, keys, err := a.Store.XMPPSCRAMKeys(ctx, u.ID)
	if err != nil {
		return nil, "", nil, err
	}
	expected := map[string]string{}
	if primary {
		expected[""] = u.PasswordHash
	}
	devices, err := a.Store.ActiveXMPPDevicePasswords(ctx, u.ID)
	if err != nil {
		return nil, "", nil, err
	}
	for _, d := range devices {
		if d.RevokedAt == nil {
			expected[d.ID] = d.SecretHash
		}
	}
	var ready []storage.XMPPSCRAMKey
	for _, k := range keys {
		if source, ok := expected[k.CredentialID]; ok && source == k.SourceHash {
			stored, e1 := base64.StdEncoding.DecodeString(k.StoredKey)
			server, e2 := base64.StdEncoding.DecodeString(k.ServerKey)
			if e1 != nil || e2 != nil || len(stored) != 32 || len(server) != 32 {
				continue
			}
			ready = append(ready, k)
			delete(expected, k.CredentialID)
		}
	}
	if len(expected) > 0 || len(ready) == 0 {
		return nil, "", nil, ErrInvalidCredentials
	}
	return u, salt, ready, nil
}

// Check the credential version again at session registration, after proof
// verification, under the same access lock used for revocation/password changes.
func (a *Layer) TrackSCRAMXMPPSession(ctx context.Context, u *storage.User, k storage.XMPPSCRAMKey, address, agent, key string, close func() error) (string, error) {
	return a.trackXMPPSession(ctx, u, k.CredentialID, address, agent, key, close, func() error {
		fresh, err := a.Store.GetUserByID(ctx, u.ID)
		if err != nil {
			return err
		}
		if k.CredentialID == "" {
			allowed, err := a.xmppPrimaryAllowed(ctx, fresh)
			if err != nil {
				return err
			}
			if !allowed || fresh.PasswordHash != k.SourceHash {
				return ErrInvalidCredentials
			}
		} else {
			d, err := a.Store.GetXMPPDevicePassword(ctx, u.ID, k.CredentialID)
			if err != nil {
				return err
			}
			if d.RevokedAt != nil || d.SecretHash != k.SourceHash {
				return ErrInvalidCredentials
			}
		}
		return nil
	})
}
