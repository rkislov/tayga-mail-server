package mattermost

import (
	"context"
	"github.com/google/uuid"
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var pushDeviceID = regexp.MustCompile(`^(apple_rn|apple_rnbeta|android_rn|android_rnbeta)(-v2)?:[a-zA-Z0-9_-]{16,512}$`)

func pushDeviceKey(access string) string { return "mattermost.push.device:" + chat.ID(access) }
func (s *API) registerPushDevice(ctx context.Context, u *storage.User, access, device string, disabled bool) error {
	if device != "" && !pushDeviceID.MatchString(device) {
		return chat.ErrInvalid
	}
	session, e := s.store.GetOAuthTokenByAccess(ctx, access)
	if e != nil {
		return e
	}
	native, e := s.chat.Session(ctx, session.ID)
	if e != nil {
		return e
	}
	if session.UserID != u.ID || native.ExpiresAt <= time.Now().UnixMilli() {
		return chat.ErrForbidden
	}
	if device == "" {
		return s.chat.RemovePush(ctx, session.ID)
	}
	return s.chat.RegisterPush(ctx, chat.PushDevice{SessionID: session.ID, UserID: u.ID, TenantID: u.TenantID, DomainID: u.DomainID, DeviceID: device, Disabled: disabled, ExpiresAt: native.ExpiresAt})
}

func (s *API) pushHTTP(w http.ResponseWriter, r *http.Request, u *storage.User, path string) bool {
	if path != "/users/sessions/device" && path != "/notifications/test" {
		return false
	}
	access := bearer(r)
	if path == "/users/sessions/device" {
		if r.Method != http.MethodPut {
			wrongMethod(w)
			return true
		}
		var req struct {
			DeviceID *string `json:"device_id"`
			Disabled *string `json:"device_notification_disabled"`
		}
		if !decode(w, r, &req) {
			return true
		}
		if req.Disabled != nil && *req.Disabled != "true" && *req.Disabled != "false" {
			s.failure(w, chat.ErrInvalid)
			return true
		}
		device, disabled := "", false
		if session, e := s.store.GetOAuthTokenByAccess(r.Context(), access); e == nil {
			if current, e := s.chat.PushDevice(r.Context(), session.ID); e == nil {
				device = current.DeviceID
				disabled = current.Disabled
			}
		}
		if req.DeviceID != nil {
			device = *req.DeviceID
			disabled = false
		}
		if req.Disabled != nil {
			disabled = *req.Disabled == "true"
		}
		if e := s.registerPushDevice(r.Context(), u, access, device, disabled); e != nil {
			s.failure(w, e)
			return true
		}
		write(w, 200, map[string]string{"status": "OK"})
		return true
	}
	if r.Method != http.MethodPost {
		wrongMethod(w)
		return true
	}
	policy, e := s.policy.Resolve(r.Context(), userScope(u))
	if e != nil || !policy.Enabled || !policy.Push.Prepared {
		fail(w, 503, "tms.push.not_prepared", "Test push gateway is not configured")
		return true
	}
	session, e := s.store.GetOAuthTokenByAccess(r.Context(), access)
	if e != nil {
		s.failure(w, e)
		return true
	}
	device, e := s.chat.PushDevice(r.Context(), session.ID)
	if e != nil || device.UserID != u.ID || device.TenantID != u.TenantID || device.DomainID != u.DomainID || device.Disabled || device.ExpiresAt <= time.Now().UnixMilli() {
		fail(w, 409, "tms.push.device_missing", "No enabled mobile device for this session")
		return true
	}

	// Bound explicit tests to one request per minute per session.
	s.pushTestMu.Lock()
	for key, stamp := range s.pushTests {
		if time.Since(stamp) >= time.Minute {
			delete(s.pushTests, key)
		}
	}
	last := s.pushTests[pushDeviceKey(access)]
	if time.Since(last) < time.Minute {
		s.pushTestMu.Unlock()
		fail(w, 429, "tms.push.test_rate_limit", "Wait one minute before another test")
		return true
	}
	s.pushTests[pushDeviceKey(access)] = time.Now()
	s.pushTestMu.Unlock()
	serverID, e := s.chat.ServerID(r.Context())
	if e != nil {
		s.failure(w, e)
		return true
	}
	var options struct {
		Visible bool `json:"visible"`
	}
	if r.ContentLength != 0 && !decode(w, r, &options) {
		return true
	}
	platform, id, _ := strings.Cut(device.DeviceID, ":")
	kind := "test"
	if options.Visible {
		kind = "message"
	}
	ackID := chat.ID("test:" + uuid.NewString())
	payload := map[string]any{"platform": platform, "device_id": id, "server_id": serverID, "type": kind, "message": "Тестовое уведомление TMS", "version": "v2", "cont_ava": 1, "is_id_loaded": false, "badge": -1}
	if e = s.chat.PushReceipt(r.Context(), ackID, device.SessionID, ""); e != nil {
		s.failure(w, e)
		return true
	}
	if e = s.signPushPayload(r.Context(), payload, ackID, id); e != nil {
		s.failure(w, e)
		return true
	}
	result, _, _ := s.sendPushPayload(r.Context(), policy.Push.ProxyURL, payload)
	if result != "accepted" {
		fail(w, 502, "tms.push.test_failed", "Push gateway did not accept the test")
		return true
	}
	s.log.Info("push gateway accepted test notification", "ack_id", ackID, "visible", options.Visible)
	write(w, 200, map[string]any{"status": "OK", "ack_id": ackID, "gateway_accepted": true, "visible": options.Visible})
	return true
}
