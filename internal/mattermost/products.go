package mattermost

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmplugins"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) serveProductHTTP(w http.ResponseWriter, r *http.Request, rt *mmplugins.Runtime, scope mmsettings.Scope, uid, path string) bool {
	if strings.HasPrefix(path, "static/plugins/") {
		parts := strings.Split(strings.TrimPrefix(path, "static/plugins/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
		} else {
			rt.ServeProductAsset(w, r, parts[0], parts[1])
		}
		return true
	}
	if strings.HasPrefix(path, "api/v4/") {
		if uid == "" {
			fail(w, 401, "tms.products.login_required", "Sign in to TMS")
			return true
		}
		// Domain-sensitive entity lookups must not escape the scoped product host.
		resource := strings.Split(strings.TrimPrefix(path, "api/v4/"), "/")
		if len(resource) > 1 && resource[0] == "users" && model.IsValidId(resource[1]) {
			target, e := s.chat.User(r.Context(), scope.TenantID, resource[1])
			if e != nil || target.DomainID != scope.DomainID {
				s.failure(w, chat.ErrForbidden)
				return true
			}
		}
		if len(resource) > 1 && resource[0] == "channels" && model.IsValidId(resource[1]) {
			actor, e := s.chat.User(r.Context(), scope.TenantID, uid)
			if e != nil {
				s.failure(w, chat.ErrForbidden)
				return true
			}
			channel, e := s.chat.Channel(r.Context(), actor, resource[1])
			if e != nil {
				s.failure(w, chat.ErrForbidden)
				return true
			}
			target, e := s.store.GetUserByID(r.Context(), channel.CreatorID)
			if e != nil || target.DomainID != scope.DomainID || target.TenantID != scope.TenantID {
				s.failure(w, chat.ErrForbidden)
				return true
			}
		}
		req := r.Clone(chat.WithDomainScope(r.Context(), scope.DomainID))
		req.URL.Path = "/" + path
		req.URL.RawPath = ""
		s.ServeHTTP(w, req)
		return true
	}
	if path == "products/session" {
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return true
		}
		if uid == "" {
			fail(w, 401, "tms.products.login_required", "Sign in to TMS")
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	if path == "products/bootstrap" {
		if r.Method != "GET" {
			wrongMethod(w)
			return true
		}
		if uid == "" {
			fail(w, 401, "tms.products.login_required", "Sign in to TMS")
			return true
		}
		user, err := s.chat.User(r.Context(), scope.TenantID, uid)
		if err != nil {
			s.failure(w, err)
			return true
		}
		if user.AuthSource == "bot" || user.AuthSource == "federated" {
			s.failure(w, chat.ErrForbidden)
			return true
		}
		team, err := s.teamModel(r, user)
		if err != nil {
			s.failure(w, err)
			return true
		}
		users, err := s.store.ListUsersByDomain(r.Context(), scope.DomainID)
		if err != nil {
			s.failure(w, err)
			return true
		}
		locale := productLocale(r)
		profile := userModel(user, true)
		profile["locale"] = locale
		profiles := []any{}
		for _, u := range users {
			if u.Enabled && u.TenantID == scope.TenantID {
				if u.ID == user.ID {
					profiles = append(profiles, profile)
				} else {
					profiles = append(profiles, userModel(u, false))
				}
			}
		}
		channels, err := s.productChannels(r, user)
		if err != nil {
			s.failure(w, err)
			return true
		}
		resolved, e := s.policy.Resolve(r.Context(), scope)
		if e != nil {
			s.failure(w, e)
			return true
		}
		roles := model.MakeDefaultRoles()
		roles["team_user"].Permissions = append(roles["team_user"].Permissions, model.PermissionPublicPlaybookCreate.Id, model.PermissionPrivatePlaybookCreate.Id, model.PermissionRunCreate.Id)
		write(w, 200, map[string]any{"user": profile, "team": team, "users": profiles, "channels": channels, "roles": roles, "preferences": []any{}, "config": map[string]string{"EnableDeveloper": "false", "EnableTesting": "false", "EnablePlugins": "true", "EnableCommands": "true", "EnableCustomEmoji": "false", "SiteName": resolved.SiteName, "DefaultClientLocale": locale, "TeammateNameDisplay": "full_name", "DiagnosticId": chat.ID(scope.Key())}})
		return true
	}
	if path == "boards" || strings.HasPrefix(path, "boards/") || path == "playbooks" || strings.HasPrefix(path, "playbooks/") {
		if r.Method != "GET" {
			wrongMethod(w)
			return true
		}
		base := "/mm/" + chat.ID(scope.Key())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		product := "boards"
		if strings.HasPrefix(path, "playbooks") {
			product = "playbooks"
		}
		locale, loading := productLocale(r), "Загрузка…"
		if locale == "en" {
			loading = "Loading…"
		}
		stringsScript := ""
		if product == "boards" && locale == "ru" {
			stringsScript = `<script defer src="/assets/tayga-boards-templates-ru.js"></script>`
		}
		w.Write([]byte(`<!doctype html><html lang="` + locale + `" data-tayga-product="` + product + `"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Tayga</title><link rel="stylesheet" href="/assets/tayga-products.css?v=1.3.0-ui.12"><style>:root{--font-family:Arial,sans-serif;--heading-font:Arial,sans-serif;--body-font:Arial,sans-serif}html,body,#root{height:100%;margin:0}body{font-family:Arial,sans-serif;color:var(--tayga-product-text);background:var(--tayga-product-panel)}*{box-sizing:border-box}</style><div id="root">` + loading + `</div>` + stringsScript + `<script defer src="/assets/tayga-product-frame.js?v=1.3.0-ui.12"></script><script defer src="` + base + `/static/plugins/playbooks/tms-host.js?v=1.3.0-ui.12"></script></html>`))
		return true
	}
	return false
}

func productLocale(r *http.Request) string {
	if r.URL.Query().Get("lang") == "en" {
		return "en"
	}
	return "ru"
}
func (s *API) productChannels(r *http.Request, u *storage.User) ([]any, error) {
	channels, err := s.chat.Channels(r.Context(), u)
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, channel := range channels {
		creator, e := s.store.GetUserByID(r.Context(), channel.CreatorID)
		if e == nil && creator.DomainID == u.DomainID && creator.TenantID == u.TenantID {
			out = append(out, channelModel(channel))
		}
	}
	return out, nil
}
