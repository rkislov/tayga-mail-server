package httpapi

import (
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"time"
)

func (s *Server) handleAdminMetricHistory(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	period := r.URL.Query().Get("period")
	duration := time.Hour
	if period == "day" {
		duration = 24 * time.Hour
	} else if period != "" && period != "hour" {
		writeJSON(w, 400, map[string]string{"error": "period must be hour or day"})
		return
	}
	now := time.Now()
	since := now.Add(-duration)
	tenant, err := s.store.MetricHistory(r.Context(), "tenant:"+admin.TenantID, since.UnixMilli())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "history unavailable"})
		return
	}
	server := []storage.MetricSample{}
	global := s.isGlobalAdminUser(admin)
	if global {
		server, err = s.store.MetricHistory(r.Context(), "server", since.UnixMilli())
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "history unavailable"})
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"from": since.UnixMilli(), "to": now.UnixMilli(), "interval_sec": 60, "server": server, "tenant": tenant, "global": global})
}
