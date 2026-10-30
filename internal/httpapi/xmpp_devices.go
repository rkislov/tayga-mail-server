package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/storage"
)

// These credentials grant native XMPP access only. The account comes from the
// bearer session; neither an administrator role nor a request field can select
// another user's credentials here.
func (s *Server) handleXMPPDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/me/xmpp-devices")
	fail := func(err error) {
		status, code := 500, "xmpp_device_operation_failed"
		switch {
		case errors.Is(err, storage.ErrNotFound):
			status, code = 404, "device_not_found"
		case errors.Is(err, storage.ErrXMPPDeviceLimit):
			status, code = 409, "device_limit"
		case errors.Is(err, storage.ErrXMPPDeviceNotTested):
			status, code = 409, "device_connection_required"
		}
		writeJSON(w, status, map[string]string{"error": code})
	}
	decode := func(v any) bool {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		dec.DisallowUnknownFields()
		if dec.Decode(v) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid_request"})
			return false
		}
		return true
	}
	drop := func(id string) {
		if gateway, ok := s.xmpp.(interface{ DropDeviceStreams(string, string) }); ok {
			gateway.DropDeviceStreams(u.ID, id)
		}
	}
	switch {
	case path == "" && r.Method == http.MethodGet:
		devices, err := s.store.ListXMPPDevicePasswords(r.Context(), u.ID)
		if err != nil {
			fail(err)
			return
		}
		only, err := s.store.XMPPDevicesOnly(r.Context(), u.ID)
		if err != nil {
			fail(err)
			return
		}
		sessions := []auth.SessionInfo{}
		for _, session := range s.authn.Sessions() {
			if session.UserID == u.ID && session.Protocol == "XMPP" {
				sessions = append(sessions, session)
			}
		}
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].Started.After(sessions[j].Started) })
		writeJSON(w, 200, map[string]any{"devices": devices, "sessions": sessions, "devices_only": only})
	case path == "" && r.Method == http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if !decode(&req) {
			return
		}
		device, password, err := s.authn.CreateXMPPDevicePassword(r.Context(), u.ID, req.Name)
		if errors.Is(err, auth.ErrInvalidDeviceName) {
			writeJSON(w, 400, map[string]string{"error": "invalid_device_name"})
			return
		}
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, 201, map[string]any{"device": device, "password": password})
	case path == "/policy" && r.Method == http.MethodPut:
		var req struct {
			Only *bool `json:"devices_only"`
		}
		if !decode(&req) {
			return
		}
		if req.Only == nil {
			writeJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if err := s.authn.SetXMPPDevicesOnly(r.Context(), u.ID, *req.Only); err != nil {
			fail(err)
			return
		}
		if *req.Only {
			drop("")
		}
		writeJSON(w, 200, map[string]bool{"devices_only": *req.Only})
	case strings.HasPrefix(path, "/sessions/") && r.Method == http.MethodDelete:
		if err := s.authn.CloseOwnXMPPSession(u.ID, strings.TrimPrefix(path, "/sessions/")); err != nil {
			fail(err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "disconnected"})
	case strings.HasPrefix(path, "/") && len(strings.Split(path, "/")) == 2 && path != "/policy" && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/")
		if err := s.authn.RevokeXMPPDevicePassword(r.Context(), u.ID, id); err != nil {
			fail(err)
			return
		}
		drop(id)
		writeJSON(w, 200, map[string]string{"status": "revoked"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
	}
}
