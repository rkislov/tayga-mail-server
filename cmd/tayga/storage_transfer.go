package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func runStorageTransfer(args []string) error {
	flags := flag.NewFlagSet("migrate-storage", flag.ContinueOnError)
	source := flags.String("config", "", "Source SQLite config (server must be stopped)")
	target := flags.String("target-config", "", "Private target PostgreSQL config (fresh database)")
	reportPath := flags.String("report", "", "New private JSON verification report")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *source == "" || *target == "" || *reportPath == "" {
		return errors.New("-config, -target-config and -report are required")
	}
	src, err := config.Load(*source)
	if err != nil {
		return errors.New("cannot load source config")
	}
	dst, err := config.Load(*target)
	if err != nil {
		return errors.New("cannot load target config")
	}
	if src.Storage.Driver != "sqlite" || dst.Storage.Driver != "postgres" {
		return errors.New("full transfer currently supports SQLite to PostgreSQL only")
	}
	if src.Mailstore.Root != dst.Mailstore.Root || src.MattermostAttachmentsPath() != dst.MattermostAttachmentsPath() {
		return errors.New("mailstore and Mattermost attachment paths must remain unchanged")
	}
	reportFile, err := os.OpenFile(*reportPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("report must be a new writable private file")
	}
	defer reportFile.Close()
	driver, err := storage.OpenOffline(context.Background(), dst.Storage)
	if err != nil {
		return errors.New("cannot initialize target PostgreSQL; credentials and SQL details suppressed")
	}
	defer driver.Close()
	report, transferErr := storage.TransferSQLiteToPostgres(context.Background(), src.Storage.SQLite.Path, driver.(*storage.Store), func(table storage.TransferTable) {
		fmt.Printf("verified %s -> %s: %d rows\n", table.Source, table.Target, table.Rows)
	})
	if err = json.NewEncoder(reportFile).Encode(report); err != nil {
		return errors.New("cannot save transfer report")
	}
	if err = reportFile.Sync(); err != nil {
		return err
	}
	if transferErr != nil {
		return transferErr
	}
	fmt.Println("All application tables verified and committed. Source SQLite and active configuration are unchanged; switch configuration separately.")
	return nil
}
