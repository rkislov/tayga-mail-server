package httpapi

import (
	"context"
	"github.com/tayga/tms/internal/scan"
	"github.com/tayga/tms/internal/spam"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"time"
)

func (s *Server) handleProtectionStats(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	st, ok := s.store.(*storage.Store)
	if !ok {
		w.WriteHeader(503)
		return
	}
	var domains []string
	if !s.isGlobalAdminUser(admin) {
		domains = []string{}
		list, err := s.store.ListDomainsByTenant(r.Context(), admin.TenantID)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		for _, d := range list {
			if s.adminCanManageDomain(r, admin, d.ID) {
				domains = append(domains, d.ID)
			}
		}
	}
	now := time.Now()
	since := now.Add(-24*time.Hour).Unix() / 3600 * 3600
	switch r.URL.Query().Get("period") {
	case "hour":
		since = now.Unix() / 3600 * 3600
	case "30d":
		since = now.Add(-30*24*time.Hour).Unix() / 3600 * 3600
	}
	stats, err := st.ProtectionStatistics(r.Context(), domains, since)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "statistics unavailable"})
		return
	}
	stats["since"] = since
	stats["observed_at"] = now.UTC().Format(time.RFC3339)
	c := s.cfgLive()
	stats["learning_enabled"] = spam.LearningEnabled(c.Spam)
	stats["antivirus_enabled"] = s.cfg.Scan.Enabled
	stats["antivirus_backend"] = s.cfg.Scan.Backend
	if s.isGlobalAdminUser(admin) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		if c.Spam.Enabled && (c.Spam.Backend == "" || c.Spam.Backend == "rspamd") {
			client, e := spam.Controller(c.Spam)
			if e == nil {
				v, e := client.Statistics(ctx)
				if e == nil {
					stats["rspamd"] = v
				} else {
					stats["rspamd_error"] = e.Error()
				}
			} else {
				stats["rspamd_error"] = e.Error()
			}
		}
		if s.cfg.Scan.Enabled && (s.cfg.Scan.Backend == "clamav" || s.cfg.Scan.Backend == "") {
			addr := s.cfg.Scan.ClamAV.Address
			if addr == "" {
				addr = "127.0.0.1:3310"
			}
			v, e := scan.NewClamAV(addr, 3*time.Second).Version(ctx)
			if e != nil {
				stats["antivirus_error"] = e.Error()
			} else {
				stats["antivirus_version"] = v
			}
		}
	}
	writeJSON(w, 200, stats)
}
