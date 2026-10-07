package imapserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const idleNotifyChannel = "tayga_idle"

// PGBus fans IMAP IDLE wakeups across Tayga nodes that share a Postgres DSN.
// Uses LISTEN/NOTIFY on channel tayga_idle. Payload: nodeID\temail\tmailbox.
type PGBus struct {
	hub    *Hub
	db     *sql.DB
	dsn    string
	nodeID string
	log    *slog.Logger
}

// NewPGBus creates a cluster bus. dsn must be a lib/pq connection string.
// db is used for NOTIFY (may be the shared store pool); LISTEN uses a dedicated connection.
func NewPGBus(hub *Hub, db *sql.DB, dsn string, log *slog.Logger) *PGBus {
	if log == nil {
		log = slog.Default()
	}
	return &PGBus{
		hub:    hub,
		db:     db,
		dsn:    dsn,
		nodeID: uuid.NewString(),
		log:    log,
	}
}

// NodeID returns this process's bus identity (used to ignore echo).
func (b *PGBus) NodeID() string { return b.nodeID }

// Publish implements ClusterPublisher.
func (b *PGBus) Publish(email, mailbox string) {
	if b == nil || b.db == nil {
		return
	}
	payload := encodeIdlePayload(b.nodeID, email, mailbox)
	if payload == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := b.db.ExecContext(ctx, `SELECT pg_notify($1, $2)`, idleNotifyChannel, payload); err != nil {
		b.log.Debug("idle cluster notify failed", "err", err)
	}
}

// Start listens for peer notifications until ctx is cancelled.
func (b *PGBus) Start(ctx context.Context) error {
	if b == nil || b.dsn == "" {
		return fmt.Errorf("pg idle bus: empty dsn")
	}
	listener := pq.NewListener(b.dsn, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			b.log.Debug("idle cluster listener event", "event", int(ev), "err", err)
		}
	})
	if err := listener.Listen(idleNotifyChannel); err != nil {
		_ = listener.Close()
		return fmt.Errorf("listen %s: %w", idleNotifyChannel, err)
	}
	b.log.Info("imap idle cluster bus started", "channel", idleNotifyChannel, "node", b.nodeID)

	go func() {
		defer listener.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case n := <-listener.Notify:
				if n == nil {
					continue
				}
				nodeID, email, mailbox, ok := decodeIdlePayload(n.Extra)
				if !ok {
					continue
				}
				if nodeID == b.nodeID {
					continue
				}
				b.hub.NotifyRemote(email, mailbox)
			case <-time.After(90 * time.Second):
				// Keepalive / reconnect probe (lib/pq Ping).
				go func() {
					if err := listener.Ping(); err != nil {
						b.log.Debug("idle cluster listener ping", "err", err)
					}
				}()
			}
		}
	}()
	return nil
}

func encodeIdlePayload(nodeID, email, mailbox string) string {
	nodeID = strings.TrimSpace(nodeID)
	email = strings.TrimSpace(email)
	mailbox = strings.TrimSpace(mailbox)
	if nodeID == "" || email == "" || mailbox == "" {
		return ""
	}
	if strings.ContainsAny(nodeID, "\t\n") || strings.ContainsAny(email, "\t\n") || strings.ContainsAny(mailbox, "\t\n") {
		return ""
	}
	s := nodeID + "\t" + email + "\t" + mailbox
	if len(s) > 7900 { // Postgres NOTIFY payload limit is 8000 bytes
		return ""
	}
	return s
}

func decodeIdlePayload(extra string) (nodeID, email, mailbox string, ok bool) {
	parts := strings.SplitN(extra, "\t", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	nodeID, email, mailbox = parts[0], parts[1], parts[2]
	if nodeID == "" || email == "" || mailbox == "" {
		return "", "", "", false
	}
	return nodeID, email, mailbox, true
}
