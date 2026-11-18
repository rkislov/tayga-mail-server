package mattermost

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

// The incoming hook ID is a capability bound to one channel and owner domain.
// It is never accepted as a bearer token for the normal API.
type incomingHook struct {
	ID          string `json:"id"`
	CreatorID   string `json:"creator_id"`
	TenantID    string `json:"tenant_id"`
	DomainID    string `json:"domain_id"`
	ChannelID   string `json:"channel_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"create_at"`
}

func hookPublic(h incomingHook) map[string]any {
	return map[string]any{"id": h.ID, "user_id": chat.ID(h.CreatorID), "channel_id": h.ChannelID, "team_id": chat.ID(h.TenantID), "display_name": h.DisplayName, "description": h.Description, "create_at": h.CreatedAt, "update_at": h.CreatedAt, "delete_at": 0}
}
func (s *API) incomingHooks(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	key := "mattermost.incoming:"
	if path == "/hooks/incoming" {
		switch r.Method {
		case http.MethodPost:
			var req struct {
				ChannelID   string `json:"channel_id"`
				DisplayName string `json:"display_name"`
				Description string `json:"description"`
			}
			if !decode(w, r, &req) {
				return
			}
			if len(req.DisplayName) > 128 || len(req.Description) > 1024 {
				s.failure(w, chat.ErrInvalid)
				return
			}
			if _, err := s.chat.Channel(r.Context(), u, req.ChannelID); err != nil {
				s.failure(w, err)
				return
			}
			h := incomingHook{chat.ID(uuid.NewString()), u.ID, u.TenantID, u.DomainID, req.ChannelID, req.DisplayName, req.Description, time.Now().UnixMilli()}
			raw, err := json.Marshal(h)
			if err != nil {
				s.failure(w, err)
				return
			}
			if err = s.store.PutSetting(r.Context(), key+h.ID, string(raw)); err != nil {
				s.failure(w, err)
				return
			}
			write(w, 201, hookPublic(h))
			return
		case http.MethodGet:
			all, err := s.store.ListSettings(r.Context())
			if err != nil {
				s.failure(w, err)
				return
			}
			out := []any{}
			for k, raw := range all {
				if !strings.HasPrefix(k, key) || raw == "" {
					continue
				}
				var h incomingHook
				if json.Unmarshal([]byte(raw), &h) != nil {
					continue
				}
				if h.CreatorID == u.ID {
					out = append(out, hookPublic(h))
				}
			}
			write(w, 200, out)
			return
		default:
			wrongMethod(w)
			return
		}
	}
	id := strings.TrimPrefix(path, "/hooks/incoming/")
	if id == "" || strings.Contains(id, "/") {
		s.unsupported(w, r)
		return
	}
	raw, exists, err := s.store.GetSetting(r.Context(), key+id)
	if err != nil {
		s.failure(w, err)
		return
	}
	var h incomingHook
	if !exists || raw == "" || json.Unmarshal([]byte(raw), &h) != nil {
		s.failure(w, storage.ErrNotFound)
		return
	}
	if h.CreatorID != u.ID {
		s.failure(w, chat.ErrForbidden)
		return
	}
	if r.Method == http.MethodGet {
		write(w, 200, hookPublic(h))
		return
	}
	if r.Method != http.MethodDelete {
		wrongMethod(w)
		return
	}
	if err = s.store.PutSetting(r.Context(), key+id, ""); err != nil {
		s.failure(w, err)
		return
	}
	write(w, 200, map[string]string{"status": "OK"})
}
func (s *API) ServeIncomingHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		wrongMethod(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/hooks/")
	if len(id) != 26 || strings.Contains(id, "/") {
		s.failure(w, storage.ErrNotFound)
		return
	}
	raw, exists, err := s.store.GetSetting(r.Context(), "mattermost.incoming:"+id)
	if err != nil {
		s.failure(w, err)
		return
	}
	var h incomingHook
	if !exists || raw == "" || json.Unmarshal([]byte(raw), &h) != nil {
		s.failure(w, storage.ErrNotFound)
		return
	}
	u, err := s.store.GetUserByID(r.Context(), h.CreatorID)
	if err != nil || !u.Enabled || u.TenantID != h.TenantID || u.DomainID != h.DomainID {
		s.failure(w, chat.ErrForbidden)
		return
	}
	policy, err := s.policy.Resolve(r.Context(), userScope(u))
	if err != nil {
		s.failure(w, err)
		return
	}
	if !policy.Enabled || !policy.PluginsEnabled {
		s.failure(w, chat.ErrForbidden)
		return
	}
	c, err := s.chat.Channel(r.Context(), u, h.ChannelID)
	if err != nil {
		s.failure(w, err)
		return
	}
	var payload struct {
		Text        string           `json:"text"`
		Channel     string           `json:"channel"`
		Username    string           `json:"username"`
		IconURL     string           `json:"icon_url"`
		Attachments []map[string]any `json:"attachments"`
		Props       map[string]any   `json:"props"`
	}
	if !decode(w, r, &payload) {
		return
	}
	if payload.Channel != "" && payload.Channel != c.ID && payload.Channel != c.Name && payload.Channel != "#"+c.Name {
		s.failure(w, chat.ErrForbidden)
		return
	}
	if len(payload.Username) > 64 || len(payload.IconURL) > 2048 || len(payload.Attachments) > 50 {
		s.failure(w, chat.ErrInvalid)
		return
	}
	props := payload.Props
	if props == nil {
		props = map[string]any{}
	}
	props["from_webhook"] = "true"
	if len(payload.Attachments) > 0 {
		props["attachments"] = payload.Attachments
	}
	if payload.Username != "" {
		props["override_username"] = payload.Username
	}
	if payload.IconURL != "" {
		props["override_icon_url"] = payload.IconURL
	}
	if strings.TrimSpace(payload.Text) == "" && len(payload.Attachments) == 0 {
		s.failure(w, chat.ErrInvalid)
		return
	}
	if _, err = s.chat.SendRich(r.Context(), u, h.ChannelID, "", payload.Text, "", props, ""); err != nil {
		s.failure(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte("ok"))
}
