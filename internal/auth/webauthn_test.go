package auth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestWebAuthnBeginRegistrationUsesUUIDSession(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "wa.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer os.RemoveAll(dir)

	tenant, err := store.CreateTenant(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := store.CreateDomain(ctx, tenant.ID, "ex.com")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.Argon2id{}.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: hash, Enabled: true, DisplayName: "User",
	})
	if err != nil {
		t.Fatal(err)
	}

	mfaCfg := config.MFAConfig{
		Issuer:          "Tayga Mail",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		ChallengeTTL:    5 * time.Minute,
		WebAuthn: config.WebAuthnConfig{
			Enabled:       true,
			RPDisplayName: "Tayga Mail",
			RPID:          "localhost",
			RPOrigins:     []string{"http://localhost:8080"},
		},
	}
	layer := auth.NewLayer(store, config.LDAPConfig{}, mfaCfg, config.OIDCConfig{})
	if !layer.WebAuthn.Enabled() {
		t.Fatal("expected webauthn enabled")
	}

	opts, sid, err := layer.WebAuthn.BeginRegistration(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if opts == nil {
		t.Fatal("expected creation options")
	}
	if len(sid) != 36 {
		t.Fatalf("expected UUID session_id, got %q", sid)
	}
	sess, err := store.GetWebAuthnSession(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID != u.ID || sess.Kind != "register" {
		t.Fatalf("unexpected session %+v", sess)
	}

	// MFA methods empty until a credential exists
	if m := layer.MFA.MFAMethods(ctx, u.ID); len(m) != 0 {
		t.Fatalf("expected no MFA methods yet, got %v", m)
	}

	// Simulate stored credential row with UUID PK
	cred, err := store.CreateWebAuthnCredential(ctx, &storage.WebAuthnCredential{
		UserID:         u.ID,
		CredentialID:   storage.EncodeCredentialID([]byte("cred-bytes")),
		Name:           "YubiKey",
		CredentialJSON: `{"id":"Y3JlZC1ieXRlcw","publicKey":"AA","attestationType":"none","flags":{},"authenticator":{}}`,
		SignCount:      0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cred.ID) != 36 {
		t.Fatalf("expected UUID credential PK, got %q", cred.ID)
	}
	if !layer.MFA.IsEnabled(ctx, u.ID) {
		t.Fatal("expected MFA enabled via webauthn")
	}
	methods := layer.MFA.MFAMethods(ctx, u.ID)
	if len(methods) != 1 || methods[0] != "webauthn" {
		t.Fatalf("methods=%v", methods)
	}

	res, err := layer.LoginWithPassword(ctx, "u@ex.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if !res.MFARequired || res.Challenge == "" {
		t.Fatalf("expected MFA challenge, got %+v", res)
	}
	found := false
	for _, m := range res.Methods {
		if m == "webauthn" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected webauthn in methods %v", res.Methods)
	}
}
