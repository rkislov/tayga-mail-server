package mattermost

import (
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBotCommandsAndButtonsPreserveCallerAndScope(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a := f.users[0]
	f.api.SetScopeAuthorizer(func(_ *http.Request, u *storage.User, scope mmsettings.Scope) bool {
		return u.ID == a.ID && scope.TenantID == a.TenantID && scope.DomainID == a.DomainID
	})
	bot := f.call(t, "POST", "/bots", f.tokens[0], map[string]string{"username": "assistant", "display_name": "Assistant"}, 201)
	token := bot["token"].(string)
	id := bot["user_id"].(string)
	registration := map[string]any{"team_id": chat.ID(a.TenantID), "trigger": "ai", "tms_bot_command": true, "auto_complete_desc": "AI", "auto_complete_hint": "help | new | chats | use ID | web on/off/status"}
	f.call(t, "POST", "/commands", f.tokens[1], registration, 403)
	f.call(t, "POST", "/commands", token, registration, 201)
	f.call(t, "POST", "/commands", token, registration, 201) // restart is idempotent
	rows, err := f.api.botCommands(ctx, a)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	foreign, _ := f.api.botCommands(ctx, f.users[3])
	if len(foreign) != 0 {
		t.Fatal("cross-tenant command")
	}
	domain, err := f.store.CreateDomain(ctx, a.TenantID, "neighbor.example.com")
	if err != nil {
		t.Fatal(err)
	}
	neighbor := *a
	neighbor.DomainID = domain.ID
	foreign, _ = f.api.botCommands(ctx, &neighbor)
	if len(foreign) != 0 {
		t.Fatal("cross-domain command")
	}
	request := httptest.NewRequest("GET", "/api/v4/commands/autocomplete?user_input=ai+web", nil)
	request.Header.Set("Authorization", "Bearer "+f.tokens[1])
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, request)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"Complete":"ai web on"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	dm, err := f.api.chat.Direct(ctx, a, id)
	if err != nil {
		t.Fatal(err)
	}
	f.call(t, "POST", "/commands/execute", f.tokens[0], map[string]string{"channel_id": dm.ID, "command": "/ai web on", "user_id": id}, 200)
	posts, _ := f.api.chat.Posts(ctx, a, dm.ID, 10, 0, 0)
	if len(posts) != 1 || posts[0].UserID != a.ID || posts[0].Message != "/ai web on" {
		t.Fatal("caller impersonated", posts)
	}
	other, _ := f.api.chat.Direct(ctx, a, chat.ID(f.users[1].ID))
	f.call(t, "POST", "/commands/execute", f.tokens[0], map[string]string{"channel_id": other.ID, "command": "/ai new"}, 403)
	f.call(t, "POST", "/commands/execute", f.tokens[1], map[string]string{"channel_id": dm.ID, "command": "/ai new"}, 404)
	props := map[string]any{"attachments": []any{map[string]any{"actions": []any{map[string]any{"id": "new", "integration": map[string]any{"url": "http://127.0.0.1/private", "context": map[string]string{"command": "/ai new"}}}}}}}
	p := f.call(t, "POST", "/posts", token, map[string]any{"channel_id": dm.ID, "message": "Menu", "props": props}, 201)
	path := "/posts/" + p["id"].(string) + "/actions/new"
	f.call(t, "POST", path, f.tokens[1], map[string]any{}, 404)
	f.call(t, "POST", path, f.tokens[0], map[string]any{"context": map[string]string{"command": "/meet"}}, 200)
	posts, _ = f.api.chat.Posts(ctx, a, dm.ID, 10, 0, 0)
	found := false
	for _, post := range posts {
		if post.UserID == a.ID && post.Message == "/ai new" {
			found = true
		}
	}
	if !found {
		raw, _ := json.Marshal(posts)
		t.Fatal(string(raw))
	}
	f.call(t, "POST", "/bots/"+id+"/disable", f.tokens[0], map[string]any{}, 200)
	rows, _ = f.api.botCommands(ctx, a)
	if len(rows) != 0 {
		t.Fatal("disabled command visible")
	}
	f.call(t, "POST", path, f.tokens[0], map[string]any{}, 403)
}
