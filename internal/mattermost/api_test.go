package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"golang.org/x/net/websocket"
)

type fixture struct {
	api    *API
	store  storage.Driver
	users  []*storage.User
	tokens []string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "chat.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfg := config.Default()
	layer := auth.NewLayer(st, config.LDAPConfig{}, cfg.MFA, config.OIDCConfig{})
	// Fast test-only password hashing; production keeps Layer's default hasher.
	layer.Hasher = auth.Argon2id{Memory: 1024, Time: 1, Threads: 1}
	hash, err := layer.Hasher.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{store: st, api: New(st, layer, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://chat.example.com")}
	for i, domain := range []string{"example.com", "other.test"} {
		tenant, err := st.CreateTenant(ctx, domain)
		if err != nil {
			t.Fatal(err)
		}
		dom, err := st.CreateDomain(ctx, tenant.ID, domain)
		if err != nil {
			t.Fatal(err)
		}
		names := []string{"alice", "bob", "charlie"}
		if i == 1 {
			names = []string{"outsider"}
		}
		for _, name := range names {
			u, err := st.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: dom.ID, Email: name + "@" + domain, LocalPart: name, DisplayName: name, AuthSource: "local", PasswordHash: hash, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			pair, err := layer.Tokens.IssueTokens(ctx, u.ID)
			if err != nil {
				t.Fatal(err)
			}
			f.users = append(f.users, u)
			f.tokens = append(f.tokens, pair.AccessToken)
		}
	}
	return f
}
func (f *fixture) call(t *testing.T, method, path, token string, body any, status int) map[string]any {
	t.Helper()
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "/api/v4"+path, bytes.NewReader(raw))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return result
}
func TestLoginDiscoveryAndRevocation(t *testing.T) {
	f := setup(t)
	f.call(t, "GET", "/system/ping", "", nil, 200)
	config := f.call(t, "GET", "/config/client?format=old", "", nil, 200)
	if config["Version"] != CompatibilityVersion {
		t.Fatal(config)
	}
	f.call(t, "GET", "/users/me", "", nil, 401)
	f.call(t, "POST", "/users/login", "", map[string]string{"login_id": f.users[0].Email, "password": "bad"}, 401)
	body := `{"login_id":"alice@example.com","password":"secret"}`
	req := httptest.NewRequest("POST", "/api/v4/users/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get("Token") == "" {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	token := w.Header().Get("Token")
	user := f.call(t, "GET", "/users/me", token, nil, 200)
	if user["id"] != chat.ID(f.users[0].ID) || len(user["id"].(string)) != 26 {
		t.Fatal(user)
	}
	if _, ok := user["password"]; ok {
		t.Fatal("password exposed")
	}
	f.call(t, "POST", "/users/logout", token, nil, 200)
	f.call(t, "GET", "/users/me", token, nil, 401)
	f.call(t, "PUT", "/config/client", f.tokens[0], nil, 501)
	req = httptest.NewRequest("POST", "/api/v4/users/login", strings.NewReader(body+body))
	w = httptest.NewRecorder()
	f.api.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("trailing JSON: %d", w.Code)
	}
}
func TestPersistentMessagesThreadsAndIsolation(t *testing.T) {
	f := setup(t)
	a, b, c, o := f.tokens[0], f.tokens[1], f.tokens[2], f.tokens[3]
	channel := f.call(t, "POST", "/channels", a, map[string]string{"team_id": chat.ID(f.users[0].TenantID), "name": "private-room", "display_name": "Private", "type": "P"}, 201)
	id := channel["id"].(string)
	f.call(t, "GET", "/channels/"+id, b, nil, 404)
	member := f.call(t, "POST", "/channels/"+id+"/members", a, map[string]string{"user_id": chat.ID(f.users[1].ID)}, 201)
	f.call(t, "GET", "/users?in_channel="+id, b, nil, 200)
	f.call(t, "GET", "/users?in_channel="+id, c, nil, 404)
	f.call(t, "GET", "/users?in_team="+chat.ID(f.users[3].TenantID), a, nil, 404)
	f.call(t, "GET", "/users/me/teams/unread", a, nil, 200)
	if member["user_id"] != chat.ID(f.users[1].ID) {
		t.Fatal(member)
	}
	f.call(t, "POST", "/channels/"+id+"/members", b, map[string]string{"user_id": chat.ID(f.users[2].ID)}, 403)
	req := map[string]string{"channel_id": id, "message": "hello", "pending_post_id": "retry-1"}
	p := f.call(t, "POST", "/posts", a, req, 201)
	again := f.call(t, "POST", "/posts", a, req, 201)
	if p["id"] != again["id"] {
		t.Fatal("duplicate post on retry")
	}
	req["message"] = "different"
	f.call(t, "POST", "/posts", a, req, 400)
	reply := f.call(t, "POST", "/posts", b, map[string]string{"channel_id": id, "root_id": p["id"].(string), "message": "reply"}, 201)
	f.call(t, "POST", "/posts", b, map[string]string{"channel_id": id, "root_id": reply["id"].(string), "message": "nested"}, 400)
	f.call(t, "GET", "/posts/"+p["id"].(string), c, nil, 404)
	f.call(t, "GET", "/channels/"+id, o, nil, 404)
	f.call(t, "POST", "/channels/"+id+"/members", a, map[string]string{"user_id": chat.ID(f.users[3].ID)}, 404)
	f.call(t, "GET", "/users/"+chat.ID(f.users[3].ID), a, nil, 404)
	thread := f.call(t, "GET", "/posts/"+p["id"].(string)+"/thread", b, nil, 200)
	if len(thread["order"].([]any)) != 2 {
		t.Fatal(thread)
	}
	got := f.call(t, "GET", "/posts/"+p["id"].(string), a, nil, 200)
	if got["reply_count"] != float64(1) {
		t.Fatal(got)
	}
	ch := f.call(t, "GET", "/channels/"+id, a, nil, 200)
	if ch["total_msg_count"] != float64(2) || ch["total_msg_count_root"] != float64(1) {
		t.Fatal(ch)
	}
	f.call(t, "POST", "/channels/members/me/view", b, map[string]string{"channel_id": id}, 200)
	viewed := f.call(t, "GET", "/channels/"+id+"/members/me", b, nil, 200)
	if viewed["msg_count"] != float64(2) {
		t.Fatal(viewed)
	}
	f.call(t, "GET", "/channels/"+id+"/posts?per_page=-1", a, nil, 400)
	f.call(t, "GET", "/channels/"+id+"/posts?since=bad", a, nil, 400)
	f.call(t, "GET", "/channels/"+id+"/posts?before=unknown", a, nil, 404)
	// Reconstruct the entire adapter to prove messages are not process-local.
	f.api = New(f.store, f.api.auth, f.api.log, "")
	history := f.call(t, "GET", "/channels/"+id+"/posts", b, nil, 200)
	if len(history["order"].([]any)) != 2 {
		t.Fatal(history)
	}
	dm := f.call(t, "POST", "/channels/direct", a, []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	reverse := f.call(t, "POST", "/channels/direct", b, []string{chat.ID(f.users[1].ID), chat.ID(f.users[0].ID)}, 201)
	if dm["id"] != reverse["id"] {
		t.Fatal("duplicate direct channel")
	}
	f.call(t, "POST", "/channels/direct", a, []string{chat.ID(f.users[0].ID), chat.ID(f.users[3].ID)}, 404)
	f.call(t, "POST", "/posts", a, map[string]any{"channel_id": id, "message": "file", "file_ids": []string{"unknown"}}, 400)
}
func TestMFACannotBeBypassed(t *testing.T) {
	f := setup(t)
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	m := &storage.UserMFA{UserID: f.users[0].ID, TOTPEnabled: true, TOTPSecret: secret, BackupCodes: "[]"}
	if err = f.store.UpsertUserMFA(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	result := f.call(t, "POST", "/users/login", "", map[string]string{"login_id": f.users[0].Email, "password": "secret"}, 401)
	if result["id"] != "mfa.validate_token.authenticate.app_error" {
		t.Fatal(result)
	}
	f.call(t, "POST", "/users/login", "", map[string]string{"login_id": f.users[0].Email, "password": "secret", "token": "not-a-code"}, 401)
}
func receive(t *testing.T, ws *websocket.Conn) map[string]any {
	t.Helper()
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var e map[string]any
	if err := websocket.JSON.Receive(ws, &e); err != nil {
		t.Fatal(err)
	}
	return e
}
func TestWebsocketChallengeDeliveryAndIsolation(t *testing.T) {
	f := setup(t)
	server := httptest.NewServer(f.api)
	defer server.Close()
	connect := func(token string) *websocket.Conn {
		ws, err := websocket.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v4/websocket", "", server.URL)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ws.Close() })
		if err = websocket.JSON.Send(ws, map[string]any{"seq": 1, "action": "authentication_challenge", "data": map[string]string{"token": token}}); err != nil {
			t.Fatal(err)
		}
		ack := receive(t, ws)
		if ack["status"] != "OK" || ack["seq_reply"] != float64(1) {
			t.Fatal(ack)
		}
		hello := receive(t, ws)
		if hello["event"] != "hello" || hello["seq"] != float64(0) {
			t.Fatal(hello)
		}
		return ws
	}
	alice := connect(f.tokens[0])
	bob := connect(f.tokens[1])
	outsider := connect(f.tokens[3])
	// Private post must be skipped for Bob; the following visible post must be
	// his first received event, proving the skipped post did not leak.
	private := f.call(t, "POST", "/channels", f.tokens[0], map[string]string{"team_id": chat.ID(f.users[0].TenantID), "name": "socket-private", "display_name": "Private", "type": "P"}, 201)
	f.call(t, "POST", "/posts", f.tokens[0], map[string]string{"channel_id": private["id"].(string), "message": "secret"}, 201)
	if e := receive(t, alice); e["event"] != "posted" {
		t.Fatal(e)
	}
	dm := f.call(t, "POST", "/channels/direct", f.tokens[0], []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	p := f.call(t, "POST", "/posts", f.tokens[0], map[string]string{"channel_id": dm["id"].(string), "message": "visible"}, 201)
	e := receive(t, bob)
	var post map[string]any
	if err := json.Unmarshal([]byte(e["data"].(map[string]any)["post"].(string)), &post); err != nil {
		t.Fatal(err)
	}
	if post["id"] != p["id"] || e["seq"] != float64(1) {
		t.Fatal(e)
	}
	// A ping response acts as a barrier after earlier events were processed.
	if err := websocket.JSON.Send(outsider, map[string]any{"seq": 2, "action": "ping"}); err != nil {
		t.Fatal(err)
	}
	if e := receive(t, outsider); e["status"] != "OK" || e["seq_reply"] != float64(2) {
		t.Fatalf("tenant event leak: %v", e)
	}
	f.call(t, "POST", "/users/logout", f.tokens[1], nil, 200)
	_ = websocket.JSON.Send(bob, map[string]any{"seq": 3, "action": "ping"})
	_ = bob.SetReadDeadline(time.Now().Add(3 * time.Second))
	var value any
	if err := websocket.JSON.Receive(bob, &value); err == nil {
		t.Fatalf("revoked session still active: %v", value)
	}
	// Browser origins must match this server or the configured public URL.
	_, err := websocket.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v4/websocket", "", "https://evil.test")
	if err == nil {
		t.Fatal("cross-origin connection accepted")
	}
}

func TestConcurrentRetriesAndSlowConsumer(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	u := f.users[0]
	c, err := f.api.chat.CreateChannel(ctx, u, "concurrent-room", "Concurrent", "O")
	if err != nil {
		t.Fatal(err)
	}
	events, cleanup := f.api.chat.Subscribe()
	defer cleanup()
	type result struct {
		post chat.Post
		err  error
	}
	results := make(chan result, 8)
	for range 8 {
		go func() { p, e := f.api.chat.Send(ctx, u, c.ID, "", "once", "same-id"); results <- result{p, e} }()
	}
	id := ""
	for range 8 {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		if id == "" {
			id = r.post.ID
		}
		if r.post.ID != id {
			t.Fatal("concurrent duplicate")
		}
	}
	select {
	case <-events:
	default:
		t.Fatal("no committed event")
	}
	select {
	case <-events:
		t.Fatal("duplicate event for retry")
	default:
	}
	// Fill the slow subscriber's queue and force it to close, while preserving
	// every committed post for reconnect/history recovery.
	for range 65 {
		if _, err = f.api.chat.Send(ctx, u, c.ID, "", "queued", ""); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for range events {
		count++
	}
	if count != 64 {
		t.Fatalf("buffered events=%d", count)
	}
	history, err := f.api.chat.Posts(ctx, u, c.ID, 100, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 66 {
		t.Fatalf("lost persisted messages: %d", len(history))
	}
	cleanup() // Also safe after the service has disconnected the subscriber.
}

// iOS 2.45.1 refuses to add a server without this field, before login.
func TestIOSDiscoveryInstallationIdentity(t *testing.T) {
	f := setup(t)
	first := f.call(t, "GET", "/config/client?format=old", "", nil, 200)
	id, ok := first["DiagnosticId"].(string)
	if !ok || len(id) != 26 {
		t.Fatalf("missing installation identity: %v", first)
	}
	f.api = New(f.store, f.api.auth, f.api.log, "https://renamed.example.com")
	second := f.call(t, "GET", "/config/client?format=old", "", nil, 200)
	if second["DiagnosticId"] != id {
		t.Fatal("installation identity changed across adapter restart or URL change")
	}
	results := make(chan string, 8)
	for range 8 {
		go func() {
			value, err := f.api.chat.ServerID(context.Background())
			if err != nil {
				results <- "error"
				return
			}
			results <- value
		}()
	}
	for range 8 {
		if got := <-results; got != id {
			t.Fatalf("concurrent identity=%s", got)
		}
	}
}

func TestPublicChannelDirectoryAndJoin(t *testing.T) {
	f := setup(t)
	team := chat.ID(f.users[0].TenantID)
	open := f.call(t, "POST", "/channels", f.tokens[0], map[string]string{
		"team_id": team, "name": "open-meeting", "display_name": "Добро пожаловать в новый канал для совещаний", "type": "O",
	}, 201)["id"].(string)
	private := f.call(t, "POST", "/channels", f.tokens[0], map[string]string{
		"team_id": team, "name": "private-meeting", "display_name": "Канал для совещаний", "type": "P",
	}, 201)["id"].(string)
	req := httptest.NewRequest("POST", "/api/v4/teams/"+team+"/channels/search", strings.NewReader(`{"term":"СОВЕЩАНИЙ"}`))
	req.Header.Set("Authorization", "Bearer "+f.tokens[1])
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, req)
	var matches []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &matches); err != nil || w.Code != 200 || len(matches) != 1 || matches[0]["id"] != open {
		t.Fatalf("invalid directory: %d %v %v", w.Code, matches, err)
	}
	f.call(t, "GET", "/channels/"+open, f.tokens[1], nil, 200)
	f.call(t, "GET", "/channels/"+open+"/posts", f.tokens[1], nil, 404)
	join := map[string]string{"user_id": chat.ID(f.users[1].ID), "channel_id": open, "post_root_id": ""}
	f.call(t, "POST", "/channels/"+private+"/members", f.tokens[1], join, 404)
	f.call(t, "POST", "/channels/"+open+"/members", f.tokens[3], map[string]string{"user_id": chat.ID(f.users[3].ID)}, 404)
	f.call(t, "POST", "/channels/"+open+"/members", f.tokens[1], join, 201)
	f.call(t, "POST", "/channels/"+open+"/members", f.tokens[1], join, 201)
	f.call(t, "GET", "/channels/"+open+"/posts", f.tokens[1], nil, 200)
}
