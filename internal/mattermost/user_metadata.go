package mattermost

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/tayga/tms/internal/avatar"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) userMetadata(w http.ResponseWriter, r *http.Request, u *storage.User, parts []string) bool {
	if len(parts) != 3 || (parts[2] != "image" && parts[2] != "status" && parts[2] != "sessions") {
		return false
	}
	self := parts[1] == "me" || parts[1] == chat.ID(u.ID)
	target := u
	if !self {
		var err error
		target, err = s.chat.User(r.Context(), u.TenantID, parts[1])
		if err != nil {
			s.failure(w, err)
			return true
		}
	}
	switch parts[2] {
	case "image":
		if r.Method != http.MethodGet {
			wrongMethod(w)
			return true
		}
		image, err := avatar.Get(r.Context(), s.store, target.ID)
		if err != nil || image.Data == "" {
			s.failure(w, storage.ErrNotFound)
			return true
		}
		raw, err := base64.StdEncoding.DecodeString(image.Data)
		if err != nil {
			s.failure(w, err)
			return true
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(200)
		_, _ = w.Write(raw)
		return true
	case "status":
		status := map[string]any{"user_id": chat.ID(target.ID), "status": "offline", "manual": false, "last_activity_at": int64(0)}
		if self {
			status["status"] = "online"
			status["last_activity_at"] = time.Now().UnixMilli()
		}
		if r.Method == http.MethodPut {
			if !self {
				s.failure(w, chat.ErrForbidden)
				return true
			}
			var req struct {
				UserID string `json:"user_id"`
				Status string `json:"status"`
			}
			if !decode(w, r, &req) {
				return true
			}
			if req.UserID != "" && req.UserID != chat.ID(u.ID) {
				s.failure(w, chat.ErrForbidden)
				return true
			}
			if req.Status != "online" && req.Status != "offline" && req.Status != "away" && req.Status != "dnd" {
				s.failure(w, chat.ErrInvalid)
				return true
			}
			status["status"] = req.Status
			status["manual"] = true
			raw, err := json.Marshal(status)
			if err != nil {
				s.failure(w, err)
				return true
			}
			if err = s.store.PutSetting(r.Context(), "mattermost.status:"+u.ID, string(raw)); err != nil {
				s.failure(w, err)
				return true
			}
		} else if r.Method == http.MethodGet {
			raw, exists, err := s.store.GetSetting(r.Context(), "mattermost.status:"+target.ID)
			if err != nil {
				s.failure(w, err)
				return true
			}
			if exists {
				if err = json.Unmarshal([]byte(raw), &status); err != nil {
					s.failure(w, err)
					return true
				}
			}
		} else {
			wrongMethod(w)
			return true
		}
		write(w, 200, status)
		return true
	case "sessions":
		if !self {
			s.failure(w, chat.ErrForbidden)
			return true
		}
		if r.Method != http.MethodGet {
			wrongMethod(w)
			return true
		}
		sessions := []any{}
		// Stateless TMS API sessions currently expose only sessions observed by this
		// running process; never expose OAuth bearer/refresh tokens to the client.
		for _, session := range s.auth.Sessions() {
			if session.UserID != u.ID || session.Protocol != "Web" {
				continue
			}
			sessions = append(sessions, map[string]any{"id": chat.ID(session.ID), "user_id": chat.ID(u.ID), "create_at": session.Started.UnixMilli(), "last_activity_at": session.Seen.UnixMilli(), "expires_at": session.Started.Add(s.auth.Tokens.Cfg.AccessTokenTTL).UnixMilli(), "device_id": "", "roles": "system_user", "is_oauth": false, "props": map[string]string{"os": r.UserAgent(), "browser": session.Agent, "platform": strings.TrimSpace(session.Agent)}})
		}
		write(w, 200, sessions)
		return true
	}
	return false
}
