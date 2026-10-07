package ha

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

// Gate reports whether this process should accept MX (unauthenticated) mail.
type Gate interface {
	IsLeader() bool
}

// AlwaysLeader is used when HA fencing is disabled.
type AlwaysLeader struct{}

func (AlwaysLeader) IsLeader() bool { return true }

// LeaseConfig configures Postgres advisory-lock fencing.
type LeaseConfig struct {
	DSN           string
	TTL           time.Duration // how often we re-check / try to acquire
	LockKey1      int32
	LockKey2      int32
	Log           *slog.Logger
}

// PGLease holds a session-level Postgres advisory lock for active/standby MX fencing.
type PGLease struct {
	db     *sql.DB
	conn   *sql.Conn
	key1   int32
	key2   int32
	ttl    time.Duration
	log    *slog.Logger
	leader atomic.Bool
}

// DefaultLockKeys returns stable advisory lock keys for Tayga MX fencing.
func DefaultLockKeys() (int32, int32) {
	h := fnv.New32a()
	_, _ = h.Write([]byte("tayga-mail-mx-leader"))
	v := h.Sum32()
	return int32(v>>1) | 1, int32(v) | 1
}

// NewPGLease opens a dedicated connection used only for the advisory lock.
func NewPGLease(ctx context.Context, cfg LeaseConfig) (*PGLease, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("ha: empty postgres dsn")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 5 * time.Second
	}
	if cfg.LockKey1 == 0 && cfg.LockKey2 == 0 {
		cfg.LockKey1, cfg.LockKey2 = DefaultLockKeys()
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ha lease conn: %w", err)
	}
	l := &PGLease{db: db, conn: conn, key1: cfg.LockKey1, key2: cfg.LockKey2, ttl: cfg.TTL, log: cfg.Log}
	l.tryAcquire(ctx)
	return l, nil
}

// Start renews / retries the lock until ctx is cancelled.
func (l *PGLease) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(l.ttl)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				l.release()
				return
			case <-t.C:
				l.tryAcquire(ctx)
			}
		}
	}()
}

func (l *PGLease) IsLeader() bool {
	if l == nil {
		return true
	}
	return l.leader.Load()
}

func (l *PGLease) tryAcquire(ctx context.Context) {
	if l.conn == nil {
		return
	}
	var ok bool
	err := l.conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1, $2)`, l.key1, l.key2).Scan(&ok)
	if err != nil {
		was := l.leader.Swap(false)
		if was {
			l.log.Warn("ha lease lost (query error)", "err", err)
		}
		return
	}
	if ok {
		if !l.leader.Swap(true) {
			l.log.Info("ha lease acquired; this node is MX leader")
		}
		return
	}
	// Someone else holds it — confirm we are not leader.
	if l.leader.Swap(false) {
		l.log.Warn("ha lease held by peer; standing by for MX")
	}
}

func (l *PGLease) release() {
	if l.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = l.conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1, $2)`, l.key1, l.key2)
	_ = l.conn.Close()
	_ = l.db.Close()
	l.leader.Store(false)
}

// Close releases the lock (also called from Start on cancel).
func (l *PGLease) Close() error {
	l.release()
	return nil
}
