package mattermost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func TestAgentCalendarUsesCurrentPermissionsAndOwner(t *testing.T) {
	f, _, _ := agentsFixture(t)
	ctx := t.Context()
	for i, subject := range []string{"Моя встреча", "PRIVATE FOREIGN EVENT"} {
		cal, err := f.store.EnsureCalendar(ctx, f.users[i].ID, "work", "Work")
		if err != nil {
			t.Fatal(err)
		}
		const prefix = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:agent-fixture\r\nDTSTAMP:20261120T000000Z\r\nDTSTART:20261120T100000Z\r\nDTEND:20261120T110000Z\r\nRRULE:FREQ=DAILY;COUNT=10\r\n"
		_, err = f.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
			CalendarID: cal.ID, HrefName: "agent-fixture.ics", UID: "agent-fixture",
			Data: prefix + "SUMMARY:" + subject + "\r\nDESCRIPTION:SECRET DETAIL\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []agentMessage `json:"messages"`
			Tools    []struct {
				Function agentFunction `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "calendar_events" {
			t.Errorf("wrong enabled tools: %#v", payload.Tools)
		}
		system := payload.Messages[0].Content
		if !strings.Contains(system, "Доступные инструменты: calendar_events") ||
			!strings.Contains(system, "имеет приоритет") || !strings.Contains(system, "Ты не имеешь доступа к календарю") {
			t.Error("the server policy must qualify custom instructions")
		}
		last := payload.Messages[len(payload.Messages)-1]
		if last.Role != "tool" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"agenda","type":"function","function":{"name":"calendar_events","arguments":"{\"from\":\"2026-11-26T00:00:00+03:00\",\"to\":\"2026-11-27T00:00:00+03:00\"}"}}]}}]}`))
			return
		}
		if last.ToolCallID != "agenda" || !strings.Contains(last.Content, "Моя встреча") ||
			!strings.Contains(last.Content, "2026-11-26T10:00:00Z") ||
			strings.Contains(last.Content, "PRIVATE") || strings.Contains(last.Content, "SECRET DETAIL") {
			t.Errorf("incorrect or overbroad calendar result: %s", last.Content)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Встреча в 13:00"}}]}`))
	}))
	defer model.Close()
	c := emptyAgentConfig()
	if err := f.api.agentLoad(ctx, agentConfigKey(f.users[0]), &c); err != nil {
		t.Fatal(err)
	}
	c.Providers[0].URL = model.URL
	c.Agents[0].Instructions = "Ты не имеешь доступа к календарю"
	c.Agents[0].Tools = []string{"calendar_read"}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], c, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	f.call(t, "POST", "/tms/agents/conversations/"+conv["id"].(string)+"/messages", f.tokens[0], map[string]string{"content": "Что у меня в календаре?"}, 200)
	if _, err := f.api.agentCalendarEvents(ctx, f.users[0], "2026-11-25T00:00:00Z", "2027-01-01T00:00:00Z"); err == nil {
		t.Fatal("unbounded date range accepted")
	}
}

func TestAgentChatSearchIncludesAuthorizedDirectMessages(t *testing.T) {
	f, _, _ := agentsFixture(t)
	ctx := t.Context()
	const query = "статус проекта"
	for _, fixture := range []struct {
		sender, peer int
		message      string
	}{{0, 1, query + " доступный"}, {1, 2, query + " ЧУЖОЙ ДИАЛОГ"}} {
		channel, err := f.api.chat.Direct(ctx, f.users[fixture.sender], chat.ID(f.users[fixture.peer].ID))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.api.chat.Send(ctx, f.users[fixture.sender], channel.ID, "", fixture.message, ""); err != nil {
			t.Fatal(err)
		}
	}
	check := func(tool string, args map[string]string, want, forbidden string) {
		t.Helper()
		result, err := f.api.agentReadTool(ctx, f.users[0], agentSearchConfig{}, tool, args)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(result)
		if err != nil || !strings.Contains(string(raw), want) || strings.Contains(string(raw), forbidden) {
			t.Fatalf("unexpected %s reply: %s, %v", tool, raw, err)
		}
	}
	check("chat_channels", nil, "bob", "charlie")
	check("chat_search_recent", map[string]string{"query": query}, "доступный", "ЧУЖОЙ ДИАЛОГ")
}
