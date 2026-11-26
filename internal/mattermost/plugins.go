package mattermost

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmplugins"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) EnablePlugins(directory string, admins []string) error {
	bindings, err := mmplugins.LoadDatabaseBindings(directory)
	if err != nil {
		return err
	}
	runtime, err := mmplugins.New(mmplugins.Options{Directory: directory, SiteURL: s.siteURL, Store: s.store, Chat: s.chat, Logger: s.log, Scope: "catalog"})
	if err != nil {
		return err
	}
	s.pluginDatabases = bindings
	s.plugins = runtime
	s.pluginDirectory = directory
	s.policy.Defaults.PluginsEnabled = true
	s.pluginAdmins = append([]string{}, admins...)
	return nil
}
func (s *API) Close() {
	s.stopPush()
	s.pluginScopeMu.Lock()
	defer s.pluginScopeMu.Unlock()
	for key, entry := range s.pluginScopes {
		entry.runtime.Close()
		delete(s.pluginScopes, key)
	}
	if s.plugins != nil {
		s.plugins.Close()
	}
}
func (s *API) pluginAdmin(u *storage.User) bool {
	for _, role := range strings.Split(u.Roles, ",") {
		if strings.TrimSpace(role) == "global_admin" {
			return true
		}
	}
	for _, email := range s.pluginAdmins {
		if strings.EqualFold(u.Email, email) {
			return true
		}
	}
	return false
}
func (s *API) pluginAPI(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	if path == "/plugins/webapp" && r.Method == http.MethodGet {
		runtime, err := s.pluginRuntimeForScope(r.Context(), userScope(u))
		if err == errPluginsDisabled {
			write(w, 200, []any{})
			return
		}
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, runtime.Webapp())
		return
	}
	scope, err := s.scopeFromRequest(r, u)
	if err != nil {
		s.failure(w, err)
		return
	}
	if !s.canManageScope(r, u, scope) {
		s.failure(w, chat.ErrForbidden)
		return
	}
	if s.plugins == nil {
		fail(w, 503, "tms.plugins.disabled", "The Mattermost plugin host is disabled")
		return
	}
	if s.patchDomainPluginConfiguration(w, r, scope, path) {
		return
	}
	runtime, err := s.pluginRuntimeForScope(r.Context(), scope)
	if err != nil {
		fail(w, 503, "tms.plugins.scope_disabled", err.Error())
		return
	}
	if path == "/plugins/statuses" && r.Method == http.MethodGet {
		statuses, err := runtime.Statuses()
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, statuses)
		return
	}
	if path == "/plugins" {
		if r.Method == http.MethodGet {
			plugins, err := runtime.List()
			if err != nil {
				s.failure(w, err)
				return
			}
			write(w, 200, plugins)
			return
		}
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			fail(w, 400, "tms.plugin.invalid_upload", "Invalid plugin upload")
			return
		}
		defer r.MultipartForm.RemoveAll()
		if !s.pluginAdmin(u) {
			s.failure(w, chat.ErrForbidden)
			return
		}
		file, _, err := r.FormFile("plugin")
		if err != nil {
			s.failure(w, chat.ErrInvalid)
			return
		}
		defer file.Close()
		if r.FormValue("force") == "true" {
			fail(w, 409, "tms.plugin.replace_requires_restart", "Stop the plugin host before replacing a shared package")
			return
		}
		m, err := s.plugins.Install(file, false)
		if err != nil {
			fail(w, 400, "tms.plugin.install_failed", err.Error())
			return
		}
		write(w, 201, m)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/plugins/"), "/")
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if !s.pluginAdmin(u) {
			s.failure(w, chat.ErrForbidden)
			return
		}
		s.pluginScopeMu.Lock()
		hasScopes := len(s.pluginScopes) > 0
		s.pluginScopeMu.Unlock()
		if hasScopes {
			fail(w, 409, "tms.plugin.remove_requires_restart", "Stop the plugin host before removing a shared package")
			return
		}
		if err := s.plugins.Remove(parts[0]); err != nil {
			fail(w, 400, "tms.plugin.remove_failed", err.Error())
			return
		}
		write(w, 200, map[string]string{"status": "OK"})
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		switch parts[1] {
		case "enable":
			if _, err := runtime.Enable(parts[0]); err != nil {
				fail(w, 400, "tms.plugin.activation_failed", err.Error())
				return
			}
		case "disable":
			if err := runtime.Disable(parts[0]); err != nil {
				fail(w, 400, "tms.plugin.deactivation_failed", err.Error())
				return
			}
		default:
			s.unsupported(w, r)
			return
		}
		write(w, 200, map[string]string{"status": "OK"})
		return
	}
	if len(parts) == 2 && parts[1] == "configuration" && r.Method == http.MethodPut {
		var config map[string]any
		if !decode(w, r, &config) {
			return
		}
		if config == nil {
			s.failure(w, chat.ErrInvalid)
			return
		}
		if err := runtime.Configure(parts[0], config); err != nil {
			fail(w, 400, "tms.plugin.configuration_failed", err.Error())
			return
		}
		write(w, 200, map[string]string{"status": "OK"})
		return
	}
	s.unsupported(w, r)
}
func (s *API) ServePluginHTTP(w http.ResponseWriter, r *http.Request) {
	token := bearer(r)
	if token == "" {
		fail(w, 401, "tms.plugins.scoped_callback_required", "Use the scoped plugin callback URL for unauthenticated webhooks")
		return
	}
	u, err := s.authenticate(r.Context(), token)
	if err != nil {
		fail(w, 401, "api.context.session_expired.app_error", "Invalid session")
		return
	}
	{
		switch r.URL.Path {
		case mattermostAppsRoute + "/bindings", mattermostAppsRoute + "/call":
			s.channelCallApp(w, r, u)
			return
		}
	}
	runtime, err := s.pluginRuntimeForScope(r.Context(), userScope(u))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	runtime.ServeHTTP(w, r, chat.ID(u.ID))
}
func (s *API) commands(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	if path == "/commands" && r.Method == http.MethodPost {
		s.registerBotCommand(w, r, u)
		return
	}
	if (path == "/commands" || path == "/commands/autocomplete") && r.Method == http.MethodGet {
		team := r.URL.Query().Get("team_id")
		if team != "" && team != chat.ID(u.TenantID) {
			s.failure(w, chat.ErrForbidden)
			return
		}
		commands := s.nativeProductCommands(u)
		rows, err := s.botCommands(r.Context(), u)
		if err != nil {
			s.failure(w, err)
			return
		}
		for _, row := range rows {
			command := row.Command
			commands = append(commands, &command)
		}
		if s.startChannelCall != nil {
			commands = append(commands, &model.Command{Trigger: "meet", AutoComplete: true, AutoCompleteDesc: "Начать групповой видеозвонок в канале", DisplayName: "Видеозвонок", TeamId: chat.ID(u.TenantID)})
		}
		if runtime, err := s.pluginRuntimeForScope(r.Context(), userScope(u)); err == nil {
			for _, command := range runtime.Commands(chat.ID(u.TenantID)) {
				if s.startChannelCall != nil && command.Trigger == "meet" {
					continue
				}
				commands = append(commands, command)
			}
		}
		if path == "/commands/autocomplete" {
			input := strings.TrimPrefix(r.URL.Query().Get("user_input"), "/")
			suggestions := []model.AutocompleteSuggestion{}
			for _, command := range commands {
				values := []string{command.Trigger}
				if command.Trigger == "agent-web" {
					values = append(values, "agent-web on", "agent-web off", "agent-web status")
				}
				if command.Trigger == "calendar-reminders" {
					values = append(values, "calendar-reminders on", "calendar-reminders off", "calendar-reminders status")
				}
				if command.PluginId == "tms.bot" {
					for _, sub := range []string{"help", "new", "chats", "use", "web on", "web off", "web status"} {
						values = append(values, command.Trigger+" "+sub)
					}
				}
				for _, value := range values {
					if strings.HasPrefix(value, input) {
						suggestions = append(suggestions, model.AutocompleteSuggestion{Complete: value, Suggestion: value, Hint: command.AutoCompleteHint, Description: command.AutoCompleteDesc})
					}
				}
			}
			write(w, 200, suggestions)
			return
		}
		write(w, 200, commands)
		return
	}
	if path != "/commands/execute" || r.Method != http.MethodPost {
		s.unsupported(w, r)
		return
	}
	var args model.CommandArgs
	if !decode(w, r, &args) {
		return
	}
	c, err := s.chat.Channel(r.Context(), u, args.ChannelId)
	if err != nil {
		s.failure(w, err)
		return
	}
	if args.RootId != "" {
		p, e := s.chat.Post(r.Context(), u, args.RootId)
		if e != nil {
			s.failure(w, e)
			return
		}
		if p.ChannelID != c.ID {
			s.failure(w, chat.ErrForbidden)
			return
		}
	}
	args.UserId = chat.ID(u.ID)
	args.TeamId = chat.ID(u.TenantID)
	args.SiteURL = s.siteURL
	if s.nativeProductCommand(w, r, u, args) {
		return
	}
	if fields := strings.Fields(args.Command); len(fields) > 0 && fields[0] == "/meet" && s.startChannelCall != nil {
		response, err := s.callCommand(r.Context(), u, args)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, response)
		return
	}
	rows, botErr := s.botCommands(r.Context(), u)
	if botErr != nil {
		s.failure(w, botErr)
		return
	}
	for _, row := range rows {
		fields := strings.Fields(args.Command)
		if len(fields) > 0 && fields[0] == "/"+row.Command.Trigger {
			if err = s.dispatchBotCommand(r, u, args, row); err != nil {
				s.failure(w, err)
				return
			}
			write(w, 200, &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: "Команда отправлена помощнику."})
			return
		}
	}
	runtime, err := s.pluginRuntimeForScope(r.Context(), userScope(u))
	if err != nil {
		fail(w, 503, "tms.plugins.scope_disabled", err.Error())
		return
	}
	result, appErr := runtime.Execute(&args)
	if appErr != nil {
		write(w, appErr.StatusCode, appErr)
		return
	}
	if result == nil {
		result = &model.CommandResponse{}
	}
	if result.ResponseType == model.CommandResponseTypeInChannel && result.Text != "" {
		props := map[string]any(result.Props)
		if props == nil {
			props = map[string]any{}
		}
		if len(result.Attachments) > 0 {
			props["attachments"] = result.Attachments
		}
		if _, err = s.chat.SendRich(r.Context(), u, c.ID, args.RootId, result.Text, "", props, result.Type); err != nil {
			s.failure(w, err)
			return
		}
	}
	write(w, 200, result)
}
