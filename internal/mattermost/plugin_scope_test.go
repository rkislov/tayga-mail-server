package mattermost

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
	"golang.org/x/net/websocket"
)

func TestPluginWebsocketEventStaysWithinDomain(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	u := f.users[0]
	dom, err := f.store.CreateDomain(ctx, u.TenantID, "sibling.test")
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := f.store.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: dom.ID, Email: "sibling@sibling.test", LocalPart: "sibling", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := f.api.auth.Tokens.IssueTokens(ctx, sibling.ID)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.api)
	defer server.Close()
	connect := func(token string) *websocket.Conn {
		ws, err := websocket.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v4/websocket", "", server.URL)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ws.Close() })
		if err := websocket.JSON.Send(ws, map[string]any{"seq": 1, "action": "authentication_challenge", "data": map[string]string{"token": token}}); err != nil {
			t.Fatal(err)
		}
		if got := receive(t, ws); got["status"] != "OK" {
			t.Fatal(got)
		}
		if got := receive(t, ws); got["event"] != "hello" || got["data"].(map[string]any)["server_version"] != "12.0.0" {
			t.Fatal(got)
		}
		return ws
	}
	owner, foreign := connect(f.tokens[0]), connect(pair.AccessToken)
	if err := f.api.chat.PublishEvent(chat.Event{Type: "custom_github_fixture", TeamID: chat.ID(u.TenantID), DomainID: u.DomainID, Data: map[string]any{"value": "private"}}); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, owner); got["event"] != "custom_github_fixture" {
		t.Fatal(got)
	}
	if err := websocket.JSON.Send(foreign, map[string]any{"seq": 2, "action": "ping"}); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, foreign); got["status"] != "OK" || got["seq_reply"] != float64(2) {
		t.Fatalf("plugin event leaked into sibling domain: %v", got)
	}
}
