package mattermost

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

// External bot credentials are specific to the Mattermost adapter. Only their
// hashes are persisted. They never authorize mail, administration or OAuth.
type externalBot struct {
	ID          string `json:"user_id"`
	UserID      string `json:"internal_user_id"`
	OwnerID     string `json:"owner_id"`
	TenantID    string `json:"tenant_id"`
	DomainID    string `json:"domain_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Hash        string `json:"token_hash"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"create_at"`
}

var externalBotName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

const externalBotKey = "mattermost.external_bot:"

func (b externalBot) public() map[string]any {
	return map[string]any{"user_id": b.ID, "owner_id": chat.ID(b.OwnerID), "username": b.Username, "display_name": b.DisplayName, "description": b.Description, "enabled": b.Enabled, "create_at": b.CreatedAt}
}
func (s *API) readExternalBot(ctx context.Context, id string) (externalBot, error) {
	var b externalBot
	raw, ok, err := s.store.GetSetting(ctx, externalBotKey+id)
	if err != nil {
		return b, err
	}
	if !ok || raw == "" {
		return b, storage.ErrNotFound
	}
	err = json.Unmarshal([]byte(raw), &b)
	return b, err
}
func (s *API) saveExternalBot(ctx context.Context, b externalBot) error {
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return s.store.PutSetting(ctx, externalBotKey+b.ID, string(raw))
}
func (b *externalBot) credential() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	token := "tmsbot_" + b.ID + "." + hex.EncodeToString(secret)
	hash := sha256.Sum256([]byte(token))
	b.Hash = hex.EncodeToString(hash[:])
	return token, nil
}
func (s *API) authenticateBot(ctx context.Context, token string) (*storage.User, error) {
	id, secret, ok := strings.Cut(strings.TrimPrefix(token, "tmsbot_"), ".")
	if !ok || len(id) != 26 || len(secret) != 64 {
		return nil, auth.ErrInvalidToken
	}
	b, err := s.readExternalBot(ctx, id)
	if err != nil || !b.Enabled {
		return nil, auth.ErrInvalidToken
	}
	hash := sha256.Sum256([]byte(token))
	stored, err := hex.DecodeString(b.Hash)
	if err != nil || subtle.ConstantTimeCompare(stored, hash[:]) != 1 {
		return nil, auth.ErrInvalidToken
	}
	owner, err := s.store.GetUserByID(ctx, b.OwnerID)
	if err != nil || !owner.Enabled {
		return nil, auth.ErrInvalidToken
	}
	u, err := s.store.GetUserByID(ctx, b.UserID)
	if err != nil || !u.Enabled || u.AuthSource != "bot" || u.TenantID != b.TenantID || u.DomainID != b.DomainID {
		return nil, auth.ErrInvalidToken
	}
	return u, nil
}
func (s *API) externalBots(w http.ResponseWriter, r *http.Request, u *storage.User, path string) {
	scope, err := s.scopeFromRequest(r, u)
	if err != nil {
		s.failure(w, err)
		return
	}
	if u.AuthSource == "bot" || !s.canManageScope(r, u, scope) {
		s.failure(w, chat.ErrForbidden)
		return
	}
	if scope.TenantID == "" || scope.DomainID == "" {
		fail(w, 400, "tms.bots.domain_required", "Select a domain for this bot")
		return
	}
	s.botMu.Lock()
	defer s.botMu.Unlock()
	if path == "/bots" {
		switch r.Method {
		case http.MethodGet:
			all, e := s.store.ListSettings(r.Context())
			if e != nil {
				s.failure(w, e)
				return
			}
			bots := []externalBot{}
			for k, raw := range all {
				if !strings.HasPrefix(k, externalBotKey) {
					continue
				}
				var b externalBot
				if json.Unmarshal([]byte(raw), &b) == nil && b.TenantID == scope.TenantID && b.DomainID == scope.DomainID {
					bots = append(bots, b)
				}
			}
			sort.Slice(bots, func(i, j int) bool { return bots[i].Username < bots[j].Username })
			out := []any{}
			for _, b := range bots {
				out = append(out, b.public())
			}
			write(w, 200, out)
		case http.MethodPost:
			var req struct {
				Username    string `json:"username"`
				DisplayName string `json:"display_name"`
				Description string `json:"description"`
			}
			if !decode(w, r, &req) {
				return
			}
			req.Username = strings.TrimSpace(req.Username)
			req.DisplayName = strings.TrimSpace(req.DisplayName)
			if !externalBotName.MatchString(req.Username) || req.DisplayName == "" || len(req.DisplayName) > 128 || len(req.Description) > 1024 {
				s.failure(w, chat.ErrInvalid)
				return
			}
			domain, e := s.store.GetDomainByID(r.Context(), scope.DomainID)
			if e != nil {
				s.failure(w, e)
				return
			}
			bot, e := s.store.CreateUser(r.Context(), &storage.User{TenantID: scope.TenantID, DomainID: scope.DomainID, LocalPart: req.Username, Email: req.Username + "@" + domain.Name, DisplayName: req.DisplayName, AuthSource: "bot", Roles: "bot", Enabled: true})
			if e != nil {
				s.failure(w, e)
				return
			}
			b := externalBot{ID: chat.ID(bot.ID), UserID: bot.ID, OwnerID: u.ID, TenantID: scope.TenantID, DomainID: scope.DomainID, Username: req.Username, DisplayName: req.DisplayName, Description: req.Description, Enabled: true, CreatedAt: time.Now().UnixMilli()}
			token, e := b.credential()
			if e == nil {
				e = s.saveExternalBot(r.Context(), b)
			}
			if e != nil {
				_ = s.store.UpdateUserEnabled(r.Context(), bot.ID, false)
				s.failure(w, e)
				return
			}
			result := b.public()
			result["token"] = token
			w.Header().Set("Cache-Control", "no-store")
			write(w, 201, result)
		default:
			wrongMethod(w)
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/bots/"), "/")
	if len(parts) != 2 || r.Method != http.MethodPost {
		s.unsupported(w, r)
		return
	}
	b, e := s.readExternalBot(r.Context(), parts[0])
	if e != nil {
		s.failure(w, e)
		return
	}
	if b.TenantID != scope.TenantID || b.DomainID != scope.DomainID {
		s.failure(w, chat.ErrForbidden)
		return
	}
	result := b.public()
	switch parts[1] {
	case "token":
		token, e := b.credential()
		if e != nil {
			s.failure(w, e)
			return
		}
		result["token"] = token
	case "disable":
		b.Enabled = false
	case "enable":
		b.Enabled = true
	default:
		s.unsupported(w, r)
		return
	}
	if e = s.saveExternalBot(r.Context(), b); e != nil {
		s.failure(w, e)
		return
	}
	result["enabled"] = b.Enabled
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, result)
}

// Bots can read their directory/channels and exchange messages/files, but cannot
// mint webhook credentials, change membership or invoke administrative plugins.
func botRequestAllowed(method, path string) bool {
	if method == http.MethodGet {
		for _, prefix := range []string{"/users/", "/teams/", "/channels/", "/posts/", "/files/"} {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		return path == "/users" || path == "/teams" || path == "/commands" || path == "/commands/autocomplete"
	}
	if method == http.MethodPost {
		return path == "/commands" || path == "/posts" || path == "/files" || path == "/users/ids" || path == "/users/search" || path == "/users/status/ids" || path == "/channels/direct" || path == "/channels/members/me/view"
	}
	return method == http.MethodPut && path == "/users/me/status"
}
