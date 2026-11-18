package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/mmsettings"
	"github.com/tayga/tms/internal/storage"
)

// Anonymous discovery advertises server capability only. Authenticated discovery
// and every delivery retain the recipient's effective tenant/domain policy.
func (s *API) canReceivePush(r *http.Request) string {
	ctx := r.Context()
	if r.Header.Get("Authorization") != "" {
		u, err := s.authenticate(ctx, bearer(r))
		if err != nil {
			return "false"
		}
		policy, err := s.policy.Resolve(ctx, userScope(u))
		if err == nil && policy.Push.Enabled {
			return "true"
		}
		return "false"
	}
	policy, err := s.policy.Resolve(ctx, mmsettings.Scope{})
	if err != nil {
		return "false"
	}
	if policy.Push.Enabled {
		return "true"
	}
	settings, err := s.store.ListSettings(ctx)
	if err != nil {
		return "false"
	}
	for key := range settings {
		if !strings.HasPrefix(key, "mattermost.policy:") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(key, "mattermost.policy:"), ":")
		scope := mmsettings.Scope{}
		switch {
		case len(parts) == 2 && parts[0] == "tenant" && parts[1] != "":
			scope.TenantID = parts[1]
		case len(parts) == 3 && parts[0] == "domain" && parts[1] != "" && parts[2] != "":
			scope.TenantID, scope.DomainID = parts[1], parts[2]
		default:
			continue
		}
		candidate, err := s.policy.Resolve(ctx, scope)
		if err == nil && candidate.Push.Enabled {
			return "true"
		}
	}
	return "false"
}
func (s *API) StartPush(parent context.Context) {
	s.pushLifecycle.Lock()
	defer s.pushLifecycle.Unlock()
	if s.pushCancel != nil {
		return
	}
	if e := s.migrateStagedPush(parent); e != nil {
		s.log.Error("staged push device migration failed")
	}
	ctx, cancel := context.WithCancel(parent)
	s.pushCancel = cancel
	s.pushDone = make(chan struct{})
	done := s.pushDone
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		cleanup := time.NewTicker(time.Hour)
		defer cleanup.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if e := s.deliverPushBatch(ctx, now.UnixMilli()); e != nil && ctx.Err() == nil {
					s.log.Error("push queue processing failed")
				}
			case now := <-cleanup.C:
				if err := s.chat.CleanupAttachments(ctx, now.UnixMilli()); err != nil && ctx.Err() == nil {
					s.log.Error("chat attachment cleanup failed")
				}
				if e := s.chat.CleanupPush(ctx, now.UnixMilli()); e != nil {
					s.log.Error("push queue cleanup failed")
				}
			}
		}
	}()
}
func (s *API) stopPush() {
	s.pushLifecycle.Lock()
	cancel, done := s.pushCancel, s.pushDone
	s.pushLifecycle.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
func (s *API) notificationMode(ctx context.Context, u *storage.User) (string, error) {
	raw, exists, e := s.store.GetSetting(ctx, "mattermost.notify.push:"+u.ID)
	if e != nil {
		return "", e
	}
	if !exists {
		return "mention", nil
	}
	if raw != "all" && raw != "mention" && raw != "none" {
		return "none", nil
	}
	return raw, nil
}
func mentionsPush(body string, u *storage.User) bool {
	tokens := strings.FieldsFunc(strings.ToLower(body), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("@_.-", r)
	})
	local, _, _ := strings.Cut(u.Email, "@")
	if local == "" {
		local = u.LocalPart
	}
	for _, word := range tokens {
		if word == "@"+strings.ToLower(local) || word == "@channel" || word == "@all" || word == "@here" {
			return true
		}
	}
	return false
}
func (s *API) deliverPushBatch(ctx context.Context, now int64) error {
	jobs, e := s.chat.DuePush(ctx, now)
	if e != nil {
		return e
	}
	for _, job := range jobs {
		claimed, e := s.chat.ClaimPush(ctx, job, now)
		if e != nil {
			return e
		}
		if !claimed {
			continue
		}
		result, transient, remove := s.deliverPushJob(ctx, job, now)
		state, next := "sent", int64(0)
		if result != "accepted" {
			state = "skipped"
		}
		if transient {
			state = "pending"
			next = now + int64(30*time.Second/time.Millisecond)*(1<<min(job.Attempts, 5))
			if job.Attempts >= 7 {
				state = "failed"
			}
		}
		if remove {
			_ = s.chat.RemovePush(ctx, job.SessionID)
		}
		if e = s.chat.FinishPush(ctx, job, state, result, next); e != nil {
			return e
		}
		if result == "accepted" {
			s.log.Info("push gateway accepted notification")
		} else if state == "failed" {
			s.log.Warn("push delivery exhausted retries", "reason", result)
		}
	}
	return nil
}
func (s *API) deliverPushJob(ctx context.Context, job chat.PushJob, now int64) (string, bool, bool) {
	d, e := s.chat.PushDevice(ctx, job.SessionID)
	if e != nil {
		if errors.Is(e, chat.ErrForbidden) {
			return "session_missing", false, false
		}
		return "session_lookup", true, false
	}
	if d.Disabled || d.ExpiresAt <= now {
		return "device_disabled_or_expired", false, false
	}
	u, e := s.store.GetUserByID(ctx, d.UserID)
	if e != nil {
		return "user_lookup", true, false
	}
	if !u.Enabled || u.TenantID != d.TenantID || u.DomainID != d.DomainID {
		return "scope_changed", false, true
	}
	policy, e := s.policy.Resolve(ctx, userScope(u))
	if e != nil {
		return "policy_lookup", true, false
	}
	if !policy.Push.Enabled {
		return "policy_disabled", false, false
	}
	p, e := s.chat.Post(ctx, u, job.PostID)
	if e != nil {
		if errors.Is(e, storage.ErrNotFound) || errors.Is(e, chat.ErrForbidden) {
			return "membership_missing", false, false
		}
		return "post_lookup", true, false
	}
	if p.UserID == u.ID {
		return "own_post", false, false
	}
	unread, e := s.chat.PushUnread(ctx, u.ID, p.ChannelID, p.CreatedAt)
	if e != nil {
		return "read_state_lookup", true, false
	}
	if !unread {
		return "already_read", false, false
	}
	c, e := s.chat.Channel(ctx, u, p.ChannelID)
	if e != nil {
		return "membership_missing", false, false
	}
	mode, e := s.notificationMode(ctx, u)
	if e != nil {
		return "preferences_lookup", true, false
	}
	if mode == "none" || (mode == "mention" && c.Kind != "D" && !mentionsPush(p.Message, u)) {
		return "not_requested", false, false
	}
	serverID, e := s.chat.ServerID(ctx)
	if e != nil {
		return "server_identity", true, false
	}
	badge, e := s.chat.Unread(ctx, u)
	if e != nil {
		return "badge_lookup", true, false
	}
	// Construct the external payload from identifiers only. No post body, sender
	// name, channel name, address, access token or rich-post properties are copied.
	platform, id, ok := strings.Cut(d.DeviceID, ":")
	if !ok || !pushDeviceID.MatchString(d.DeviceID) {
		return "unsupported_device", false, true
	}
	payload := map[string]any{"platform": platform, "device_id": id, "server_id": serverID, "type": "message", "message": "Новое сообщение", "version": "v2", "cont_ava": 1, "is_id_loaded": false, "channel_id": p.ChannelID, "post_id": p.ID, "team_id": chat.ID(u.TenantID), "root_id": p.RootID, "badge": badge}
	ackID := chat.ID("push:" + job.PostID + ":" + job.SessionID)
	if e = s.chat.PushReceipt(ctx, ackID, d.SessionID, p.ID); e != nil {
		return "receipt_storage", true, false
	}
	if e = s.signPushPayload(ctx, payload, ackID, id); e != nil {
		return "signing", true, false
	}
	return s.sendPushPayload(ctx, policy.Push.ProxyURL, payload)
}
func (s *API) sendPushPayload(ctx context.Context, endpoint string, payload map[string]any) (string, bool, bool) {
	raw, e := json.Marshal(payload)
	if e != nil {
		return "encoding", false, false
	}
	req, e := http.NewRequestWithContext(ctx, "POST", endpoint+"/api/v1/send_push", bytes.NewReader(raw))
	if e != nil {
		return "gateway_url", false, false
	}
	req.Header.Set("Content-Type", "application/json")
	client := s.pushClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, e := client.Do(req)
	if e != nil {
		return "gateway_connection", true, false
	}
	defer res.Body.Close()
	if res.StatusCode == 429 || res.StatusCode >= 500 {
		return fmt.Sprintf("gateway_%d", res.StatusCode), true, false
	}
	if res.StatusCode != 200 {
		return fmt.Sprintf("gateway_%d", res.StatusCode), false, false
	}
	var reply struct {
		Status string `json:"status"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&reply); e != nil {
		return "gateway_invalid_response", true, false
	}
	switch reply.Status {
	case "OK":
		return "accepted", false, false
	case "REMOVE":
		return "device_invalid", false, true
	default:
		return "gateway_rejected", true, false
	}
}
