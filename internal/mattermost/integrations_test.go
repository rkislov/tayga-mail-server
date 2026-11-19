package mattermost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
)

func TestAPI12DiscoveryAndIntegrationContracts(t *testing.T) {
	f := setup(t)
	config := f.call(t, "GET", "/config/client?format=old", "", nil, 200)
	if config["Version"] != "12.0.0" {
		t.Fatal("incorrect discovery version")
	}
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, httptest.NewRequest("GET", "/api/v4/system/ping", nil))
	if !strings.HasPrefix(w.Header().Get("X-Version-ID"), "12.0.0.") {
		t.Fatal("discovery header disagrees with version")
	}
	a, b := f.tokens[0], f.tokens[1]
	teamName := "tms-" + chat.ID(f.users[0].TenantID)
	channel := f.call(t, "POST", "/channels", a, map[string]string{"team_id": chat.ID(f.users[0].TenantID), "name": "alerts", "display_name": "Alerts", "type": "O"}, 201)["id"].(string)
	f.call(t, "POST", "/channels/"+channel+"/members", a, map[string]string{"user_id": chat.ID(f.users[1].ID)}, 201)
	// Zabbix's official media type uses team/channel names, API posts and replies.
	got := f.call(t, "GET", "/teams/name/"+teamName+"/channels/name/alerts", b, nil, 200)
	if got["id"] != channel {
		t.Fatal("channel name lookup changed identity")
	}
	f.call(t, "GET", "/teams/name/"+teamName, b, nil, 200)
	f.call(t, "GET", "/teams/name/"+teamName+"/channels/name/alerts", f.tokens[3], nil, 404)
	problem := f.call(t, "POST", "/posts", b, map[string]any{"channel_id": channel, "props": map[string]any{"attachments": []any{map[string]any{"color": "#E97659", "title": "Fixture incident", "text": "Problem started"}}}}, 201)
	f.call(t, "POST", "/posts", b, map[string]any{"channel_id": channel, "root_id": problem["id"], "props": map[string]any{"attachments": []any{map[string]any{"color": "#009900", "text": "Problem resolved"}}}}, 201)
	thread := f.call(t, "GET", "/posts/"+problem["id"].(string)+"/thread", a, nil, 200)
	if len(thread["order"].([]any)) != 2 {
		t.Fatal("Zabbix event thread lost")
	}
	// Existing versions correctly reject edits rather than pretending an alarm
	// was resolved. Zabbix send_mode=event is the accepted contract here.
	f.call(t, "PUT", "/posts/"+problem["id"].(string), b, map[string]any{"message": "resolution"}, 501)
	on := true
	if err := f.api.Settings().Save(t.Context(), mmsettings.Scope{}, mmsettings.Overrides{PluginsEnabled: &on}); err != nil {
		t.Fatal(err)
	}
	hook := f.call(t, "POST", "/hooks/incoming", a, map[string]string{"channel_id": channel, "display_name": "Fixture integrations"}, 201)["id"].(string)
	invoke := func(contentType, body string, status int) {
		t.Helper()
		r := httptest.NewRequest("POST", "/hooks/"+hook, strings.NewReader(body))
		r.Header.Set("Content-Type", contentType)
		w := httptest.NewRecorder()
		f.api.ServeIncomingHook(w, r)
		if w.Code != status {
			t.Fatalf("hook response: %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	redmine := `{"text":"Issue #42 updated","channel":"#alerts","username":"Redmine","attachments":[{"title":"Fixture issue","text":"Status changed"}]}`
	invoke("application/x-www-form-urlencoded", url.Values{"payload": {redmine}}.Encode(), 200)
	jenkins := `{"text":"Build notification","channel":"alerts","username":"Jenkins","attachments":[{"color":"good","text":"Fixture build succeeded"}]}`
	invoke("application/json", jenkins, 200)
	invoke("application/x-www-form-urlencoded", "payload=%7B%7D&payload=%7B%7D", 400)
	invoke("application/x-www-form-urlencoded", "payload=not-json", 400)
	invoke("application/json", `{"text":"escape","channel":"another-channel"}`, 403)
	fetched := f.call(t, "GET", "/channels/"+channel+"/posts", b, nil, 200)
	raw, err := json.Marshal(fetched)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Fixture build succeeded") || !strings.Contains(string(raw), "Fixture issue") {
		t.Fatal("integration attachments lost")
	}
	f.call(t, "DELETE", "/hooks/incoming/"+hook, a, nil, 200)
	invoke("application/json", jenkins, http.StatusNotFound)
}
