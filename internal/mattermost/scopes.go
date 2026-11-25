package mattermost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmplugins"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

var errPluginsDisabled = errors.New("plugins are disabled for this scope")

type scopedRuntime struct {
	runtime     *mmplugins.Runtime
	fingerprint string
}

func (s *API) Settings() *mmsettings.Service { return s.policy }
func (s *API) SetScopeAuthorizer(check func(*http.Request, *storage.User, mmsettings.Scope) bool) {
	s.scopeAuthorize = check
}
func userScope(u *storage.User) mmsettings.Scope {
	return mmsettings.Scope{TenantID: u.TenantID, DomainID: u.DomainID}
}
func (s *API) pluginRuntimeForScope(ctx context.Context, scope mmsettings.Scope) (*mmplugins.Runtime, error) {
	if s.plugins == nil || scope.TenantID == "" || scope.DomainID == "" {
		return nil, errPluginsDisabled
	}
	resolved, err := s.policy.Resolve(ctx, scope)
	if err != nil {
		return nil, err
	}
	if !resolved.Enabled || !resolved.PluginsEnabled {
		return nil, errPluginsDisabled
	}
	raw, err := json.Marshal(resolved.Plugins)
	if err != nil {
		return nil, err
	}
	fingerprint := string(raw)
	key := scope.Key()
	s.pluginScopeMu.Lock()
	defer s.pluginScopeMu.Unlock()
	if old, ok := s.pluginScopes[key]; ok {
		if old.fingerprint == fingerprint {
			return old.runtime, nil
		}
		old.runtime.Close()
		delete(s.pluginScopes, key)
	}
	scopeRef := chat.ID(key)
	record, _ := json.Marshal(scope)
	if err = s.store.PutSetting(ctx, "mattermost.runtime_scope:"+scopeRef, string(record)); err != nil {
		return nil, err
	}
	options := mmplugins.Options{PreferencesMutex: &s.preferencesMu, Directory: s.pluginDirectory, SiteURL: strings.TrimRight(s.siteURL, "/") + "/mm/" + scopeRef, Store: s.store, Chat: s.chat, Logger: s.log, TenantID: scope.TenantID, DomainID: scope.DomainID, Scope: key, Admins: s.pluginAdmins,
		LoadConfiguration: func(id string) (map[string]any, error) {
			p, e := s.policy.Resolve(context.Background(), scope)
			if e != nil {
				return nil, e
			}
			return p.Plugins[id].Configuration, nil
		},
		SaveConfiguration: func(id string, c map[string]any) error {
			return s.policy.SetPluginConfig(context.Background(), scope, id, c)
		},
		ReadEnabled: func(id string) (bool, error) {
			p, e := s.policy.Resolve(context.Background(), scope)
			return e == nil && p.Plugins[id].Enabled, e
		},
		WriteEnabled: func(id string, value bool) error {
			return s.policy.SetPluginEnabled(context.Background(), scope, id, value)
		},
	}
	if s.pluginDatabases != nil {
		s.pluginDatabases.Configure(&options)
	}
	runtime, err := mmplugins.New(options)
	if err != nil {
		return nil, err
	}
	s.pluginScopes[key] = scopedRuntime{runtime, fingerprint}
	return runtime, nil
}
func (s *API) scopeFromRequest(r *http.Request, u *storage.User) (mmsettings.Scope, error) {
	scope := userScope(u)
	if id := r.URL.Query().Get("tenant_id"); id != "" {
		scope.TenantID = id
		scope.DomainID = ""
	}
	if id := r.URL.Query().Get("domain_id"); id != "" {
		scope.DomainID = id
	}
	if err := s.policy.Validate(r.Context(), scope); err != nil {
		return scope, err
	}
	return scope, nil
}
func (s *API) canManageScope(r *http.Request, u *storage.User, scope mmsettings.Scope) bool {
	if s.scopeAuthorize != nil {
		return s.scopeAuthorize(r, u, scope)
	}
	return s.pluginAdmin(u)
}
func (s *API) ServeScopedPluginHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/mm/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	raw, ok, err := s.store.GetSetting(r.Context(), "mattermost.runtime_scope:"+parts[0])
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	var scope mmsettings.Scope
	if json.Unmarshal([]byte(raw), &scope) != nil || chat.ID(scope.Key()) != parts[0] {
		http.NotFound(w, r)
		return
	}
	if parts[1] == "products/session" && r.Method == http.MethodDelete {
		if origin := r.Header.Get("Origin"); origin != "" {
			configured, e := url.Parse(s.siteURL)
			if e != nil || origin != configured.Scheme+"://"+configured.Host {
				s.failure(w, chat.ErrForbidden)
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: "tms_product_session", Value: "", Path: "/mm/" + parts[0] + "/", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil || strings.HasPrefix(s.siteURL, "https://"), SameSite: http.SameSiteStrictMode})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	uid := ""
	token := bearer(r)
	cookieAuth := false
	if token == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		if cookie, e := r.Cookie("tms_product_session"); e == nil {
			token = cookie.Value
			cookieAuth = true
		}
	}
	if token != "" {
		u, e := s.authenticate(r.Context(), token)
		if e != nil {
			fail(w, 401, "api.context.session_expired.app_error", "Invalid session")
			return
		}
		if userScope(u) != scope {
			s.failure(w, chat.ErrForbidden)
			return
		}
		uid = chat.ID(u.ID)
		if cookieAuth {
			r = r.Clone(r.Context())
			r.Header.Set("Authorization", "Bearer "+token)
		} else {
			// Same-session, read-only cookie transport for original <img> / download URLs.
			// Mutations always require the explicit bearer header and are not authorized
			// by this cookie. Revocation and token expiry are checked on every request.
			http.SetCookie(w, &http.Cookie{Name: "tms_product_session", Value: token, Path: "/mm/" + parts[0] + "/", MaxAge: 120, HttpOnly: true, Secure: r.TLS != nil || strings.HasPrefix(s.siteURL, "https://"), SameSite: http.SameSiteStrictMode})
		}
	}

	runtime, err := s.pluginRuntimeForScope(r.Context(), scope)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if s.serveProductHTTP(w, r, runtime, scope, uid, parts[1]) {
		return
	}
	if !strings.HasPrefix(parts[1], "plugins/") {
		http.NotFound(w, r)
		return
	}
	req := r.Clone(r.Context())
	req.URL.Path = "/" + parts[1]
	req.URL.RawPath = ""
	runtime.ServeHTTP(w, req, uid)
}

func (s *API) PluginCatalog() (map[string]any, error) {
	if s.plugins == nil {
		return map[string]any{"active": []any{}, "inactive": []any{}}, nil
	}
	return s.plugins.List()
}
