package mattermost

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/tayga/tms/internal/chat"
)

func categoriesFrom(t *testing.T, m map[string]any) []sidebarCategory {
	t.Helper()
	raw, err := json.Marshal(m["categories"])
	if err != nil {
		t.Fatal(err)
	}
	var categories []sidebarCategory
	if err = json.Unmarshal(raw, &categories); err != nil {
		t.Fatal(err)
	}
	return categories
}
func TestIOSSidebarCategoriesAndIsolation(t *testing.T) {
	f := setup(t)
	a, b, o := f.tokens[0], f.tokens[1], f.tokens[3]
	team := chat.ID(f.users[0].TenantID)
	path := "/users/me/teams/" + team + "/channels/categories"
	result := f.call(t, "GET", path, a, nil, 200)
	cats := categoriesFrom(t, result)
	if len(cats) != 3 || len(result["order"].([]any)) != 3 {
		t.Fatal(result)
	}
	if cats[0].Type != "favorites" || cats[1].Type != "channels" || cats[2].Type != "direct_messages" || len(cats[1].ChannelIDs) != 1 {
		t.Fatal(cats)
	}
	channelID := cats[1].ChannelIDs[0]
	dm := f.call(t, "POST", "/channels/direct", a, []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	cats = categoriesFrom(t, f.call(t, "GET", path, a, nil, 200))
	if len(cats[2].ChannelIDs) != 1 || cats[2].ChannelIDs[0] != dm["id"] {
		t.Fatal(cats)
	}
	favorite := cats[0]
	favorite.ChannelIDs = []string{channelID}
	favorite.Collapsed = true
	f.call(t, "PUT", path, a, []sidebarCategory{favorite}, 200)
	f.api = New(f.store, f.api.auth, f.api.log, "")
	cats = categoriesFrom(t, f.call(t, "GET", path, a, nil, 200))
	if len(cats[0].ChannelIDs) != 1 || !cats[0].Collapsed || len(cats[1].ChannelIDs) != 0 {
		t.Fatal(cats)
	}
	// Preferences are private even between users in the same team.
	other := categoriesFrom(t, f.call(t, "GET", path, b, nil, 200))
	if len(other[0].ChannelIDs) != 0 || other[0].ID == cats[0].ID {
		t.Fatal(other)
	}
	f.call(t, "GET", "/users/"+chat.ID(f.users[0].ID)+"/teams/"+team+"/channels/categories", b, nil, 403)
	f.call(t, "GET", path, o, nil, 404)
	private := f.call(t, "POST", "/channels", b, map[string]string{"team_id": team, "name": "bob-only", "display_name": "Private", "type": "P"}, 201)
	favorite.ChannelIDs = []string{private["id"].(string)}
	f.call(t, "PUT", path, a, []sidebarCategory{favorite}, 403)
	favorite.ChannelIDs = []string{channelID}
	favorite.UserID = chat.ID(f.users[1].ID)
	f.call(t, "PUT", path, a, []sidebarCategory{favorite}, 403)
	order := []string{cats[2].ID, cats[0].ID, cats[1].ID}
	f.call(t, "PUT", path+"/order", a, order, 200)
	cats = categoriesFrom(t, f.call(t, "GET", path, a, nil, 200))
	if cats[0].Type != "direct_messages" {
		t.Fatal(cats)
	}
	f.call(t, "PUT", path+"/order", a, []string{cats[0].ID, cats[0].ID, cats[1].ID}, 400)
	// Channels joined after a stored sidebar was written must remain visible.
	c, err := f.api.chat.CreateChannel(context.Background(), f.users[0], "new-room", "New", "O")
	if err != nil {
		t.Fatal(err)
	}
	cats = categoriesFrom(t, f.call(t, "GET", path, a, nil, 200))
	found := false
	for _, cat := range cats {
		for _, id := range cat.ChannelIDs {
			if id == c.ID {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("new channel missing from sidebar")
	}
	req := httptest.NewRequest("GET", "/", nil)
	state, _, err := f.api.sidebarState(req, f.users[0])
	if err != nil || len(state) != 3 {
		t.Fatalf("sidebar state: %v", err)
	}
}

func TestMobileMetadataBootstrap(t *testing.T) {
	f := setup(t)
	token := f.tokens[0]
	status := f.call(t, "GET", "/users/me/status", token, nil, 200)
	if status["user_id"] != chat.ID(f.users[0].ID) {
		t.Fatal(status)
	}
	f.call(t, "POST", "/users/status/ids", token, []string{chat.ID(f.users[0].ID)}, 200)
	f.call(t, "GET", "/users/me/sessions", token, nil, 200)
	f.call(t, "GET", "/plugins/webapp", token, nil, 200)
	f.call(t, "PUT", "/users/me/status", token, map[string]string{"status": "dnd"}, 200)
	status = f.call(t, "GET", "/users/me/status", token, nil, 200)
	if status["status"] != "dnd" {
		t.Fatal(status)
	}
	f.call(t, "PUT", "/users/"+chat.ID(f.users[1].ID)+"/status", token, map[string]string{"status": "dnd"}, 403)
	f.call(t, "GET", "/users/"+chat.ID(f.users[3].ID)+"/image", token, nil, 404)
	f.call(t, "GET", "/users/"+chat.ID(f.users[1].ID)+"/sessions", token, nil, 403)
}
