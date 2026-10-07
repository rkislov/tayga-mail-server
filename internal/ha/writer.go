package ha

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tayga/tms/internal/storage"
)

// WriterGate decides whether this process may mutate a user's maildir.
type WriterGate interface {
	AllowWrite(ctx context.Context, userID string) error
}

// ClusterWriters gates all users behind the global HA leader lease.
type ClusterWriters struct {
	Gate Gate
}

func (c ClusterWriters) AllowWrite(_ context.Context, _ string) error {
	if c.Gate != nil && !c.Gate.IsLeader() {
		return ErrStandby
	}
	return nil
}

// Sticky assigns each user to one node via Postgres/SQLite lease rows.
type Sticky struct {
	store  storage.Driver
	nodeID string
	ttl    time.Duration
	log    *slog.Logger
}

// StickyConfig configures per-user writer leases.
type StickyConfig struct {
	Store  storage.Driver
	NodeID string // empty → hostname-uuid
	TTL    time.Duration
	Log    *slog.Logger
}

func NewSticky(cfg StickyConfig) *Sticky {
	nodeID := cfg.NodeID
	if nodeID == "" {
		host, _ := os.Hostname()
		if host == "" {
			host = "node"
		}
		nodeID = host + "-" + uuid.NewString()[:8]
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}
	return &Sticky{store: cfg.Store, nodeID: nodeID, ttl: ttl, log: log}
}

func (s *Sticky) NodeID() string {
	if s == nil {
		return ""
	}
	return s.nodeID
}

func (s *Sticky) AllowWrite(ctx context.Context, userID string) error {
	if s == nil || s.store == nil || userID == "" {
		return nil
	}
	ok, holder, err := s.store.TryAcquireUserWriter(ctx, userID, s.nodeID, s.ttl)
	if err != nil {
		return fmt.Errorf("sticky lease: %w", err)
	}
	if ok {
		return nil
	}
	s.log.Debug("sticky writer held by peer", "user", userID, "holder", holder, "node", s.nodeID)
	return fmt.Errorf("%w (held by %s)", ErrStandby, holder)
}
