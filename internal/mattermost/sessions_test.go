package mattermost

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/chat"
	"golang.org/x/net/websocket"
)

func TestNativeSessionSurvivesOAuthExpiryWithoutExtendingOtherProtocols(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.call(t, "GET", "/users/me", f.tokens[0], nil, 200)
	backing, e := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	if e != nil {
		t.Fatal(e)
	}
	native, e := f.api.chat.Session(ctx, backing.ID)
	if e != nil {
		t.Fatal(e)
	}
	if native.ExpiresAt <= backing.ExpiresAt.UnixMilli() || native.ExpiresAt > backing.RefreshExpiresAt.UnixMilli() {
		t.Fatal("native lifetime not bounded by refresh policy")
	}
	if _, e = f.store.DB().Exec(`UPDATE oauth_tokens SET expires_at=? WHERE id=?`, time.Now().Add(-time.Hour).UTC(), backing.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.api.auth.AuthenticateToken(ctx, "", f.tokens[0]); e == nil {
		t.Fatal("ordinary OAuth access was expanded")
	}
	f.call(t, "GET", "/users/me", f.tokens[0], nil, 200)
	restarted := New(f.store, f.api.auth, f.api.log, "")
	if _, e = restarted.authenticate(ctx, f.tokens[0]); e != nil {
		t.Fatal("session did not survive adapter restart", e)
	}
	pushRegister(t, f, 0, "x")
	device, e := f.api.chat.PushDevice(ctx, backing.ID)
	if e != nil || device.ExpiresAt != native.ExpiresAt {
		t.Fatal("push lifetime mismatch", e)
	}
	f.call(t, "POST", "/users/logout", f.tokens[0], nil, 200)
	f.call(t, "GET", "/users/me", f.tokens[0], nil, 401)
	if _, e = f.api.chat.PushDevice(ctx, backing.ID); e == nil {
		t.Fatal("push survived logout")
	}
	if _, e = f.api.chat.Session(ctx, backing.ID); e == nil {
		t.Fatal("native session survived logout")
	}
}
func TestNativeSessionsNeverReviveExpiredCredentialsAndEnforceScope(t *testing.T) {
	for _, scenario := range []string{"unregistered", "absolute-expiry", "disabled", "scope-changed", "revoked"} {
		t.Run(scenario, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			backing, _ := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
			if scenario != "unregistered" {
				f.call(t, "GET", "/users/me", f.tokens[0], nil, 200)
			}
			switch scenario {
			case "unregistered":
				_, _ = f.store.DB().Exec(`UPDATE oauth_tokens SET expires_at=? WHERE id=?`, time.Now().Add(-time.Hour).UTC(), backing.ID)
			case "absolute-expiry":
				_, _ = f.store.DB().Exec(`UPDATE chat_sessions SET expires_at=? WHERE session_id=?`, time.Now().Add(-time.Second).UnixMilli(), backing.ID)
			case "disabled":
				if e := f.store.UpdateUserEnabled(ctx, f.users[0].ID, false); e != nil {
					t.Fatal(e)
				}
			case "scope-changed":
				_, _ = f.store.DB().Exec(`UPDATE users SET tenant_id=?,domain_id=? WHERE id=?`, f.users[3].TenantID, f.users[3].DomainID, f.users[0].ID)
			case "revoked":
				if e := f.store.DeleteOAuthToken(ctx, backing.ID); e != nil {
					t.Fatal(e)
				}
			}
			f.call(t, "GET", "/users/me", f.tokens[0], nil, 401)
		})
	}
}
func TestSessionListAndRevocationArePersistentPrivateAndUserScoped(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, i := range []int{0, 1} {
		f.call(t, "GET", "/users/me", f.tokens[i], nil, 200)
	}
	backing, err := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.api.auth.Tokens.IssueTokens(ctx, f.users[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	f.call(t, "GET", "/users/me", other.AccessToken, nil, 200)
	req := httptest.NewRequest("GET", "/api/v4/users/me/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+f.tokens[0])
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), f.tokens[0]) || strings.Contains(w.Body.String(), backing.RefreshToken) {
		t.Fatal("credentials exposed")
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatal(list, err)
	}
	for _, session := range list {
		if session["is_current"] != (session["id"] == chat.ID(backing.ID)) {
			t.Fatalf("incorrect current session marker: %v", session)
		}
	}
	body := map[string]string{"session_id": chat.ID(backing.ID)}
	f.call(t, "POST", "/users/sessions/revoke", f.tokens[1], body, 404)
	f.call(t, "GET", "/users/me", f.tokens[0], nil, 200)
	f.call(t, "POST", "/users/sessions/revoke", f.tokens[0], body, 200)
	f.call(t, "GET", "/users/me", f.tokens[0], nil, 401)
}

func TestNativeWebsocketReconnectAfterOAuthExpiry(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.call(t, "GET", "/users/me", f.tokens[0], nil, 200)
	backing, _ := f.store.GetOAuthTokenByAccess(ctx, f.tokens[0])
	if _, e := f.store.DB().Exec(`UPDATE oauth_tokens SET expires_at=? WHERE id=?`, time.Now().Add(-time.Hour).UTC(), backing.ID); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(f.api)
	defer server.Close()
	ws, e := websocket.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v4/websocket", "", server.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer ws.Close()
	if e = websocket.JSON.Send(ws, map[string]any{"seq": 1, "action": "authentication_challenge", "data": map[string]string{"token": f.tokens[0]}}); e != nil {
		t.Fatal(e)
	}
	if got := receive(t, ws); got["status"] != "OK" {
		t.Fatal(got)
	}
	if got := receive(t, ws); got["event"] != "hello" {
		t.Fatal(got)
	}
	f.call(t, "POST", "/users/logout", f.tokens[0], nil, 200)
	_ = websocket.JSON.Send(ws, map[string]any{"seq": 2, "action": "ping"})
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var response map[string]any
	if e = websocket.JSON.Receive(ws, &response); e == nil {
		t.Fatal("revoked websocket remained authenticated", response)
	}
}
