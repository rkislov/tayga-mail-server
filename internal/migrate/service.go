package migrate

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Service runs background migration jobs.
type Service struct {
	cfg      *config.Config
	log      *slog.Logger
	store    storage.Driver
	ms       *mailstore.Store
	db       *sql.DB
	postgres bool
	key      []byte
}

// New builds a migration service. Returns nil if store has no SQL backend.
func New(cfg *config.Config, log *slog.Logger, store storage.Driver, ms *mailstore.Store) (*Service, error) {
	if log == nil {
		log = slog.Default()
	}
	sqlStore, ok := store.(*storage.Store)
	if !ok || sqlStore == nil {
		return nil, nil
	}
	dataDir := "./data"
	if cfg != nil && cfg.Mailstore.Root != "" {
		dataDir = filepath.Dir(cfg.Mailstore.Root)
		if dataDir == "." || dataDir == "" {
			dataDir = "./data"
		}
	}
	secret := ""
	if cfg != nil {
		secret = cfg.Server.SecretsKey
	}
	key, err := ResolveKey(secret, dataDir)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:      cfg,
		log:      log,
		store:    store,
		ms:       ms,
		db:       sqlStore.DB(),
		postgres: cfg != nil && cfg.Storage.Driver == "postgres",
		key:      key,
	}, nil
}

// Start polls for pending jobs until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.db == nil {
		return
	}
	go s.loop(ctx)
	s.log.Info("migration worker started")
}

func (s *Service) loop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.drainOnce(ctx)
		}
	}
}

func (s *Service) drainOnce(ctx context.Context) {
	for {
		job, err := s.claimPending(ctx)
		if err != nil {
			s.log.Warn("migration claim", "err", err)
			return
		}
		if job == nil {
			return
		}
		s.runJob(ctx, job)
	}
}

func (s *Service) runJob(ctx context.Context, job *Job) {
	s.log.Info("migration job started", "id", job.ID, "kind", job.Kind, "user", job.UserID)
	var err error
	switch job.Kind {
	case KindIMAP:
		err = s.runIMAP(ctx, job)
	case KindCalDAV:
		err = s.runCalDAV(ctx, job)
	case KindCardDAV:
		err = s.runCardDAV(ctx, job)
	default:
		err = errKind(job.Kind)
	}
	if s.isCancelled(ctx, job.ID) {
		_ = s.finishJob(ctx, job.ID, StatusCancelled, "")
		return
	}
	if err != nil {
		s.log.Warn("migration job failed", "id", job.ID, "err", err)
		_ = s.finishJob(ctx, job.ID, StatusFailed, err.Error())
		return
	}
	_ = s.finishJob(ctx, job.ID, StatusDone, "")
	s.log.Info("migration job done", "id", job.ID, "copied", job.Copied, "skipped", job.Skipped, "errors", job.Errors)
}

type kindError string

func (e kindError) Error() string { return "unknown migration kind: " + string(e) }

func errKind(k string) error { return kindError(k) }
