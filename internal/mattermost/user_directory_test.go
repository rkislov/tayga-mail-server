package mattermost

import (
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMobileUserDirectoryAndNameFallback(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	actor := f.users[0]
	sibling, _ := f.store.CreateDomain(ctx, actor.TenantID, "sibling.test")
	_, e := f.store.CreateUser(ctx, &storage.User{TenantID: actor.TenantID, DomainID: sibling.ID, Email: "hidden@sibling.test", LocalPart: "hidden", Enabled: true, AuthSource: "local"})
	if e != nil {
		t.Fatal(e)
	}
	request := func(method, path, body string, status int) []map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, "/api/v4"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+f.tokens[0])
		w := httptest.NewRecorder()
		f.api.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s %d: %s", method, path, w.Code, w.Body)
		}
		var list []map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &list)
		return list
	}
	list := request("POST", "/users/search", `{"term":"BOB","team_id":"`+chat.ID(actor.TenantID)+`","allow_inactive":false}`, 200)
	if len(list) != 1 || list[0]["id"] != chat.ID(f.users[1].ID) {
		t.Fatal(list)
	}
	if list = request("POST", "/users/search", `{"term":"hidden"}`, 200); len(list) != 0 {
		t.Fatal("foreign domain leaked", list)
	}
	if list = request("POST", "/users/search", `{"term":"outsider"}`, 200); len(list) != 0 {
		t.Fatal("foreign tenant leaked", list)
	}
	list = request("GET", "/users?in_team="+chat.ID(actor.TenantID)+"&sort=last_activity_at&page=0&per_page=100", "", 200)
	for _, model := range list {
		if model["username"] != model["nickname"] {
			t.Fatalf("directory still exposes generated username: %v", model)
		}
	}
	if len(list) != 3 {
		t.Fatal(list)
	}
	request("POST", "/users/search", `{"term":"","team_id":"`+chat.ID(f.users[3].TenantID)+`"}`, 404)
	request("POST", "/users/search", `{"term":"","limit":201}`, 400)
	ac := f.call(t, "GET", "/users/autocomplete?name=bo&in_team="+chat.ID(actor.TenantID), f.tokens[0], nil, 200)
	if len(ac["users"].([]any)) != 1 {
		t.Fatal(ac)
	}
	private := f.call(t, "POST", "/channels", f.tokens[1], map[string]string{"team_id": chat.ID(actor.TenantID), "name": "hidden-room", "display_name": "Private", "type": "P"}, 201)
	request("GET", "/users/autocomplete?name=b&in_channel="+private["id"].(string), "", 404)
	f.call(t, "GET", "/users/username/bob", f.tokens[0], nil, 200)
	f.call(t, "GET", "/users/email/outsider@other.test", f.tokens[0], nil, 404)
	u := &storage.User{ID: "test", DisplayName: "  Роман Кислов  ", Email: "roman@example.com", LocalPart: "roman"}
	if userModel(u, true)["nickname"] != "Роман Кислов" {
		t.Fatal(userModel(u, true))
	}
	u.DisplayName = "  "
	if userModel(u, true)["nickname"] != u.Email {
		t.Fatal(userModel(u, true))
	}
	u.Email = ""
	if u.DisplayLabel() != "roman" {
		t.Fatal(u.DisplayLabel())
	}
	prefs := request("GET", "/users/me/preferences", "", 200)
	if len(prefs) != 1 || prefs[0]["value"] != "nickname_full_name" {
		t.Fatal(prefs)
	}
}

func TestReadableUsernamesMatchAcrossResponses(t *testing.T) {
	u := &storage.User{ID: "stable-id", DisplayName: "Роман Кислов", Email: "roman@example.test", LocalPart: "legacy-roman"}
	for _, self := range []bool{false, true} {
		model := userModel(u, self)
		if model["username"] != "roman" || model["nickname"] != "Роман Кислов" || model["id"] != chat.ID(u.ID) {
			t.Fatal(model)
		}
		u.DisplayName = ""
		if userModel(u, self)["nickname"] != "roman@example.test" {
			t.Fatal("email fallback missing")
		}
		u.DisplayName = "Роман Кислов"
	}
	u.Email = ""
	if userModel(u, false)["username"] != "legacy-roman" {
		t.Fatal("local-part fallback missing")
	}
}
