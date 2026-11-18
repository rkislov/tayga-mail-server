package mattermost

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *API) migrateStagedPush(ctx context.Context) error {
	settings, e := s.store.ListSettings(ctx)
	if e != nil {
		return e
	}
	staged := map[string]string{}
	for key, value := range settings {
		if strings.HasPrefix(key, "mattermost.push.device:") {
			staged[key] = value
		}
	}
	if len(staged) == 0 {
		return nil
	}
	sessions, e := s.chat.LegacyPushSessions(ctx)
	if e != nil {
		return e
	}
	restored := 0
	for key, raw := range staged {
		d, ok := sessions[key]
		if !ok {
			continue
		}
		var old struct {
			UserID   string `json:"user_id"`
			TenantID string `json:"tenant_id"`
			DomainID string `json:"domain_id"`
			DeviceID string `json:"device_id"`
			Disabled bool   `json:"disabled"`
		}
		if json.Unmarshal([]byte(raw), &old) != nil || old.UserID != d.UserID || old.TenantID != d.TenantID || old.DomainID != d.DomainID || !pushDeviceID.MatchString(old.DeviceID) {
			continue
		}
		d.DeviceID, d.Disabled = old.DeviceID, old.Disabled
		if e = s.chat.RegisterPush(ctx, d); e != nil {
			return e
		}
		restored++
	}
	if e = s.chat.ClearLegacyPush(ctx); e != nil {
		return e
	}
	s.log.Info("migrated staged push device registrations", "count", restored)
	return nil
}
