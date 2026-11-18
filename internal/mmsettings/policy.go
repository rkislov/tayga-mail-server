// Package mmsettings resolves Mattermost policy and plugin configuration by
// server, tenant and domain. A scope can only override its own stored values.
package mmsettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/tayga/tms/internal/storage"
)

type Scope struct {
	TenantID string `json:"tenant_id"`
	DomainID string `json:"domain_id"`
}

func (s Scope) Key() string {
	if s.DomainID != "" {
		return "domain:" + s.TenantID + ":" + s.DomainID
	}
	if s.TenantID != "" {
		return "tenant:" + s.TenantID
	}
	return "server"
}

type Plugin struct {
	Enabled       *bool          `json:"enabled,omitempty"`
	Configuration map[string]any `json:"configuration,omitempty"`
}
type Overrides struct {
	Push           *PushOverrides    `json:"push,omitempty"`
	Email          *EmailOverrides   `json:"email,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty"`
	PluginsEnabled *bool             `json:"plugins_enabled,omitempty"`
	SiteName       *string           `json:"site_name,omitempty"`
	Plugins        map[string]Plugin `json:"plugins,omitempty"`
}
type EffectivePlugin struct {
	Enabled       bool           `json:"enabled"`
	Configuration map[string]any `json:"configuration"`
}
type Effective struct {
	Push           PushEffective              `json:"push"`
	Email          EmailEffective             `json:"email"`
	Enabled        bool                       `json:"enabled"`
	PluginsEnabled bool                       `json:"plugins_enabled"`
	SiteName       string                     `json:"site_name"`
	Plugins        map[string]EffectivePlugin `json:"plugins"`
	Sources        map[string]string          `json:"sources"`
}
type Defaults struct {
	Enabled, PluginsEnabled bool
	SiteName                string
}
type Service struct {
	Store    storage.Driver
	Defaults Defaults
	mu       sync.Mutex
}

var ErrInvalid = errors.New("invalid Mattermost scope or configuration")
var secretKey = regexp.MustCompile(`(?i)(password|secret|token|private.?key|encryption.?key|api.?key)`)
var validPlugin = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,189}$`)

func (s *Service) Validate(ctx context.Context, scope Scope) error {
	if scope.DomainID != "" && scope.TenantID == "" {
		return ErrInvalid
	}
	if scope.TenantID != "" {
		if _, err := s.Store.GetTenantByID(ctx, scope.TenantID); err != nil {
			return err
		}
	}
	if scope.DomainID != "" {
		domains, err := s.Store.ListDomainsByTenant(ctx, scope.TenantID)
		if err != nil {
			return err
		}
		for _, d := range domains {
			if d.ID == scope.DomainID {
				return nil
			}
		}
		return storage.ErrNotFound
	}
	return nil
}
func (s *Service) Read(ctx context.Context, scope Scope) (Overrides, error) {
	if err := s.Validate(ctx, scope); err != nil {
		return Overrides{}, err
	}
	raw, ok, err := s.Store.GetSetting(ctx, "mattermost.policy:"+scope.Key())
	if err != nil || !ok {
		return Overrides{}, err
	}
	var value Overrides
	err = json.Unmarshal([]byte(raw), &value)
	return value, err
}
func (s *Service) Resolve(ctx context.Context, scope Scope) (Effective, error) {
	if err := s.Validate(ctx, scope); err != nil {
		return Effective{}, err
	}
	result := Effective{Enabled: s.Defaults.Enabled, PluginsEnabled: s.Defaults.PluginsEnabled, SiteName: s.Defaults.SiteName, Plugins: map[string]EffectivePlugin{}, Sources: map[string]string{"enabled": "bootstrap", "plugins_enabled": "bootstrap", "site_name": "bootstrap"}}
	scopes := []Scope{{}}
	if scope.TenantID != "" {
		scopes = append(scopes, Scope{TenantID: scope.TenantID})
	}
	if scope.DomainID != "" {
		scopes = append(scopes, scope)
	}
	result.Email = EmailEffective{SubjectPrefix: "[TMS]", UserOptInRequired: true, RecipientPolicy: "own_address"}
	for _, level := range scopes {
		override, err := s.Read(ctx, level)
		if err != nil {
			return Effective{}, err
		}
		if override.Enabled != nil {
			result.Enabled = *override.Enabled
			result.Sources["enabled"] = level.Key()
		}
		if override.PluginsEnabled != nil {
			result.PluginsEnabled = *override.PluginsEnabled
			result.Sources["plugins_enabled"] = level.Key()
		}
		if override.SiteName != nil {
			result.SiteName = *override.SiteName
			result.Sources["site_name"] = level.Key()
		}
		applyEmail(&result, override.Email, level)
		if override.Push != nil && override.Push.ProxyURL != nil {
			result.Push.ProxyURL = *override.Push.ProxyURL
			result.Push.Prepared = result.Push.ProxyURL == TPNSURL
			result.Sources["push.proxy_url"] = level.Key()
		}
		if override.Push != nil && override.Push.Enabled != nil {
			result.Push.Enabled = *override.Push.Enabled
			result.Sources["push.enabled"] = level.Key()
		}
		for id, p := range override.Plugins {
			current := result.Plugins[id]
			if current.Configuration == nil {
				current.Configuration = map[string]any{}
			}
			if p.Enabled != nil {
				current.Enabled = *p.Enabled
				result.Sources["plugins."+id+".enabled"] = level.Key()
			}
			for key, value := range p.Configuration {
				current.Configuration[key] = value
				result.Sources["plugins."+id+".configuration."+key] = level.Key()
			}
			result.Plugins[id] = current
		}
	}
	result.Push.Enabled = result.Enabled && result.Push.Prepared && result.Push.Enabled
	return result, nil
}
func (s *Service) Save(ctx context.Context, scope Scope, value Overrides) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(ctx, scope, value)
}
func (s *Service) save(ctx context.Context, scope Scope, value Overrides) error {
	if err := s.Validate(ctx, scope); err != nil {
		return err
	}
	if value.SiteName != nil && (strings.TrimSpace(*value.SiteName) == "" || len(*value.SiteName) > 128) {
		return ErrInvalid
	}
	if err := validatePush(value.Push); err != nil {
		return err
	}
	if err := s.validateEmail(ctx, scope, value.Email); err != nil {
		return err
	}
	current, err := s.Read(ctx, scope)
	if err != nil {
		return err
	}
	for id, p := range value.Plugins {
		if !validPlugin.MatchString(id) {
			return ErrInvalid
		}
		p.Configuration = preserveMasked(p.Configuration, current.Plugins[id].Configuration)
		value.Plugins[id] = p
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return ErrInvalid
	}
	return s.Store.PutSetting(ctx, "mattermost.policy:"+scope.Key(), string(raw))
}
func (s *Service) SetPluginEnabled(ctx context.Context, scope Scope, id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.Read(ctx, scope)
	if err != nil {
		return err
	}
	if value.Plugins == nil {
		value.Plugins = map[string]Plugin{}
	}
	p := value.Plugins[id]
	p.Enabled = &enabled
	value.Plugins[id] = p
	return s.save(ctx, scope, value)
}
func (s *Service) SetPluginConfig(ctx context.Context, scope Scope, id string, configuration map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.Read(ctx, scope)
	if err != nil {
		return err
	}
	if value.Plugins == nil {
		value.Plugins = map[string]Plugin{}
	}
	p := value.Plugins[id]
	p.Configuration = configuration
	value.Plugins[id] = p
	return s.save(ctx, scope, value)
}
func preserveMasked(next, old map[string]any) map[string]any {
	if next == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range next {
		masked, _ := v.(string)
		if secretKey.MatchString(k) && masked == "***" {
			if previous, ok := old[k]; ok {
				out[k] = previous
			}
			continue
		}
		if object, ok := v.(map[string]any); ok {
			previous, _ := old[k].(map[string]any)
			out[k] = preserveMasked(object, previous)
		} else {
			out[k] = v
		}
	}
	return out
}
func Redact(value any) any {
	raw, _ := json.Marshal(value)
	var result any
	_ = json.Unmarshal(raw, &result)
	var visit func(any)
	visit = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			for key, value := range node {
				text, isText := value.(string)
				if secretKey.MatchString(key) && value != nil && (!isText || text != "") {
					node[key] = "***"
				} else {
					visit(value)
				}
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(result)
	return result
}
func (s Scope) String() string { return fmt.Sprintf("tenant=%s domain=%s", s.TenantID, s.DomainID) }
