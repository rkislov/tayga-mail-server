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
	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/httpapi"
	imapserver "github.com/tayga/tms/internal/imap"
	"github.com/tayga/tms/internal/logging"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/managesieve"
	"github.com/tayga/tms/internal/metrics"
	"github.com/tayga/tms/internal/pop3"
	"github.com/tayga/tms/internal/seed"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/smtp"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "backup":
			if err := runBackup(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "tayga backup: %v\n", err)
				os.Exit(1)
			}
			return
		case "restore":
			if err := runRestore(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "tayga restore: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	cfgPath := flag.String("config", "configs/tayga.example.yaml", "path to YAML config")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "tayga: %v\n", err)
		os.Exit(1)
	}
}

func runBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	cfgPath := fs.String("config", "configs/tayga.example.yaml", "path to YAML config")
	out := fs.String("out", "", "output .tar.gz path (required)")
	tenant := fs.String("tenant", "", "limit to tenant UUID (default: all)")
	includeMail := fs.Bool("include-mail", true, "include maildir files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("-out is required")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	opts := backup.Options{
		TenantID:    *tenant,
		IncludeMail: *includeMail,
		MailRoot:    cfg.Mailstore.Root,
	}
	if err := backup.WriteTarGz(ctx, store, opts, f); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", *out)
	return nil
}

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	cfgPath := fs.String("config", "configs/tayga.example.yaml", "path to YAML config")
	in := fs.String("in", "", "input .tar.gz path (required)")
	includeMail := fs.Bool("include-mail", true, "restore maildir files and reindex")
	skipExisting := fs.Bool("skip-existing", false, "do not update existing users")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("-in is required")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()

	f, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer f.Close()
	rep, err := backup.RestoreTarGz(ctx, store, f, backup.RestoreOptions{
		MailRoot:     cfg.Mailstore.Root,
		IncludeMail:  *includeMail,
		SkipExisting: *skipExisting,
	})
	if err != nil {
		return err
	}
	fmt.Printf("restore ok: tenants=%d domains=%d users=%d skipped=%d scripts=%d mail_files=%d\n",
		rep.TenantsCreated, rep.DomainsCreated, rep.UsersCreated, rep.UsersSkipped, rep.ScriptsRestored, rep.MailFiles)
	return nil
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

	metrics.Register(store)

	authn := auth.NewLayer(store, cfg.LDAP, cfg.MFA, cfg.OIDC)
	ms := mailstore.New(cfg.Mailstore.Root)
	if cfg.Mailstore.ObjectStore.Enabled {
		blob, err := mailstore.NewS3Blob(mailstore.S3Config{
			Endpoint:  cfg.Mailstore.ObjectStore.Endpoint,
			Region:    cfg.Mailstore.ObjectStore.Region,
			Bucket:    cfg.Mailstore.ObjectStore.Bucket,
			AccessKey: cfg.Mailstore.ObjectStore.AccessKey,
			SecretKey: cfg.Mailstore.ObjectStore.SecretKey,
			Prefix:    cfg.Mailstore.ObjectStore.Prefix,
			PathStyle: cfg.Mailstore.ObjectStore.PathStyle,
		})
		if err != nil {
			return fmt.Errorf("object store: %w", err)
		}
		ms.SetObjectStore(blob, log)
		log.Info("mailstore object store enabled",
			"endpoint", cfg.Mailstore.ObjectStore.Endpoint,
			"bucket", cfg.Mailstore.ObjectStore.Bucket,
		)
	}
	tlsMgr, err := tlsutil.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("tls: %w", err)
	}
	imapHub := imapserver.NewHub(store)
	sieveEng := sieve.New(store, ms, log)
	sieveEng.Notifier = imapHub

	if cfg.Storage.Driver == "postgres" {
		if sqlStore, ok := store.(*storage.Store); ok {
			bus := imapserver.NewPGBus(imapHub, sqlStore.DB(), cfg.Storage.Postgres.DSN, log)
			imapHub.SetCluster(bus)
			if err := bus.Start(ctx); err != nil {
				return fmt.Errorf("imap idle cluster: %w", err)
			}
		}
	}

	if err := seed.Run(ctx, cfg.Seed, store, authn.Hasher, ms, log); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	smtpSrv := smtp.New(cfg, log, store, authn, ms, sieveEng, tlsMgr)
	if err := smtpSrv.Start(ctx); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}

	imapSrv := imapserver.New(cfg, log, store, authn, ms, imapHub, tlsMgr)
	if err := imapSrv.Start(ctx); err != nil {
		return fmt.Errorf("imap: %w", err)
	}

	pop3Srv := pop3.New(cfg, log, store, authn, ms, tlsMgr)
	if err := pop3Srv.Start(ctx); err != nil {
		return fmt.Errorf("pop3: %w", err)
	}

	msieveSrv := managesieve.New(cfg, log, store, authn, tlsMgr)
	if err := msieveSrv.Start(ctx); err != nil {
		return fmt.Errorf("managesieve: %w", err)
	}

	httpSrv := httpapi.New(cfg, log, store, authn, ms, tlsMgr)
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
	_ = msieveSrv.Shutdown(shutdownCtx)
	_ = pop3Srv.Shutdown(shutdownCtx)
	_ = imapSrv.Shutdown(shutdownCtx)
	_ = smtpSrv.Shutdown(shutdownCtx)
	return nil
}
