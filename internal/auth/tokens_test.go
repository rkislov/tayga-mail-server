package auth_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestTokenIssueValidateRefresh(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
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
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: "x", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ts := &auth.TokenService{Store: store, Cfg: config.MFAConfig{
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		ChallengeTTL:    5 * time.Minute,
	}}
	pair, err := ts.IssueTokens(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.ValidateAccessToken(ctx, pair.AccessToken)
	if err != nil || got.Email != u.Email {
		t.Fatalf("validate: %v %#v", err, got)
	}
	pair2, err := ts.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if pair2.AccessToken == pair.AccessToken {
		t.Fatal("expected rotated access token")
	}
	_, err = ts.ValidateAccessToken(ctx, pair.AccessToken)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("old access should be invalid after refresh, got %v", err)
	}
}

func TestLoginMFAFlow(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	layer := auth.NewLayer(store, config.LDAPConfig{}, config.MFAConfig{
		Issuer:                  "Test",
		AccessTokenTTL:          time.Hour,
		RefreshTokenTTL:         24 * time.Hour,
		ChallengeTTL:            5 * time.Minute,
		RequireTokenForMFAUsers: true,
	}, config.OIDCConfig{})

	tenant, _ := store.CreateTenant(ctx, "t")
	dom, _ := store.CreateDomain(ctx, tenant.ID, "ex.com")
	hash, err := layer.Hasher.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.CreateUser(ctx, &storage.User{
		TenantID: tenant.ID, DomainID: dom.ID,
		Email: "u@ex.com", LocalPart: "u", AuthSource: "local",
		PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := layer.LoginWithPassword(ctx, "u@ex.com", "secret")
	if err != nil || res.MFARequired || res.Tokens == nil {
		t.Fatalf("login without mfa: %#v %v", res, err)
	}

	secret, _, _, err := layer.MFA.BeginSetup(ctx, u.ID, u.Email)
	if err != nil {
		t.Fatal(err)
	}
	// Find a valid TOTP by brute-forcing current window via ValidateTOTP helper:
	// generate codes from secret using package by enabling with a probed code.
	code := findValidTOTP(t, secret)
	if err := layer.MFA.ConfirmSetup(ctx, u.ID, code); err != nil {
		t.Fatal(err)
	}

	_, err = layer.Authenticate(ctx, "u@ex.com", "secret")
	if !errors.Is(err, auth.ErrMFARequired) {
		t.Fatalf("protocol auth should require token, got %v", err)
	}

	res, err = layer.LoginWithPassword(ctx, "u@ex.com", "secret")
	if err != nil || !res.MFARequired || res.Challenge == "" {
		t.Fatalf("expected mfa challenge: %#v %v", res, err)
	}
	pair, err := layer.CompleteMFA(ctx, res.Challenge, code)
	if err != nil || pair.AccessToken == "" {
		t.Fatalf("complete mfa: %v %#v", err, pair)
	}
	got, err := layer.AuthenticateToken(ctx, "u@ex.com", pair.AccessToken)
	if err != nil || got.ID != u.ID {
		t.Fatalf("token auth: %v", err)
	}
}

func findValidTOTP(t *testing.T, secret string) string {
	t.Helper()
	// Probe by validating successive candidates from a tiny search is not possible
	// without the generator. Recompute using the same algorithm via trial of
	// current unix step — we import nothing private, so call ValidateTOTP after
	// constructing codes with a local copy of hotp logic.
	for _, skew := range []int64{-1, 0, 1} {
		code := testHOTP(secret, time.Now().Unix()/30+skew)
		if auth.ValidateTOTP(secret, code) {
			return code
		}
	}
	t.Fatal("could not find valid totp")
	return ""
}
