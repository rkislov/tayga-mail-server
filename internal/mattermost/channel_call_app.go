package mattermost

import (
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

const (
	mattermostAppsRoute = "/plugins/com.mattermost.apps/api/v1"
	channelCallAppID    = "tms-jitsi"
	channelCallAppPath  = "/start-channel-call"
)

func (s *API) channelCallApp(w http.ResponseWriter, r *http.Request, u *storage.User) {
	w.Header().Set("Cache-Control", "no-store")
	ctx := r.Context()
	policy, err := s.policy.Resolve(ctx, userScope(u))
	if err != nil {
		s.failure(w, err)
		return
	}
	if !policy.Enabled {
		s.failure(w, chat.ErrForbidden)
		return
	}
	switch r.URL.Path {
	case mattermostAppsRoute + "/bindings":
		if r.Method != http.MethodGet {
			wrongMethod(w)
			return
		}
		s.channelCallBindings(w, r, u)
	case mattermostAppsRoute + "/call":
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		var call struct {
			Path    string `json:"path"`
			Context struct {
				AppID     string `json:"app_id"`
				ChannelID string `json:"channel_id"`
			} `json:"context"`
		}
		if !decode(w, r, &call) {
			return
		}
		if call.Context.AppID == "tms-boards" && call.Path == "/open-boards" {
			if _, err := s.chat.Channel(ctx, u, call.Context.ChannelID); err != nil {
				s.failure(w, err)
				return
			}
			if !s.boardsAvailable(r, u) {
				s.failure(w, storage.ErrNotFound)
				return
			}
			write(w, 200, map[string]string{"type": "navigate", "navigate_to_url": strings.TrimRight(s.siteURL, "/") + "/mm/" + chat.ID(userScope(u).Key()) + "/boards"})
			return
		}
		if call.Context.AppID != channelCallAppID || call.Path != channelCallAppPath || call.Context.ChannelID == "" {
			s.failure(w, chat.ErrInvalid)
			return
		}
		link, err := s.publishChannelCall(ctx, u, call.Context.ChannelID)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, http.StatusOK, map[string]string{"type": "navigate", "navigate_to_url": link})
	}
}

func (s *API) channelCallBindings(w http.ResponseWriter, r *http.Request, u *storage.User) {
	ctx := r.Context()
	query := r.URL.Query()
	if id := query.Get("user_id"); id != "" && id != chat.ID(u.ID) {
		s.failure(w, chat.ErrForbidden)
		return
	}
	bindings := []any{}
	channelID := query.Get("channel_id")
	if channelID != "" {
		_, err := s.chat.Channel(ctx, u, channelID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			s.failure(w, err)
			return
		}
		if err == nil {
			allowed := false
			var err error
			if s.canStartChannelCall != nil {
				allowed, err = s.canStartChannelCall(ctx, u)
			}
			if err != nil {
				s.failure(w, err)
				return
			}
			if s.boardsAvailable(r, u) {
				bindings = append(bindings, map[string]any{"location": "/channel_header", "bindings": []any{map[string]any{"app_id": "tms-boards", "location": "tms-boards", "label": "Доски Boards", "icon": "apps-outline", "submit": map[string]string{"path": "/open-boards"}}}})
			}
			if allowed {
				bindings = append(bindings, map[string]any{
					"location": "/channel_header",
					"bindings": []any{map[string]any{
						"app_id":   channelCallAppID,
						"location": "tms-jitsi",
						"label":    "Начать звонок Jitsi",
						"icon":     "video-outline",
						"submit":   map[string]string{"path": channelCallAppPath},
					}},
				})
			}
		}
	}
	write(w, http.StatusOK, bindings)
}
