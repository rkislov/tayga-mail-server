package mattermost

import (
	"context"
	"encoding/json"
	"golang.org/x/net/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func TestExternalBotLifecycleAndIsolation(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a := f.users[0]
	f.api.SetScopeAuthorizer(func(_ *http.Request, u *storage.User, s mmsettings.Scope) bool {
		return u.ID == a.ID && s.TenantID == a.TenantID && s.DomainID == a.DomainID
	})
	req := map[string]string{"username": "notify-bot", "display_name": "Уведомления"}
	f.call(t, "POST", "/bots", f.tokens[1], req, 403)
	bot := f.call(t, "POST", "/bots", f.tokens[0], req, 201)
	token := bot["token"].(string)
	id := bot["user_id"].(string)
	if _, ok := bot["token_hash"]; ok {
		t.Fatal("hash exposed")
	}
	f.call(t, "POST", "/bots", token, req, 403)
	user := f.call(t, "GET", "/users/me", token, nil, 200)
	if user["is_bot"] != true {
		t.Fatal(user)
	}
	if _, err := f.api.auth.AuthenticateToken(ctx, "", token); err == nil {
		t.Fatal("bot token authorized general OAuth")
	}
	record, err := f.api.readExternalBot(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(record)
	if strings.Contains(string(raw), token) {
		t.Fatal("plaintext token stored")
	}
	c, e := f.api.chat.CreateChannel(ctx, a, "bots-test", "Bots", "P")
	if e != nil {
		t.Fatal(e)
	}
	f.call(t, "POST", "/posts", token, map[string]string{"channel_id": c.ID, "message": "Before membership"}, 404)
	if e = f.api.chat.AddMember(ctx, a, c.ID, id); e != nil {
		t.Fatal(e)
	}
	f.call(t, "POST", "/posts", token, map[string]string{"channel_id": c.ID, "message": "Hello"}, 201)
	foreign, _ := f.api.chat.CreateChannel(ctx, f.users[3], "other-channel", "Other", "O")
	f.call(t, "GET", "/channels/"+foreign.ID+"/posts", token, nil, 404)
	f.call(t, "POST", "/bots/"+id+"/disable", f.tokens[3], map[string]any{}, 403)
	server := httptest.NewServer(f.api)
	defer server.Close()
	ws, e := websocket.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v4/websocket", "", server.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer ws.Close()
	_ = websocket.JSON.Send(ws, map[string]any{"seq": 1, "action": "authentication_challenge", "data": map[string]string{"token": token}})
	if receive(t, ws)["status"] != "OK" || receive(t, ws)["event"] != "hello" {
		t.Fatal("bot websocket handshake failed")
	}
	f.call(t, "POST", "/posts", f.tokens[0], map[string]string{"channel_id": c.ID, "message": "Bot receives this event"}, 201)
	if receive(t, ws)["event"] != "posted" {
		t.Fatal("bot did not receive post")
	}
	f.call(t, "POST", "/bots/"+id+"/disable", f.tokens[0], map[string]any{}, 200)
	_ = websocket.JSON.Send(ws, map[string]any{"seq": 2, "action": "ping"})
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var event any
	if e = websocket.JSON.Receive(ws, &event); e == nil {
		t.Fatal("disabled bot websocket survived")
	}

	f.call(t, "GET", "/users/me", token, nil, 401)
	f.call(t, "POST", "/bots/"+id+"/enable", f.tokens[0], map[string]any{}, 200)
	f.call(t, "GET", "/users/me", token, nil, 200)
	rotated := f.call(t, "POST", "/bots/"+id+"/token", f.tokens[0], map[string]any{}, 200)["token"].(string)
	f.call(t, "GET", "/users/me", token, nil, 401)
	f.call(t, "GET", "/users/me", rotated, nil, 200)
	// No default-channel membership is granted by token authentication.
	botUser, _ := f.store.GetUserByID(ctx, record.UserID)
	if _, err = f.api.chat.Channel(ctx, botUser, chat.ID("town-square:"+a.TenantID)); err == nil {
		t.Fatal("automatically joined common channel")
	}
	request := httptest.NewRequest("GET", "/api/v4/bots", nil)
	request.Header.Set("Authorization", "Bearer "+f.tokens[0])
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, request)
	if strings.Contains(w.Body.String(), rotated) || strings.Contains(w.Body.String(), record.Hash) {
		t.Fatal("credentials exposed in list")
	}
	_ = f.store.UpdateUserEnabled(ctx, a.ID, false)
	f.call(t, "GET", "/users/me", rotated, nil, 401)
}
