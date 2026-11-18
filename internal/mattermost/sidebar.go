package mattermost

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type sidebarCategory struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	TeamID      string   `json:"team_id"`
	DisplayName string   `json:"display_name"`
	Type        string   `json:"type"`
	Sorting     string   `json:"sorting"`
	SortOrder   int64    `json:"sort_order"`
	Muted       bool     `json:"muted"`
	Collapsed   bool     `json:"collapsed"`
	ChannelIDs  []string `json:"channel_ids"`
}

func sidebarKey(u *storage.User) string { return "mattermost.sidebar:" + u.TenantID + ":" + u.ID }
func (s *API) sidebarState(r *http.Request, u *storage.User) ([]sidebarCategory, map[string]string, error) {
	channels, err := s.chat.Channels(r.Context(), u)
	if err != nil {
		return nil, nil, err
	}
	allowed := make(map[string]string, len(channels))
	for _, c := range channels {
		allowed[c.ID] = c.Kind
	}
	categories := []sidebarCategory{}
	for i, kind := range []string{"favorites", "channels", "direct_messages"} {
		name, sorting := "Favorites", "manual"
		if kind == "channels" {
			name, sorting = "Channels", "alpha"
		}
		if kind == "direct_messages" {
			name, sorting = "Direct Messages", "recent"
		}
		categories = append(categories, sidebarCategory{ID: chat.ID("sidebar:" + u.TenantID + ":" + u.ID + ":" + kind), UserID: chat.ID(u.ID), TeamID: chat.ID(u.TenantID), DisplayName: name, Type: kind, Sorting: sorting, SortOrder: int64(i) * 1024, ChannelIDs: []string{}})
	}
	raw, exists, err := s.store.GetSetting(r.Context(), sidebarKey(u))
	if err != nil {
		return nil, nil, err
	}
	if exists {
		var saved []sidebarCategory
		if err = json.Unmarshal([]byte(raw), &saved); err != nil {
			return nil, nil, err
		}
		// Only these built-in categories can be persisted by the current adapter.
		expected := map[string]sidebarCategory{}
		for _, c := range categories {
			expected[c.ID] = c
		}
		if len(saved) != len(categories) {
			return nil, nil, chat.ErrInvalid
		}
		seen := map[string]bool{}
		for _, c := range saved {
			e, ok := expected[c.ID]
			if !ok || seen[c.ID] || c.Type != e.Type || c.UserID != e.UserID || c.TeamID != e.TeamID {
				return nil, nil, chat.ErrInvalid
			}
			seen[c.ID] = true
		}
		categories = saved
	}
	seen := map[string]bool{}
	for i := range categories {
		ids := []string{}
		for _, id := range categories[i].ChannelIDs {
			if _, ok := allowed[id]; ok && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		categories[i].ChannelIDs = ids
		categories[i].SortOrder = int64(i) * 1024
	}
	for _, c := range channels {
		if seen[c.ID] {
			continue
		}
		kind := "channels"
		if c.Kind == "D" {
			kind = "direct_messages"
		}
		for i := range categories {
			if categories[i].Type == kind {
				categories[i].ChannelIDs = append(categories[i].ChannelIDs, c.ID)
				break
			}
		}
	}
	return categories, allowed, nil
}
func sidebarOrder(categories []sidebarCategory) []string {
	out := []string{}
	for _, c := range categories {
		out = append(out, c.ID)
	}
	return out
}
func (s *API) saveSidebar(r *http.Request, u *storage.User, categories []sidebarCategory) error {
	raw, err := json.Marshal(categories)
	if err != nil {
		return err
	}
	return s.store.PutSetting(r.Context(), sidebarKey(u), string(raw))
}
func (s *API) sidebar(w http.ResponseWriter, r *http.Request, u *storage.User, suffix string) {
	s.sidebarMu.Lock()
	defer s.sidebarMu.Unlock()
	categories, allowed, err := s.sidebarState(r, u)
	if err != nil {
		s.failure(w, err)
		return
	}
	if suffix == "order" {
		if r.Method == http.MethodGet {
			write(w, 200, sidebarOrder(categories))
			return
		}
		if r.Method != http.MethodPut {
			wrongMethod(w)
			return
		}
		var order []string
		if !decode(w, r, &order) {
			return
		}
		if len(order) != len(categories) {
			s.failure(w, chat.ErrInvalid)
			return
		}
		byID := map[string]sidebarCategory{}
		for _, c := range categories {
			byID[c.ID] = c
		}
		ordered := []sidebarCategory{}
		for i, id := range order {
			c, ok := byID[id]
			if !ok {
				s.failure(w, chat.ErrInvalid)
				return
			}
			delete(byID, id)
			c.SortOrder = int64(i) * 1024
			ordered = append(ordered, c)
		}
		if err = s.saveSidebar(r, u, ordered); err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, order)
		return
	}
	if r.Method == http.MethodGet {
		if suffix == "" {
			write(w, 200, map[string]any{"categories": categories, "order": sidebarOrder(categories)})
			return
		}
		for _, c := range categories {
			if c.ID == suffix {
				write(w, 200, c)
				return
			}
		}
		s.failure(w, storage.ErrNotFound)
		return
	}
	if r.Method != http.MethodPut {
		s.unsupported(w, r)
		return
	}
	updates := []sidebarCategory{}
	if suffix == "" {
		if !decode(w, r, &updates) {
			return
		}
	} else {
		var c sidebarCategory
		if !decode(w, r, &c) {
			return
		}
		if c.ID != suffix {
			s.failure(w, chat.ErrInvalid)
			return
		}
		updates = append(updates, c)
	}
	if len(updates) == 0 || len(updates) > len(categories) {
		s.failure(w, chat.ErrInvalid)
		return
	}
	indexes := map[string]int{}
	for i, c := range categories {
		indexes[c.ID] = i
	}
	seen := map[string]bool{}
	moved := map[string]string{}
	for _, update := range updates {
		index, ok := indexes[update.ID]
		if !ok || seen[update.ID] {
			s.failure(w, chat.ErrInvalid)
			return
		}
		seen[update.ID] = true
		current := categories[index]
		if (update.UserID != "" && update.UserID != current.UserID) || (update.TeamID != "" && update.TeamID != current.TeamID) {
			s.failure(w, chat.ErrForbidden)
			return
		}
		if update.Type != "" && update.Type != current.Type {
			s.failure(w, chat.ErrInvalid)
			return
		}
		if update.Sorting != "" && update.Sorting != "manual" && update.Sorting != "alpha" && update.Sorting != "recent" && update.Sorting != "default" {
			s.failure(w, chat.ErrInvalid)
			return
		}
		for _, id := range update.ChannelIDs {
			if _, ok := allowed[id]; !ok {
				s.failure(w, chat.ErrForbidden)
				return
			}
			if _, ok := moved[id]; ok {
				s.failure(w, chat.ErrInvalid)
				return
			}
			moved[id] = update.ID
		}
		current.Muted = update.Muted
		current.Collapsed = update.Collapsed
		if update.Sorting != "" {
			current.Sorting = update.Sorting
		}
		current.ChannelIDs = append([]string{}, update.ChannelIDs...)
		categories[index] = current
	}
	for i := range categories {
		filtered := []string{}
		for _, id := range categories[i].ChannelIDs {
			if target, ok := moved[id]; !ok || target == categories[i].ID {
				filtered = append(filtered, id)
			}
		}
		categories[i].ChannelIDs = filtered
	}
	if err = s.saveSidebar(r, u, categories); err != nil {
		s.failure(w, err)
		return
	}
	if suffix != "" {
		write(w, 200, categories[indexes[suffix]])
		return
	}
	result := []sidebarCategory{}
	for _, update := range updates {
		result = append(result, categories[indexes[update.ID]])
	}
	write(w, 200, result)
}

func sidebarSuffix(parts []string) (string, bool) {
	// teams/{team}/channels/categories[/{category or order}]
	if len(parts) < 4 || len(parts) > 5 || parts[2] != "channels" || parts[3] != "categories" {
		return "", false
	}
	if len(parts) == 5 {
		return strings.TrimSpace(parts[4]), true
	}
	return "", true
}
