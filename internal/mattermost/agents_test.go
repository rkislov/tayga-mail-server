package mattermost

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tayga/tms/internal/chat"
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

func TestAgentsWebAndMattermostShareThread(t *testing.T) {
	f, _, sent := agentsFixture(t)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	id, channel, root := conv["id"].(string), conv["channel_id"].(string), conv["root_id"].(string)
	f.call(t, "POST", "/tms/agents/conversations/"+id+"/messages", f.tokens[0], map[string]string{"content": "from web"}, 200)
	f.call(t, "POST", "/posts", f.tokens[0], map[string]string{"channel_id": channel, "message": "from phone"}, 201)
	got := waitAgentMessages(t, f, f.tokens[0], id, 4)
	if len(got["messages"].([]any)) != 4 || len(*sent) != 4 || (*sent)[3].Content != "from phone" {
		t.Fatalf("histories diverged: %#v", got)
	}
	f.call(t, "GET", "/posts/"+root+"/thread", f.tokens[1], nil, 404)
	f.call(t, "POST", "/posts", f.tokens[1], map[string]string{"channel_id": channel, "message": "intrusion"}, 404)
	// The other user has a different DM and model context, even with the same bot.
	bot, err := f.api.agentBot(t.Context(), f.users[1], "helper")
	if err != nil {
		t.Fatal(err)
	}
	dm, err := f.api.chat.Direct(t.Context(), f.users[1], chat.ID(bot.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.call(t, "POST", "/posts", f.tokens[1], map[string]string{"channel_id": dm.ID, "message": "bob phone"}, 201)
	f.api.agentsMu.Lock()
	bobHistory := agentHistory{}
	_ = f.api.agentLoad(t.Context(), agentHistoryKey(f.users[1]), &bobHistory)
	f.api.agentsMu.Unlock()
	waitAgentMessages(t, f, f.tokens[1], bobHistory.Conversations[0].ID, 2)
	for _, m := range *sent {
		if strings.Contains(m.Content, "from web") || strings.Contains(m.Content, "from phone") {
			t.Fatal("cross-user leak")
		}
	}
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
			answer, err := agentCompletionWithClient(t.Context(), agentProvider{Kind: "deepseek", URL: "https://api.deepseek.com", Model: "deepseek-flash", APIKey: "deepseek-test-key", Thinking: thinking}, []agentMessage{{Role: "user", Content: "Тест"}}, agentFixtureClient(server.URL))
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

// Only synthetic tests remap a pinned cloud endpoint to a loopback fixture.
type agentFixtureTransport func(*http.Request) (*http.Response, error)

func (fn agentFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func agentFixtureClient(target string) *http.Client {
	return &http.Client{Transport: agentFixtureTransport(func(r *http.Request) (*http.Response, error) {
		local := r.Clone(r.Context())
		endpoint, _ := url.Parse(target)
		copyURL := *r.URL
		copyURL.Scheme = endpoint.Scheme
		copyURL.Host = endpoint.Host
		local.URL = &copyURL
		return http.DefaultTransport.RoundTrip(local)
	})}
}
func TestAgentsOpenAIRequest(t *testing.T) {
	client := &http.Client{Transport: agentFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.openai.com/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer openai-fixture-key" {
			t.Error("incorrect OpenAI endpoint or authentication")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["max_completion_tokens"] != float64(16384) || payload["max_tokens"] != nil || payload["store"] != false || payload["model"] != "gpt-6.2-sol" {
			t.Error("OpenAI token limit or storage policy was not transmitted")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"OpenAI answer"}}]}`)), Header: make(http.Header)}, nil
	})}
	answer, err := agentCompletionWithClient(t.Context(), agentProvider{Kind: "openai", URL: "https://api.openai.com/v1", Model: "gpt-6.2-sol", APIKey: "openai-fixture-key"}, []agentMessage{{Role: "user", Content: "fixture question"}}, client)
	if err != nil || answer != "OpenAI answer" {
		t.Fatalf("OpenAI adapter: %q %v", answer, err)
	}
}
func TestAgentsNativeOllamaConversation(t *testing.T) {
	f, _, _ := agentsFixture(t)
	var sent []agentMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Header.Get("Authorization") != "" {
			t.Error("Ollama native endpoint or optional authentication is incorrect")
		}
		var payload struct {
			Messages []agentMessage `json:"messages"`
			Stream   bool           `json:"stream"`
			Options  struct {
				Predict int `json:"num_predict"`
			} `json:"options"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Stream || payload.Options.Predict != 4096 {
			t.Error("Ollama request did not disable streaming or bound output")
		}
		sent = payload.Messages
		write(w, 200, map[string]any{"message": map[string]any{"role": "assistant", "content": "Ollama answer", "thinking": "NOT_STORED"}, "done": true})
	}))
	defer server.Close()
	c := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &c); err != nil {
		t.Fatal(err)
	}
	c.Providers[0].Kind = "ollama"
	c.Providers[0].URL = server.URL
	c.Providers[0].APIKey = ""
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], c, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	result := f.call(t, "POST", "/tms/agents/conversations/"+conv["id"].(string)+"/messages", f.tokens[0], map[string]string{"content": "only this user's question"}, 200)
	raw, _ := json.Marshal(result)
	if !strings.Contains(string(raw), "Ollama answer") || strings.Contains(string(raw), "NOT_STORED") {
		t.Fatal("incorrect saved Ollama response")
	}
	if len(sent) != 2 || sent[1].Content != "only this user's question" {
		t.Fatal("incorrect private context")
	}
	f.call(t, "GET", "/tms/agents/conversations/"+conv["id"].(string), f.tokens[1], nil, 404)
}
func TestAgentsProviderEndpointPolicyHTTP(t *testing.T) {
	f := setup(t)
	f.api.pluginAdmins = []string{f.users[0].Email}
	version := int64(0)
	for i, tc := range []struct {
		name, kind, endpoint, key string
		status                    int
	}{
		{"OpenAI", "openai", "https://api.openai.com/v1", "fixture", 200},
		{"OpenAI missing key", "openai", "https://api.openai.com/v1", "", 400},
		{"OpenAI wrong host", "openai", "https://api.openai.com.attacker.test/v1", "fixture", 400},
		{"Ollama loopback", "ollama", "http://127.0.0.1:11434", "", 200},
		{"Ollama private LAN", "ollama", "http://192.168.1.20:11434", "", 200},
		{"Ollama IPv6 ULA", "ollama", "http://[fd00::1]:11434", "", 200},
		{"Ollama public HTTP", "ollama", "http://8.8.8.8:11434", "", 400},
		{"Ollama metadata", "ollama", "http://169.254.169.254", "", 400},
		{"Ollama credentials in URL", "ollama", "http://user:pass@127.0.0.1:11434", "", 400},
		{"Ollama HTTPS", "ollama", "https://ollama.example.test", "", 200},
		{"Compatible private HTTP still denied", "openai-compatible", "http://192.168.1.20:11434/v1", "", 400},
		{"Unknown kind", "unknown", "https://example.test/v1", "fixture", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := emptyAgentConfig()
			config.Version = version
			config.Providers = []agentProvider{{ID: fmt.Sprintf("provider-%d", i), Kind: tc.kind, Name: tc.name, URL: tc.endpoint, Model: "fixture", APIKey: tc.key}}
			result := f.call(t, "PUT", "/tms/agents/config", f.tokens[0], config, tc.status)
			if tc.status == 200 {
				version = int64(result["version"].(float64))
			}
		})
	}
}

func waitAgentMessages(t *testing.T, f *fixture, token, id string, n int) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := f.call(t, "GET", "/tms/agents/conversations/"+id, token, nil, 200)
		if len(got["messages"].([]any)) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatal("agent response timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAgentsRepliesKeepCausalPostOrder(t *testing.T) {
	f, _, _ := agentsFixture(t)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	root := conv["root_id"].(string)
	if _, err := f.store.DB().ExecContext(t.Context(), `UPDATE chat_posts SET created_at=? WHERE id=?`, time.Now().UnixMilli()+10000, root); err != nil {
		t.Fatal(err)
	}
	f.call(t, "POST", "/tms/agents/conversations/"+conv["id"].(string)+"/messages", f.tokens[0], map[string]string{"content": "ordered"}, 200)
	posts, err := f.api.chat.Thread(t.Context(), f.users[0], root)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 3 || posts[1].UserID != f.users[0].ID || posts[1].CreatedAt <= posts[0].CreatedAt || posts[2].CreatedAt <= posts[1].CreatedAt {
		t.Fatalf("causality lost: %#v", posts)
	}
}

func TestAgentTitlesSurviveProviderRejection(t *testing.T) {
	f, _, _ := agentsFixture(t)
	rejected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		write(w, 429, map[string]any{"error": map[string]string{"code": "credit_balance_exhausted", "message": "PRIVATE_CREDIT_DETAIL"}})
	}))
	defer rejected.Close()
	cfg := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Providers[0].URL = rejected.URL + "/v1"
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], cfg, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	path := "/tms/agents/conversations/" + conv["id"].(string)
	question := "Первый вопрос\n" + strings.Repeat("длинный ", 14)
	got := f.call(t, "POST", path+"/messages", f.tokens[0], map[string]string{"content": question}, 502)
	text, _ := got["message"].(string)
	if !strings.Contains(text, "баланс") || strings.Contains(text, "PRIVATE_CREDIT_DETAIL") {
		t.Fatalf("unexpected rejection message %q", text)
	}
	saved := f.call(t, "GET", path, f.tokens[0], nil, 200)
	title, _ := saved["title"].(string)
	if !strings.HasPrefix(title, "Первый вопрос ") || !strings.HasSuffix(title, "…") || len([]rune(title)) > 60 {
		t.Fatalf("unexpected title %q", title)
	}
}
