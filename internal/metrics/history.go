package metrics

import (
	"context"
	"github.com/tayga/tms/internal/storage"
	"log/slog"
	"runtime"
	"time"
)

// RunHistory samples independently of browsers and persists 25 hours of observations.
func RunHistory(ctx context.Context, store storage.Driver, log *slog.Logger) {
	boot := time.Now().UTC().Format(time.RFC3339Nano)
	collect := func() {
		sampleCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		if err := collectHistory(sampleCtx, store, boot, time.Now()); err != nil && ctx.Err() == nil {
			log.Warn("metric history sampling failed", "error", err)
		}
	}
	collect()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}
func collectHistory(ctx context.Context, store storage.Driver, boot string, now time.Time) error {
	st, err := store.ServerStats(ctx)
	if err != nil {
		return err
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	values := map[string]float64{"messages": float64(st.Messages), "bytes": float64(st.BytesStored), "queue": float64(st.OutboundQueued), "goroutines": float64(runtime.NumGoroutine()), "heap": float64(mem.HeapAlloc)}
	protection := ProtectionSnapshot()
	for name, value := range protection["decisions"].(map[string]uint64) {
		values["protection."+name] = float64(value)
	}
	values["newsletters"] = float64(protection["newsletters"].(uint64))
	network := NetworkSnapshot()
	if interfaces, ok := network["interfaces"].([]map[string]any); ok {
		for _, iface := range interfaces {
			for _, dir := range []string{"rx_bytes", "tx_bytes"} {
				values["network."+iface["name"].(string)+"."+dir] = float64(iface[dir].(uint64))
			}
		}
	}
	sample := storage.MetricSample{At: now.UnixMilli(), Boot: boot, Values: values}
	if err = store.SaveMetricSample(ctx, "server", sample); err != nil {
		return err
	}
	tenants, err := store.ListTenants(ctx)
	if err != nil {
		return err
	}
	for _, tenant := range tenants {
		st, err := store.TenantStats(ctx, tenant.ID)
		if err != nil {
			return err
		}
		sample.Values = map[string]float64{"messages": float64(st.Messages), "bytes": float64(st.BytesStored)}
		if err = store.SaveMetricSample(ctx, "tenant:"+tenant.ID, sample); err != nil {
			return err
		}
	}
	return store.PruneMetricHistory(ctx, now.Add(-25*time.Hour).UnixMilli())
}
