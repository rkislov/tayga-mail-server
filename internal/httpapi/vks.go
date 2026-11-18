package httpapi

import (
	"bytes"
	"encoding/json"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/vks"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

func (s *Server) vksPolicies() vks.Policies {
	c := s.cfgLive().VKS
	return vks.Policies{Store: s.store, Defaults: vks.Effective{Enabled: c.Enabled, STUNURLs: c.STUNURLs, TURNURLs: c.TURNURLs}}
}
func (s *Server) handleVKS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cfg := s.cfgLive().VKS
	if !cfg.Enabled {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/.well-known/vks/keys/") {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/.well-known/vks/keys/") != vks.KeyFilename(cfg.KeyID) {
			http.NotFound(w, r)
			return
		}
		key, err := vks.LoadSigningKey(cfg.SigningKeyFile)
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "signing key unavailable"})
			return
		}
		data, err := vks.PublicKeyPEM(key)
		if err != nil {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Write(data)
		return
	}
	meetings := vks.Meetings{Store: s.store}
	if r.URL.Path == "/api/v1/vks/branding" {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		meeting, err := meetings.Get(r.Context(), r.URL.Query().Get("conferenceFqn"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		policy, err := s.vksPolicies().Resolve(r.Context(), vks.Scope{TenantID: meeting.TenantID, DomainID: meeting.DomainID})
		if err != nil || !policy.Enabled {
			http.NotFound(w, r)
			return
		}
		brand, err := s.readBranding(r.Context(), "branding.domains."+meeting.DomainID)
		if err == nil && !brand.Enabled {
			brand, err = s.readBranding(r.Context(), "branding.tenants."+meeting.TenantID)
		}
		if err != nil {
			w.WriteHeader(503)
			return
		}
		if r.Header.Get("Origin") == cfg.PublicURL {
			w.Header().Set("Access-Control-Allow-Origin", cfg.PublicURL)
			w.Header().Set("Vary", "Origin")
		}
		out := map[string]any{"logoClickUrl": s.cfgLive().HTTP.PublicURL}
		if brand.Enabled {
			out["logoImageUrl"] = brand.Image
		}
		writeJSON(w, 200, out)
		return
	}
	session, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	user, err := s.store.GetUserByID(r.Context(), session.ID)
	if err != nil || !user.Enabled {
		w.WriteHeader(401)
		return
	}
	policy, err := s.vksPolicies().Resolve(r.Context(), vks.Scope{TenantID: user.TenantID, DomainID: user.DomainID})
	if err != nil || !policy.Enabled {
		writeJSON(w, 403, map[string]string{"error": "conferencing disabled for this domain"})
		return
	}
	switch r.URL.Path {
	case "/api/v1/vks/ice":
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		secret := ""
		if len(policy.TURNURLs) > 0 {
			raw, e := os.ReadFile(cfg.TURNSecretFile)
			if e != nil {
				writeJSON(w, 503, map[string]string{"error": "TURN secret unavailable"})
				return
			}
			secret = strings.TrimSpace(string(raw))
		}
		credentials, e := (vks.ICEConfig{STUNURLs: policy.STUNURLs, TURNURLs: policy.TURNURLs, Secret: secret, TTL: cfg.TURNTTL}).Credentials(time.Now(), user.TenantID, user.DomainID, user.ID)
		if e != nil {
			writeJSON(w, 503, map[string]string{"error": "invalid ICE service configuration"})
			return
		}
		writeJSON(w, 200, credentials)
	case "/api/v1/vks/meetings":
		if r.Method == http.MethodGet {
			settings, e := s.store.ListSettings(r.Context())
			if e != nil {
				w.WriteHeader(500)
				return
			}
			items := []vks.Meeting{}
			for k, raw := range settings {
				if strings.HasPrefix(k, "vks.meeting:") {
					var m vks.Meeting
					if json.Unmarshal([]byte(raw), &m) == nil && s.allowsVKSMeeting(r.Context(), m, user) {
						items = append(items, m)
					}
				}
			}
			writeJSON(w, 200, map[string]any{"meetings": items})
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		var req struct {
			Title        string   `json:"title"`
			Participants []string `json:"participants"`
		}
		if !decodeVKS(w, r, &req) {
			return
		}
		meeting, e := meetings.Create(r.Context(), user, req.Title, req.Participants)
		if e != nil {
			writeJSON(w, 400, map[string]string{"error": e.Error()})
			return
		}
		writeJSON(w, 201, meeting)
	default:
		room, ok := strings.CutPrefix(r.URL.Path, "/api/v1/vks/join/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		meeting, e := meetings.Get(r.Context(), room)
		if e != nil || !s.allowsVKSMeeting(r.Context(), meeting, user) {
			http.NotFound(w, r)
			return
		}
		key, e := vks.LoadSigningKey(cfg.SigningKeyFile)
		if e != nil {
			writeJSON(w, 503, map[string]string{"error": "signing key unavailable"})
			return
		}
		host, e := url.Parse(cfg.PublicURL)
		if e != nil || host.Scheme != "https" || host.Host == "" {
			w.WriteHeader(503)
			return
		}
		token, e := meetings.JoinToken(r.Context(), key, cfg.KeyID, host.Hostname(), meeting, user, time.Now())
		if e != nil {
			w.WriteHeader(403)
			return
		}
		brand, e := s.readBranding(r.Context(), "branding.domains."+user.DomainID)
		if e == nil && !brand.Enabled {
			brand, e = s.readBranding(r.Context(), "branding.tenants."+user.TenantID)
		}
		if e != nil {
			w.WriteHeader(503)
			return
		}
		if !brand.Enabled {
			brand = brandingConfig{}
		}
		writeJSON(w, 200, map[string]any{"room": room, "public_url": cfg.PublicURL, "jwt": token, "expires_at": time.Now().Add(30 * time.Minute).Unix(), "branding": brand})
	}
}
func decodeVKS(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "invalid request"})
		return false
	}
	return true
}
func (s *Server) handleVKSSettings(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	scope := vks.Scope{TenantID: r.URL.Query().Get("tenant_id"), DomainID: r.URL.Query().Get("domain_id")}
	if !s.canManageMattermostScope(r, admin, mmsettings.Scope{TenantID: scope.TenantID, DomainID: scope.DomainID}) {
		w.WriteHeader(403)
		return
	}
	policies := s.vksPolicies()
	if policies.Validate(r.Context(), scope) != nil {
		w.WriteHeader(400)
		return
	}
	if r.Method == http.MethodPut {
		var value vks.Policy
		if !decodeVKS(w, r, &value) {
			return
		}
		if err := policies.Save(r.Context(), scope, value); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
	} else if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	value, err := policies.Read(r.Context(), scope)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	effective, err := policies.Resolve(r.Context(), scope)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	writeJSON(w, 200, map[string]any{"scope": scope, "overrides": value, "effective": effective, "host_enabled": s.cfgLive().VKS.Enabled})
}

// Native Jitsi's nginx listens only on loopback. TMS retains ports 80/443 and
// routes the configured meeting hostname, including BOSH and WebSocket upgrades.
func (s *Server) withVKSProxy(next http.Handler) http.Handler {
	cfg := s.cfgLive().VKS
	if !cfg.Enabled || cfg.PublicURL == "" || cfg.Upstream == "" {
		return next
	}
	public, e := url.Parse(cfg.PublicURL)
	if e != nil || public.Host == "" {
		return next
	}
	upstream, e := url.Parse(cfg.Upstream)
	if e != nil || upstream.Scheme != "http" || (upstream.Hostname() != "127.0.0.1" && upstream.Hostname() != "::1") {
		return next
	}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	original := proxy.Director
	proxy.Director = func(r *http.Request) {
		original(r)
		r.Host = public.Host
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Header.Set("X-Forwarded-Host", public.Host)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Host, public.Host) {
			if strings.HasPrefix(path.Clean(r.URL.Path), "/conference-request/") {
				if r.URL.Path != "/conference-request/v1" || r.Method != http.MethodPost {
					http.NotFound(w, r)
					return
				}
				key, e := vks.LoadSigningKey(cfg.SigningKeyFile)
				if e != nil {
					w.WriteHeader(503)
					return
				}
				authorization := r.Header.Get("Authorization")
				token, ok := strings.CutPrefix(authorization, "Bearer ")
				if !ok {
					w.WriteHeader(401)
					return
				}
				room, userID, e := vks.VerifyJoinToken(&key.PublicKey, cfg.KeyID, public.Hostname(), token, time.Now())
				if e != nil {
					w.WriteHeader(401)
					return
				}
				meeting, e := (vks.Meetings{Store: s.store}).Get(r.Context(), room)
				if e != nil {
					w.WriteHeader(403)
					return
				}
				user, e := s.store.GetUserByID(r.Context(), userID)
				if e != nil || !s.allowsVKSMeeting(r.Context(), meeting, user) {
					w.WriteHeader(403)
					return
				}
				policy, e := s.vksPolicies().Resolve(r.Context(), vks.Scope{TenantID: user.TenantID, DomainID: user.DomainID})
				if e != nil || !policy.Enabled {
					w.WriteHeader(403)
					return
				}
				raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
				if e != nil {
					w.WriteHeader(400)
					return
				}
				var req struct {
					Room string `json:"room"`
				}
				if json.Unmarshal(raw, &req) != nil || req.Room != room+"@conference."+public.Hostname() {
					w.WriteHeader(403)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(raw))
			}
			proxy.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
