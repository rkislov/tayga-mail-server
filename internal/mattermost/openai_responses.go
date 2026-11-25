package mattermost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// GPT-6.2 supports reasoning and function tools together on Responses.
// Keep the returned reasoning items only for the current tool loop.
func openAIResponsesReply(ctx context.Context, p agentProvider, messages []agentMessage, tools []agentFunction, client *http.Client) (agentMessage, error) {
	input := make([]any, 0, len(messages))
	for _, message := range messages {
		switch {
		case len(message.ResponseItems) > 0:
			for _, item := range message.ResponseItems {
				input = append(input, item)
			}
		case message.Role == "tool":
			input = append(input, map[string]string{"type": "function_call_output", "call_id": message.ToolCallID, "output": message.Content})
		default:
			input = append(input, map[string]string{"role": message.Role, "content": message.Content})
		}
	}

	type functionTool struct {
		Type string `json:"type"`
		agentFunction
	}
	wireTools := make([]functionTool, 0, len(tools))
	for _, tool := range tools {
		wireTools = append(wireTools, functionTool{Type: "function", agentFunction: tool})
	}
	body := map[string]any{
		"model": p.Model, "input": input, "tools": wireTools,
		"max_output_tokens": 16384, "store": false,
		"include": []string{"reasoning.encrypted_content"},
	}
	data, err := sendAgentProviderRequest(ctx, p, "https://api.openai.com/v1/responses", body, client)
	if err != nil {
		return agentMessage{}, err
	}
	var result struct {
		Status string            `json:"status"`
		Output []json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Status != "completed" {
		return agentMessage{}, errors.New("invalid or incomplete response")
	}

	reply := agentMessage{Role: "assistant", ResponseItems: result.Output}
	var content strings.Builder
	for _, raw := range result.Output {
		var item struct {
			Type      string `json:"type"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return agentMessage{}, errors.New("invalid response item")
		}
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					content.WriteString(part.Text)
				case "refusal":
					content.WriteString(part.Refusal)
				}
			}
		case "function_call":
			if item.CallID == "" || item.Name == "" {
				return agentMessage{}, errors.New("invalid function call")
			}
			call := agentToolCall{ID: item.CallID, Type: "function"}
			call.Function.Name = item.Name
			call.Function.Arguments, _ = json.Marshal(item.Arguments)
			reply.ToolCalls = append(reply.ToolCalls, call)
		}
	}
	reply.Content = content.String()
	if (strings.TrimSpace(reply.Content) == "" && len(reply.ToolCalls) == 0) || len(reply.Content) > 64000 || len(reply.ToolCalls) > 8 {
		return agentMessage{}, errors.New("invalid model response")
	}
	return reply, nil
}
