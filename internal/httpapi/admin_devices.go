package httpapi

import (
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"strings"
)

func (s *Server) handleAdminDevices(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	list, err := s.store.ListFlowSyncDevices(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot list devices"})
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/devices"), "/")
	w.Header().Set("Cache-Control", "no-store")
	var selected *storage.FlowSyncDevice
	out := []map[string]any{}
	for _, device := range list {
		user, err := s.store.GetUserByID(r.Context(), device.UserID)
		if err != nil || !s.adminCanManageDomain(r, admin, user.DomainID) {
			continue
		}
		if device.ID == id {
			selected = device
		}
		out = append(out, map[string]any{"id": device.ID, "user_id": device.UserID, "email": user.Email, "device_id": device.DeviceID, "type": device.DeviceType, "address": device.Address, "agent": device.Agent, "last_seen": device.UpdatedAt, "blocked": device.Blocked, "wipe_status": device.WipeStatus, "account_wipe_supported": device.Provisioned && device.Version == "16.1"})
	}
	if r.Method == http.MethodGet && id == "" {
		writeJSON(w, 200, map[string]any{"devices": out})
		return
	}
	if r.Method != http.MethodPost || selected == nil {
		writeJSON(w, 404, map[string]string{"error": "device not found"})
		return
	}
	var req struct {
		Action          string `json:"action"`
		ConfirmDeviceID string `json:"confirm_device_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	blocked, wipe := selected.Blocked, selected.WipeStatus
	switch req.Action {
	case "block":
		blocked = true
	case "unblock":
		if wipe == "pending" || wipe == "sent" || wipe == "acknowledged" {
			writeJSON(w, 409, map[string]string{"error": "wipe must be resolved before unblocking"})
			return
		}
		blocked = false
	case "wipe_account":
		if !selected.Provisioned || selected.Version != "16.1" {
			writeJSON(w, 409, map[string]string{"error": "account-only wipe requires provisioned protocol 16.1 client"})
			return
		}
		if req.ConfirmDeviceID != selected.DeviceID {
			writeJSON(w, 400, map[string]string{"error": "confirm device ID required"})
			return
		}
		if wipe == "pending" || wipe == "sent" || wipe == "acknowledged" {
			writeJSON(w, 409, map[string]string{"error": "wipe already requested"})
			return
		}
		blocked = true
		wipe = "pending"
	case "cancel_wipe":
		if wipe != "pending" {
			writeJSON(w, 409, map[string]string{"error": "only an unsent wipe can be cancelled"})
			return
		}
		wipe = "cancelled"
	default:
		writeJSON(w, 400, map[string]string{"error": "unknown action"})
		return
	}
	if err = s.store.SetFlowSyncDeviceControl(r.Context(), selected.ID, blocked, wipe); err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot update device"})
		return
	}
	writeJSON(w, 200, map[string]any{"blocked": blocked, "wipe_status": wipe})
}
