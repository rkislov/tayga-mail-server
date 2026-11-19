package mmplugins

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

type hostAPI struct {
	*unsupportedAPI
	runtime  *Runtime
	manifest *model.Manifest
}

var _ plugin.API = (*hostAPI)(nil)

func (a *hostAPI) GetServerVersion() string { return Version }
func (a *hostAPI) GetDiagnosticId() string {
	id, err := a.runtime.options.Chat.ServerID(context.Background())
	if err != nil {
		a.log.Error("plugin installation identity", "error", err)
	}
	return id
}
func (a *hostAPI) GetTelemetryId() string { return a.GetDiagnosticId() }
func (a *hostAPI) GetBundlePath() (string, error) {
	return filepath.Join(a.runtime.options.Directory, a.manifest.Id), nil
}
func (a *hostAPI) GetPluginConfig() map[string]any {
	result := map[string]any{}
	if a.manifest.SettingsSchema != nil {
		for _, s := range a.manifest.SettingsSchema.Settings {
			result[s.Key] = s.Default
		}
	}
	stored, err := a.runtime.readConfig(a.manifest.Id)
	if err != nil {
		a.log.Error("plugin config read", "plugin_id", a.manifest.Id, "error", err)
		return result
	}
	for k, v := range stored {
		result[k] = v
	}

	return result
}
func (a *hostAPI) LoadPluginConfiguration(dest any) error {
	raw, err := json.Marshal(a.GetPluginConfig())
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}
func (a *hostAPI) SavePluginConfig(config map[string]any) *model.AppError {
	raw, err := json.Marshal(config)
	if err != nil {
		return appError(err)
	}
	if len(raw) > 1<<20 {
		return appError(ErrInvalid)
	}
	return appError(a.runtime.writeConfig(a.manifest.Id, config))
}
func (a *hostAPI) GetConfig() *model.Config {
	c := &model.Config{}
	c.SetDefaults()
	c.ServiceSettings.SiteURL = &a.runtime.options.SiteURL
	c.LogSettings.EnableDiagnostics = new(false)
	c.PluginSettings.Enable = new(true)
	c.PluginSettings.EnableUploads = new(true)
	c.PluginSettings.Directory = &a.runtime.options.Directory
	c.PluginSettings.Plugins = map[string]map[string]any{a.manifest.Id: a.GetPluginConfig()}
	return c
}
func (a *hostAPI) GetUnsanitizedConfig() *model.Config { return a.GetConfig() }
func (a *hostAPI) RegisterCommand(c *model.Command) error {
	if c == nil || len(c.Trigger) == 0 || strings.ContainsAny(c.Trigger, " /\t\r\n") {
		return ErrInvalid
	}
	clone := *c
	clone.Id = chat.ID("command:" + a.manifest.Id + ":" + c.TeamId + ":" + c.Trigger)
	clone.PluginId = a.manifest.Id
	a.runtime.commandMu.Lock()
	defer a.runtime.commandMu.Unlock()
	key := c.TeamId + ":" + c.Trigger
	if existing, ok := a.runtime.commands[key]; ok && existing.PluginID != a.manifest.Id {
		return errors.New("slash command already registered")
	}
	a.runtime.commands[key] = registeredCommand{a.manifest.Id, &clone}
	return nil
}
func (a *hostAPI) UnregisterCommand(teamID, trigger string) error {
	a.runtime.commandMu.Lock()
	defer a.runtime.commandMu.Unlock()
	key := teamID + ":" + trigger
	if c, ok := a.runtime.commands[key]; ok && c.PluginID == a.manifest.Id {
		delete(a.runtime.commands, key)
	}
	return nil
}
func (a *hostAPI) LogDebug(msg string, kv ...any) {
	a.log.Debug(msg, append([]any{"plugin_id", a.manifest.Id}, kv...)...)
}
func (a *hostAPI) LogInfo(msg string, kv ...any) {
	a.log.Info(msg, append([]any{"plugin_id", a.manifest.Id}, kv...)...)
}
func (a *hostAPI) LogWarn(msg string, kv ...any) {
	a.log.Warn(msg, append([]any{"plugin_id", a.manifest.Id}, kv...)...)
}
func (a *hostAPI) LogError(msg string, kv ...any) {
	a.log.Error(msg, append([]any{"plugin_id", a.manifest.Id}, kv...)...)
}

type kvValue struct {
	Data      []byte `json:"data"`
	ExpiresAt int64  `json:"expires_at"`
}

func (a *hostAPI) kvPrefix() string {
	return "mmplugin.kv:" + a.runtime.options.Scope + ":" + a.manifest.Id + ":"
}
func (a *hostAPI) kvKey(key string) string {
	return a.kvPrefix() + base64.RawURLEncoding.EncodeToString([]byte(key))
}
func (a *hostAPI) readKV(key string) ([]byte, error) {
	raw, ok, err := a.runtime.options.Store.GetSetting(context.Background(), a.kvKey(key))
	if err != nil || !ok {
		return nil, err
	}
	var value kvValue
	if err = json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	if value.ExpiresAt > 0 && value.ExpiresAt <= time.Now().Unix() {
		return nil, nil
	}
	return value.Data, nil
}
func (a *hostAPI) KVGet(key string) ([]byte, *model.AppError) {
	value, err := a.readKV(key)
	return value, appError(err)
}
func (a *hostAPI) KVSet(key string, value []byte) *model.AppError {
	_, err := a.KVSetWithOptions(key, value, model.PluginKVSetOptions{})
	return err
}
func (a *hostAPI) KVSetWithExpiry(key string, value []byte, expire int64) *model.AppError {
	_, err := a.KVSetWithOptions(key, value, model.PluginKVSetOptions{ExpireInSeconds: expire})
	return err
}
func (a *hostAPI) KVSetWithOptions(key string, value []byte, options model.PluginKVSetOptions) (bool, *model.AppError) {
	if len(key) > 1024 || len(value) > 1<<20 || options.ExpireInSeconds < 0 {
		return false, appError(ErrInvalid)
	}
	a.runtime.kvMu.Lock()
	defer a.runtime.kvMu.Unlock()
	if options.Atomic {
		old, err := a.readKV(key)
		if err != nil {
			return false, appError(err)
		}
		if !bytes.Equal(old, options.OldValue) {
			return false, nil
		}
	}
	expiry := int64(0)
	if options.ExpireInSeconds > 0 {
		expiry = time.Now().Unix() + options.ExpireInSeconds
	}
	raw, err := json.Marshal(kvValue{value, expiry})
	if err != nil {
		return false, appError(err)
	}
	err = a.runtime.options.Store.PutSetting(context.Background(), a.kvKey(key), string(raw))
	return err == nil, appError(err)
}
func (a *hostAPI) KVCompareAndSet(key string, oldValue, newValue []byte) (bool, *model.AppError) {
	return a.KVSetWithOptions(key, newValue, model.PluginKVSetOptions{Atomic: true, OldValue: oldValue})
}
func (a *hostAPI) KVDelete(key string) *model.AppError {
	_, err := a.KVSetWithOptions(key, nil, model.PluginKVSetOptions{})
	return err
}
func (a *hostAPI) KVCompareAndDelete(key string, oldValue []byte) (bool, *model.AppError) {
	return a.KVCompareAndSet(key, oldValue, nil)
}
func (a *hostAPI) KVList(page, perPage int) ([]string, *model.AppError) {
	if page < 0 || page > 1000000 || perPage < 1 || perPage > 200 {
		return nil, appError(ErrInvalid)
	}
	all, err := a.runtime.options.Store.ListSettings(context.Background())
	if err != nil {
		return nil, appError(err)
	}
	keys := []string{}
	for key := range all {
		if !strings.HasPrefix(key, a.kvPrefix()) {
			continue
		}
		raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, a.kvPrefix()))
		if e != nil {
			continue
		}
		data, e := a.readKV(string(raw))
		if e != nil {
			return nil, appError(e)
		}
		if data != nil {
			keys = append(keys, string(raw))
		}
	}
	sort.Strings(keys)
	start := page * perPage
	if start >= len(keys) {
		return []string{}, nil
	}
	end := start + perPage
	if end > len(keys) {
		end = len(keys)
	}
	return keys[start:end], nil
}
func (a *hostAPI) KVDeleteAll() *model.AppError {
	// Iterate unpaginated storage keys so deletion cannot skip later pages.
	all, err := a.runtime.options.Store.ListSettings(context.Background())
	if err != nil {
		return appError(err)
	}
	for key := range all {
		if strings.HasPrefix(key, a.kvPrefix()) {
			raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, a.kvPrefix()))
			if e == nil {
				if appErr := a.KVDelete(string(raw)); appErr != nil {
					return appErr
				}
			}
		}
	}
	return nil
}
func (a *hostAPI) user(id string) (*storage.User, error) {
	tenants, err := a.runtime.options.Store.ListTenants(context.Background())
	if err != nil {
		return nil, err
	}
	for _, t := range tenants {
		if a.runtime.options.TenantID != "" && t.ID != a.runtime.options.TenantID {
			continue
		}
		u, e := a.runtime.options.Chat.User(context.Background(), t.ID, id)
		if e == nil && a.userInScope(u) {
			return u, nil
		}
		if e != nil && !errors.Is(e, storage.ErrNotFound) {
			return nil, e
		}
	}
	return nil, storage.ErrNotFound
}
func (a *hostAPI) userInScope(u *storage.User) bool {
	return u != nil && u.Enabled && (a.runtime.options.TenantID == "" || u.TenantID == a.runtime.options.TenantID) && (a.runtime.options.DomainID == "" || u.DomainID == a.runtime.options.DomainID)
}
func modelUser(u *storage.User) *model.User {
	return &model.User{Id: chat.ID(u.ID), Username: botUsername(u), Email: u.Email, Nickname: u.DisplayName, Roles: "system_user", Locale: "ru", IsBot: u.AuthSource == "bot", CreateAt: u.CreatedAt.UnixMilli(), UpdateAt: u.CreatedAt.UnixMilli(), Props: model.StringMap{}}
}
func (a *hostAPI) GetUser(id string) (*model.User, *model.AppError) {
	u, err := a.user(id)
	if err != nil {
		return nil, appError(err)
	}
	return modelUser(u), nil
}
func (a *hostAPI) GetUserByEmail(email string) (*model.User, *model.AppError) {
	u, err := a.runtime.options.Store.GetUserByEmail(context.Background(), email)
	if err != nil {
		return nil, appError(err)
	}
	if !a.userInScope(u) {
		return nil, appError(storage.ErrNotFound)
	}
	return modelUser(u), nil
}
func (a *hostAPI) GetUserByUsername(username string) (*model.User, *model.AppError) {
	ts, err := a.runtime.options.Store.ListTenants(context.Background())
	if err != nil {
		return nil, appError(err)
	}
	for _, t := range ts {
		if a.runtime.options.TenantID != "" && t.ID != a.runtime.options.TenantID {
			continue
		}
		users, e := a.runtime.options.Store.ListUsersByTenant(context.Background(), t.ID)
		if e != nil {
			return nil, appError(e)
		}
		for _, u := range users {
			m := modelUser(u)
			if a.userInScope(u) && m.Username == username {
				return m, nil
			}
		}
	}
	return nil, appError(storage.ErrNotFound)
}
func (a *hostAPI) GetTeam(id string) (*model.Team, *model.AppError) {
	ts, err := a.runtime.options.Store.ListTenants(context.Background())
	if err != nil {
		return nil, appError(err)
	}
	for _, t := range ts {
		if a.runtime.options.TenantID != "" && t.ID != a.runtime.options.TenantID {
			continue
		}
		if chat.ID(t.ID) == id {
			return &model.Team{Id: id, Name: "tms-" + id, DisplayName: t.Name, Type: model.TeamInvite}, nil
		}
	}
	return nil, appError(storage.ErrNotFound)
}
func (a *hostAPI) GetTeams() ([]*model.Team, *model.AppError) {
	ts, err := a.runtime.options.Store.ListTenants(context.Background())
	if err != nil {
		return nil, appError(err)
	}
	out := []*model.Team{}
	for _, t := range ts {
		if a.runtime.options.TenantID != "" && t.ID != a.runtime.options.TenantID {
			continue
		}
		m, e := a.GetTeam(chat.ID(t.ID))
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, nil
}
func (a *hostAPI) GetSystemInstallDate() (int64, *model.AppError) {
	ts, err := a.runtime.options.Store.ListTenants(context.Background())
	if err != nil {
		return 0, appError(err)
	}
	if len(ts) == 0 {
		return 0, nil
	}
	return ts[0].CreatedAt.UnixMilli(), nil
}
