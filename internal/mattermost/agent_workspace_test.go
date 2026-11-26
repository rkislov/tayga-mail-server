package mattermost

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAgentFileResultIsPrivateAndExpires(t *testing.T) {
	f, _, _ := agentsFixture(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []agentMessage
			Tools    []agentFunction
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if input.Messages[len(input.Messages)-1].Role == "tool" {
			write(w, 200, map[string]any{"choices": []any{map[string]any{"message": agentMessage{Role: "assistant", Content: "Документ готов"}}}})
			return
		}
		args, _ := json.Marshal(map[string]string{"name": "Отчёт", "format": "docx", "content": "Первая строка\nСумма < 10"})
		message := agentMessage{Role: "assistant", ToolCalls: []agentToolCall{{ID: "file", Type: "function"}}}
		message.ToolCalls[0].Function.Name = "file_create"
		message.ToolCalls[0].Function.Arguments, _ = json.Marshal(string(args))
		write(w, 200, map[string]any{"choices": []any{map[string]any{"message": message}}})
	}))
	defer provider.Close()
	cfg := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Providers[0].URL = provider.URL + "/v1"
	cfg.Agents[0].Tools = []string{"file_create"}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], cfg, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	path := "/tms/agents/conversations/" + conv["id"].(string)
	_ = f.call(t, "POST", path+"/messages", f.tokens[0], map[string]string{"content": "Сделай документ"}, 200)
	restored := f.call(t, "GET", path, f.tokens[0], nil, 200)
	msgs := restored["messages"].([]any)
	artifacts, ok := msgs[len(msgs)-1].(map[string]any)["artifacts"].([]any)
	if !ok || len(artifacts) != 1 {
		t.Fatalf("missing artifact %v", msgs)
	}
	file := artifacts[0].(map[string]any)
	download := "/api/v4/tms/agents/artifacts/" + file["id"].(string)
	get := func(token string, expected int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", download, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		f.api.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("download: got %d, want %d", w.Code, expected)
		}
		return w
	}
	get(f.tokens[1], 404)
	res := get(f.tokens[0], 200)
	if !strings.Contains(res.Header().Get("Content-Disposition"), ".docx") {
		t.Fatal(res.Header())
	}
	document, err := zip.NewReader(bytes.NewReader(res.Body.Bytes()), int64(res.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	xmlData, err := document.Open("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(bytes.Buffer)
	_, err = decoded.ReadFrom(xmlData)
	xmlData.Close()
	if err != nil || !strings.Contains(decoded.String(), "Сумма &lt; 10") {
		t.Fatal("document lost content")
	}
	if err = f.store.CleanupAgentArtifacts(t.Context(), time.Now().Add(25*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	get(f.tokens[0], 404)
}

func TestAgentScheduleRunsOnceInPrivateConversation(t *testing.T) {
	f, _, _ := agentsFixture(t)
	var hits atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var input struct {
			Messages []agentMessage
			Tools    []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if len(input.Tools) != 0 || input.Messages[len(input.Messages)-1].Content != "Личный отчёт" {
			t.Error("wrong scheduled model input")
		}
		if !strings.Contains(input.Messages[0].Content, "запущен сервером из сохранённого расписания") {
			t.Error("model was not told this is a scheduled run")
		}
		write(w, 200, map[string]any{"choices": []any{map[string]any{"message": agentMessage{Role: "assistant", Content: "Отчёт по расписанию"}}}})
	}))
	defer provider.Close()
	cfg := emptyAgentConfig()
	if err := f.api.agentLoad(t.Context(), agentConfigKey(f.users[0]), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Providers[0].URL = provider.URL + "/v1"
	cfg.Agents[0].Tools = []string{"chat_send"}
	f.call(t, "PUT", "/tms/agents/config", f.tokens[0], cfg, 200)
	conv := f.call(t, "POST", "/tms/agents/conversations", f.tokens[0], map[string]string{"agent_id": "helper"}, 201)
	form := map[string]any{
		"conversation_id": conv["id"], "prompt": "Личный отчёт", "frequency": "once",
		"timezone": "UTC", "local_time": time.Now().UTC().Add(5 * time.Minute).Format(agentScheduleTimeFormat),
	}
	f.call(t, "POST", "/tms/agents/schedules", f.tokens[1], form, 404)
	f.call(t, "POST", "/tms/agents/schedules", f.tokens[0], form, 204)
	items, err := f.store.ListAgentSchedules(t.Context(), f.users[0].ID, userScope(f.users[0]).Key())
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	id := items[0].ID
	f.call(t, "DELETE", "/tms/agents/schedules/"+id, f.tokens[1], nil, 404)
	form["paused"] = true
	f.call(t, "PUT", "/tms/agents/schedules/"+id, f.tokens[0], form, 204)
	due := time.UnixMilli(items[0].NextAt).Add(time.Second)
	if err = f.api.runDueAgentSchedules(t.Context(), due); err != nil || hits.Load() != 0 {
		t.Fatal("paused job ran", err)
	}
	form["paused"] = false
	f.call(t, "PUT", "/tms/agents/schedules/"+id, f.tokens[0], form, 204)
	for i := 0; i < 2; i++ {
		if err = f.api.runDueAgentSchedules(t.Context(), due); err != nil {
			t.Fatal(err)
		}
	}
	items, err = f.store.ListAgentSchedules(t.Context(), f.users[0].ID, userScope(f.users[0]).Key())
	if err != nil || len(items) != 1 || items[0].Status != "completed" || items[0].NextAt != 0 || hits.Load() != 1 {
		t.Fatalf("scheduled run not completed once: items=%v hits=%d error=%v", items, hits.Load(), err)
	}
	result := f.call(t, "GET", "/tms/agents/conversations/"+conv["id"].(string), f.tokens[0], nil, 200)
	messages := result["messages"].([]any)
	if len(messages) != 2 || messages[1].(map[string]any)["content"] != "Отчёт по расписанию" {
		t.Fatal(messages)
	}
}

func TestAgentWebFilesTextAndLocalAddressGate(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("local address must not be contacted")
	}))
	defer local.Close()
	f := agentFiles{downloads: map[string]agentDownloadedFile{"xml": {data: []byte(strings.Repeat("я", 2999) + "ЖЖ€</xml>")}}}
	if _, err := f.download(t.Context(), local.URL); err == nil {
		t.Fatal("accepted local address or port")
	}
	for _, raw := range []string{"http://127.0.0.1/", "http://[::ffff:127.0.0.1]/"} {
		if _, err := f.download(t.Context(), raw); err == nil {
			t.Fatal("accepted local address", raw)
		}
	}
	first, _ := f.read("xml", "0")
	second, _ := f.read("xml", strconv.Itoa(first["next_offset"].(int)))
	if first["content"].(string)+second["content"].(string) != string(f.downloads["xml"].data) || second["eof"] != true {
		t.Fatal("text lost across paged reads")
	}
	if _, err := (new(agentFiles)).read("xml", "0"); err == nil {
		t.Fatal("another request read the temporary file")
	}
}

func TestAgentWeeklyScheduleRespectsLocalTime(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	spec := agentScheduleSpec{Frequency: "weekly", LocalTime: "2026-11-26T07:00", Timezone: loc.String(), Weekdays: []time.Weekday{time.Friday}}
	next, err := spec.next(time.Date(2026, 11, 26, 15, 0, 0, 0, loc))
	if err != nil || !time.UnixMilli(next).Equal(time.Date(2026, 11, 27, 4, 0, 0, 0, time.UTC)) {
		t.Fatal(next, err)
	}
}
