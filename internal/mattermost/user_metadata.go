package mattermost

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
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
	if !s.federatedUserVisible(r.Context(), u, target) {
		s.failure(w, storage.ErrNotFound)
		return true
	}
	switch parts[2] {
	case "image":
		if r.Method != http.MethodGet {
			wrongMethod(w)
			return true
		}
		image, err := avatar.Get(r.Context(), s.store, target.ID)
		if err != nil {
			s.failure(w, err)
			return true
		}
		var raw []byte
		if image.Data != "" {
			raw, err = base64.StdEncoding.DecodeString(image.Data)
		}
		if image.Data == "" || err != nil {
			raw, err = avatar.InitialsPNG(target.DisplayName, target.LocalPart)
		}
		if err != nil {
			s.failure(w, err)
			return true
		}
		w.Header().Set("Cache-Control", "private, max-age=60")
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
		sessions, err := s.sessionModels(r.Context(), u, bearer(r))
		if err != nil {
			s.failure(w, err)
			return true
		}
		write(w, 200, sessions)
		return true
	}
	return false
}
