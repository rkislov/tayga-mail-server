package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/ha"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
)

// Server wraps one or more go-smtp listeners (MX / submission / SMTPS).
type Server struct {
	cfg       *config.Config
	log       *slog.Logger
	store     storage.Driver
	authn     *auth.Layer
	mailstore *mailstore.Store
	sieve     *sieve.Engine
	tls       *tlsutil.Manager
	ha        ha.Gate
	servers   []*gosmtp.Server
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store, eng *sieve.Engine, tlsMgr *tlsutil.Manager, gate ha.Gate) *Server {
	if gate == nil {
		gate = ha.AlwaysLeader{}
	}
	return &Server{cfg: cfg, log: log, store: store, authn: authn, mailstore: ms, sieve: eng, tls: tlsMgr, ha: gate}
}

func (s *Server) outbound() (*outboundRelay, *dkimSigner, error) {
	rel := newOutboundRelay(s.cfg.SMTP.Relay, s.cfg.SMTP.WriteTimeout)
	sig, err := newDKIMSigner(s.cfg.SMTP.DKIM)
	if err != nil {
		return nil, nil, err
	}
	return rel, sig, nil
}

func (s *Server) Start(ctx context.Context) error {
	tlsCfg := s.tlsConfig()

	type listenSpec struct {
		addr        string
		name        string
		implicit    bool // SMTPS
		requireAuth bool
	}
	var specs []listenSpec
	if s.cfg.SMTP.MX != "" {
		specs = append(specs, listenSpec{addr: s.cfg.SMTP.MX, name: "mx", requireAuth: false})
	}
	if s.cfg.SMTP.Submission != "" {
		specs = append(specs, listenSpec{addr: s.cfg.SMTP.Submission, name: "submission", requireAuth: true})
	}
	if s.cfg.SMTP.SMTPS != "" {
		specs = append(specs, listenSpec{addr: s.cfg.SMTP.SMTPS, name: "smtps", implicit: true, requireAuth: true})
	}

	relay, dkimSig, err := s.outbound()
	if err != nil {
		return fmt.Errorf("smtp dkim: %w", err)
	}
	if relay != nil {
		s.log.Info("smtp outbound relay configured", "host", s.cfg.SMTP.Relay.Host, "dkim", dkimSig != nil)
	}

	for _, spec := range specs {
		be := &backend{
			log:         s.log,
			store:       s.store,
			authn:       s.authn,
			mailstore:   s.mailstore,
			sieve:       s.sieve,
			hostname:    s.cfg.Server.Hostname,
			maxSize:     s.cfg.SMTP.MaxSize,
			requireAuth: spec.requireAuth,
			ha:          s.ha,
			relay:       relay,
			dkim:        dkimSig,
		}
		srv := gosmtp.NewServer(be)
		srv.Domain = s.cfg.Server.Hostname
		srv.MaxMessageBytes = s.cfg.SMTP.MaxSize
		srv.MaxRecipients = 100
		srv.AllowInsecureAuth = tlsCfg == nil
		srv.ReadTimeout = s.cfg.SMTP.ReadTimeout
		srv.WriteTimeout = s.cfg.SMTP.WriteTimeout
		if tlsCfg != nil {
			srv.TLSConfig = tlsCfg
		}

		var ln net.Listener
		var err error
		if spec.implicit {
			if tlsCfg == nil {
				s.log.Warn("smtps configured but TLS certs missing; skipping", "addr", spec.addr)
				continue
			}
			ln, err = tls.Listen("tcp", spec.addr, tlsCfg)
		} else {
			ln, err = net.Listen("tcp", spec.addr)
		}
		if err != nil {
			_ = s.Shutdown(context.Background())
			return err
		}

		s.servers = append(s.servers, srv)
		s.log.Info("smtp listening", "name", spec.name, "addr", spec.addr)

		go func(name string, srv *gosmtp.Server, ln net.Listener) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, gosmtp.ErrServerClosed) {
				s.log.Error("smtp serve stopped", "name", name, "err", err)
			}
		}(spec.name, srv, ln)
	}

	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	var first error
	for _, srv := range s.servers {
		if err := srv.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Server) tlsConfig() *tls.Config {
	if s.tls != nil {
		return s.tls.TLSConfig()
	}
	cfg, _ := tlsutil.Load(s.cfg)
	return cfg
}

type backend struct {
	log         *slog.Logger
	store       storage.Driver
	authn       *auth.Layer
	mailstore   *mailstore.Store
	sieve       *sieve.Engine
	hostname    string
	maxSize     int64
	requireAuth bool
	ha          ha.Gate
	relay       *outboundRelay
	dkim        *dkimSigner
}

func (b *backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	return &session{
		backend: b,
		remote:  c.Conn().RemoteAddr().String(),
	}, nil
}

type session struct {
	backend *backend
	remote  string
	user    *storage.User
	from    string
	toLocal []string
	toRemote []string
	opts    *gosmtp.MailOptions
}

func (s *session) AuthMechanisms() []string {
	return []string{sasl.Plain, sasl.Login, sasl.OAuthBearer, auth.XOAuth2}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	authFailed := &gosmtp.SMTPError{Code: 535, EnhancedCode: gosmtp.EnhancedCode{5, 7, 8}, Message: "Authentication failed"}
	switch strings.ToUpper(mech) {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			u, err := s.backend.authn.Authenticate(context.Background(), username, password)
			if err != nil {
				return authFailed
			}
			s.user = u
			return nil
		}), nil
	case sasl.Login:
		return newLoginServer(func(username, password string) error {
			u, err := s.backend.authn.Authenticate(context.Background(), username, password)
			if err != nil {
				return authFailed
			}
			s.user = u
			return nil
		}), nil
	case sasl.OAuthBearer:
		return sasl.NewOAuthBearerServer(func(opts sasl.OAuthBearerOptions) *sasl.OAuthBearerError {
			u, err := s.backend.authn.AuthenticateToken(context.Background(), opts.Username, opts.Token)
			if err != nil {
				return &sasl.OAuthBearerError{Status: "invalid_token", Schemes: "bearer"}
			}
			s.user = u
			return nil
		}), nil
	case auth.XOAuth2:
		return auth.NewXOAuth2Server(func(username, token string) error {
			u, err := s.backend.authn.AuthenticateToken(context.Background(), username, token)
			if err != nil {
				return authFailed
			}
			s.user = u
			return nil
		}), nil
	default:
		return nil, gosmtp.ErrAuthUnsupported
	}
}

func (s *session) Mail(from string, opts *gosmtp.MailOptions) error {
	if s.backend.requireAuth && s.user == nil {
		return &gosmtp.SMTPError{Code: 530, EnhancedCode: gosmtp.EnhancedCode{5, 7, 0}, Message: "Authentication required"}
	}
	// Unauthenticated MX: only the HA lease holder accepts new mail.
	if !s.backend.requireAuth && s.backend.ha != nil && !s.backend.ha.IsLeader() {
		return &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 3, 2}, Message: "Not MX leader; try later"}
	}
	s.from = from
	s.opts = opts
	s.toLocal = nil
	s.toRemote = nil
	return nil
}

func (s *session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	to = normalizeAddr(to)
	_, err := s.backend.store.ResolveRecipient(context.Background(), to)
	if err == nil {
		s.toLocal = append(s.toLocal, to)
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		s.backend.log.Error("rcpt resolve failed", "to", to, "err", err)
		return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Temporary failure"}
	}
	// External recipient: authenticated submission + smart-host only.
	if s.user != nil && s.backend.relay != nil {
		s.toRemote = append(s.toRemote, to)
		return nil
	}
	return &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 1, 1}, Message: "User unknown"}
}

func (s *session) Data(r io.Reader) error {
	if len(s.toLocal) == 0 && len(s.toRemote) == 0 {
		return &gosmtp.SMTPError{Code: 554, EnhancedCode: gosmtp.EnhancedCode{5, 5, 0}, Message: "No valid recipients"}
	}
	data, err := io.ReadAll(io.LimitReader(r, s.backend.maxSize+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > s.backend.maxSize {
		return &gosmtp.SMTPError{Code: 552, EnhancedCode: gosmtp.EnhancedCode{5, 3, 4}, Message: "Message too large"}
	}

	ctx := context.Background()
	msgid := extractMessageID(data)
	for _, rcpt := range s.toLocal {
		if err := s.deliver(ctx, rcpt, data, msgid); err != nil {
			var smtpErr *gosmtp.SMTPError
			if errors.As(err, &smtpErr) {
				return smtpErr
			}
			s.backend.log.Error("delivery failed", "rcpt", rcpt, "err", err)
			return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Delivery failed"}
		}
	}
	if len(s.toRemote) > 0 {
		out := data
		if s.backend.dkim != nil {
			signed, err := s.backend.dkim.Sign(out)
			if err != nil {
				s.backend.log.Error("dkim sign failed", "err", err)
				return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "DKIM signing failed"}
			}
			out = signed
		}
		from := normalizeAddr(s.from)
		if err := s.backend.relay.Send(from, s.toRemote, out); err != nil {
			s.backend.log.Error("outbound relay failed", "err", err, "recipients", len(s.toRemote))
			return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Relay failed"}
		}
	}
	s.backend.log.Info("message accepted",
		"from", s.from,
		"local", len(s.toLocal),
		"remote", len(s.toRemote),
		"size", len(data),
		"peer", s.remote,
	)
	return nil
}

func (s *session) deliver(ctx context.Context, rcpt string, data []byte, msgid string) error {
	u, err := s.backend.store.ResolveRecipient(ctx, rcpt)
	if err != nil {
		return err
	}
	if s.backend.sieve != nil {
		err := s.backend.sieve.Deliver(ctx, u, s.from, rcpt, data, msgid)
		if errors.Is(err, sieve.ErrRejected) {
			return &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 7, 1}, Message: "Message rejected by filter"}
		}
		if errors.Is(err, storage.ErrQuotaExceeded) {
			return &gosmtp.SMTPError{Code: 552, EnhancedCode: gosmtp.EnhancedCode{5, 2, 2}, Message: "Mailbox full"}
		}
		return err
	}
	if err := storage.EnsureQuota(ctx, s.backend.store, u, int64(len(data))); err != nil {
		if errors.Is(err, storage.ErrQuotaExceeded) {
			return &gosmtp.SMTPError{Code: 552, EnhancedCode: gosmtp.EnhancedCode{5, 2, 2}, Message: "Mailbox full"}
		}
		return err
	}
	if _, err := s.backend.mailstore.EnsureUser(u.Email); err != nil {
		return err
	}
	mb, err := s.backend.store.EnsureMailbox(ctx, u.ID, "INBOX", s.backend.mailstore.UserRoot(u.Email))
	if err != nil {
		return err
	}
	rel, size, err := s.backend.mailstore.Deliver(u.Email, "INBOX", data)
	if err != nil {
		return err
	}
	_, err = s.backend.store.InsertMessage(ctx, &storage.Message{
		MailboxID:    mb.ID,
		Size:         size,
		Flags:        "",
		InternalDate: time.Now().UTC(),
		FilePath:     rel,
		MessageID:    msgid,
	})
	return err
}

func (s *session) Reset() {
	s.from = ""
	s.toLocal = nil
	s.toRemote = nil
	s.opts = nil
}

func (s *session) Logout() error { return nil }

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.Trim(addr, "<>")
	return strings.ToLower(addr)
}

func extractMessageID(data []byte) string {
	const max = 64 << 10
	head := data
	if len(head) > max {
		head = head[:max]
	}
	for _, line := range strings.Split(string(head), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "message-id:") {
			return strings.TrimSpace(line[len("Message-ID:"):])
		}
	}
	return ""
}
