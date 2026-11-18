package mattermost

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
)

func TestMattChaUnreadWindowAndHistoryCursors(t *testing.T) {
	f := setup(t)
	a, b := f.tokens[0], f.tokens[1]
	dm := f.call(t, "POST", "/channels/direct", a, []string{chat.ID(f.users[0].ID), chat.ID(f.users[1].ID)}, 201)
	id := dm["id"].(string)
	posts := []map[string]any{}
	for i := 0; i < 5; i++ {
		p := f.call(t, "POST", "/posts", b, map[string]string{"channel_id": id, "message": "history"}, 201)
		posts = append(posts, p)
		if _, e := f.store.DB().Exec(`UPDATE chat_posts SET created_at=? WHERE id=?`, 1000+i, p["id"]); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := f.store.DB().Exec(`UPDATE chat_members SET last_viewed_at=? WHERE channel_id=? AND user_id=?`, 1001, id, f.users[0].ID); e != nil {
		t.Fatal(e)
	}
	r := f.call(t, "GET", "/users/me/channels/"+id+"/unread", a, nil, 200)
	if r["msg_count"] != float64(3) || r["mention_count"] != float64(3) {
		t.Fatal(r)
	}
	window := f.call(t, "GET", "/users/me/channels/"+id+"/posts/unread?limit_before=1&limit_after=2", a, nil, 200)
	order := window["order"].([]any)
	if len(order) != 3 || order[0] != posts[3]["id"] || order[2] != posts[1]["id"] {
		t.Fatal(order)
	}
	before := f.call(t, "GET", "/channels/"+id+"/posts?before="+posts[2]["id"].(string)+"&per_page=2", a, nil, 200)["order"].([]any)
	if len(before) != 2 || before[0] != posts[1]["id"] || before[1] != posts[0]["id"] {
		t.Fatal(before)
	}
	after := f.call(t, "GET", "/channels/"+id+"/posts?after="+posts[2]["id"].(string)+"&per_page=2", a, nil, 200)["order"].([]any)
	if len(after) != 2 || after[0] != posts[4]["id"] || after[1] != posts[3]["id"] {
		t.Fatal(after)
	}
	f.call(t, "GET", "/users/me/channels/"+id+"/unread", f.tokens[2], nil, 404)
	f.call(t, "GET", "/users/"+chat.ID(f.users[1].ID)+"/channels/"+id+"/unread", a, nil, 403)
	f.call(t, "GET", "/users/me/channels/"+id+"/posts/unread?limit_before=200&limit_after=1", a, nil, 400)
	f.call(t, "GET", "/channels/"+id+"/posts?before="+posts[0]["id"].(string)+"&after="+posts[1]["id"].(string), a, nil, 400)
	req := httptest.NewRequest("GET", "/api/v4/users/me/channels/"+id+"/posts/unread?limit_before=0&limit_after=0", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer "+a)
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, req)
	var result struct {
		Order []string `json:"order"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Order == nil {
		t.Fatal(w.Body.String())
	}
}
