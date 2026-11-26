package vks

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type Provider string

const (
	ProviderJitsi  Provider = "jitsi"
	ProviderKontur Provider = "kontur_talk"
)

var konturSpaceHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.ktalk\.(ru|host)$`)

// KonturSpace accepts a public Kontur Talk space, without credentials or room paths.
func KonturSpace(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || u.Opaque != "" || (u.EscapedPath() != "" && u.EscapedPath() != "/") ||
		!konturSpaceHost.MatchString(strings.ToLower(u.Host)) {
		return "", errors.New("use the HTTPS address of a Kontur Talk space, for example https://team.ktalk.ru")
	}
	return "https://" + strings.ToLower(u.Host), nil
}

type Scope struct {
	TenantID string `json:"tenant_id"`
	DomainID string `json:"domain_id"`
}

func (s Scope) key() string {
	if s.DomainID != "" {
		return "domain:" + s.TenantID + ":" + s.DomainID
	}
	if s.TenantID != "" {
		return "tenant:" + s.TenantID
	}
	return "server"
}

type Policy struct {
	Provider  *Provider `json:"provider,omitempty"`
	KonturURL *string   `json:"kontur_url,omitempty"`
	Enabled   *bool     `json:"enabled,omitempty"`
	STUNURLs  *[]string `json:"stun_urls,omitempty"`
	TURNURLs  *[]string `json:"turn_urls,omitempty"`
}
type Effective struct {
	Provider  Provider `json:"provider"`
	KonturURL string   `json:"kontur_url,omitempty"`
	Enabled   bool     `json:"enabled"`
	STUNURLs  []string `json:"stun_urls"`
	TURNURLs  []string `json:"turn_urls"`
}
type Policies struct {
	Store    storage.Driver
	Defaults Effective
}

func (p Policies) Validate(ctx context.Context, s Scope) error {
	if s.DomainID != "" && s.TenantID == "" {
		return errors.New("domain requires tenant")
	}
	if s.TenantID != "" {
		if _, err := p.Store.GetTenantByID(ctx, s.TenantID); err != nil {
			return err
		}
	}
	if s.DomainID != "" {
		d, err := p.Store.GetDomainByID(ctx, s.DomainID)
		if err != nil {
			return err
		}
		if d.TenantID != s.TenantID {
			return errors.New("domain belongs to another tenant")
		}
	}
	return nil
}
func (p Policies) Read(ctx context.Context, s Scope) (Policy, error) {
	var out Policy
	if err := p.Validate(ctx, s); err != nil {
		return out, err
	}
	raw, ok, err := p.Store.GetSetting(ctx, "vks.policy:"+s.key())
	if err != nil || !ok {
		return out, err
	}
	err = json.Unmarshal([]byte(raw), &out)
	return out, err
}
func (p Policies) Save(ctx context.Context, s Scope, value Policy) error {
	if err := p.Validate(ctx, s); err != nil {
		return err
	}
	if value.Provider != nil && *value.Provider != ProviderJitsi && *value.Provider != ProviderKontur {
		return errors.New("unknown conferencing provider")
	}
	if value.KonturURL != nil {
		normalized := strings.TrimSpace(*value.KonturURL)
		if normalized != "" {
			var err error
			normalized, err = KonturSpace(normalized)
			if err != nil {
				return err
			}
		}
		value.KonturURL = &normalized
	}
	if value.STUNURLs != nil {
		if err := ValidateURLs(*value.STUNURLs, false); err != nil {
			return err
		}
	}
	if value.TURNURLs != nil {
		if err := ValidateURLs(*value.TURNURLs, true); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return p.Store.PutSetting(ctx, "vks.policy:"+s.key(), string(raw))
}
func (p Policies) Resolve(ctx context.Context, s Scope) (Effective, error) {
	out := p.Defaults
	if out.Provider == "" {
		out.Provider = ProviderJitsi
	}
	if err := p.Validate(ctx, s); err != nil {
		return out, err
	}
	scopes := []Scope{{}}
	if s.TenantID != "" {
		scopes = append(scopes, Scope{TenantID: s.TenantID})
	}
	if s.DomainID != "" {
		scopes = append(scopes, s)
	}
	for _, scope := range scopes {
		v, err := p.Read(ctx, scope)
		if err != nil {
			return out, err
		}
		if v.Enabled != nil {
			out.Enabled = *v.Enabled
		}
		if v.Provider != nil {
			out.Provider = *v.Provider
		}
		if v.KonturURL != nil {
			out.KonturURL = *v.KonturURL
		}
		if v.STUNURLs != nil {
			out.STUNURLs = *v.STUNURLs
		}
		if v.TURNURLs != nil {
			out.TURNURLs = *v.TURNURLs
		}
	}
	return out, nil
}
