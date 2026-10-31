package xmpp

import (
	"bufio"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
)

// Server is the Stage-1 XMPP C2S listener (TMS-XMPP-001).
type Server struct {
	mucMu      sync.Mutex
	muc        map[string]map[*Session]mucOccupant
	mucPending map[string]*Session
	mucWeb     map[string]map[string]webRoomOccupant // protected by mucMu

	pushMu       sync.Mutex
	pepPublishMu sync.Mutex
	pepOwners    map[string]*pepOwnerLock // Per-owner commit/notification order.

	resumeMu       sync.Mutex
	resumable      map[string]resumableStream
	closingStreams map[*Session]struct{} // transports being cleaned up before SM snapshot publication
	upload         UploadHandler
	uploadMu       sync.RWMutex
	cfg            *config.Config
	log            *slog.Logger
	store          storage.Driver
	authn          *auth.Layer
	tls            *tlsutil.Manager
	tlsCfg         *tls.Config
	hub            *Hub
	domain         string
	db             *sql.DB

	webPresenceMu  sync.Mutex
	presence       map[string]map[string]presenceState
	mu             sync.Mutex
	ln             []net.Listener
	closed         bool
	webSockets     map[*Session]struct{} // protected by mu, including pre-auth connections
	webSocketCount int                   // includes HTTP upgrades in flight; protected by mu
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, tlsMgr *tlsutil.Manager) *Server {
	domain := ""
	if cfg != nil {
		domain = cfg.Server.Hostname
	}
	var db *sql.DB
	if sqlStore, ok := store.(*storage.Store); ok {
		db = sqlStore.DB()
	}
	srv := &Server{
		cfg: cfg, log: log, store: store, authn: authn, tls: tlsMgr,
		hub: NewHub(), domain: domain, db: db,
	}
	srv.hub.nativePushSink = srv.enqueueNativePush
	return srv
}

func (s *Server) Start(ctx context.Context) error {
	if s.cfg == nil || !s.cfg.XMPP.Enabled {
		s.log.Info("xmpp disabled")
		return nil
	}
	s.tlsCfg = nil
	if s.tls != nil && s.tls.Enabled() {
		s.tlsCfg = s.tls.TLSConfig()
	}

	type spec struct {
		addr     string
		name     string
		implicit bool
	}
	var specs []spec
	if s.cfg.XMPP.Listen != "" {
		specs = append(specs, spec{addr: s.cfg.XMPP.Listen, name: "xmpp-c2s"})
	}
	if s.cfg.XMPP.ListenTLS != "" {
		specs = append(specs, spec{addr: s.cfg.XMPP.ListenTLS, name: "xmpp-c2s-tls", implicit: true})
	}
	if len(specs) == 0 && !s.cfg.XMPP.WebSocket {
		s.log.Warn("xmpp enabled but no listen addresses")
		return nil
	}

	for _, sp := range specs {
		var ln net.Listener
		var err error
		if sp.implicit {
			if s.tlsCfg == nil {
				s.log.Warn("xmpp listen_tls set but TLS missing; skipping", "addr", sp.addr)
				continue
			}
			ln, err = net.Listen("tcp", sp.addr)
		} else {
			ln, err = net.Listen("tcp", sp.addr)
		}
		if err != nil {
			_ = s.Shutdown(context.Background())
			return err
		}
		s.mu.Lock()
		s.ln = append(s.ln, ln)
		s.mu.Unlock()
		s.log.Info("xmpp listening", "name", sp.name, "addr", sp.addr)
		go s.acceptLoop(ln, sp.implicit)
	}

	if err := s.startComponents(ctx); err != nil {
		_ = s.Shutdown(context.Background())
		return err
	}

	go s.runPushWorker(ctx)
	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()
	return nil
}

func (s *Server) acceptLoop(ln net.Listener, alreadyTLS bool) {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			s.log.Debug("xmpp accept", "err", err)
			continue
		}
		session := newSession(s, c, alreadyTLS)
		_, wrappedTLS := c.(*tls.Conn)
		if alreadyTLS && !wrappedTLS && session.tlsCfg != nil {
			tc := tls.Server(c, session.sessionTLSConfig())
			session.conn = tc
			session.br = bufio.NewReader(tc)
			session.bw = bufio.NewWriter(tc)
			session.dec = xml.NewDecoder(session.br)
		}
		go session.serve()
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	lns := append([]net.Listener(nil), s.ln...)
	s.ln = nil
	webSockets := make([]*Session, 0, len(s.webSockets))
	for session := range s.webSockets {
		webSockets = append(webSockets, session)
	}
	s.mu.Unlock()
	for _, session := range webSockets {
		_ = session.conn.Close() // Unblock readers, including unauthenticated handshakes.
	}
	for _, ln := range lns {
		_ = ln.Close()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *Server) rebind(q string) string {
	if s.cfg == nil || s.cfg.Storage.Driver != "postgres" {
		return q
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			fmt.Fprintf(&b, "%d", n)
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

func (s *Server) storeOffline(ctx context.Context, bareJID, stanza string) error {
	if s.db == nil {
		return nil
	}
	u, err := s.store.GetUserByEmail(ctx, bareJID)
	if err != nil || u == nil {
		return err
	}
	id := storage.NewID()
	_, err = s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO xmpp_offline (id, user_id, stanza, created_at) VALUES (?, ?, ?, ?)`),
		id, u.ID, stanza, time.Now().UTC(),
	)
	return err
}

func (s *Server) archiveMAM(ctx context.Context, ownerBare, withBare, stanzaID, stanza string) error {
	if s.db == nil {
		return nil
	}
	allowed, err := s.shouldArchive(ctx, ownerBare, withBare, stanza)
	if err != nil || !allowed {
		return err
	}
	if stanzaID == "" {
		stanzaID = randomID()
	}
	id := storage.NewID()
	_, err = s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO xmpp_mam (id, owner_bare_jid, with_bare_jid, stanza_id, stanza, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`),
		id, ownerBare, withBare, stanzaID, stanza, time.Now().UTC(),
	)
	return err
}
