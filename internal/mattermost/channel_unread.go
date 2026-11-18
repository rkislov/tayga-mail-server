package mattermost

import (
	"net/http"
	"strconv"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) userChannelUnread(w http.ResponseWriter, r *http.Request, u *storage.User, parts []string) bool {
	match := len(parts) == 5 && parts[2] == "channels" && parts[4] == "unread" || len(parts) == 6 && parts[2] == "channels" && parts[4] == "posts" && parts[5] == "unread"
	if !match {
		return false
	}
	if parts[1] != "me" && parts[1] != chat.ID(u.ID) {
		s.failure(w, chat.ErrForbidden)
		return true
	}
	if r.Method != http.MethodGet {
		wrongMethod(w)
		return true
	}
	c, e := s.chat.Channel(r.Context(), u, parts[3])
	if e != nil {
		s.failure(w, e)
		return true
	}
	if len(parts) == 5 {
		state, e := s.chat.ReadState(r.Context(), u, c.ID)
		if e != nil {
			s.failure(w, e)
			return true
		}
		mentions := int64(0)
		if c.Kind == "D" {
			mentions = state.Unread
		}
		write(w, 200, map[string]any{"team_id": chat.ID(u.TenantID), "channel_id": c.ID, "msg_count": state.Unread, "msg_count_root": state.UnreadRoots, "mention_count": mentions, "mention_count_root": min(mentions, state.UnreadRoots), "urgent_mention_count": 0, "last_viewed_at": state.LastViewedAt})
		return true
	}
	before, after := 30, 30
	for key, target := range map[string]*int{"limit_before": &before, "limit_after": &after} {
		if value := r.URL.Query().Get(key); value != "" {
			n, e := strconv.Atoi(value)
			if e != nil || n < 0 || n > 200 {
				s.failure(w, chat.ErrInvalid)
				return true
			}
			*target = n
		}
	}
	list, e := s.chat.PostsAroundUnread(r.Context(), u, c.ID, before, after)
	if e != nil {
		s.failure(w, e)
		return true
	}
	write(w, 200, postList(list))
	return true
}
