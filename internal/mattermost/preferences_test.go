package mattermost

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
)

func TestMattChaPreferencesAndEmptyThreadInbox(t *testing.T) {
	f := setup(t)
	get := func(path string) []preference {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v4"+path, nil)
		r.Header.Set("Authorization", "Bearer "+f.tokens[0])
		w := httptest.NewRecorder()
		f.api.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var out []preference
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil || out == nil {
			t.Fatal("expected JSON array", w.Body.String(), e)
		}
		return out
	}
	for _, category := range []string{"teams_order", "group_channel_show", "direct_channel_show", "favorite_channel", "flagged_post"} {
		if len(get("/users/me/preferences/"+category)) != 0 {
			t.Fatal(category)
		}
	}
	saved := []preference{{UserID: chat.ID(f.users[0].ID), Category: "favorite_channel", Name: "test-channel", Value: "true"}}
	f.call(t, "PUT", "/users/me/preferences", f.tokens[0], saved, 200)
	if p := get("/users/me/preferences/favorite_channel"); len(p) != 1 || p[0].Value != "true" {
		t.Fatal(p)
	}
	f.call(t, "GET", "/users/me/preferences/favorite_channel/name/test-channel", f.tokens[0], nil, 200)
	f.call(t, "GET", "/users/"+chat.ID(f.users[0].ID)+"/preferences", f.tokens[1], nil, 403)
	saved[0].UserID = chat.ID(f.users[1].ID)
	f.call(t, "PUT", "/users/me/preferences", f.tokens[0], saved, 403)
	saved[0].UserID = ""
	f.call(t, "POST", "/users/me/preferences/delete", f.tokens[0], saved, 200)
	if len(get("/users/me/preferences/favorite_channel")) != 0 {
		t.Fatal("delete failed")
	}
	f.call(t, "PUT", "/users/me/preferences", f.tokens[0], []preference{{Category: "invalid\x00category", Name: "test"}}, 400)
	path := "/users/me/teams/" + chat.ID(f.users[0].TenantID) + "/threads?per_page=30&extended=true"
	result := f.call(t, "GET", path, f.tokens[0], nil, 200)
	if result["total"] != float64(0) || len(result["threads"].([]any)) != 0 {
		t.Fatal(result)
	}
	f.call(t, "GET", strings.Replace(path, chat.ID(f.users[0].TenantID), chat.ID(f.users[3].TenantID), 1), f.tokens[0], nil, 404)
}
