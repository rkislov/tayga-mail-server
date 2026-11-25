package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type agentNativeKey struct{}
type agentNativeRequest struct{ PendingID string }
type agentDiscardWriter struct{}

func (agentDiscardWriter) Header() http.Header         { return make(http.Header) }
func (agentDiscardWriter) WriteHeader(int)             {}
func (agentDiscardWriter) Write(p []byte) (int, error) { return len(p), nil }

func agentBotInternalID(u *storage.User, id string) string {
	return "tms-agent:" + chat.ID(userScope(u).Key()+"\x00"+id)
}
func (s *API) agentBot(ctx context.Context, u *storage.User, id string) (*storage.User, error) {
	b, err := s.store.GetUserByID(ctx, agentBotInternalID(u, id))
	if err != nil {
		return nil, err
	}
	if !b.Enabled || b.AuthSource != "bot" || b.TenantID != u.TenantID || b.DomainID != u.DomainID {
		return nil, chat.ErrForbidden
	}
	return b, nil
}

// Native agent identities have no password, token, owner credentials or mailbox.
func (s *API) syncAgentBots(ctx context.Context, u *storage.User, c, old agentConfiguration) error {
	domain, err := s.store.GetDomainByID(ctx, u.DomainID)
	if err != nil {
		return err
	}
	for _, a := range c.Agents {
		id := agentBotInternalID(u, a.ID)
		b, e := s.store.GetUserByID(ctx, id)
		if errors.Is(e, storage.ErrNotFound) {
			name := "agent-" + chat.ID(userScope(u).Key()+"\x00"+a.ID)
			_, e = s.store.CreateUser(ctx, &storage.User{ID: id, TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: name, Email: name + "@" + domain.Name, DisplayName: a.Name, AuthSource: "bot", Roles: "bot", Enabled: a.Enabled})
		} else if e == nil {
			if b.AuthSource != "bot" || b.TenantID != u.TenantID || b.DomainID != u.DomainID {
				return chat.ErrForbidden
			}
			if b.DisplayName != a.Name {
				e = s.store.UpdateUserProfile(ctx, id, a.Name)
			}
			if e == nil && b.Enabled != a.Enabled {
				e = s.store.UpdateUserEnabled(ctx, id, a.Enabled)
			}
		}
		if e != nil {
			return e
		}
	}
	for _, a := range old.Agents {
		found := false
		for _, next := range c.Agents {
			if a.ID == next.ID {
				found = true
			}
		}
		if !found {
			if e := s.store.UpdateUserEnabled(ctx, agentBotInternalID(u, a.ID), false); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *API) linkAgentConversation(ctx context.Context, u *storage.User, c *agentConversation) error {
	if c.ChannelID != "" {
		return nil
	}
	bot, err := s.agentBot(ctx, u, c.AgentID)
	if err != nil {
		return err
	}
	channel, err := s.chat.Direct(chat.WithDomainScope(ctx, u.DomainID), u, chat.ID(bot.ID))
	if err != nil {
		return err
	}
	root, err := s.chat.Send(ctx, bot, channel.ID, "", "Диалог: "+c.Title, "agent-root-"+c.ID)
	if err != nil {
		return err
	}
	c.ChannelID, c.RootID = channel.ID, root.ID
	// Existing private histories are copied only into this user's two-member DM.
	for i, m := range c.Messages {
		actor := u
		if m.Role == "assistant" {
			actor = bot
		}
		if _, err = s.chat.Send(ctx, actor, c.ChannelID, c.RootID, m.Content, fmt.Sprintf("agent-migrate-%s-%d", c.ID, i)); err != nil {
			return err
		}
	}
	return nil
}
func (s *API) refreshAgentConversation(ctx context.Context, u *storage.User, c *agentConversation) error {
	if c.ChannelID == "" {
		return nil
	}
	bot, err := s.agentBot(ctx, u, c.AgentID)
	if err != nil {
		return err
	}
	channel, err := s.chat.Channel(ctx, u, c.ChannelID)
	if err != nil {
		return err
	}
	if channel.Kind != "D" {
		return chat.ErrForbidden
	}
	posts, err := s.chat.Thread(ctx, u, c.RootID)
	if err != nil {
		return err
	}
	c.Messages = []agentMessage{}
	for _, p := range posts {
		if p.ChannelID != c.ChannelID {
			return chat.ErrForbidden
		}
		if p.ID == c.RootID {
			continue
		}
		role := "user"
		if p.UserID == bot.ID {
			role = "assistant"
		} else if p.UserID != u.ID {
			return chat.ErrForbidden
		}
		if role == "user" && c.Title == newAgentConversationTitle && strings.TrimSpace(p.Message) != "" {
			c.Title = agentConversationTitle(p.Message)
		}
		c.Messages = append(c.Messages, agentMessage{Role: role, Content: p.Message})
		c.Updated = p.CreatedAt
	}
	return nil
}
func (s *API) clearAgentClaim(ctx context.Context, u *storage.User, id, claim string) {
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()
	h := agentHistory{}
	if s.agentLoad(ctx, agentHistoryKey(u), &h) != nil {
		return
	}
	for i := range h.Conversations {
		if h.Conversations[i].ID == id && h.Conversations[i].Pending == claim {
			h.Conversations[i].Pending = ""
			h.Conversations[i].PendingAt = 0
		}
	}
	_ = s.agentSave(ctx, agentHistoryKey(u), h)
}

// Intercept only a two-member DM with a native agent from the same domain.
// HTTP authentication and the ordinary membership check happen before dispatch.
func (s *API) nativeAgentPost(w http.ResponseWriter, r *http.Request, u *storage.User, channelID, rootID, content, pendingID string) bool {
	if u.AuthSource == "bot" || u.AuthSource == "federated" {
		return false
	}
	ctx := chat.WithDomainScope(r.Context(), u.DomainID)
	channel, err := s.chat.Channel(ctx, u, channelID)
	if err != nil || channel.Kind != "D" {
		return false
	}
	members, err := s.chat.Members(ctx, u, channelID)
	if err != nil || len(members) != 2 {
		return false
	}
	config := emptyAgentConfig()
	if s.agentLoad(ctx, agentConfigKey(u), &config) != nil {
		return false
	}
	definitionID := ""
	for _, a := range config.Agents {
		for _, m := range members {
			if m.UserID == agentBotInternalID(u, a.ID) {
				definitionID = a.ID
			}
		}
	}
	if definitionID == "" {
		return false
	}
	s.agentsMu.Lock()
	if pendingID != "" {
		posts, e := s.chat.Posts(ctx, u, channelID, 200, 0, 0)
		if e != nil {
			s.agentsMu.Unlock()
			s.failure(w, e)
			return true
		}
		for _, p := range posts {
			if p.UserID == u.ID && p.PendingID == pendingID {
				s.agentsMu.Unlock()
				if p.Message != content {
					s.failure(w, chat.ErrInvalid)
				} else {
					write(w, 201, postModel(p))
				}
				return true
			}
		}
	}
	h := agentHistory{}
	err = s.agentLoad(ctx, agentHistoryKey(u), &h)
	selected := ""
	for i := len(h.Conversations) - 1; i >= 0; i-- {
		c := h.Conversations[i]
		if c.AgentID == definitionID && (rootID == "" || rootID == c.RootID) {
			selected = c.ID
			break
		}
	}
	if err == nil && selected == "" && rootID == "" && len(h.Conversations) < 100 {
		c := agentConversation{ID: agentID(), AgentID: definitionID, Title: newAgentConversationTitle, Updated: time.Now().UnixMilli(), Messages: []agentMessage{}}
		err = s.linkAgentConversation(ctx, u, &c)
		if err == nil {
			h.Conversations = append(h.Conversations, c)
			err = s.agentSave(ctx, agentHistoryKey(u), h)
			selected = c.ID
		}
	}
	s.agentsMu.Unlock()
	if err != nil {
		s.failure(w, err)
		return true
	}
	if selected == "" {
		s.failure(w, chat.ErrInvalid)
		return true
	}
	payload, _ := json.Marshal(map[string]string{"content": content})
	req := r.Clone(context.WithValue(r.Context(), agentNativeKey{}, agentNativeRequest{PendingID: pendingID}))
	req.Body = io.NopCloser(bytes.NewReader(payload))
	s.agentTurn(w, req, u, selected)
	return true
}
