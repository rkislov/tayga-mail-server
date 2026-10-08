package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/backup"
	"github.com/tayga/tms/internal/climenu"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/ha"
	"github.com/tayga/tms/internal/httpapi"
	imapserver "github.com/tayga/tms/internal/imap"
	"github.com/tayga/tms/internal/logging"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/managesieve"
	"github.com/tayga/tms/internal/metrics"
	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/pop3"
	"github.com/tayga/tms/internal/seed"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/siem"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/smtp"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
	"github.com/tayga/tms/internal/version"
	"github.com/tayga/tms/internal/xmpp"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-version", "--version":
			fmt.Println(version.String())
			return
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
		case "sync-objects":
			if err := runSyncObjects(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "tayga sync-objects: %v\n", err)
				os.Exit(1)
			}
			return
		case "ldap-sync":
			if err := runLDAPSync(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "tayga ldap-sync: %v\n", err)
				os.Exit(1)
			}
			return
		case "menu", "setup", "console":
			cfgPath := "configs/tayga.example.yaml"
			if len(os.Args) > 2 {
				fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
				p := fs.String("config", cfgPath, "path to YAML config")
				_ = fs.Parse(os.Args[2:])
				cfgPath = *p
			}
			if err := climenu.Run(cfgPath); err != nil {
				fmt.Fprintf(os.Stderr, "tayga menu: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	cfgPath := flag.String("config", "configs/tayga.example.yaml", "path to YAML config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}

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

func runSyncObjects(args []string) error {
	fs := flag.NewFlagSet("sync-objects", flag.ExitOnError)
	cfgPath := fs.String("config", "configs/tayga.example.yaml", "path to YAML config")
	dryRun := fs.Bool("dry-run", false, "report actions without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if !cfg.Mailstore.ObjectStore.Enabled {
		return fmt.Errorf("mailstore.object_store.enabled must be true")
	}
	log, closeLog, err := logging.New(cfg.Log.Level, cfg.Log.Format, cfg.Log.File)
	if err != nil {
		return err
	}
	defer closeLog()
	ctx := context.Background()
	store, err := storage.Open(ctx, cfg.Storage)
	if err != nil {
		return err
	}
	defer store.Close()
	ms := mailstore.New(cfg.Mailstore.Root)
	blob, err := openObjectStore(cfg)
	if err != nil {
		return err
	}
	ms.SetObjectStore(blob, log)
	rep, err := mailstore.SyncObjects(ctx, store, ms, blob, mailstore.SyncOptions{DryRun: *dryRun, Log: log})
	if err != nil {
		return err
	}
	fmt.Printf("sync-objects ok: messages=%d pushed=%d pulled=%d unchanged=%d missing=%d errors=%d dry_run=%v\n",
		rep.Messages, rep.Pushed, rep.Pulled, rep.Unchanged, rep.Missing, rep.Errors, *dryRun)
	return nil
}

func runLDAPSync(args []string) error {
	fs := flag.NewFlagSet("ldap-sync", flag.ExitOnError)
	cfgPath := fs.String("config", "configs/tayga.example.yaml", "path to YAML config")
	domain := fs.String("domain", "", "limit to one mail domain (default: all enabled)")
	if err := fs.Parse(args); err != nil {
		return err
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
	dir := auth.NewDirectory(store, cfg.LDAP)
	if *domain != "" {
		rep, err := dir.SyncDomain(ctx, *domain)
		if rep != nil {
			fmt.Printf("ldap-sync %s: created=%d updated=%d admins=%d errors=%d skipped=%d\n",
				rep.Domain, rep.Created, rep.Updated, rep.Admins, rep.Errors, rep.Skipped)
		}
		return err
	}
	reps, err := dir.SyncAll(ctx)
	for _, rep := range reps {
		fmt.Printf("ldap-sync %s: created=%d updated=%d admins=%d errors=%d skipped=%d\n",
			rep.Domain, rep.Created, rep.Updated, rep.Admins, rep.Errors, rep.Skipped)
	}
	return err
}

func openObjectStore(cfg *config.Config) (mailstore.Blob, error) {
	o := cfg.Mailstore.ObjectStore
	return mailstore.NewS3Blob(mailstore.S3Config{
		Endpoint:  o.Endpoint,
		Region:    o.Region,
		Bucket:    o.Bucket,
		AccessKey: o.AccessKey,
		SecretKey: o.SecretKey,
		Prefix:    o.Prefix,
		PathStyle: o.PathStyle,
	})
}

func runObjectSyncLoop(ctx context.Context, store storage.Driver, ms *mailstore.Store, blob mailstore.Blob, interval time.Duration, log *slog.Logger, gate ha.Gate, fenceWriters bool) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if fenceWriters && gate != nil && !gate.IsLeader() {
				continue
			}
			rep, err := mailstore.SyncObjects(ctx, store, ms, blob, mailstore.SyncOptions{})
			if err != nil {
				log.Warn("object store sync failed", "err", err)
				continue
			}
			log.Info("object store sync",
				"messages", rep.Messages,
				"pushed", rep.Pushed,
				"pulled", rep.Pulled,
				"unchanged", rep.Unchanged,
				"missing", rep.Missing,
				"errors", rep.Errors,
			)
		}
	}
}

func run(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

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

	hub := settings.NewHub(store, cfg)
	if err := hub.Load(ctx); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	cfg = hub.Config()

	log, closeLog, err := logging.New(cfg.Log.Level, cfg.Log.Format, cfg.Log.File)
	if err != nil {
		return fmt.Errorf("log: %w", err)
	}
	defer closeLog()
	log.Info("file logging enabled", "file", cfg.Log.File, "level", cfg.Log.Level, "format", cfg.Log.Format)

	siemExp, err := siem.New(siem.Config{
		Enabled:       cfg.SIEM.Enabled,
		Protocol:      cfg.SIEM.Protocol,
		Address:       cfg.SIEM.Address,
		Facility:      cfg.SIEM.Facility,
		Format:        cfg.SIEM.Format,
		Vendor:        cfg.SIEM.Vendor,
		Product:       cfg.SIEM.Product,
		Version:       firstNonEmpty(cfg.SIEM.Version, version.Version),
		Hostname:      firstNonEmpty(cfg.SIEM.Hostname, cfg.Server.Hostname),
		TLSSkipVerify: cfg.SIEM.TLSSkipVerify,
		QueueSize:     cfg.SIEM.QueueSize,
	}, log)
	if err != nil {
		return fmt.Errorf("siem: %w", err)
	}
	if siemExp != nil {
		defer siemExp.Close()
		log.Info("siem syslog CEF enabled", "protocol", cfg.SIEM.Protocol, "address", cfg.SIEM.Address)
	}

	metrics.Register(store)

	authn := auth.NewLayer(store, cfg.LDAP, cfg.MFA, cfg.OIDC)
	if dir, ok := authn.LDAP.(*auth.Directory); ok {
		dir.StartGroupSyncLoop(ctx, log)
	}
	ms := mailstore.New(cfg.Mailstore.Root)
	var objBlob mailstore.Blob
	var objSyncInterval time.Duration
	if cfg.Mailstore.ObjectStore.Enabled {
		blob, err := openObjectStore(cfg)
		if err != nil {
			return fmt.Errorf("object store: %w", err)
		}
		ms.SetObjectStore(blob, log)
		objBlob = blob
		objSyncInterval = cfg.Mailstore.ObjectStore.SyncInterval
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
	notifyHub := notify.NewHub()
	sieveEng := sieve.New(store, ms, log)
	sieveEng.Notifier = notify.Fanout{
		Idle: imapHub,
		Hub:  notifyHub,
		ResolveUserID: func(email string) string {
			u, err := store.GetUserByEmail(context.Background(), email)
			if err != nil || u == nil {
				return ""
			}
			return u.ID
		},
	}
	(&notify.ReminderLoop{Hub: notifyHub, Store: store, Log: log}).Start(ctx)

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

	var gate ha.Gate = ha.AlwaysLeader{}
	var writers ha.WriterGate
	switch cfg.HA.Mode {
	case "active_standby":
		lease, err := ha.NewPGLease(ctx, ha.LeaseConfig{
			DSN: cfg.Storage.Postgres.DSN,
			TTL: cfg.HA.LeaseTTL,
			Log: log,
		})
		if err != nil {
			return fmt.Errorf("ha lease: %w", err)
		}
		lease.Start(ctx)
		gate = lease
		if cfg.HA.FenceWriters() {
			writers = ha.ClusterWriters{Gate: lease}
		}
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					metrics.SetHALeader(lease.IsLeader())
				}
			}
		}()
		log.Info("ha active_standby enabled",
			"lease_ttl", cfg.HA.LeaseTTL.String(),
			"fence", cfg.HA.Fence,
			"is_leader", lease.IsLeader(),
		)
	case "sticky":
		st := ha.NewSticky(ha.StickyConfig{
			Store: store, NodeID: cfg.HA.NodeID, TTL: cfg.HA.StickyTTL, Log: log,
		})
		writers = st
		metrics.SetHALeader(true) // sticky has no single cluster leader
		log.Info("ha sticky writers enabled", "node_id", st.NodeID(), "sticky_ttl", cfg.HA.StickyTTL.String())
	}

	if objBlob != nil && objSyncInterval > 0 {
		go runObjectSyncLoop(ctx, store, ms, objBlob, objSyncInterval, log, gate, cfg.HA.FenceWriters())
		log.Info("mailstore object store sync ticker", "interval", objSyncInterval.String())
	}

	smtpSrv := smtp.New(cfg, log, store, authn, ms, sieveEng, tlsMgr, gate, writers)
	smtpSrv.SetSIEM(siemExp)
	if err := smtpSrv.Start(ctx); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}

	imapSrv := imapserver.New(cfg, log, store, authn, ms, imapHub, tlsMgr, writers)
	if err := imapSrv.Start(ctx); err != nil {
		return fmt.Errorf("imap: %w", err)
	}

	pop3Srv := pop3.New(cfg, log, store, authn, ms, tlsMgr, writers)
	if err := pop3Srv.Start(ctx); err != nil {
		return fmt.Errorf("pop3: %w", err)
	}

	msieveSrv := managesieve.New(cfg, log, store, authn, tlsMgr)
	if err := msieveSrv.Start(ctx); err != nil {
		return fmt.Errorf("managesieve: %w", err)
	}

	xmppSrv := xmpp.New(cfg, log, store, authn, tlsMgr)
	if err := xmppSrv.Start(ctx); err != nil {
		return fmt.Errorf("xmpp: %w", err)
	}

	migSvc, err := migrate.New(cfg, log, store, ms)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if migSvc != nil {
		migSvc.Start(ctx)
	}

	httpSrv := httpapi.New(cfg, log, store, authn, ms, tlsMgr, hub)
	httpSrv.SetXMPP(xmppSrv)
	httpSrv.SetMigrate(migSvc)
	httpSrv.SetNotify(notifyHub)
	httpSrv.SetSIEM(siemExp)
	if err := httpSrv.Start(ctx); err != nil {
		return fmt.Errorf("http: %w", err)
	}

	log.Info("tayga mail server started",
		"version", version.Version,
		"commit", version.Commit,
		"hostname", cfg.Server.Hostname,
		"storage", cfg.Storage.Driver,
	)

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = xmppSrv.Shutdown(shutdownCtx)
	_ = msieveSrv.Shutdown(shutdownCtx)
	_ = pop3Srv.Shutdown(shutdownCtx)
	_ = imapSrv.Shutdown(shutdownCtx)
	_ = smtpSrv.Shutdown(shutdownCtx)
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
