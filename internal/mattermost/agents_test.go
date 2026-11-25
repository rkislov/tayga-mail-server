package mattermost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func agentsFixture(t *testing.T) (*fixture, *httptest.Server, *[]agentMessage) {
	t.Helper()
	f := setup(t)
	f.api.scopeAuthorize = func(_ *http.Request, u *storage.User, scope mmsettings.Scope) bool {
		return u.ID == f.users[0].ID && scope.TenantID == u.TenantID && scope.DomainID == u.DomainID
	}
	messages := []agentMessage{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer sentinel-provider-secret" {
			t.Error("incorrect provider request")
		}
		var payload struct {
			Messages []agentMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		messages = payload.Messages
		write(w, 200, map[string]any{"choices": []any{map[string]any{"message": agentMessage{Role: "assistant", Content: "Ответ модели"}}}})
	}))
	t.Cleanup(server.Close)
	config := agentConfiguration{Providers: []agentProvider{{ID: "model", Name: "Test", URL: server.URL + "/v1", Model: "fixture", APIKey: "sentinel-provider-secret"}}, Agents: []agentDefinition{{ID: "helper", Name: "Помощник", ProviderID: "model", Enabled: true, Instructions: "PRIVATE_SYSTEM", SkillIDs: []string{"brief"}}}, Skills: []agentSkill{{ID: "brief", Name: "Кратко", Instructions: "SKILL_INSTRUCTIONS"}}}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], config, 200)
	return f, server, &messages
}
func TestAgentsScopedCatalogAndPrivateHistory(t *testing.T) {
	f, _, sent := agentsFixture(t)
	bootstrap := f.call(t, "GET", "/tms/agents", f.tokens[1], nil, 200)
	raw, _ := json.Marshal(bootstrap)
	for _, secret := range []string{"sentinel-provider-secret", "PRIVATE_SYSTEM", "SKILL_INSTRUCTIONS"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("catalog leaked %s", secret)
		}
	}
	f.call(t, "GET", "/tms/agents/config", f.tokens[1], nil, 403)
	f.call(t, "POST", "/tms/agents/conversations", f.tokens[3], map[string]string{"agent_id": "helper"}, 404)
	// Another domain in the same tenant must not inherit providers or agents.
	domain, err := f.store.CreateDomain(t.Context(), f.users[0].TenantID, "agents-sibling.test")
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := f.store.CreateUser(t.Context(), &storage.User{TenantID: f.users[0].TenantID, DomainID: domain.ID, Email: "hidden@agents-sibling.test", LocalPart: "hidden", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := f.api.auth.Tokens.IssueTokens(t.Context(), sibling.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.call(t, "POST", "/tms/agents/conversations", pair.AccessToken, map[string]string{"agent_id": "helper"}, 404)
	chat := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	id := chat["id"].(string)
	for _, token := range []string{f.tokens[1], f.tokens[3], pair.AccessToken} {
		for _, method := range []string{"GET", "DELETE"} {
			f.call(t, method, "/tms/agents/conversations/"+id, token, nil, 404)
		}
		f.call(t, "POST", "/tms/agents/conversations/"+id+"/messages", token, map[string]string{"content": "intrusion"}, 404)
	}
	reply := f.call(t, "POST", "/tms/agents/conversations/"+id+"/messages", f.tokens[0], map[string]string{"content": "owner-private"}, 200)
	if len(reply["messages"].([]any)) != 2 {
		t.Fatal("missing saved turn")
	}
	if len(*sent) != 2 || !strings.Contains((*sent)[0].Content, "SKILL_INSTRUCTIONS") || (*sent)[1].Content != "owner-private" {
		t.Fatal("model instructions or history missing")
	}
	bob := f.call(t, "POST", "/tms/agents/conversations", f.tokens[1], map[string]string{"agent_id": "helper"}, 201)
	f.call(t, "POST", "/tms/agents/conversations/"+bob["id"].(string)+"/messages", f.tokens[1], map[string]string{"content": "bob-only"}, 200)
	for _, m := range *sent {
		if strings.Contains(m.Content, "owner-private") {
			t.Fatal("another user's context reached provider")
		}
	}
	f.call(t, "GET", "/tms/agents", "", nil, 401)
}
func TestAgentsSecretRetentionVersionAndDisabledAgent(t *testing.T) {
	f, _, _ := agentsFixture(t)
	raw := f.call(t, "GET", "/tms/agents/config", f.tokens[0], nil, 200)
	data, _ := json.Marshal(raw)
	if strings.Contains(string(data), "sentinel-provider-secret") {
		t.Fatal("admin response echoed credential")
	}
	var config agentConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	chat := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	config.Agents[0].Enabled = false
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], config, 200)
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], config, 409)
	stored := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Providers[0].APIKey != "sentinel-provider-secret" {
		t.Fatal("key lost by edit")
	}
	f.call(t, "POST", "/tms/agents/conversations/"+chat["id"].(string)+"/messages", f.tokens[0], map[string]string{"content": "disabled"}, 404)
}
func TestAgentsConcurrentTurnAndRevocation(t *testing.T) {
	f, _, _ := agentsFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(entered)
		<-release
		write(w, 200, map[string]any{"choices": []any{map[string]any{"message": agentMessage{Content: "must not commit"}}}})
	}))
	defer server.Close()
	c := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &c); err != nil {
		t.Fatal(err)
	}
	c.Providers[0].URL = server.URL
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], c, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	path := "/api/v4/tms/agents/conversations/" + conv["id"].(string) + "/messages"
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest("POST", path, strings.NewReader(`{"content":"one"}`))
		r.Header.Set("Authorization", "Bearer "+f.tokens[0])
		w := httptest.NewRecorder()
		f.api.ServeHTTP(w, r)
		done <- w
	}()
	<-entered
	f.call(t, "POST", strings.TrimPrefix(path, "/api/v4"), f.tokens[0], map[string]string{"content": "duplicate"}, 409)
	if err := f.api.auth.Tokens.RevokeAccess(t.Context(), f.tokens[0]); err != nil {
		t.Fatal(err)
	}
	close(release)
	if w := <-done; w.Code != 502 {
		t.Fatalf("revoked turn: %d", w.Code)
	}
	h := agentHistory{}
	if err := f.api.agentLoad(t.Context(), agentHistoryKey(f.users[0]), &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Conversations[0].Messages) != 0 || h.Conversations[0].Pending != "" || calls.Load() != 1 {
		t.Fatal("revocation or duplicate protection failed")
	}
}
func TestAgentsProviderRejectsRedirectAndInvalidEndpoint(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Store(true) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer redirect.Close()
	_, err := agentCompletion(t.Context(), agentProvider{URL: redirect.URL, APIKey: "secret"}, []agentMessage{{Role: "user", Content: "private"}})
	if err == nil || leaked.Load() {
		t.Fatal("provider followed redirect")
	}
	for _, url := range []string{"http://example.com/v1", "https://name:password@example.com", "https://example.com/?api_key=secret", "file:///etc/passwd"} {
		if validAgentEndpoint(url) {
			t.Fatalf("accepted unsafe endpoint %s", url)
		}
	}
}

func TestAgentsDeepSeekConfigurationAndRequest(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		t.Run(map[bool]string{false: "chat", true: "thinking"}[thinking], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer deepseek-test-key" {
					t.Error("incorrect DeepSeek endpoint or authentication")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				mode := "disabled"
				if thinking {
					mode = "enabled"
				}
				if body["model"] != "deepseek-flash" || body["thinking"].(map[string]any)["type"] != mode {
					t.Error("DeepSeek model or thinking switch not transmitted")
				}
				write(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "Ответ DeepSeek", "reasoning_content": "NOT_STORED"}}}})
			}))
			defer server.Close()
			answer, err := agentCompletion(t.Context(), agentProvider{Kind: "deepseek", URL: server.URL, Model: "deepseek-flash", APIKey: "deepseek-test-key", Thinking: thinking}, []agentMessage{{Role: "user", Content: "Тест"}})
			if err != nil || answer != "Ответ DeepSeek" {
				t.Fatalf("DeepSeek adapter: %q %v", answer, err)
			}
		})
	}
	for _, url := range []string{"https://api.deepseek.com", "https://api.deepseek.com/v1", "https://attacker.test"} {
		c := emptyAgentConfig()
		c.Providers = []agentProvider{{ID: "deepseek", Kind: "deepseek", Name: "DeepSeek", URL: url, Model: "deepseek-flash", APIKey: "test-key"}}
		if got := c.validate(emptyAgentConfig()); got != (url != "https://attacker.test") {
			t.Fatalf("DeepSeek endpoint validation: %s", url)
		}
	}
}
