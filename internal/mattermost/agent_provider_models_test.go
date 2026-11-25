package mattermost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func TestAgentProviderCatalogUsesOnlyScopedSettings(t *testing.T) {
	f := setup(t)
	f.api.scopeAuthorize = func(_ *http.Request, u *storage.User, _ mmsettings.Scope) bool {
		return u.ID == f.users[0].ID || u.ID == f.users[3].ID
	}
	const secret = "private-fixture-model-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("discovery must use GET")
		}
		switch r.URL.Path {
		case "/v1/models":
			if r.Header.Get("Authorization") != "Bearer "+secret {
				write(w, 401, map[string]any{"error": map[string]string{"message": secret}})
				return
			}
			write(w, 200, map[string]any{"data": []map[string]string{{"id": "z-model"}, {"id": "a-model"}, {"id": "a-model"}, {"id": secret}}})
		case "/api/tags":
			if r.Header.Get("Authorization") != "" {
				t.Error("stored key sent to a different provider")
			}
			write(w, 200, map[string]any{"models": []map[string]string{{"model": "ollama:latest"}, {"name": "older:latest"}}})
		case "/redirect/models":
			http.Redirect(w, r, "/redirect-target", http.StatusFound)
		default:
			t.Error("unexpected provider path", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("TAYGA_AGENT_MODEL_CATALOG_URLS", server.URL+"/v1,"+server.URL+","+server.URL+"/redirect")
	provider := agentProvider{ID: "demo", Kind: "openai-compatible", Name: "Demo", URL: server.URL + "/v1", Model: "previous", APIKey: secret}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], agentConfiguration{Providers: []agentProvider{provider}}, 200)
	const path = "/tms/agents/config/models"
	provider.APIKey, provider.Model = "", ""
	f.call(t, "POST", path, f.tokens[1], provider, 403)
	foreign := f.call(t, "POST", path, f.tokens[3], provider, 502)
	if !strings.Contains(foreign["message"].(string), "401") {
		t.Fatal("provider error was not specific", foreign)
	}
	found := f.call(t, "POST", path, f.tokens[0], provider, 200)
	if !reflect.DeepEqual(found["models"], []any{"a-model", "z-model"}) {
		t.Fatal("unexpected provider catalog", found)
	}
	ollama := f.call(t, "POST", path, f.tokens[0], agentProvider{ID: provider.ID, Kind: "ollama", URL: server.URL + "/api/"}, 200)
	if !reflect.DeepEqual(ollama["models"], []any{"older:latest", "ollama:latest"}) {
		t.Fatal("unexpected Ollama catalog", ollama)
	}
	f.call(t, "POST", path, f.tokens[0], agentProvider{Kind: "openai-compatible", URL: server.URL + "/redirect"}, 502)
	f.call(t, "POST", path, f.tokens[0], agentProvider{Kind: "openai-compatible", URL: server.URL + "/not-approved"}, 403)
	stored := f.call(t, "GET", "/tms/agents/config", f.tokens[0], nil, 200)
	b, _ := json.Marshal(stored)
	if strings.Contains(string(b), secret) || !strings.Contains(string(b), "previous") || stored["version"] != float64(1) {
		t.Fatal("checking a model must not change configuration or return keys")
	}
	if strings.Contains(foreign["message"].(string), secret) {
		t.Fatal("provider error leaked key")
	}
}
