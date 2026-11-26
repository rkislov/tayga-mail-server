package mattermost

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type agentProviderTransportFunc func(*http.Request) (*http.Response, error)

func (f agentProviderTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestOpenAIResponsesToolResultKeepsReasoningAndCallID(t *testing.T) {
	provider := agentProvider{Kind: "openai", URL: "https://api.openai.com/v1", Model: "gpt-6.2-sol", APIKey: "test-key"}
	messages := []agentMessage{{Role: "system", Content: "Diagnostic"}, {Role: "user", Content: "Call the test tool"}}
	tools := agentToolSchemas(agentDefinition{Tools: []string{"chat_read"}}, false, agentSearchConfig{}, false, false)
	step := 0
	client := &http.Client{Transport: agentProviderTransportFunc(func(r *http.Request) (*http.Response, error) {
		step++
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("unexpected Responses request")
		}
		var body struct {
			Store   bool             `json:"store"`
			Include []string         `json:"include"`
			Input   []map[string]any `json:"input"`
			Tools   []map[string]any `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Store || len(body.Include) != 1 || body.Include[0] != "reasoning.encrypted_content" || len(body.Tools) < 1 || body.Tools[0]["name"] != "chat_channels" {
			t.Fatalf("invalid function or retention settings: %+v", body)
		}
		result := `{"status":"completed","output":[{"type":"reasoning","id":"rs_test","encrypted_content":"opaque","summary":[]},{"type":"function_call","id":"fc_test","call_id":"call_test","name":"chat_channels","arguments":"{}"}]}`
		if step == 2 {
			if len(body.Input) != 5 || body.Input[2]["encrypted_content"] != "opaque" || body.Input[3]["call_id"] != "call_test" || body.Input[4]["call_id"] != "call_test" || body.Input[4]["type"] != "function_call_output" || body.Input[4]["output"] != "[]" {
				t.Fatalf("unexpected second turn: %+v", body.Input)
			}
			result = `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(result)), Header: make(http.Header)}, nil
	})}
	first, err := agentModelReply(t.Context(), provider, messages, tools, client)
	if err != nil || len(first.ToolCalls) != 1 || first.ToolCalls[0].ID != "call_test" {
		t.Fatalf("first call got %+v, %v", first, err)
	}
	payload, _ := json.Marshal(first)
	if strings.Contains(string(payload), "opaque") {
		t.Fatal("reasoning must never be part of saved conversation JSON")
	}
	messages = append(messages, first, agentMessage{Role: "tool", ToolCallID: first.ToolCalls[0].ID, Content: "[]"})
	second, err := agentModelReply(t.Context(), provider, messages, tools, client)
	if err != nil || second.Content != "OK" || step != 2 {
		t.Fatalf("second call got %+v, %v", second, err)
	}
}
