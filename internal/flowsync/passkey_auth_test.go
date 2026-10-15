// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

func TestPasskeyDoesNotDisableFlowSyncPassword(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "auth.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	tenant, err := store.CreateTenant(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := store.CreateDomain(ctx, tenant.ID, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	policy := config.MFAConfig{RequireTokenForMFAUsers: true, AccessTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour}
	layer := auth.NewLayer(store, config.LDAPConfig{}, policy, config.OIDCConfig{})
	hash, err := layer.Hasher.Hash("test-password")
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "user@example.com", LocalPart: "user", AuthSource: "local", PasswordHash: hash, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureMailbox(ctx, u.ID, "INBOX", filepath.Join(dir, "mail")); err != nil {
		t.Fatal(err)
	}
	mount := func() *http.ServeMux {
		mux := http.NewServeMux()
		flowsync.Mount(mux, &config.Config{FlowSync: config.FlowSyncConfig{Enabled: true}}, slog.New(slog.NewTextHandler(io.Discard, nil)), store, layer, mailstore.New(filepath.Join(dir, "mail")))
		return mux
	}
	mux := mount()
	check := func(t *testing.T, password, token string, status int) {
		t.Helper()
		for _, endpoint := range []struct{ name, path, body, success string }{
			{"ActiveSync", "/Microsoft-Server-ActiveSync?Cmd=FolderSync&DeviceId=passkey-test&DeviceType=Test", "", "<Status>1</Status>"},
			{"EWS", "/EWS/Exchange.asmx", `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><s:Body><m:GetFolder><m:FolderShape><t:BaseShape>Default</t:BaseShape></m:FolderShape><m:FolderIds><t:DistinguishedFolderId Id="inbox"/></m:FolderIds></m:GetFolder></s:Body></s:Envelope>`, "<m:ResponseCode>NoError</m:ResponseCode>"},
		} {
			t.Run(endpoint.name, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodPost, endpoint.path, strings.NewReader(endpoint.body))
				r.SetBasicAuth(u.Email, password)
				if token != "" {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, r)
				if w.Code != status || (status == http.StatusOK && !strings.Contains(w.Body.String(), endpoint.success)) {
					t.Fatalf("HTTP %d (want %d): %s", w.Code, status, w.Body.String())
				}
			})
		}
	}
	t.Run("before passkey", func(t *testing.T) { check(t, "test-password", "", http.StatusOK) })
	credential, err := store.CreateWebAuthnCredential(ctx, &storage.WebAuthnCredential{UserID: u.ID, CredentialID: storage.EncodeCredentialID([]byte("test-passkey")), Name: "Test device", CredentialJSON: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("after passkey", func(t *testing.T) { check(t, "test-password", "", http.StatusOK) })
	t.Run("wrong password", func(t *testing.T) { check(t, "wrong", "", http.StatusUnauthorized) })
	result, err := layer.LoginWithPassword(ctx, u.Email, "test-password")
	if err != nil || !result.MFARequired || result.Tokens != nil || result.Challenge == "" || len(result.Methods) != 1 || result.Methods[0] != "webauthn" {
		t.Fatalf("web login must still require passkey: %#v, %v", result, err)
	}
	if err := store.UpsertUserMFA(ctx, &storage.UserMFA{UserID: u.ID, TOTPEnabled: false, TOTPSecret: "pending", BackupCodes: "[]"}); err != nil {
		t.Fatal(err)
	}
	t.Run("pending TOTP", func(t *testing.T) { check(t, "test-password", "", http.StatusOK) })
	if err := store.UpsertUserMFA(ctx, &storage.UserMFA{UserID: u.ID, TOTPEnabled: true, BackupCodes: "[]"}); err != nil {
		t.Fatal(err)
	}
	t.Run("enabled TOTP still requires token", func(t *testing.T) { check(t, "test-password", "", http.StatusUnauthorized) })
	pair, err := layer.Tokens.IssueTokens(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("token with both factors", func(t *testing.T) { check(t, "", pair.AccessToken, http.StatusOK) })
	policy.RequireTokenForMFAUsers = false
	layer = auth.NewLayer(store, config.LDAPConfig{}, policy, config.OIDCConfig{})
	mux = mount()
	t.Run("explicit password policy", func(t *testing.T) { check(t, "test-password", "", http.StatusOK) })
	if err := store.DeleteWebAuthnCredential(ctx, u.ID, credential.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertUserMFA(ctx, &storage.UserMFA{UserID: u.ID, BackupCodes: "[]"}); err != nil {
		t.Fatal(err)
	}
	policy.RequireTokenForMFAUsers = true
	layer = auth.NewLayer(store, config.LDAPConfig{}, policy, config.OIDCConfig{})
	mux = mount()
	t.Run("removed factors", func(t *testing.T) { check(t, "test-password", "", http.StatusOK) })
}
