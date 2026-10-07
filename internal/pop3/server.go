package pop3

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/ha"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
)

// Server is a minimal RFC 1939 POP3 server for INBOX.
type Server struct {
	cfg          *config.Config
	log          *slog.Logger
	store        storage.Driver
	authn        *auth.Layer
	mailstore    *mailstore.Store
	tls          *tlsutil.Manager
	tlsCfg       *tls.Config
	ha           ha.Gate
	fenceWriters bool

	mu     sync.Mutex
	ln     []net.Listener
	closed bool
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store, tlsMgr *tlsutil.Manager, gate ha.Gate) *Server {
	if gate == nil {
		gate = ha.AlwaysLeader{}
	}
	return &Server{
		cfg: cfg, log: log, store: store, authn: authn, mailstore: ms, tls: tlsMgr,
		ha: gate, fenceWriters: cfg.HA.FenceWriters(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	tlsCfg := s.tlsConfig()
	s.tlsCfg = tlsCfg

	type spec struct {
		addr     string
		name     string
		implicit bool
	}
	var specs []spec
	if s.cfg.POP3.Listen != "" {
		specs = append(specs, spec{addr: s.cfg.POP3.Listen, name: "pop3"})
	}
	if s.cfg.POP3.POP3S != "" {
		specs = append(specs, spec{addr: s.cfg.POP3.POP3S, name: "pop3s", implicit: true})
	}

	for _, sp := range specs {
		var ln net.Listener
		var err error
		if sp.implicit {
			if tlsCfg == nil {
				s.log.Warn("pop3s configured but TLS certs missing; skipping", "addr", sp.addr)
				continue
			}
			ln, err = tls.Listen("tcp", sp.addr, tlsCfg)
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
		s.log.Info("pop3 listening", "name", sp.name, "addr", sp.addr)
		go s.serve(ln, sp.implicit)
	}

	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()
	return nil
}

func (s *Server) serve(ln net.Listener, alreadyTLS bool) {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			s.log.Error("pop3 accept", "err", err)
			return
		}
		go s.handle(c, alreadyTLS)
	}
}

func (s *Server) Shutdown(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var first error
	for _, ln := range s.ln {
		if err := ln.Close(); err != nil && first == nil {
			first = err
		}
	}
	s.ln = nil
	return first
}

func (s *Server) tlsConfig() *tls.Config {
	if s.tls != nil {
		return s.tls.TLSConfig()
	}
	cfg, _ := tlsutil.Load(s.cfg)
	return cfg
}

type session struct {
	s       *Server
	rw      *bufio.ReadWriter
	conn    net.Conn
	user    *storage.User
	msgs    []*storage.Message
	deleted map[int]bool // 1-based index
	authed  bool
	tlsOn   bool
}

func (s *Server) handle(c net.Conn, alreadyTLS bool) {
	defer c.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))
	sess := &session{s: s, rw: rw, conn: c, deleted: map[int]bool{}, tlsOn: alreadyTLS}
	_ = sess.ok("Tayga POP3 ready")
	for {
		line, err := sess.rw.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		cmd, arg, _ := strings.Cut(line, " ")
		cmd = strings.ToUpper(cmd)
		arg = strings.TrimSpace(arg)

		var errResp error
		switch cmd {
		case "USER":
			errResp = sess.cmdUSER(arg)
		case "PASS":
			errResp = sess.cmdPASS(arg)
		case "STAT":
			errResp = sess.cmdSTAT()
		case "LIST":
			errResp = sess.cmdLIST(arg)
		case "RETR":
			errResp = sess.cmdRETR(arg)
		case "DELE":
			errResp = sess.cmdDELE(arg)
		case "NOOP":
			errResp = sess.ok("")
		case "RSET":
			errResp = sess.cmdRSET()
		case "QUIT":
			_ = sess.cmdQUIT()
			return
		case "UIDL":
			errResp = sess.cmdUIDL(arg)
		case "TOP":
			errResp = sess.cmdTOP(arg)
		case "CAPA":
			errResp = sess.cmdCAPA()
		case "STLS":
			errResp = sess.cmdSTLS()
		default:
			errResp = sess.err("unknown command")
		}
		if errResp != nil {
			return
		}
	}
}

func (sess *session) ok(msg string) error {
	if msg == "" {
		msg = "OK"
	}
	_, err := fmt.Fprintf(sess.rw, "+OK %s\r\n", msg)
	if err != nil {
		return err
	}
	return sess.rw.Flush()
}

func (sess *session) err(msg string) error {
	_, err := fmt.Fprintf(sess.rw, "-ERR %s\r\n", msg)
	if err != nil {
		return err
	}
	return sess.rw.Flush()
}

func (sess *session) requireAuth() error {
	if !sess.authed {
		return sess.err("authorization required")
	}
	return nil
}

func (sess *session) cmdUSER(arg string) error {
	if arg == "" {
		return sess.err("mailbox required")
	}
	sess.user = &storage.User{Email: strings.ToLower(arg)}
	return sess.ok("user accepted")
}

func (sess *session) cmdPASS(arg string) error {
	if sess.user == nil || sess.user.Email == "" {
		return sess.err("USER first")
	}
	u, err := sess.s.authn.Authenticate(context.Background(), sess.user.Email, arg)
	if err != nil {
		return sess.err("auth failed")
	}
	sess.user = u
	mb, err := sess.s.store.EnsureMailbox(context.Background(), u.ID, "INBOX", sess.s.mailstore.UserRoot(u.Email))
	if err != nil {
		return sess.err("mailbox error")
	}
	msgs, err := sess.s.store.ListMessages(context.Background(), mb.ID)
	if err != nil {
		return sess.err("mailbox error")
	}
	sess.msgs = msgs
	sess.authed = true
	return sess.ok(fmt.Sprintf("%s has %d messages", u.Email, len(msgs)))
}

func (sess *session) active() []*storage.Message {
	var out []*storage.Message
	for i, m := range sess.msgs {
		if sess.deleted[i+1] {
			continue
		}
		out = append(out, m)
	}
	return out
}

func (sess *session) cmdSTAT() error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	var size int64
	n := 0
	for i, m := range sess.msgs {
		if sess.deleted[i+1] {
			continue
		}
		n++
		size += m.Size
	}
	return sess.ok(fmt.Sprintf("%d %d", n, size))
}

func (sess *session) cmdLIST(arg string) error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	if arg != "" {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(sess.msgs) || sess.deleted[n] {
			return sess.err("no such message")
		}
		return sess.ok(fmt.Sprintf("%d %d", n, sess.msgs[n-1].Size))
	}
	_ = sess.ok(fmt.Sprintf("%d messages", len(sess.active())))
	for i, m := range sess.msgs {
		if sess.deleted[i+1] {
			continue
		}
		fmt.Fprintf(sess.rw, "%d %d\r\n", i+1, m.Size)
	}
	fmt.Fprintf(sess.rw, ".\r\n")
	return sess.rw.Flush()
}

func (sess *session) cmdRETR(arg string) error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(sess.msgs) || sess.deleted[n] {
		return sess.err("no such message")
	}
	data, err := sess.s.mailstore.Read(sess.msgs[n-1].FilePath)
	if err != nil {
		return sess.err("read failed")
	}
	_ = sess.ok(fmt.Sprintf("%d octets", len(data)))
	return writeDotStuff(sess.rw, data)
}

func (sess *session) cmdTOP(arg string) error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	parts := strings.Fields(arg)
	if len(parts) != 2 {
		return sess.err("TOP msg n")
	}
	n, err1 := strconv.Atoi(parts[0])
	lines, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || n < 1 || n > len(sess.msgs) || sess.deleted[n] || lines < 0 {
		return sess.err("no such message")
	}
	data, err := sess.s.mailstore.Read(sess.msgs[n-1].FilePath)
	if err != nil {
		return sess.err("read failed")
	}
	top := topLines(data, lines)
	_ = sess.ok("top follows")
	return writeDotStuff(sess.rw, top)
}

func (sess *session) cmdDELE(arg string) error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	if sess.s.fenceWriters && sess.s.ha != nil && !sess.s.ha.IsLeader() {
		return sess.err("standby; try active node")
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(sess.msgs) || sess.deleted[n] {
		return sess.err("no such message")
	}
	sess.deleted[n] = true
	return sess.ok("deleted")
}

func (sess *session) cmdRSET() error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	sess.deleted = map[int]bool{}
	return sess.ok("reset")
}

func (sess *session) cmdUIDL(arg string) error {
	if err := sess.requireAuth(); err != nil {
		return err
	}
	if arg != "" {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 || n > len(sess.msgs) || sess.deleted[n] {
			return sess.err("no such message")
		}
		return sess.ok(fmt.Sprintf("%d %d", n, sess.msgs[n-1].UID))
	}
	_ = sess.ok("uidl follows")
	for i, m := range sess.msgs {
		if sess.deleted[i+1] {
			continue
		}
		fmt.Fprintf(sess.rw, "%d %d\r\n", i+1, m.UID)
	}
	fmt.Fprintf(sess.rw, ".\r\n")
	return sess.rw.Flush()
}

func (sess *session) cmdCAPA() error {
	_ = sess.ok("capability list follows")
	fmt.Fprintf(sess.rw, "USER\r\nUIDL\r\nTOP\r\n")
	if sess.s.tlsCfg != nil && !sess.tlsOn {
		fmt.Fprintf(sess.rw, "STLS\r\n")
	}
	fmt.Fprintf(sess.rw, ".\r\n")
	return sess.rw.Flush()
}

func (sess *session) cmdSTLS() error {
	if sess.tlsOn {
		return sess.err("TLS already active")
	}
	if sess.s.tlsCfg == nil {
		return sess.err("TLS not available")
	}
	if sess.authed {
		return sess.err("STLS after auth not allowed")
	}
	if err := sess.ok("Begin TLS negotiation"); err != nil {
		return err
	}
	tlsConn := tls.Server(sess.conn, sess.s.tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	sess.conn = tlsConn
	sess.rw = bufio.NewReadWriter(bufio.NewReader(tlsConn), bufio.NewWriter(tlsConn))
	sess.tlsOn = true
	return nil
}

func (sess *session) cmdQUIT() error {
	if sess.authed {
		hasDel := false
		for i := range sess.msgs {
			if sess.deleted[i+1] {
				hasDel = true
				break
			}
		}
		if hasDel && sess.s.fenceWriters && sess.s.ha != nil && !sess.s.ha.IsLeader() {
			_ = sess.err("standby; deletions not committed")
			return nil
		}
		ctx := context.Background()
		for i := range sess.msgs {
			if !sess.deleted[i+1] {
				continue
			}
			m := sess.msgs[i]
			_ = sess.s.mailstore.Delete(m.FilePath)
			_ = sess.s.store.DeleteMessage(ctx, m.ID)
		}
	}
	_ = sess.ok("bye")
	return nil
}

func writeDotStuff(w *bufio.ReadWriter, data []byte) error {
	r := bufio.NewReader(bytesReader(data))
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			if line[0] == '.' {
				if _, e := w.Write([]byte{'.'}); e != nil {
					return e
				}
			}
			if _, e := w.Write(line); e != nil {
				return e
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	if _, err := w.WriteString(".\r\n"); err != nil {
		return err
	}
	return w.Flush()
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func topLines(data []byte, n int) []byte {
	text := string(data)
	parts := strings.SplitN(text, "\r\n\r\n", 2)
	if len(parts) == 1 {
		parts = strings.SplitN(text, "\n\n", 2)
	}
	if len(parts) == 1 {
		return data
	}
	header := parts[0]
	body := parts[1]
	var out strings.Builder
	out.WriteString(header)
	out.WriteString("\r\n\r\n")
	scanner := bufio.NewScanner(strings.NewReader(body))
	for i := 0; i < n && scanner.Scan(); i++ {
		out.WriteString(scanner.Text())
		out.WriteString("\r\n")
	}
	return []byte(out.String())
}
