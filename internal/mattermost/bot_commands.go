package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

// Mailbox commands post as the authenticated caller in a DM with the registered
// bot. No external callback, administrator role or service impersonation exists.
type botCommand struct {
	BotID, TenantID, DomainID string
	Command                   model.Command
}

const botCommandPrefix = "mattermost.bot_command."

var botTrigger = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func (s *API) botCommands(ctx context.Context, u *storage.User) ([]botCommand, error) {
	settings, err := s.store.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := []botCommand{}
	for key, raw := range settings {
		if !strings.HasPrefix(key, botCommandPrefix) {
			continue
		}
		var row botCommand
		if json.Unmarshal([]byte(raw), &row) != nil || row.TenantID != u.TenantID || row.DomainID != u.DomainID {
			continue
		}
		bot, e := s.store.GetUserByID(ctx, row.BotID)
		if e != nil || !bot.Enabled || bot.AuthSource != "bot" {
			continue
		}
		// External-bot enable/disable state is separate from the user record.
		external, e := s.readExternalBot(ctx, chat.ID(bot.ID))
		if e != nil || !external.Enabled {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *API) registerBotCommand(w http.ResponseWriter, r *http.Request, u *storage.User) {
	if u.AuthSource != "bot" {
		s.failure(w, chat.ErrForbidden)
		return
	}
	var req struct {
		model.Command
		Mailbox bool `json:"tms_bot_command"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !req.Mailbox || req.TeamId != chat.ID(u.TenantID) || !botTrigger.MatchString(req.Trigger) || (req.Trigger == "meet" || req.Trigger == "agents" || req.Trigger == "agent-web" || req.Trigger == "agent-confirm" || req.Trigger == "boards" || req.Trigger == "calendar-reminders") || req.URL != "" || len(req.AutoCompleteDesc) > 256 || len(req.AutoCompleteHint) > 256 {
		s.failure(w, chat.ErrInvalid)
		return
	}
	if runtime, err := s.pluginRuntimeForScope(r.Context(), userScope(u)); err == nil {
		for _, command := range runtime.Commands(chat.ID(u.TenantID)) {
			if command.Trigger == req.Trigger {
				s.failure(w, chat.ErrForbidden)
				return
			}
		}
	}
	s.botMu.Lock()
	defer s.botMu.Unlock()
	rows, err := s.botCommands(r.Context(), u)
	if err != nil {
		s.failure(w, err)
		return
	}
	for _, row := range rows {
		if row.BotID != u.ID && row.Command.Trigger == req.Trigger {
			s.failure(w, chat.ErrForbidden)
			return
		}
	}
	command := model.Command{Id: chat.ID("bot-command:" + u.ID), CreatorId: chat.ID(u.ID), TeamId: chat.ID(u.TenantID), Trigger: req.Trigger, AutoComplete: true, AutoCompleteDesc: req.AutoCompleteDesc, AutoCompleteHint: req.AutoCompleteHint, DisplayName: req.DisplayName, PluginId: "tms.bot"}
	row := botCommand{BotID: u.ID, TenantID: u.TenantID, DomainID: u.DomainID, Command: command}
	raw, err := json.Marshal(row)
	if err == nil {
		err = s.store.PutSetting(r.Context(), botCommandPrefix+u.ID, string(raw))
	}
	if err != nil {
		s.failure(w, err)
		return
	}
	write(w, 201, command)
}

func (s *API) dispatchBotCommand(r *http.Request, u *storage.User, args model.CommandArgs, row botCommand) error {
	if u.AuthSource == "bot" || u.AuthSource == "federated" {
		return chat.ErrForbidden
	}
	c, err := s.chat.Channel(r.Context(), u, args.ChannelId)
	if err != nil {
		return err
	}
	if c.Kind != "D" {
		return chat.ErrForbidden
	}
	members, err := s.chat.Members(r.Context(), u, c.ID)
	if err != nil {
		return err
	}
	found := false
	for _, member := range members {
		if member.UserID == row.BotID {
			found = true
		}
	}
	if !found || len(members) != 2 {
		return chat.ErrForbidden
	}
	if len(args.Command) > 4096 {
		return chat.ErrInvalid
	}
	_, err = s.chat.Send(r.Context(), u, c.ID, args.RootId, args.Command, "")
	return err
}

func (s *API) botPostAction(w http.ResponseWriter, r *http.Request, u *storage.User, postID, actionID string) {
	post, err := s.chat.Post(r.Context(), u, postID)
	if err != nil {
		s.failure(w, err)
		return
	}
	if strings.HasPrefix(actionID, "agentconfirm-") {
		s.nativeAgentAction(w, r, u, post, actionID)
		return
	}
	rows, err := s.botCommands(r.Context(), u)
	if err != nil {
		s.failure(w, err)
		return
	}
	for _, row := range rows {
		if row.BotID != post.UserID {
			continue
		}
		// Only a command embedded by this bot in the stored post may be dispatched.
		data, _ := json.Marshal(post.Props["attachments"])
		var attachments []struct {
			Actions []struct {
				ID          string `json:"id"`
				Integration struct {
					Context struct {
						Command string `json:"command"`
					} `json:"context"`
				} `json:"integration"`
			} `json:"actions"`
		}
		_ = json.Unmarshal(data, &attachments)
		for _, attachment := range attachments {
			for _, action := range attachment.Actions {
				if action.ID != actionID {
					continue
				}
				text := action.Integration.Context.Command
				fields := strings.Fields(text)
				if len(fields) == 0 || fields[0] != "/"+row.Command.Trigger {
					continue
				}
				err = s.dispatchBotCommand(r, u, model.CommandArgs{ChannelId: post.ChannelID, Command: text}, row)
				if err != nil {
					s.failure(w, err)
					return
				}
				write(w, 200, map[string]string{"status": "OK"})
				return
			}
		}
	}
	s.failure(w, chat.ErrForbidden)
}
