package mattermost

// Agents is a native TMS product following the Mattermost Agents navigation.
// It deliberately has no access to the mail store, plugin tools, or the owner's
// aiagent credentials. Only the current user's messages reach the chosen model.
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type agentProvider struct {
	Kind     string `json:"kind"`
	Thinking bool   `json:"thinking"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`
	HasKey   bool   `json:"has_key"`
}
type agentDefinition struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	ProviderID   string   `json:"provider_id"`
	Enabled      bool     `json:"enabled"`
	SkillIDs     []string `json:"skill_ids"`
}
type agentSkill struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
}
type agentConfiguration struct {
	Version   int64             `json:"version"`
	Providers []agentProvider   `json:"providers"`
	Agents    []agentDefinition `json:"agents"`
	Skills    []agentSkill      `json:"skills"`
}
type agentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type agentConversation struct {
	ID        string         `json:"id"`
	AgentID   string         `json:"agent_id"`
	Title     string         `json:"title"`
	Updated   int64          `json:"updated_at"`
	Messages  []agentMessage `json:"messages"`
	Pending   string         `json:"pending,omitempty"`
	PendingAt int64          `json:"pending_at,omitempty"`
}
type agentHistory struct {
	Conversations []agentConversation `json:"conversations"`
}

func agentID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func agentConfigKey(u *storage.User) string {
	return "mattermost.agents.config:" + chat.ID(userScope(u).Key())
}
func agentHistoryKey(u *storage.User) string {
	return "mattermost.agents.history:" + chat.ID(userScope(u).Key()+"\x00"+u.ID)
}
func (s *API) agentLoad(ctx context.Context, key string, dst any) error {
	raw, ok, err := s.store.GetSetting(ctx, key)
	if err != nil || !ok {
		return err
	}
	return json.Unmarshal([]byte(raw), dst)
}
func (s *API) agentSave(ctx context.Context, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > 16<<20 {
		return chat.ErrInvalid
	}
	return s.store.PutSetting(ctx, key, string(data))
}
func emptyAgentConfig() agentConfiguration {
	return agentConfiguration{Providers: []agentProvider{}, Agents: []agentDefinition{}, Skills: []agentSkill{}}
}
func (c agentConfiguration) public(admin bool) agentConfiguration {
	out := emptyAgentConfig()
	out.Version = c.Version
	if admin {
		out.Providers = append(out.Providers, c.Providers...)
		out.Skills = append(out.Skills, c.Skills...)
	} else {
		for _, skill := range c.Skills {
			skill.Instructions = ""
			out.Skills = append(out.Skills, skill)
		}
	}
	for i := range out.Providers {
		out.Providers[i].HasKey = out.Providers[i].APIKey != ""
		out.Providers[i].APIKey = ""
	}
	for _, a := range c.Agents {
		if admin || a.Enabled {
			if !admin {
				a.Instructions = ""
				a.ProviderID = ""
			}
			out.Agents = append(out.Agents, a)
		}
	}
	return out
}
func validAgentID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
func validAgentEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" || u.Hostname() == "localhost")
}

// Provider kinds bind both protocol and endpoint policy. Only the native
// Ollama connector allows unencrypted RFC1918/ULA hosts for private deployments;
// public HTTP and link-local metadata services remain disallowed.
func validAgentProviderEndpoint(p agentProvider) bool {
	switch p.Kind {
	case "openai":
		return strings.TrimRight(p.URL, "/") == "https://api.openai.com/v1"
	case "deepseek":
		return strings.TrimRight(p.URL, "/") == "https://api.deepseek.com" || strings.TrimRight(p.URL, "/") == "https://api.deepseek.com/v1"
	case "", "openai-compatible":
		return validAgentEndpoint(p.URL)
	case "ollama":
		if validAgentEndpoint(p.URL) {
			return true
		}
		u, err := url.Parse(p.URL)
		if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return false
		}
		ip, err := netip.ParseAddr(u.Hostname())
		return err == nil && (ip.IsPrivate() || ip.IsLoopback())
	default:
		return false
	}
}
func (c *agentConfiguration) validate(old agentConfiguration) bool {
	if len(c.Providers) > 30 || len(c.Agents) > 100 || len(c.Skills) > 100 {
		return false
	}
	providers, agents, skills := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i := range c.Providers {
		p := &c.Providers[i]
		if !validAgentID(p.ID) || providers[p.ID] || strings.TrimSpace(p.Name) == "" || len(p.Name) > 100 || !validAgentProviderEndpoint(*p) || len(p.URL) > 2048 || strings.TrimSpace(p.Model) == "" || len(p.Model) > 200 || len(p.APIKey) > 8192 {
			return false
		}
		providers[p.ID] = true
		// Empty key retains the secret of precisely this provider, never another.
		if p.APIKey == "" {
			for _, previous := range old.Providers {
				if previous.ID == p.ID && previous.URL == p.URL && previous.Kind == p.Kind {
					p.APIKey = previous.APIKey
				}
			}
		}
		p.HasKey = false
		if (p.Kind == "deepseek" || p.Kind == "openai") && strings.TrimSpace(p.APIKey) == "" {
			return false
		}
	}
	for _, k := range c.Skills {
		if !validAgentID(k.ID) || skills[k.ID] || strings.TrimSpace(k.Name) == "" || len(k.Name) > 100 || len(k.Instructions) > 16000 {
			return false
		}
		skills[k.ID] = true
	}
	for _, a := range c.Agents {
		if !validAgentID(a.ID) || agents[a.ID] || strings.TrimSpace(a.Name) == "" || len(a.Name) > 100 || len(a.Description) > 1000 || len(a.Instructions) > 32000 || !providers[a.ProviderID] || len(a.SkillIDs) > 20 {
			return false
		}
		agents[a.ID] = true
		for _, id := range a.SkillIDs {
			if !skills[id] {
				return false
			}
		}
	}
	if c.Providers == nil {
		c.Providers = []agentProvider{}
	}
	if c.Agents == nil {
		c.Agents = []agentDefinition{}
	}
	if c.Skills == nil {
		c.Skills = []agentSkill{}
	}
	return true
}
func (s *API) agentsHTTP(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	if u.AuthSource == "bot" || u.AuthSource == "federated" {
		s.failure(w, chat.ErrForbidden)
		return
	}
	admin := s.canManageScope(r, u, userScope(u))
	if path == "/config" {
		if !admin {
			s.failure(w, chat.ErrForbidden)
			return
		}
		s.agentsMu.Lock()
		defer s.agentsMu.Unlock()
		c := emptyAgentConfig()
		if err := s.agentLoad(r.Context(), agentConfigKey(u), &c); err != nil {
			s.failure(w, err)
			return
		}
		if r.Method == http.MethodGet {
			write(w, 200, c.public(true))
			return
		}
		if r.Method != http.MethodPut {
			wrongMethod(w)
			return
		}
		var next agentConfiguration
		if !decode(w, r, &next) {
			return
		}
		if next.Version != c.Version {
			fail(w, 409, "tms.agents.config_changed", "Настройки изменены в другом окне. Откройте форму заново")
			return
		}
		if !next.validate(c) {
			s.failure(w, chat.ErrInvalid)
			return
		}
		next.Version++
		if err := s.agentSave(r.Context(), agentConfigKey(u), next); err != nil {
			s.failure(w, err)
			return
		}
		s.agentAudit(u, "configuration", "")
		write(w, 200, next.public(true))
		return
	}
	if path == "" && r.Method == http.MethodGet {
		c := emptyAgentConfig()
		if err := s.agentLoad(r.Context(), agentConfigKey(u), &c); err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, map[string]any{"tenant_id": u.TenantID, "domain_id": u.DomainID, "user_id": chat.ID(u.ID), "can_manage": admin, "catalog": c.public(false)})
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 1 || parts[0] != "conversations" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 3 && parts[2] == "messages" && r.Method == http.MethodPost {
		s.agentTurn(w, r, u, parts[1])
		return
	}
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()
	h := agentHistory{Conversations: []agentConversation{}}
	key := agentHistoryKey(u)
	if err := s.agentLoad(r.Context(), key, &h); err != nil {
		s.failure(w, err)
		return
	}
	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			out := []agentConversation{}
			for _, c := range h.Conversations {
				c.Messages = nil
				c.Pending = ""
				c.PendingAt = 0
				out = append(out, c)
			}
			write(w, 200, out)
			return
		}
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if !decode(w, r, &req) {
			return
		}
		c := emptyAgentConfig()
		if err := s.agentLoad(r.Context(), agentConfigKey(u), &c); err != nil {
			s.failure(w, err)
			return
		}
		found := false
		for _, a := range c.Agents {
			if a.ID == req.AgentID && a.Enabled {
				found = true
			}
		}
		if !found {
			s.failure(w, storage.ErrNotFound)
			return
		}
		if len(h.Conversations) >= 100 {
			fail(w, 409, "tms.agents.history_full", "Удалите один из 100 диалогов перед созданием нового")
			return
		}
		conversation := agentConversation{ID: agentID(), AgentID: req.AgentID, Title: "Новый диалог", Updated: time.Now().UnixMilli(), Messages: []agentMessage{}}
		h.Conversations = append(h.Conversations, conversation)
		if err := s.agentSave(r.Context(), key, h); err != nil {
			s.failure(w, err)
			return
		}
		s.agentAudit(u, "conversation_created", conversation.ID)
		write(w, 201, conversation)
		return
	}
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	for i, c := range h.Conversations {
		if c.ID != parts[1] {
			continue
		}
		if r.Method == http.MethodGet {
			c.Pending = ""
			c.PendingAt = 0
			write(w, 200, c)
			return
		}
		if r.Method != http.MethodDelete {
			wrongMethod(w)
			return
		}
		if c.Pending != "" && time.Now().UnixMilli()-c.PendingAt < 150000 {
			fail(w, 409, "tms.agents.busy", "Дождитесь завершения ответа")
			return
		}
		h.Conversations = append(h.Conversations[:i], h.Conversations[i+1:]...)
		if err := s.agentSave(r.Context(), key, h); err != nil {
			s.failure(w, err)
			return
		}
		s.agentAudit(u, "conversation_deleted", c.ID)
		write(w, 200, map[string]string{"status": "OK"})
		return
	}
	s.failure(w, storage.ErrNotFound)
}
func (s *API) agentAudit(u *storage.User, event, id string) {
	s.log.Info("agents audit", "event", event, "tenant_id", u.TenantID, "domain_id", u.DomainID, "user_id", u.ID, "object_id", id)
}

func (s *API) agentTurn(w http.ResponseWriter, r *http.Request, u *storage.User, id string) {
	var req struct {
		Content string `json:"content"`
	}
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Content) == "" || len(req.Content) > 16000 {
		s.failure(w, chat.ErrInvalid)
		return
	}
	key, claim := agentHistoryKey(u), agentID()
	// Claim only this conversation. Network I/O never holds the storage mutex.
	s.agentsMu.Lock()
	if s.agentsRequests == nil {
		s.agentsRequests = make(chan struct{}, 4)
	}
	select {
	case s.agentsRequests <- struct{}{}:
	default:
		s.agentsMu.Unlock()
		fail(w, 429, "tms.agents.capacity", "Все агенты заняты. Повторите позже")
		return
	}
	defer func() { <-s.agentsRequests }()
	h := agentHistory{Conversations: []agentConversation{}}
	c := emptyAgentConfig()
	err := s.agentLoad(r.Context(), key, &h)
	if err == nil {
		err = s.agentLoad(r.Context(), agentConfigKey(u), &c)
	}
	if err != nil {
		s.agentsMu.Unlock()
		s.failure(w, err)
		return
	}
	index := -1
	for i := range h.Conversations {
		if h.Conversations[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		s.agentsMu.Unlock()
		s.failure(w, storage.ErrNotFound)
		return
	}
	conv := h.Conversations[index]
	if conv.Pending != "" && time.Now().UnixMilli()-conv.PendingAt < 150000 {
		s.agentsMu.Unlock()
		fail(w, 409, "tms.agents.busy", "Ответ уже готовится")
		return
	}
	if len(conv.Messages) >= 200 {
		s.agentsMu.Unlock()
		fail(w, 409, "tms.agents.limit", "Создайте новый диалог: достигнут предел истории")
		return
	}
	var definition *agentDefinition
	for i := range c.Agents {
		if c.Agents[i].ID == conv.AgentID && c.Agents[i].Enabled {
			definition = &c.Agents[i]
		}
	}
	if definition == nil {
		s.agentsMu.Unlock()
		s.failure(w, storage.ErrNotFound)
		return
	}
	var provider *agentProvider
	for i := range c.Providers {
		if c.Providers[i].ID == definition.ProviderID {
			provider = &c.Providers[i]
		}
	}
	if provider == nil {
		s.agentsMu.Unlock()
		s.failure(w, storage.ErrNotFound)
		return
	}
	messages := []agentMessage{{Role: "system", Content: definition.Instructions}}
	for _, sid := range definition.SkillIDs {
		for _, skill := range c.Skills {
			if skill.ID == sid {
				messages[0].Content += "\n\n" + skill.Instructions
			}
		}
	}
	messages = append(messages, conv.Messages...)
	messages = append(messages, agentMessage{Role: "user", Content: req.Content})
	h.Conversations[index].Pending = claim
	h.Conversations[index].PendingAt = time.Now().UnixMilli()
	err = s.agentSave(r.Context(), key, h)
	s.agentsMu.Unlock()
	if err != nil {
		s.failure(w, err)
		return
	}
	s.agentAudit(u, "turn_started", id)
	answer, providerErr := agentCompletion(r.Context(), *provider, messages)
	// Clear the persisted claim even when the request disconnects. No user text is
	// committed on failure, so retries cannot leave duplicate unmatched messages.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()
	current := agentHistory{Conversations: []agentConversation{}}
	if err = s.agentLoad(ctx, key, &current); err != nil {
		s.failure(w, err)
		return
	}
	for i := range current.Conversations {
		v := &current.Conversations[i]
		if v.ID != id || v.Pending != claim {
			continue
		}
		v.Pending = ""
		v.PendingAt = 0
		// A revoked login cannot commit a model response after logout.
		_, authErr := s.authenticate(ctx, bearer(r))
		if authErr != nil {
			providerErr = errors.New("session revoked")
		}
		latest := emptyAgentConfig()
		if e := s.agentLoad(ctx, agentConfigKey(u), &latest); e != nil || latest.Version != c.Version {
			providerErr = errors.New("configuration changed")
		}
		policy, e := s.policy.Resolve(ctx, userScope(u))
		if e != nil || !policy.Enabled {
			providerErr = errors.New("scope disabled")
		}
		if providerErr == nil {
			v.Messages = append(v.Messages, agentMessage{Role: "user", Content: req.Content}, agentMessage{Role: "assistant", Content: answer})
			v.Updated = time.Now().UnixMilli()
			if len(v.Messages) == 2 {
				v.Title = string([]rune(req.Content)[:min(60, len([]rune(req.Content)))])
			}
		}
		if err = s.agentSave(ctx, key, current); err != nil {
			s.failure(w, err)
			return
		}
		if providerErr != nil {
			s.agentAudit(u, "turn_failed", id)
			fail(w, 502, "tms.agents.provider_error", "Модель не ответила. Проверьте подключение и настройки провайдера")
			return
		}
		s.agentAudit(u, "turn_completed", id)
		write(w, 200, v)
		return
	}
	fail(w, 409, "tms.agents.turn_changed", "Состояние диалога изменилось")
}
func agentCompletion(ctx context.Context, p agentProvider, messages []agentMessage) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("provider redirects disabled") }}
	return agentCompletionWithClient(ctx, p, messages, client)
}
func agentCompletionWithClient(ctx context.Context, p agentProvider, messages []agentMessage, client *http.Client) (string, error) {
	if !validAgentProviderEndpoint(p) {
		return "", errors.New("invalid provider")
	}
	body := map[string]any{"model": p.Model, "messages": messages, "stream": false, "max_tokens": 4096}
	if p.Kind == "deepseek" {
		mode := "disabled"
		if p.Thinking {
			mode = "enabled"
			body["max_tokens"] = 16384
		}
		body["thinking"] = map[string]string{"type": mode}
	}
	endpoint := strings.TrimRight(p.URL, "/") + "/chat/completions"
	if p.Kind == "openai" {
		delete(body, "max_tokens")
		body["max_completion_tokens"] = 16384
		body["store"] = false
	}
	if p.Kind == "ollama" {
		delete(body, "max_tokens")
		body["options"] = map[string]int{"num_predict": 4096}
		base := strings.TrimRight(p.URL, "/")
		if strings.HasSuffix(base, "/api") {
			endpoint = base + "/chat"
		} else {
			endpoint = base + "/api/chat"
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", errors.New("provider unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", errors.New("provider rejected request")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil || len(data) > 2<<20 {
		return "", errors.New("provider response too large")
	}
	var result struct {
		Message agentMessage `json:"message"`
		Done    bool         `json:"done"`
		Choices []struct {
			Message agentMessage `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &result) != nil {
		return "", errors.New("invalid model response")
	}
	answer := ""
	if p.Kind == "ollama" {
		if !result.Done {
			return "", errors.New("incomplete model response")
		}
		answer = result.Message.Content
	} else if len(result.Choices) > 0 {
		answer = result.Choices[0].Message.Content
	}
	if strings.TrimSpace(answer) == "" || len(answer) > 64000 {
		return "", errors.New("invalid model response")
	}
	return answer, nil
}
