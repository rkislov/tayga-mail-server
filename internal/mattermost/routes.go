package mattermost

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) users(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if s.directoryUsers(w, r, u, path) {
		return
	}
	if s.userChannelUnread(w, r, u, parts) {
		return
	}
	if s.pushPreferences(w, r, u, parts) {
		return
	}
	if s.userPreferences(w, r, u, parts) {
		return
	}
	if s.userMetadata(w, r, u, parts) {
		return
	}
	if path == "/users/ids" && r.Method == http.MethodPost {
		var ids []string
		if !decode(w, r, &ids) {
			return
		}
		if len(ids) > 200 {
			s.failure(w, chat.ErrInvalid)
			return
		}
		out := []any{}
		for _, id := range ids {
			user, err := s.chat.User(r.Context(), u.TenantID, id)
			if err == storage.ErrNotFound {
				continue
			}
			if err != nil {
				s.failure(w, err)
				return
			}
			if s.federatedUserVisible(r.Context(), u, user) {
				out = append(out, userModel(user, user.ID == u.ID))
			}
		}
		write(w, 200, out)
		return
	}
	if path == "/users/status/ids" && r.Method == http.MethodPost {
		var ids []string
		if !decode(w, r, &ids) {
			return
		}
		if len(ids) > 200 {
			s.failure(w, chat.ErrInvalid)
			return
		}
		out := []any{}
		for _, id := range ids {
			user, err := s.chat.User(r.Context(), u.TenantID, id)
			if err == storage.ErrNotFound {
				continue
			}
			if err != nil {
				s.failure(w, err)
				return
			}
			out = append(out, map[string]any{"user_id": chat.ID(user.ID), "status": "offline", "manual": false, "last_activity_at": 0})
		}
		write(w, 200, out)
		return
	}
	if path == "/users" && r.Method == http.MethodGet {
		allowedUsers := map[string]bool(nil)
		for key := range r.URL.Query() {
			if key != "page" && key != "per_page" && key != "in_channel" && key != "in_team" && key != "sort" && key != "active" && key != "inactive" {
				s.unsupported(w, r)
				return
			}
		}
		if team := r.URL.Query().Get("in_team"); team != "" && team != chat.ID(u.TenantID) {
			s.failure(w, storage.ErrNotFound)
			return
		}
		if channelID := r.URL.Query().Get("in_channel"); channelID != "" {
			members, err := s.chat.Members(r.Context(), u, channelID)
			if err != nil {
				s.failure(w, err)
				return
			}
			allowedUsers = make(map[string]bool)
			for _, m := range members {
				allowedUsers[m.UserID] = true
			}
		}
		limit, offset, err := page(r)
		if err != nil {
			s.failure(w, err)
			return
		}
		users, err := s.store.ListUsersByTenant(r.Context(), u.TenantID)
		if err != nil {
			s.failure(w, err)
			return
		}
		sort.Slice(users, func(i, j int) bool {
			return strings.ToLower(users[i].DisplayLabel()+users[i].ID) < strings.ToLower(users[j].DisplayLabel()+users[j].ID)
		})
		out := []any{}
		for _, user := range users {
			if !s.federatedUserVisible(r.Context(), u, user) || user.AuthSource == "resource" || (allowedUsers == nil && user.DomainID != u.DomainID) || (allowedUsers != nil && !allowedUsers[user.ID]) {
				continue
			}
			if offset > 0 {
				offset--
				continue
			}
			if len(out) >= limit {
				break
			}
			out = append(out, userModel(user, user.ID == u.ID))
		}
		write(w, 200, out)
		return
	}
	if len(parts) < 2 {
		s.unsupported(w, r)
		return
	}
	self := parts[1] == "me" || parts[1] == chat.ID(u.ID)
	if len(parts) == 2 && r.Method == http.MethodGet {
		target := u
		var err error
		if !self {
			target, err = s.chat.User(r.Context(), u.TenantID, parts[1])
			if err != nil {
				s.failure(w, err)
				return
			}
		}
		if !s.federatedUserVisible(r.Context(), u, target) {
			s.failure(w, storage.ErrNotFound)
			return
		}
		model := userModel(target, self)
		if self {
			if mode, e := s.notificationMode(r.Context(), u); e == nil {
				model["notify_props"].(map[string]string)["push"] = mode
			}
		}
		write(w, 200, model)
		return
	}
	if !self {
		s.failure(w, chat.ErrForbidden)
		return
	}
	suffix := strings.Join(parts[2:], "/")
	switch {
	case suffix == "teams" && r.Method == http.MethodGet:
		t, err := s.teamModel(r, u)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, []any{t})
	case suffix == "teams/members" && r.Method == http.MethodGet:
		write(w, 200, []any{map[string]any{"team_id": chat.ID(u.TenantID), "user_id": chat.ID(u.ID), "roles": "team_user", "delete_at": 0, "scheme_user": true, "scheme_admin": false, "scheme_guest": false}})
	case suffix == "preferences" && r.Method == http.MethodGet:
		write(w, 200, []any{map[string]string{"user_id": chat.ID(u.ID), "category": "display_settings", "name": "name_format", "value": "nickname_full_name"}})
	case suffix == "channels" && r.Method == http.MethodGet:
		s.listChannels(w, r, u)
	case suffix == "teams/unread" && r.Method == http.MethodGet:
		count, err := s.chat.Unread(r.Context(), u)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, []any{map[string]any{"team_id": chat.ID(u.TenantID), "msg_count": count, "mention_count": 0}})
	case strings.HasPrefix(suffix, "teams/"):
		segments := strings.Split(suffix, "/")
		if len(segments) < 3 || segments[1] != chat.ID(u.TenantID) {
			s.failure(w, storage.ErrNotFound)
			return
		}
		if suffix, ok := sidebarSuffix(segments); ok {
			s.sidebar(w, r, u, suffix)
			return
		}
		switch {
		case len(segments) == 3 && segments[2] == "threads" && r.Method == http.MethodGet:
			// Collapsed threads are disabled in client configuration and the
			// prototype has no following subscriptions. Root/reply history
			// remains available through the post/thread endpoints.
			write(w, 200, map[string]any{"total": 0, "total_unread_threads": 0, "total_unread_mentions": 0, "total_unread_urgent_mentions": 0, "threads": []any{}})
		case len(segments) == 3 && segments[2] == "channels" && r.Method == http.MethodGet:
			s.listChannels(w, r, u)
		case len(segments) == 4 && segments[2] == "channels" && segments[3] == "members" && r.Method == http.MethodGet:
			channels, err := s.chat.Channels(r.Context(), u)
			if err != nil {
				s.failure(w, err)
				return
			}
			out := []any{}
			for _, c := range channels {
				members, err := s.chat.Members(r.Context(), u, c.ID)
				if err != nil {
					s.failure(w, err)
					return
				}
				for _, m := range members {
					if m.UserID == u.ID {
						out = append(out, memberModel(c.ID, m))
					}
				}
			}
			write(w, 200, out)
		default:
			s.unsupported(w, r)
		}
	default:
		s.unsupported(w, r)
	}
}
func (s *API) teams(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 && parts[1] == "name" && r.Method == http.MethodGet {
		team, err := s.teamModel(r, u)
		if err != nil {
			s.failure(w, err)
			return
		}
		if parts[2] != team["name"] {
			s.failure(w, storage.ErrNotFound)
			return
		}
		if len(parts) == 3 {
			write(w, 200, team)
			return
		}
		if len(parts) == 6 && parts[3] == "channels" && parts[4] == "name" {
			channel, err := s.chat.ChannelByName(r.Context(), u, parts[5])
			if err != nil {
				s.failure(w, err)
				return
			}
			write(w, 200, channelModel(channel))
			return
		}
		s.unsupported(w, r)
		return
	}
	if len(parts) < 2 || parts[1] != chat.ID(u.TenantID) {
		s.failure(w, storage.ErrNotFound)
		return
	}
	if len(parts) == 4 && parts[2] == "commands" && parts[3] == "autocomplete" && r.Method == http.MethodGet {
		s.commands(w, r, u, "/commands")
		return
	}
	if len(parts) >= 3 && parts[2] == "channels" {
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			s.publicChannels(w, r, u)
			return
		case len(parts) == 4 && parts[3] == "search" && r.Method == http.MethodPost:
			s.publicChannels(w, r, u)
			return
		case len(parts) == 5 && parts[3] == "name" && r.Method == http.MethodGet:
			c, err := s.chat.ChannelByName(r.Context(), u, parts[4])
			if err != nil {
				s.failure(w, err)
				return
			}
			write(w, 200, channelModel(c))
			return
		}
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		t, err := s.teamModel(r, u)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, t)
		return
	}

	s.unsupported(w, r)
}
func (s *API) listChannels(w http.ResponseWriter, r *http.Request, u *storage.User) {
	channels, err := s.chat.Channels(r.Context(), u)
	if err != nil {
		s.failure(w, err)
		return
	}
	out := []any{}
	for _, c := range channels {
		out = append(out, channelModel(c))
	}
	write(w, 200, out)
}
func (s *API) channels(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	if path == "/channels/members/me/view" && r.Method == http.MethodPost {
		var req struct {
			ChannelID     string `json:"channel_id"`
			PrevChannelID string `json:"prev_channel_id"`
		}
		if !decode(w, r, &req) {
			return
		}
		if err := s.chat.View(r.Context(), u, req.ChannelID); err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, map[string]any{"status": "OK", "last_viewed_at_times": map[string]int64{req.ChannelID: time.Now().UnixMilli()}})
		return
	}

	if path == "/channels/search" && r.Method == http.MethodPost || path == "/channels" && r.Method == http.MethodGet {
		s.publicChannels(w, r, u)
		return
	}
	if path == "/channels" {
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		var req struct {
			TeamID      string `json:"team_id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.TeamID != chat.ID(u.TenantID) {
			s.failure(w, chat.ErrForbidden)
			return
		}
		c, err := s.chat.CreateChannel(r.Context(), u, req.Name, req.DisplayName, req.Type)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 201, channelModel(c))
		return
	}
	if path == "/channels/direct" {
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		var ids []string
		if !decode(w, r, &ids) {
			return
		}
		if len(ids) != 2 || (ids[0] != chat.ID(u.ID) && ids[1] != chat.ID(u.ID)) {
			s.failure(w, chat.ErrInvalid)
			return
		}
		other := ids[0]
		if other == chat.ID(u.ID) {
			other = ids[1]
		}
		c, err := s.chat.Direct(r.Context(), u, other)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 201, channelModel(c))
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		s.unsupported(w, r)
		return
	}
	var newMemberID string
	if len(parts) == 3 && parts[2] == "members" && r.Method == http.MethodPost {
		var req struct {
			UserID string `json:"user_id"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.UserID == chat.ID(u.ID) {
			if err := s.chat.JoinPublicChannel(r.Context(), u, parts[1]); err != nil {
				s.failure(w, err)
				return
			}
		} else if err := s.chat.AddMember(r.Context(), u, parts[1], req.UserID); err != nil {
			s.failure(w, err)
			return
		}
		newMemberID = req.UserID
	}
	c, err := s.chat.Channel(r.Context(), u, parts[1])
	if errors.Is(err, storage.ErrNotFound) && len(parts) == 2 && r.Method == http.MethodGet {
		c, err = s.chat.PublicChannel(r.Context(), u, parts[1])
	}
	if err != nil {
		s.failure(w, err)
		return
	}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			wrongMethod(w)
			return
		}
		write(w, 200, channelModel(c))
		return
	}
	switch parts[2] {
	case "posts":
		if len(parts) != 3 || r.Method != http.MethodGet {
			s.unsupported(w, r)
			return
		}
		before, after := r.URL.Query().Get("before"), r.URL.Query().Get("after")
		if before != "" && after != "" || (before != "" || after != "") && r.URL.Query().Get("since") != "" && r.URL.Query().Get("since") != "0" {
			s.failure(w, chat.ErrInvalid)
			return
		}

		limit, offset, err := page(r)
		if err != nil {
			s.failure(w, err)
			return
		}
		var since int64
		if q := r.URL.Query().Get("since"); q != "" {
			since, err = strconv.ParseInt(q, 10, 64)
			if err != nil || since < 0 {
				s.failure(w, chat.ErrInvalid)
				return
			}
		}
		var posts []chat.Post
		if before != "" || after != "" {
			cursor := before
			if after != "" {
				cursor = after
			}
			posts, err = s.chat.PostsCursor(r.Context(), u, c.ID, cursor, after != "", limit, offset)
		} else {
			posts, err = s.chat.Posts(r.Context(), u, c.ID, limit, offset, since)
		}
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, postList(posts))
	case "members":
		targetID := newMemberID
		if targetID == "" && (r.Method != http.MethodGet || (len(parts) != 3 && len(parts) != 4)) {
			s.unsupported(w, r)
			return
		}
		if len(parts) == 4 {
			targetID = parts[3]
			if targetID == "me" {
				targetID = chat.ID(u.ID)
			}
		}
		members, err := s.chat.Members(r.Context(), u, c.ID)
		if err != nil {
			s.failure(w, err)
			return
		}
		out := []any{}
		for _, m := range members {
			if targetID != "" {
				if chat.ID(m.UserID) == targetID {
					status := 200
					if r.Method == http.MethodPost {
						status = 201
					}
					write(w, status, memberModel(c.ID, m))
					return
				}
			} else {
				out = append(out, memberModel(c.ID, m))
			}
		}
		if targetID != "" {
			s.failure(w, storage.ErrNotFound)
			return
		}
		write(w, 200, out)
	case "stats":
		if len(parts) != 3 || r.Method != http.MethodGet {
			wrongMethod(w)
			return
		}
		members, err := s.chat.Members(r.Context(), u, c.ID)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, map[string]any{"channel_id": c.ID, "member_count": len(members), "guest_count": 0, "pinnedpost_count": 0})
	default:
		s.unsupported(w, r)
	}
}
func postList(posts []chat.Post) map[string]any {
	order := []string{}
	items := map[string]any{}
	for _, p := range posts {
		order = append(order, p.ID)
		items[p.ID] = postModel(p)
	}
	return map[string]any{"order": order, "posts": items, "next_post_id": "", "prev_post_id": "", "has_next": false}
}
func (s *API) posts(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	actionParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(actionParts) == 4 && actionParts[2] == "actions" && r.Method == http.MethodPost {
		s.botPostAction(w, r, u, actionParts[1], actionParts[3])
		return
	}
	if path == "/posts" {
		if r.Method != http.MethodPost {
			wrongMethod(w)
			return
		}
		var req struct {
			ChannelID string         `json:"channel_id"`
			Message   string         `json:"message"`
			RootID    string         `json:"root_id"`
			PendingID string         `json:"pending_post_id"`
			FileIDs   []string       `json:"file_ids"`
			Props     map[string]any `json:"props"`
		}
		if !decode(w, r, &req) {
			return
		}
		if len(req.FileIDs) == 0 && s.nativeAgentPost(w, r, u, req.ChannelID, req.RootID, req.Message, req.PendingID) {
			return
		}
		p, err := s.chat.SendWithFiles(r.Context(), u, req.ChannelID, req.RootID, req.Message, req.PendingID, req.Props, req.FileIDs)
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 201, postModel(p))
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[2] == "files" && r.Method == http.MethodGet {
		post, err := s.chat.Post(r.Context(), u, parts[1])
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, http.StatusOK, fileModels(post.Attachments))
		return
	}
	if len(parts) == 3 && parts[2] == "thread" && r.Method == http.MethodGet {
		posts, err := s.chat.Thread(r.Context(), u, parts[1])
		if err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, postList(posts))
		return
	}
	if len(parts) != 2 || r.Method != http.MethodGet {
		s.unsupported(w, r)
		return
	}
	p, err := s.chat.Post(r.Context(), u, parts[1])
	if err != nil {
		s.failure(w, err)
		return
	}
	write(w, 200, postModel(p))
}

func (s *API) publicChannels(w http.ResponseWriter, r *http.Request, u *storage.User) {
	limit, offset, err := page(r)
	if err != nil {
		s.failure(w, chat.ErrInvalid)
		return
	}
	term := ""
	if r.Method == http.MethodPost {
		var input struct {
			Term    string   `json:"term"`
			TeamIDs []string `json:"team_ids"`
			Deleted bool     `json:"deleted"`
		}
		if !decode(w, r, &input) {
			return
		}
		for _, teamID := range input.TeamIDs {
			if teamID != chat.ID(u.TenantID) {
				s.failure(w, chat.ErrForbidden)
				return
			}
		}
		if input.Deleted {
			write(w, 200, []any{})
			return
		}
		term = input.Term
	}
	channels, err := s.chat.PublicChannels(r.Context(), u, term, limit, offset)
	if err != nil {
		s.failure(w, err)
		return
	}
	out := make([]any, 0, len(channels))
	for _, c := range channels {
		out = append(out, channelModel(c))
	}
	write(w, 200, out)
}
