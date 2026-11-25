package mattermost

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func agentActionProps(actions []agentAction) map[string]any {
	if len(actions) == 0 {
		return nil
	}
	buttons := []any{}
	for _, a := range actions {
		label := "Отправить сообщение"
		if a.Tool == "mail_draft" {
			label = "Отправить письмо → " + a.Arguments["to"]
		}
		buttons = append(buttons, map[string]any{"id": "agentconfirm-" + a.ID, "name": label, "type": "button", "integration": map[string]any{"url": "/api/v4/tms/agents/actions", "context": map[string]string{"action_id": a.ID}}})
	}
	return map[string]any{"attachments": []any{map[string]any{"text": "Подтверждение отправки текста выше. Действует 10 минут.", "actions": buttons}}}
}
func (s *API) nativeAgentAction(w http.ResponseWriter, r *http.Request, u *storage.User, post chat.Post, actionID string) {
	s.agentsMu.Lock()
	h := agentHistory{}
	err := s.agentLoad(r.Context(), agentHistoryKey(u), &h)
	conversationID := ""
	id := strings.TrimPrefix(actionID, "agentconfirm-")
	if err == nil {
		for _, c := range h.Conversations {
			if c.ChannelID != post.ChannelID || c.RootID != post.RootID || post.UserID != agentBotInternalID(u, c.AgentID) {
				continue
			}
			for _, a := range c.Actions {
				if a.ID == id {
					conversationID = c.ID
				}
			}
		}
	}
	s.agentsMu.Unlock()
	if err != nil {
		s.failure(w, err)
		return
	}
	if conversationID == "" {
		s.failure(w, chat.ErrForbidden)
		return
	}
	response := httptest.NewRecorder()
	s.confirmAgentAction(response, r, u, conversationID, id)
	if response.Code != 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.Code)
		_, _ = w.Write(response.Body.Bytes())
		return
	}
	write(w, 200, map[string]string{"ephemeral_text": "Отправка принята.", "status": "OK"})
}
