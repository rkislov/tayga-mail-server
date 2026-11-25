package mattermost

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) nativeProductCommands(u *storage.User) []*model.Command {
	out := []*model.Command{}
	for _, c := range []struct{ name, description string }{{"agents", "Доступные агенты этого домена"}, {"agent-web", "Веб-поиск агента: on, off, status"}, {"agent-confirm", "Подтвердить конкретный черновик агента"}, {"boards", "Открыть Boards и создать доску"}} {
		out = append(out, &model.Command{Trigger: c.name, AutoComplete: true, AutoCompleteDesc: c.description, TeamId: chat.ID(u.TenantID)})
	}
	return out
}
func (s *API) nativeProductCommand(w http.ResponseWriter, r *http.Request, u *storage.User, args model.CommandArgs) bool {
	fields := strings.Fields(args.Command)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "/agents":
		if err := s.nativeAgentsReady(r, u); err != nil {
			s.failure(w, err)
			return true
		}
		c := emptyAgentConfig()
		if err := s.agentLoad(r.Context(), agentConfigKey(u), &c); err != nil {
			s.failure(w, err)
			return true
		}
		text := "Агенты этого домена:\n"
		for _, a := range c.Agents {
			if a.Enabled {
				bot, err := s.agentBot(r.Context(), u, a.ID)
				if err == nil {
					text += "- **" + a.Name + "** @" + bot.LocalPart + " — " + a.Description + "\n"
				}
			}
		}
		text += "\nОткройте профиль агента и личное сообщение. Каждый диалог веба — отдельная ветка этого чата. Сообщение без ветки продолжает последний диалог."
		write(w, 200, &model.CommandResponse{ResponseType: "ephemeral", Text: text})
		return true
	case "/boards":
		text := "Boards пока не установлен или не включён для этого домена. Администратору нужно установить плагин и настроить отдельную базу продукта."
		if s.boardsAvailable(r, u) {
			text = "[Открыть Boards и создать доску](" + strings.TrimRight(s.siteURL, "/") + "/mm/" + chat.ID(userScope(u).Key()) + "/boards). В Boards нажмите «+» → «Create new board». При открытии в браузере войдите в TMS под своей учётной записью."
		}
		write(w, 200, &model.CommandResponse{ResponseType: "ephemeral", Text: text})
		return true
	case "/agent-web", "/agent-confirm":
		s.agentsMu.Lock()
		h := agentHistory{}
		err := s.agentLoad(r.Context(), agentHistoryKey(u), &h)
		index := -1
		for i := len(h.Conversations) - 1; i >= 0; i-- {
			c := h.Conversations[i]
			if c.ChannelID == args.ChannelId && (args.RootId == "" || args.RootId == c.RootID) {
				index = i
				break
			}
		}
		if err != nil || index < 0 {
			s.agentsMu.Unlock()
			s.failure(w, chat.ErrForbidden)
			return true
		}
		c := h.Conversations[index]
		if fields[0] == "/agent-confirm" {
			s.agentsMu.Unlock()
			if len(fields) != 2 {
				s.failure(w, chat.ErrInvalid)
				return true
			}
			rec := httptest.NewRecorder()
			s.confirmAgentAction(rec, r, u, c.ID, fields[1])
			if rec.Code != 200 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(rec.Code)
				_, _ = w.Write(rec.Body.Bytes())
				return true
			}
			write(w, 200, &model.CommandResponse{ResponseType: "ephemeral", Text: "Отправка принята."})
			return true
		}
		if len(fields) != 2 || (fields[1] != "on" && fields[1] != "off" && fields[1] != "status") {
			s.agentsMu.Unlock()
			s.failure(w, chat.ErrInvalid)
			return true
		}
		if fields[1] != "status" {
			h.Conversations[index].WebEnabled = fields[1] == "on"
			err = s.agentSave(r.Context(), agentHistoryKey(u), h)
		}
		enabled := h.Conversations[index].WebEnabled
		s.agentsMu.Unlock()
		if err != nil {
			s.failure(w, err)
			return true
		}
		status := "выключен"
		if enabled {
			status = "включён (при разрешении агента и настроенном поисковом сервисе)"
		}
		write(w, 200, &model.CommandResponse{ResponseType: "ephemeral", Text: "Веб-поиск диалога: " + status})
		return true
	}
	return false
}
func (s *API) boardsAvailable(r *http.Request, u *storage.User) bool {
	rt, err := s.pluginRuntimeForScope(r.Context(), userScope(u))
	if err != nil {
		return false
	}
	for _, p := range rt.Webapp() {
		if p.Id == "focalboard" {
			return true
		}
	}
	return false
}
func (s *API) nativeAgentsReady(r *http.Request, u *storage.User) error {
	s.agentsMu.Lock()
	defer s.agentsMu.Unlock()
	c := emptyAgentConfig()
	if err := s.agentLoad(r.Context(), agentConfigKey(u), &c); err != nil {
		return err
	}
	if len(c.Agents) == 0 {
		return nil
	}
	return s.syncAgentBots(r.Context(), u, c, c)
}

// Keep command payload parsing on the normal authenticated /commands/execute
// route; no command can select another user's history or tenant.
