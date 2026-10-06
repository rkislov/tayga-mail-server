package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	imapserver "github.com/tayga/tms/internal/imap"
	"github.com/tayga/tms/internal/logging"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/pop3"
	"github.com/tayga/tms/internal/seed"
	"github.com/tayga/tms/internal/smtp"
	"github.com/tayga/tms/internal/storage"
)

func main() {
	cfgPath := flag.String("config", "configs/tayga.example.yaml", "path to YAML config")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "tayga: %v\n", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	log := logging.New(cfg.Log.Level, cfg.Log.Format)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Storage.Driver == "sqlite" {
		if err := os.MkdirAll(filepath.Dir(cfg.Storage.SQLite.Path), 0o750); err != nil {
			return fmt.Errorf("create data dir: %w", err)
		}
	}
	if err := os.MkdirAll(cfg.Mailstore.Root, 0o750); err != nil {
		return fmt.Errorf("create mailstore: %w", err)
	}

	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()

	authn := auth.NewLayer(store)
	ms := mailstore.New(cfg.Mailstore.Root)

	if err := seed.Run(ctx, cfg.Seed, store, authn.Hasher, ms, log); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	smtpSrv := smtp.New(cfg, log, store, authn, ms)
	if err := smtpSrv.Start(ctx); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}

	imapSrv := imapserver.New(cfg, log, store, authn, ms)
	if err := imapSrv.Start(ctx); err != nil {
		return fmt.Errorf("imap: %w", err)
	}

	pop3Srv := pop3.New(cfg, log, store, authn, ms)
	if err := pop3Srv.Start(ctx); err != nil {
		return fmt.Errorf("pop3: %w", err)
	}

	httpSrv := httpapi.New(cfg.HTTP.Listen, log, store)
	if err := httpSrv.Start(ctx); err != nil {
		return fmt.Errorf("http: %w", err)
	}

	log.Info("tayga mail server started",
		"hostname", cfg.Server.Hostname,
		"storage", cfg.Storage.Driver,
	)

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = pop3Srv.Shutdown(shutdownCtx)
	_ = imapSrv.Shutdown(shutdownCtx)
	_ = smtpSrv.Shutdown(shutdownCtx)
	return nil
}
