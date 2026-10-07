package managesieve

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

// Server implements a minimal ManageSieve (RFC 5804) listener.
type Server struct {
	cfg   *config.Config
	log   *slog.Logger
	store storage.Driver
	authn *auth.Layer

	mu     sync.Mutex
	ln     []net.Listener
	closed bool
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer) *Server {
	return &Server{cfg: cfg, log: log, store: store, authn: authn}
}

func (s *Server) Start(ctx context.Context) error {
	if s.cfg.ManageSieve.Listen == "" {
		return nil
	}
	tlsCfg, err := s.loadTLS()
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", s.cfg.ManageSieve.Listen)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ln = append(s.ln, ln)
	s.mu.Unlock()
	s.log.Info("managesieve listening", "addr", s.cfg.ManageSieve.Listen)

	go s.serve(ln, tlsCfg)

	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()
	return nil
}

func (s *Server) serve(ln net.Listener, tlsCfg *tls.Config) {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			s.log.Error("managesieve accept", "err", err)
			return
		}
		go s.handle(c, tlsCfg)
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

func (s *Server) loadTLS() (*tls.Config, error) {
	if !s.cfg.TLSEnabled() {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, nil
}

type session struct {
	s      *Server
	rw     *bufio.ReadWriter
	conn   net.Conn
	user   *storage.User
	tlsCfg *tls.Config
}

func (s *Server) handle(c net.Conn, tlsCfg *tls.Config) {
	defer c.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))
	sess := &session{s: s, rw: rw, conn: c, tlsCfg: tlsCfg}
	_ = sess.greeting()
	for {
		line, err := rw.ReadString('\n')
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

		var e error
		switch cmd {
		case "CAPABILITY":
			e = sess.cmdCapability()
		case "AUTHENTICATE":
			e = sess.cmdAuthenticate(arg)
		case "LOGOUT":
			_ = sess.ok("\"Logout completed\"")
			return
		case "NOOP":
			e = sess.ok("\"Done\"")
		case "STARTTLS":
			e = sess.cmdStartTLS()
		case "HAVESPACE":
			e = sess.requireAuthThen(func() error { return sess.ok("") })
		case "PUTSCRIPT":
			e = sess.requireAuthThen(func() error { return sess.cmdPutScript(arg) })
		case "LISTSCRIPTS":
			e = sess.requireAuthThen(sess.cmdListScripts)
		case "SETACTIVE":
			e = sess.requireAuthThen(func() error { return sess.cmdSetActive(arg) })
		case "GETSCRIPT":
			e = sess.requireAuthThen(func() error { return sess.cmdGetScript(arg) })
		case "DELETESCRIPT":
			e = sess.requireAuthThen(func() error { return sess.cmdDeleteScript(arg) })
		case "CHECKSCRIPT":
			e = sess.requireAuthThen(func() error { return sess.cmdCheckScript(arg) })
		case "RENAMESCRIPT":
			e = sess.requireAuthThen(func() error { return sess.cmdRenameScript(arg) })
		default:
			e = sess.no("\"Unknown command\"")
		}
		if e != nil {
			return
		}
	}
}

func (sess *session) greeting() error {
	caps := []string{`"IMPLEMENTATION" "Tayga ManageSieve"`, `"VERSION" "1.0"`, `"SASL" "PLAIN"`, `"SIEVE" "fileinto reject envelope imap4flags variables relational copy subaddress body"`}
	if sess.tlsCfg != nil {
		caps = append(caps, `"STARTTLS"`)
	}
	for _, c := range caps {
		if _, err := fmt.Fprintf(sess.rw, "%s\r\n", c); err != nil {
			return err
		}
	}
	return sess.ok("\"ManageSieve ready.\"")
}

func (sess *session) cmdCapability() error {
	return sess.greeting()
}

func (sess *session) ok(msg string) error {
	if msg == "" {
		_, err := fmt.Fprintf(sess.rw, "OK\r\n")
		if err != nil {
			return err
		}
		return sess.rw.Flush()
	}
	_, err := fmt.Fprintf(sess.rw, "OK %s\r\n", msg)
	if err != nil {
		return err
	}
	return sess.rw.Flush()
}

func (sess *session) no(msg string) error {
	_, err := fmt.Fprintf(sess.rw, "NO %s\r\n", msg)
	if err != nil {
		return err
	}
	return sess.rw.Flush()
}

func (sess *session) requireAuthThen(fn func() error) error {
	if sess.user == nil {
		return sess.no("\"Authentication required\"")
	}
	return fn()
}

func (sess *session) cmdAuthenticate(arg string) error {
	mech, rest, _ := strings.Cut(arg, " ")
	if !strings.EqualFold(strings.Trim(mech, `"`), "PLAIN") {
		return sess.no("\"Unsupported mechanism\"")
	}
	rest = strings.TrimSpace(rest)
	var raw []byte
	var err error
	if rest == "" {
		// client sends literal next — ask for continuation
		if _, err := fmt.Fprintf(sess.rw, "\"\"\r\n"); err != nil {
			return err
		}
		if err := sess.rw.Flush(); err != nil {
			return err
		}
		line, err := sess.rw.ReadString('\n')
		if err != nil {
			return err
		}
		raw, err = base64.StdEncoding.DecodeString(strings.TrimSpace(line))
		if err != nil {
			return sess.no("\"Invalid base64\"")
		}
	} else if strings.HasPrefix(rest, "{") {
		n, payload, err := readLiteral(sess.rw, rest)
		if err != nil {
			return sess.no(fmt.Sprintf("%q", err.Error()))
		}
		_ = n
		raw = payload
		// PLAIN initial response may still be base64 inside literal — ManageSieve often sends raw after AUTHENTICATE PLAIN {n+}
		if decoded, err := base64.StdEncoding.DecodeString(string(payload)); err == nil && bytesContainNull(decoded) {
			raw = decoded
		}
	} else {
		raw, err = base64.StdEncoding.DecodeString(strings.Trim(rest, `"`))
		if err != nil {
			return sess.no("\"Invalid base64\"")
		}
	}
	parts := strings.Split(string(raw), "\x00")
	if len(parts) < 3 {
		return sess.no("\"Invalid PLAIN\"")
	}
	username, password := parts[1], parts[2]
	u, err := sess.s.authn.Authenticate(context.Background(), username, password)
	if err != nil {
		return sess.no("\"Authentication failed\"")
	}
	sess.user = u
	return sess.ok("\"Authenticated\"")
}

func bytesContainNull(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func (sess *session) cmdStartTLS() error {
	if sess.tlsCfg == nil {
		return sess.no("\"TLS not available\"")
	}
	if err := sess.ok("\"Begin TLS\""); err != nil {
		return err
	}
	tlsConn := tls.Server(sess.conn, sess.tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	sess.conn = tlsConn
	sess.rw = bufio.NewReadWriter(bufio.NewReader(tlsConn), bufio.NewWriter(tlsConn))
	return sess.greeting()
}

func (sess *session) cmdPutScript(arg string) error {
	name, rest, ok := splitQuoted(arg)
	if !ok || name == "" {
		return sess.no("\"Invalid arguments\"")
	}
	rest = strings.TrimSpace(rest)
	n, body, err := readLiteral(sess.rw, rest)
	if err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	_ = n
	if err := sieve.CheckScript(string(body)); err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	if _, err := sess.s.store.PutSieveScript(context.Background(), sess.user.ID, name, string(body)); err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	return sess.ok("\"Script saved\"")
}

func (sess *session) cmdCheckScript(arg string) error {
	arg = strings.TrimSpace(arg)
	_, body, err := readLiteral(sess.rw, arg)
	if err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	if err := sieve.CheckScript(string(body)); err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	return sess.ok("\"Script ok\"")
}

func (sess *session) cmdListScripts() error {
	list, err := sess.s.store.ListSieveScripts(context.Background(), sess.user.ID)
	if err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	for _, sc := range list {
		if sc.Active {
			fmt.Fprintf(sess.rw, "\"%s\" ACTIVE\r\n", escapeQuote(sc.Name))
		} else {
			fmt.Fprintf(sess.rw, "\"%s\"\r\n", escapeQuote(sc.Name))
		}
	}
	return sess.ok("\"List complete\"")
}

func (sess *session) cmdSetActive(arg string) error {
	name, _, ok := splitQuoted(arg)
	if !ok {
		// SETACTIVE "" deactivates
		if strings.TrimSpace(arg) == `""` || strings.TrimSpace(arg) == "" {
			if err := sess.s.store.SetActiveSieveScript(context.Background(), sess.user.ID, ""); err != nil {
				return sess.no(fmt.Sprintf("%q", err.Error()))
			}
			return sess.ok("\"Active script cleared\"")
		}
		return sess.no("\"Invalid arguments\"")
	}
	if err := sess.s.store.SetActiveSieveScript(context.Background(), sess.user.ID, name); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return sess.no("\"Script not found\"")
		}
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	return sess.ok("\"Active script set\"")
}

func (sess *session) cmdGetScript(arg string) error {
	name, _, ok := splitQuoted(arg)
	if !ok {
		return sess.no("\"Invalid arguments\"")
	}
	sc, err := sess.s.store.GetSieveScript(context.Background(), sess.user.ID, name)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return sess.no("\"Script not found\"")
		}
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	fmt.Fprintf(sess.rw, "{%d}\r\n%s", len(sc.Script), sc.Script)
	if !strings.HasSuffix(sc.Script, "\n") {
		fmt.Fprintf(sess.rw, "\r\n")
	}
	return sess.ok("")
}

func (sess *session) cmdDeleteScript(arg string) error {
	name, _, ok := splitQuoted(arg)
	if !ok {
		return sess.no("\"Invalid arguments\"")
	}
	if err := sess.s.store.DeleteSieveScript(context.Background(), sess.user.ID, name); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return sess.no("\"Script not found\"")
		}
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	return sess.ok("\"Script deleted\"")
}

func (sess *session) cmdRenameScript(arg string) error {
	oldName, rest, ok := splitQuoted(arg)
	if !ok {
		return sess.no("\"Invalid arguments\"")
	}
	newName, _, ok := splitQuoted(strings.TrimSpace(rest))
	if !ok {
		return sess.no("\"Invalid arguments\"")
	}
	sc, err := sess.s.store.GetSieveScript(context.Background(), sess.user.ID, oldName)
	if err != nil {
		return sess.no("\"Script not found\"")
	}
	if _, err := sess.s.store.GetSieveScript(context.Background(), sess.user.ID, newName); err == nil {
		return sess.no("\"Already exists\"")
	}
	if _, err := sess.s.store.PutSieveScript(context.Background(), sess.user.ID, newName, sc.Script); err != nil {
		return sess.no(fmt.Sprintf("%q", err.Error()))
	}
	if sc.Active {
		_ = sess.s.store.SetActiveSieveScript(context.Background(), sess.user.ID, newName)
	}
	_ = sess.s.store.DeleteSieveScript(context.Background(), sess.user.ID, oldName)
	return sess.ok("\"Script renamed\"")
}

func splitQuoted(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, `"`) {
		return "", "", false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if s[i] == '"' {
			return b.String(), strings.TrimSpace(s[i+1:]), true
		}
		b.WriteByte(s[i])
		i++
	}
	return "", "", false
}

func escapeQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// readLiteral parses {n[+]} and reads n bytes from the connection.
func readLiteral(rw *bufio.ReadWriter, token string) (int, []byte, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "{") {
		return 0, nil, fmt.Errorf("literal expected")
	}
	end := strings.Index(token, "}")
	if end < 0 {
		return 0, nil, fmt.Errorf("bad literal")
	}
	num := strings.TrimSuffix(token[1:end], "+")
	n, err := strconv.Atoi(num)
	if err != nil || n < 0 {
		return 0, nil, fmt.Errorf("bad literal size")
	}
	buf := make([]byte, n)
	if _, err := rw.Read(buf); err != nil {
		return 0, nil, err
	}
	// consume trailing CRLF if present after literal
	return n, buf, nil
}
