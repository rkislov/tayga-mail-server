// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/mmfederation"
	"github.com/tayga/tms/internal/mmsettings"
)

func (s *Server) federationCipher() (cipher.AEAD, error) {
	cfg := s.cfgLive()
	key, e := migrate.ResolveKey(cfg.Server.SecretsKey, filepath.Dir(cfg.Mailstore.Root))
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(append(key, []byte("mattermost-federation-v1")...))
	block, e := aes.NewCipher(sum[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(block)
}
func (s *Server) handleMattermostFederation(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	scope := mmsettings.Scope{TenantID: r.URL.Query().Get("tenant_id"), DomainID: r.URL.Query().Get("domain_id")}
	if !s.canManageMattermostScope(r, admin, scope) {
		writeJSON(w, 403, map[string]string{"error": "scope access denied"})
		return
	}
	if e := s.mattermostSettings().Validate(r.Context(), scope); e != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid scope"})
		return
	}
	if s.mattermostAPI == nil || s.mattermostAPI.Federation() == nil {
		writeJSON(w, 503, map[string]string{"error": "Federation is disabled in server settings"})
		return
	}
	service := s.mattermostAPI.Federation()
	if scope.DomainID == "" || scope.TenantID == "" {
		writeJSON(w, 200, map[string]any{"requires_domain": true, "peers": []any{}})
		return
	}
	base := "/api/v1/admin/mattermost/federation"
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, base), "/")
	parts := []string{}
	if path != "" {
		parts = strings.Split(path, "/")
	}
	failure := func() { writeJSON(w, 400, map[string]string{"error": "federation operation rejected"}) }
	read := func(v any) bool {
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil {
			failure()
			return false
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			failure()
			return false
		}
		return true
	}
	if len(parts) == 0 {
		switch r.Method {
		case "GET":
			peers, e := service.Peers(r.Context(), scope.TenantID, scope.DomainID)
			if e != nil {
				failure()
				return
			}
			writeJSON(w, 200, map[string]any{"peers": peers})
		case "POST":
			var request struct {
				Name        string `json:"name"`
				Mode        string `json:"mode"`
				SiteURL     string `json:"site_url"`
				OwnerID     string `json:"owner_id"`
				AccessToken string `json:"access_token"`
				Password    string `json:"password"`
				InviteCode  string `json:"invite_code"`
			}
			if !read(&request) {
				return
			}
			owner := request.OwnerID
			if owner == "" {
				owner = admin.ID
			}
			p := mmfederation.Peer{TenantID: scope.TenantID, DomainID: scope.DomainID, OwnerID: owner, Name: request.Name, Mode: request.Mode, SiteURL: request.SiteURL}
			var e error
			var invite string
			if p.Mode == "native" && request.InviteCode != "" {
				p, e = service.AcceptInvite(r.Context(), p, request.InviteCode, request.Password)
			} else {
				if p.Mode == "native" && len(request.Password) < 12 {
					failure()
					return
				}
				p, e = service.CreatePeer(r.Context(), p, mmfederation.Credentials{AccessToken: request.AccessToken})
				if e == nil && p.Mode == "native" {
					invite, e = service.Invite(r.Context(), p, request.Password)
				}
			}
			if e != nil {
				failure()
				return
			}
			s.log.Info("federation peer created", "actor_id", admin.ID, "peer_id", p.ID, "tenant_id", p.TenantID, "domain_id", p.DomainID, "mode", p.Mode)
			writeJSON(w, 201, map[string]any{"peer": p, "invite_code": invite})
		default:
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		}
		return
	}
	p, e := service.Peer(r.Context(), parts[0])
	if e != nil || p.TenantID != scope.TenantID || p.DomainID != scope.DomainID {
		writeJSON(w, 404, map[string]string{"error": "peer not found"})
		return
	}
	if len(parts) == 1 && r.Method == "DELETE" {
		if e = service.Disable(r.Context(), p); e != nil {
			failure()
			return
		}
		s.log.Info("federation peer disabled", "actor_id", admin.ID, "peer_id", p.ID, "domain_id", p.DomainID)
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if len(parts) == 2 && parts[1] == "invite" && r.Method == "POST" {
		var request struct {
			Password string `json:"password"`
		}
		if !read(&request) {
			return
		}
		code, e := service.Invite(r.Context(), p, request.Password)
		if e != nil {
			failure()
			return
		}
		writeJSON(w, 200, map[string]string{"invite_code": code})
		return
	}
	if len(parts) == 2 && parts[1] == "links" {
		switch r.Method {
		case "GET":
			links, e := service.Links(r.Context(), p)
			if e != nil {
				failure()
				return
			}
			pending, e := service.Pending(r.Context(), p)
			if e != nil {
				failure()
				return
			}
			owner, e := s.store.GetUserByID(r.Context(), p.OwnerID)
			if e != nil {
				failure()
				return
			}
			channels, e := service.Channels(r.Context(), p, owner)
			if e != nil {
				failure()
				return
			}
			writeJSON(w, 200, map[string]any{"links": links, "pending": pending, "channels": channels})
		case "POST":
			var request struct {
				ChannelID       string `json:"channel_id"`
				RemoteChannelID string `json:"remote_channel_id"`
				ReadOnly        bool   `json:"readonly"`
				Backfill        bool   `json:"backfill"`
			}
			if !read(&request) {
				return
			}
			if request.RemoteChannelID == "" && p.Mode == "native" {
				request.RemoteChannelID = request.ChannelID
			}
			link, e := service.CreateLink(r.Context(), p, mmfederation.Link{ChannelID: request.ChannelID, RemoteChannelID: request.RemoteChannelID, ReadOnly: request.ReadOnly}, request.Backfill)
			if e != nil {
				failure()
				return
			}
			s.log.Info("federation channel enabled", "actor_id", admin.ID, "peer_id", p.ID, "channel_id", link.ChannelID, "remote_channel_id", link.RemoteChannelID, "domain_id", p.DomainID)
			writeJSON(w, 201, link)
		default:
			writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		}
		return
	}
	if len(parts) == 3 && parts[1] == "links" && r.Method == "DELETE" {
		if e = service.DisableLink(r.Context(), p, parts[2]); e != nil {
			failure()
			return
		}
		s.log.Info("federation channel disabled", "actor_id", admin.ID, "peer_id", p.ID, "link_id", parts[2], "domain_id", p.DomainID)
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	http.NotFound(w, r)
}
