package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func runSyncCompaction(args []string) error {
	flags := flag.NewFlagSet("compact-sync-state", flag.ContinueOnError)
	cfgPath := flags.String("config", "", "Tayga configuration; server must be stopped")
	reportPath := flags.String("report", "", "New private JSON verification report")
	vacuum := flags.Bool("vacuum", false, "Reclaim physical space after compaction (exclusive offline operation)")
	apply := flags.Bool("apply", false, "Apply lossless compaction; default is storage statistics only")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" || *reportPath == "" {
		return errors.New("-config and -report are required")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return errors.New("cannot load configuration")
	}
	output, err := os.OpenFile(*reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer output.Close()
	driver, err := storage.OpenOffline(context.Background(), cfg.Storage)
	if err != nil {
		return errors.New("cannot open storage exclusively; stop Tayga first and inspect connection settings")
	}
	defer driver.Close()
	store := driver.(*storage.Store)
	var report storage.SyncCompactionReport
	if *apply {
		report, err = store.CompactSyncStates(context.Background(), func(n int64) { fmt.Printf("verified %d synchronization states\n", n) })
	} else {
		err = store.DB().QueryRow(`SELECT COUNT(*),COALESCE(SUM(LENGTH(state)),0) FROM flowsync_states`).Scan(&report.Examined, &report.TemplateBytes)
	}
	if err == nil && *apply && *vacuum {
		statement := "VACUUM"
		if cfg.Storage.Driver == "postgres" {
			statement = "VACUUM (FULL, ANALYZE) flowsync_states, flowsync_state_blobs"
		}
		_, err = store.DB().Exec(statement)
	}
	writeErr := json.NewEncoder(output).Encode(report)
	if writeErr != nil {
		return writeErr
	}
	if e := output.Sync(); e != nil {
		return e
	}
	if err != nil {
		return err
	}
	fmt.Printf("examined=%d compacted=%d logical_bytes=%d templates=%d snapshots=%d snapshot_bytes=%d verified=%t\n", report.Examined, report.Compacted, report.LogicalBytes, report.TemplateBytes, report.Snapshots, report.SnapshotBytes, report.Verified)
	return nil
}
func runSyncStateMaintenance(ctx context.Context, store *storage.Store, days int, log *slog.Logger) {
	timer := time.NewTimer(10 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			job, cancel := context.WithTimeout(ctx, 30*time.Second)
			expired, orphans, err := store.CleanupSyncStates(job, time.Duration(days)*24*time.Hour)
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Warn("sync state maintenance failed")
			} else if expired > 0 || orphans > 0 {
				log.Info("sync state maintenance", "expired_ews_contexts", expired, "orphan_snapshots", orphans)
			}
			timer.Reset(6 * time.Hour)
		}
	}
}
