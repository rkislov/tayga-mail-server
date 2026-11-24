package mmplugins

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func preferenceKey(u *storage.User) string {
	return "mattermost.preferences:" + u.TenantID + ":" + u.DomainID + ":" + u.ID
}
func (a *hostAPI) preferencesLock() *sync.Mutex {
	if a.runtime.options.PreferencesMutex != nil {
		return a.runtime.options.PreferencesMutex
	}
	return &a.runtime.kvMu
}
func (a *hostAPI) readPreferences(u *storage.User) ([]model.Preference, error) {
	raw, ok, err := a.runtime.options.Store.GetSetting(context.Background(), preferenceKey(u))
	if err != nil {
		return nil, err
	}
	values := []model.Preference{{UserId: chat.ID(u.ID), Category: "display_settings", Name: "name_format", Value: "nickname_full_name"}}
	if ok {
		var saved []model.Preference
		if err = json.Unmarshal([]byte(raw), &saved); err != nil {
			return nil, err
		}
		values = saved
	}
	for i := range values {
		values[i].UserId = chat.ID(u.ID)
	}
	return values, nil
}
func (a *hostAPI) GetPreferencesForUser(userID string) ([]model.Preference, *model.AppError) {
	u, err := a.user(userID)
	if err != nil {
		return nil, appError(err)
	}
	lock := a.preferencesLock()
	lock.Lock()
	defer lock.Unlock()
	values, err := a.readPreferences(u)
	return values, appError(err)
}
func (a *hostAPI) UpdatePreferencesForUser(userID string, incoming []model.Preference) *model.AppError {
	return a.changePreferences(userID, incoming, false)
}
func (a *hostAPI) DeletePreferencesForUser(userID string, incoming []model.Preference) *model.AppError {
	return a.changePreferences(userID, incoming, true)
}
func (a *hostAPI) changePreferences(userID string, incoming []model.Preference, deleting bool) *model.AppError {
	u, err := a.user(userID)
	if err != nil {
		return appError(err)
	}
	if len(incoming) > 200 {
		return appError(ErrInvalid)
	}
	for i, p := range incoming {
		if p.UserId != "" && p.UserId != userID {
			return appError(chat.ErrForbidden)
		}
		if p.Category == "" || len(p.Category) > 80 || strings.ContainsAny(p.Category, "/\x00") || p.Name == "" || len(p.Name) > 256 || strings.ContainsRune(p.Name, '\x00') || len(p.Value) > 65536 {
			return appError(ErrInvalid)
		}
		incoming[i].UserId = userID
	}
	lock := a.preferencesLock()
	lock.Lock()
	defer lock.Unlock()
	existing, err := a.readPreferences(u)
	if err != nil {
		return appError(err)
	}
	values := map[string]model.Preference{}
	for _, p := range existing {
		values[p.Category+"\x00"+p.Name] = p
	}
	for _, p := range incoming {
		key := p.Category + "\x00" + p.Name
		if deleting {
			delete(values, key)
		} else {
			values[key] = p
		}
	}
	if len(values) > 5000 {
		return appError(ErrInvalid)
	}
	out := make([]model.Preference, 0, len(values))
	for _, p := range values {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Name < out[j].Name
		}
		return out[i].Category < out[j].Category
	})
	encoded, err := json.Marshal(out)
	if err != nil {
		return appError(err)
	}
	if len(encoded) > 1<<20 {
		return appError(ErrInvalid)
	}
	if err = a.runtime.options.Store.PutSetting(context.Background(), preferenceKey(u), string(encoded)); err != nil {
		return appError(err)
	}
	event := "preferences_changed"
	if deleting {
		event = "preferences_deleted"
	}
	raw, err := json.Marshal(incoming)
	if err != nil {
		return appError(err)
	}
	return appError(a.runtime.options.Chat.PublishEvent(chat.Event{Type: event, UserID: userID, DomainID: u.DomainID, Data: map[string]any{"preferences": string(raw)}}))
}
