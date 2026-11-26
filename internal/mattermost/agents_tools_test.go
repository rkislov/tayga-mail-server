package mattermost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAgentsToolsPrepareConfirmAndPreventReplay(t *testing.T) {
	f, _, _ := agentsFixture(t)
	channel, err := f.api.chat.CreateChannel(t.Context(), f.users[0], "agent-target", "Test target", "O")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []agentMessage `json:"messages"`
			Tools    []any          `json:"tools"`
		}
		if json.NewDecoder(r.Body).Decode(&payload) != nil {
			t.Error("decode")
			return
		}
		if len(payload.Tools) != 1 {
			t.Errorf("tools: %d", len(payload.Tools))
		}
		if calls.Add(1)%2 == 1 {
			args, _ := json.Marshal(map[string]string{"channel_id": channel.ID, "text": "ONLY AFTER CONFIRMATION"})
			write(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "draft_call", "type": "function", "function": map[string]any{"name": "chat_draft", "arguments": string(args)}}}}}}})
			return
		}
		last := payload.Messages[len(payload.Messages)-1]
		if last.Role != "tool" || last.ToolCallID != "draft_call" {
			t.Errorf("incorrect tool result: %#v", last)
		}
		write(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": "Черновик готов"}}}})
	}))
	defer model.Close()
	config := emptyAgentConfig()
	if err = f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &config); err != nil {
		t.Fatal(err)
	}
	config.Providers[0].URL = model.URL
	config.Agents[0].Tools = []string{"chat_send"}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], config, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	id := conv["id"].(string)
	turn := f.call(t, "POST", "/tms/agents/conversations/"+id+"/messages", f.tokens[0], map[string]string{"content": "Подготовь сообщение"}, 200)
	posts, _ := f.api.chat.Posts(t.Context(), f.users[0], channel.ID, 100, 0, 0)
	if len(posts) != 0 {
		t.Fatal("sent without confirmation")
	}
	action := turn["actions"].([]any)[0].(map[string]any)
	actionID := action["id"].(string)
	path := "/tms/agents/conversations/" + id + "/actions/" + actionID
	f.call(t, "POST", path, f.tokens[1], map[string]any{}, 404)
	thread, err := f.api.chat.Thread(t.Context(), f.users[0], conv["root_id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	botPost := ""
	for _, p := range thread {
		if len(p.Props) > 0 {
			botPost = p.ID
		}
	}
	if botPost == "" {
		t.Fatal("missing native confirmation button")
	}
	f.call(t, "POST", "/posts/"+botPost+"/actions/agentconfirm-"+actionID, f.tokens[0], map[string]string{"text": "ATTACKER OVERRIDE", "channel_id": "other"}, 200)
	f.call(t, "POST", path, f.tokens[0], map[string]any{}, 409)
	posts, _ = f.api.chat.Posts(t.Context(), f.users[0], channel.ID, 100, 0, 0)
	if len(posts) != 1 || posts[0].Message != "ONLY AFTER CONFIRMATION" {
		t.Fatal("confirmation not exact or replayed")
	}
}

func TestAgentsWebStateAndSearchIsolation(t *testing.T) {
	f, _, _ := agentsFixture(t)
	searchCalls := 0
	search := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchCalls++
		if r.URL.Path != "/search" || r.URL.Query().Get("q") != "test" || r.Header.Get("Authorization") != "" {
			t.Error("invalid search request")
		}
		write(w, 200, map[string]any{"results": []any{map[string]string{"title": "source", "url": "https://example.com/source", "content": "untrusted"}, map[string]string{"title": "bad", "url": "javascript:alert(1)"}}})
	}))
	defer search.Close()
	config := emptyAgentConfig()
	_ = f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &config)
	config.Search = agentSearchConfig{Kind: "searxng", URL: search.URL}
	config.Agents[0].Tools = []string{"web_search"}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], config, 200)
	if len(agentToolSchemas(config.Agents[0], false, config.Search, true, false)) != 0 {
		t.Fatal("web allowed while off")
	}
	if len(agentToolSchemas(config.Agents[0], true, config.Search, true, false)) != 1 {
		t.Fatal("missing web tool")
	}
	result, err := agentWebSearch(t.Context(), config.Search, "test")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	if searchCalls != 1 || strings.Contains(string(raw), "javascript:") {
		t.Fatal("unsafe search result")
	}
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	id := conv["id"].(string)
	f.call(t, "PATCH", "/tms/agents/conversations/"+id, f.tokens[1], map[string]bool{"web_enabled": true}, 404)
	f.call(t, "PATCH", "/tms/agents/conversations/"+id, f.tokens[0], map[string]bool{"web_enabled": true}, 200)
	got := f.call(t, "GET", "/tms/agents/conversations/"+id, f.tokens[0], nil, 200)
	if got["web_enabled"] != true {
		t.Fatal("web state not persisted")
	}
	// No tools or owner secrets are inherited by an unrelated domain.
	foreign := agentDefinition{}
	if len(agentToolSchemas(foreign, true, config.Search, true, false)) != 0 {
		t.Fatal("tools inherited")
	}
	if _, err = f.api.agentReadTool(t.Context(), f.users[1], config.Search, "chat_search", map[string]string{"channel_id": conv["channel_id"].(string)}); err == nil {
		t.Fatal("private DM read by other user")
	}
}
