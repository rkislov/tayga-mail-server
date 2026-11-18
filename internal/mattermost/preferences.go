package mattermost

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type preference struct {
	UserID   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

func preferenceID(p preference) string { return p.Category + "\x00" + p.Name }
func (s *API) userPreferences(w http.ResponseWriter, r *http.Request, u *storage.User, parts []string) bool {
	if len(parts) < 3 || parts[2] != "preferences" {
		return false
	}
	if parts[1] != "me" && parts[1] != chat.ID(u.ID) {
		s.failure(w, chat.ErrForbidden)
		return true
	}
	s.preferencesMu.Lock()
	defer s.preferencesMu.Unlock()
	key := "mattermost.preferences:" + u.TenantID + ":" + u.DomainID + ":" + u.ID
	raw, exists, err := s.store.GetSetting(r.Context(), key)
	if err != nil {
		s.failure(w, err)
		return true
	}
	saved := []preference{}
	if exists {
		if err = json.Unmarshal([]byte(raw), &saved); err != nil {
			s.failure(w, err)
			return true
		}
	}
	values := map[string]preference{}
	base := preference{UserID: chat.ID(u.ID), Category: "display_settings", Name: "name_format", Value: "nickname_full_name"}
	values[preferenceID(base)] = base
	for _, p := range saved {
		p.UserID = chat.ID(u.ID)
		values[preferenceID(p)] = p
	}
	if r.Method == http.MethodGet {
		category := ""
		if len(parts) >= 4 {
			category = parts[3]
		}
		if len(parts) != 3 && len(parts) != 4 && !(len(parts) == 6 && parts[4] == "name") {
			s.unsupported(w, r)
			return true
		}
		if len(parts) == 6 {
			p, ok := values[category+"\x00"+parts[5]]
			if !ok {
				s.failure(w, storage.ErrNotFound)
			} else {
				write(w, 200, p)
			}
			return true
		}
		list := []preference{}
		for _, p := range values {
			if category == "" || p.Category == category {
				list = append(list, p)
			}
		}
		sort.Slice(list, func(i, j int) bool { return preferenceID(list[i]) < preferenceID(list[j]) })
		write(w, 200, list)
		return true
	}
	deleting := r.Method == http.MethodPost && len(parts) == 4 && parts[3] == "delete"
	if !(r.Method == http.MethodPut && len(parts) == 3) && !deleting {
		s.unsupported(w, r)
		return true
	}
	var incoming []preference
	if !decode(w, r, &incoming) {
		return true
	}
	if incoming == nil || len(incoming) > 200 {
		s.failure(w, chat.ErrInvalid)
		return true
	}
	for _, p := range incoming {
		if p.UserID != "" && p.UserID != chat.ID(u.ID) {
			s.failure(w, chat.ErrForbidden)
			return true
		}
		if p.Category == "" || len(p.Category) > 80 || strings.ContainsAny(p.Category, "/\x00") || p.Name == "" || len(p.Name) > 256 || strings.ContainsRune(p.Name, '\x00') || len(p.Value) > 65536 {
			s.failure(w, chat.ErrInvalid)
			return true
		}
	}
	for i, p := range incoming {
		p.UserID = chat.ID(u.ID)
		incoming[i] = p
		if deleting {
			delete(values, preferenceID(p))
		} else {
			values[preferenceID(p)] = p
		}
	}
	out := make([]preference, 0, len(values))
	for _, p := range values {
		out = append(out, p)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		s.failure(w, err)
		return true
	}
	if len(encoded) > 1<<20 || len(out) > 5000 {
		s.failure(w, chat.ErrInvalid)
		return true
	}
	if err = s.store.PutSetting(r.Context(), key, string(encoded)); err != nil {
		s.failure(w, err)
		return true
	}
	event := "preferences_changed"
	if deleting {
		event = "preferences_deleted"
	}
	data, _ := json.Marshal(incoming)
	_ = s.chat.PublishEvent(chat.Event{Type: event, UserID: chat.ID(u.ID), Data: map[string]any{"preferences": string(data)}})
	write(w, 200, map[string]string{"status": "OK"})
	return true
}
