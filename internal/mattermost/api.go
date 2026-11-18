// Package mattermost provides an experimental, independently implemented API v4
// compatibility adapter. It does not embed the Mattermost server or web client.
package mattermost

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmplugins"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

// CompatibilityVersion identifies the wire contract, not a full server release.
const CompatibilityVersion = "10.11.0"

type API struct {
	startChannelCall    ChannelCallStarter
	canStartChannelCall func(context.Context, *storage.User) (bool, error)
	fileRequests        chan struct{}
	fileUploads         chan struct{}
	pushSignMu          sync.Mutex
	pushSigningKey      *ecdsa.PrivateKey
	pushCancel          context.CancelFunc
	pushDone            chan struct{}
	pushClient          *http.Client
	pushLifecycle       sync.Mutex
	preferencesMu       sync.Mutex
	pushTestMu          sync.Mutex
	pushTests           map[string]time.Time
	policy              *mmsettings.Service
	pluginScopeMu       sync.Mutex
	pluginScopes        map[string]scopedRuntime
	pluginDirectory     string
	scopeAuthorize      func(*http.Request, *storage.User, mmsettings.Scope) bool
	plugins             *mmplugins.Runtime
	pluginAdmins        []string
	sidebarMu           sync.Mutex
	store               storage.Driver
	auth                *auth.Layer
	chat                *chat.Service
	log                 *slog.Logger
	siteURL             string
}

func New(store storage.Driver, layer *auth.Layer, log *slog.Logger, siteURL string) *API {
	if log == nil {
		log = slog.Default()
	}
	return &API{fileRequests: make(chan struct{}, 4), fileUploads: make(chan struct{}, 2), store: store, auth: layer, chat: chat.New(store), log: log, siteURL: siteURL, policy: &mmsettings.Service{Store: store, Defaults: mmsettings.Defaults{Enabled: true, SiteName: "Tayga Chat"}}, pushTests: map[string]time.Time{}, pluginScopes: map[string]scopedRuntime{}}
}
func (s *API) SetAttachmentDirectory(dir string) error { return s.chat.SetAttachmentDirectory(dir) }

func (s *API) RepairAttachmentImageDimensions(ctx context.Context) {
	const key = "chat.attachment_display_dimensions.v1"
	if !s.chat.AttachmentsEnabled() {
		return
	}
	_, done, err := s.store.GetSetting(ctx, key)
	if err == nil && !done {
		if err = s.chat.RepairAttachmentImageDimensions(ctx); err == nil {
			err = s.store.PutSetting(ctx, key, "complete")
		}
	}
	if err != nil && ctx.Err() == nil {
		s.log.Error("repair chat attachment image dimensions", "error", err)
	}
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, id, message string) {
	write(w, status, map[string]any{"id": id, "message": message, "detailed_error": "", "request_id": "", "status_code": status})
}
func (s *API) failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		fail(w, 404, "api.context.invalid_param.app_error", "Resource not found")
	case errors.Is(err, chat.ErrForbidden):
		fail(w, 403, "api.context.permissions.app_error", "Permission denied")
	case errors.Is(err, chat.ErrInvalid):
		fail(w, 400, "api.context.invalid_param.app_error", "Invalid request")
	default:
		s.log.Error("mattermost API", "error", err)
		fail(w, 500, "api.internal.app_error", "Internal server error")
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	err := d.Decode(v)
	if err == nil {
		var extra any
		if d.Decode(&extra) != io.EOF {
			err = chat.ErrInvalid
		}
	}
	if err != nil {
		fail(w, 400, "api.context.invalid_body_param.app_error", "Invalid JSON body")
		return false
	}
	return true
}
func bearer(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
func (s *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Version-ID", CompatibilityVersion+".tms-prototype")
	w.Header().Set("Cache-Control", "no-store")
	path := strings.TrimPrefix(r.URL.Path, "/api/v4")
	if path == "/system/ping" && r.Method == http.MethodGet {
		write(w, 200, map[string]string{"status": "OK", "AndroidLatestVersion": "", "AndroidMinVersion": "", "IosLatestVersion": "", "IosMinVersion": "", "CanReceiveNotifications": s.canReceivePush(r)})
		return
	}
	if path == "/config/client" && r.Method == http.MethodGet {
		diagnosticID, err := s.chat.ServerID(r.Context())
		if err != nil {
			s.failure(w, err)
			return
		}
		scope := mmsettings.Scope{}
		if token := bearer(r); token != "" {
			if u, e := s.authenticate(r.Context(), token); e == nil {
				scope = userScope(u)
			}
		}
		resolved, e := s.policy.Resolve(r.Context(), scope)
		if e != nil {
			s.failure(w, e)
			return
		}
		signingPublicKey, e := s.pushPublicKey(r.Context())
		if e != nil {
			s.failure(w, e)
			return
		}
		featurePlugins := s.plugins != nil && resolved.PluginsEnabled
		write(w, 200, map[string]string{
			"AsymmetricSigningPublicKey": signingPublicKey,
			"DiagnosticId":               diagnosticID, "Version": CompatibilityVersion, "BuildNumber": "tms-prototype", "SiteName": resolved.SiteName, "SiteURL": s.siteURL,
			"EnableSignInWithEmail": "true", "EnableSignInWithUsername": "false", "EnableUserCreation": "false", "EnableOpenServer": "false",
			"EnableSignUpWithEmail": "false", "EnableEmailInvitations": "false", "EnableFileAttachments": strconv.FormatBool(resolved.Enabled && s.chat.AttachmentsEnabled()), "MaxFileSize": strconv.Itoa(chat.MaxAttachmentBytes), "EnableCustomEmoji": "false",
			"EnableCommands": strconv.FormatBool(featurePlugins || s.startChannelCall != nil), "EnablePlugins": strconv.FormatBool(featurePlugins), "EnableIncomingWebhooks": strconv.FormatBool(resolved.PluginsEnabled), "EnableOutgoingWebhooks": "false", "EnableOAuthServiceProvider": "false",
			"FeatureFlagAppsEnabled": strconv.FormatBool(resolved.Enabled && s.startChannelCall != nil),
			"SendPushNotifications":  s.canReceivePush(r), "PushNotificationContents": "generic_no_channel",
			"EnableMultifactorAuthentication": "true", "EnforceMultifactorAuthentication": "false", "EnableSaml": "false", "EnableLdap": "false",
			"CollapsedThreads": "disabled", "FeatureFlagCollapsedThreads": "false", "EnableMobileFileUpload": strconv.FormatBool(resolved.Enabled && s.chat.AttachmentsEnabled()), "EnableMobileFileDownload": strconv.FormatBool(resolved.Enabled),
			"TeammateNameDisplay": "nickname_full_name", "EnableUserStatuses": "false", "EnableCustomUserStatuses": "false", "EnableLinkPreviews": "false", "EnableTutorial": "false",
			"EnableChannelCategorySorting": "true", "EnableManagedChannelCategories": "false", "EnableChannelViewedMessages": "false", "EnableChannelHeaderMarkdown": "false", "EnablePostSearch": "false",
		})
		return
	}
	if path == "/license/client" && r.Method == http.MethodGet {
		write(w, 200, map[string]string{"IsLicensed": "false"})
		return
	}
	if path == "/notifications/ack" && r.Method == http.MethodPost {
		s.pushReceiptHTTP(w, r)
		return
	}
	if path == "/users/mfa" && r.Method == http.MethodPost {
		s.mfaRequired(w, r)
		return
	}
	if path == "/users/login" && r.Method == http.MethodPost {
		s.login(w, r)
		return
	}
	if path == "/websocket" && r.Method == http.MethodGet {
		s.websocket(w, r)
		return
	}
	token := bearer(r)
	u, err := s.authenticate(r.Context(), token)
	if err != nil {
		s.log.Warn("mattermost authentication rejected", "stage", "session", "method", r.Method, "path", r.URL.Path, "user_agent", r.UserAgent())
		fail(w, 401, "api.context.session_expired.app_error", "Invalid or expired session")
		return
	}
	resolved, e := s.policy.Resolve(r.Context(), userScope(u))
	if e != nil {
		s.failure(w, e)
		return
	}
	if !resolved.Enabled {
		fail(w, 403, "tms.mattermost.scope_disabled", "Mattermost is disabled for this tenant or domain")
		return
	}
	s.auth.TrackToken(u, token, r.RemoteAddr, r.UserAgent())
	if err = s.chat.EnsureDefault(r.Context(), u); err != nil {
		s.failure(w, err)
		return
	}
	if path == "/users/logout" && r.Method == http.MethodPost {
		if err = s.auth.Tokens.RevokeAccess(r.Context(), token); err != nil {
			s.failure(w, err)
			return
		}
		write(w, 200, map[string]string{"status": "OK"})
		return
	}
	if path == "/users/sessions/revoke" {
		s.revokeSessionHTTP(w, r, u)
		return
	}
	if s.pushHTTP(w, r, u, path) {
		return
	}
	switch {
	case path == "/users" || strings.HasPrefix(path, "/users/"):
		s.users(w, r, u, path)
	case path == "/teams" || strings.HasPrefix(path, "/teams/"):
		s.teams(w, r, u, path)
	case path == "/channels" || strings.HasPrefix(path, "/channels/"):
		s.channels(w, r, u, path)
	case path == "/files" || strings.HasPrefix(path, "/files/"):
		s.files(w, r, u, path)
	case path == "/posts" || strings.HasPrefix(path, "/posts/"):
		s.posts(w, r, u, path)
	case path == "/plugins" || strings.HasPrefix(path, "/plugins/"):
		s.pluginAPI(w, r, u, path)
	case path == "/commands" || strings.HasPrefix(path, "/commands/"):
		s.commands(w, r, u, path)
	case path == "/hooks/incoming" || strings.HasPrefix(path, "/hooks/incoming/"):
		s.incomingHooks(w, r, u, path)
	case path == "/roles/names" && r.Method == http.MethodPost:
		s.roles(w, r)
	case path == "/system/timezones" && r.Method == http.MethodGet:
		write(w, 200, []string{"UTC", "Europe/Moscow"})
	default:
		s.unsupported(w, r)
	}
}
func (s *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		LoginID  string `json:"login_id"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if !decode(w, r, &req) {
		s.log.Warn("mattermost login rejected", "stage", "request_format", "user_agent", r.UserAgent())
		return
	}
	res, err := s.auth.LoginWithPassword(r.Context(), req.LoginID, req.Password)
	if err != nil {
		s.log.Warn("mattermost login rejected", "stage", "credentials", "login_present", req.LoginID != "", "password_present", req.Password != "", "user_agent", r.UserAgent())
		fail(w, 401, "api.user.login.invalid_credentials_email_username", "Invalid credentials")
		return
	}
	pair := res.Tokens
	if res.MFARequired {
		if req.Token == "" {
			s.log.Warn("mattermost login rejected", "stage", "mfa_required", "user_agent", r.UserAgent())
			fail(w, 401, "mfa.validate_token.authenticate.app_error", "MFA token required")
			return
		}
		pair, err = s.auth.CompleteMFA(r.Context(), res.Challenge, req.Token)
		if err != nil {
			fail(w, 401, "mfa.validate_token.authenticate.app_error", "Invalid MFA token")
			return
		}
	}
	u, err := s.authenticate(r.Context(), pair.AccessToken)
	if err != nil {
		s.failure(w, err)
		return
	}
	if err = s.chat.EnsureDefault(r.Context(), u); err != nil {
		_ = s.auth.Tokens.RevokeAccess(r.Context(), pair.AccessToken)
		s.failure(w, err)
		return
	}
	resolved, e := s.policy.Resolve(r.Context(), userScope(u))
	if e != nil || !resolved.Enabled {
		_ = s.auth.Tokens.RevokeAccess(r.Context(), pair.AccessToken)
		if e != nil {
			s.failure(w, e)
		} else {
			fail(w, 403, "tms.mattermost.scope_disabled", "Mattermost is disabled for this tenant or domain")
		}
		return
	}
	s.log.Info("mattermost login accepted", "user_agent", r.UserAgent())
	w.Header().Set("Token", pair.AccessToken)
	if req.DeviceID != "" && pushDeviceID.MatchString(req.DeviceID) {
		_ = s.registerPushDevice(r.Context(), u, pair.AccessToken, req.DeviceID, false)
	}
	model := userModel(u, true)
	if mode, e := s.notificationMode(r.Context(), u); e == nil {
		model["notify_props"].(map[string]string)["push"] = mode
	}
	write(w, 200, model)
}
func userModel(u *storage.User, self bool) map[string]any {
	username := strings.TrimSpace(u.LocalPart)
	if local, _, ok := strings.Cut(strings.TrimSpace(u.Email), "@"); ok && local != "" && u.AuthSource != "bot" {
		username = local
	}
	m := map[string]any{"id": chat.ID(u.ID), "create_at": u.CreatedAt.UnixMilli(), "update_at": u.CreatedAt.UnixMilli(), "delete_at": int64(0),
		"username": username, "auth_service": "", "nickname": u.DisplayLabel(), "first_name": strings.TrimSpace(u.DisplayName), "last_name": "", "position": "", "roles": "system_user", "locale": "ru", "timezone": map[string]string{"useAutomaticTimezone": "true", "automaticTimezone": "Europe/Moscow", "manualTimezone": ""}, "notify_props": map[string]string{"desktop": "mention", "push": "mention", "email": "false", "mark_unread": "all", "mention_keys": ""}, "props": map[string]string{}, "is_bot": u.AuthSource == "bot", "last_picture_update": 0}
	if u.AuthSource == "bot" {
		m["username"] = u.LocalPart
	}
	if self {
		m["email"] = u.Email
		m["email_verified"] = true
	}
	return m
}
func (s *API) teamModel(r *http.Request, u *storage.User) (map[string]any, error) {
	t, err := s.store.GetTenantByID(r.Context(), u.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": chat.ID(t.ID), "name": "tms-" + chat.ID(t.ID), "display_name": t.Name, "description": "", "type": "I", "create_at": t.CreatedAt.UnixMilli(), "update_at": t.CreatedAt.UnixMilli(), "delete_at": 0, "allow_open_invite": false, "scheme_id": "", "group_constrained": false}, nil
}
func channelModel(c chat.Channel) map[string]any {
	team := chat.ID(c.TenantID)
	if c.Kind == "D" {
		team = ""
	}
	return map[string]any{"id": c.ID, "team_id": team, "name": c.Name, "display_name": c.DisplayName, "type": c.Kind, "creator_id": chat.ID(c.CreatorID), "create_at": c.CreatedAt, "update_at": c.CreatedAt, "delete_at": 0, "last_post_at": c.LastPostAt, "total_msg_count": c.TotalMsgCount, "total_msg_count_root": c.RootMsgCount, "header": "", "purpose": "", "scheme_id": "", "group_constrained": false, "props": map[string]any{}}
}
func postModel(p chat.Post) map[string]any {
	ids := make([]string, 0, len(p.Attachments))
	for _, f := range p.Attachments {
		ids = append(ids, f.ID)
	}
	metadata := map[string]any{}
	if len(ids) > 0 {
		metadata["files"] = fileModels(p.Attachments)
	}
	return map[string]any{"id": p.ID, "channel_id": p.ChannelID, "user_id": chat.ID(p.UserID), "root_id": p.RootID, "message": p.Message, "pending_post_id": p.PendingID, "create_at": p.CreatedAt, "update_at": p.CreatedAt, "edit_at": 0, "delete_at": 0, "is_pinned": false, "type": p.Type, "props": p.Props, "hashtags": "", "file_ids": ids, "metadata": metadata, "reply_count": p.ReplyCount}
}
func memberModel(channelID string, m chat.Member) map[string]any {
	return map[string]any{"channel_id": channelID, "user_id": chat.ID(m.UserID), "roles": "channel_user", "last_viewed_at": m.LastViewedAt, "msg_count": m.ReadCount, "msg_count_root": m.ReadCount, "mention_count": 0, "mention_count_root": 0, "notify_props": map[string]string{"desktop": "default", "push": "default", "email": "default", "mark_unread": "all"}, "scheme_user": true, "scheme_admin": false, "scheme_guest": false}
}
func page(r *http.Request) (limit, offset int, err error) {
	limit = 60
	var p int
	if q := r.URL.Query().Get("per_page"); q != "" {
		limit, err = strconv.Atoi(q)
		if err != nil {
			return 0, 0, chat.ErrInvalid
		}
	}
	if q := r.URL.Query().Get("page"); q != "" {
		p, err = strconv.Atoi(q)
		if err != nil {
			return 0, 0, chat.ErrInvalid
		}
	}
	if limit < 1 || limit > 200 || p < 0 || p > 1000000 {
		return 0, 0, chat.ErrInvalid
	}
	return limit, p * limit, nil
}
func (s *API) unsupported(w http.ResponseWriter, r *http.Request) {
	s.log.Warn("mattermost unsupported API", "method", r.Method, "path", r.URL.Path, "user_agent", r.UserAgent())
	fail(w, 501, "tms.mattermost.unsupported", "This operation is not implemented by the TMS compatibility prototype")
}
func wrongMethod(w http.ResponseWriter) {
	fail(w, 405, "api.context.method_not_allowed.app_error", "Method not allowed")
}
func (s *API) roles(w http.ResponseWriter, r *http.Request) {
	var names []string
	if !decode(w, r, &names) {
		return
	}
	out := []any{}
	permissions := map[string][]string{"system_user": {"create_direct_channel", "view_members"}, "team_user": {"view_team", "list_team_channels", "create_public_channel", "create_private_channel"}, "channel_user": {"read_channel", "create_post"}}
	for _, name := range names {
		if p, ok := permissions[name]; ok {
			out = append(out, map[string]any{"id": chat.ID("role:" + name), "name": name, "display_name": name, "permissions": p, "scheme_managed": true, "built_in": true})
		}
	}
	write(w, 200, out)
}
