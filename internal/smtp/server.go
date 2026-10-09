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
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/scan"
	"github.com/tayga/tms/internal/siem"
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/spam"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
)

// Server wraps one or more go-smtp listeners (MX / submission / SMTPS).
type Server struct {
	cfg          *config.Config
	log          *slog.Logger
	store        storage.Driver
	authn        *auth.Layer
	mailstore    *mailstore.Store
	sieve        *sieve.Engine
	tls          *tlsutil.Manager
	ha           ha.Gate
	fenceWriters bool
	writers      ha.WriterGate
	scan         *scanPolicy
	spam         *spamPolicy
	dkimVerify   *dkimVerifyPolicy
	spf          *spfPolicy
	iprev        *iprevPolicy
	helo         *heloPolicy
	greylist     *greylistPolicy
	dmarc        *dmarcPolicy
	dmarcReport  *dmarcReporter
	arc          *arcPolicy
	tlsrpt       *tlsReporter
	queue        *OutboundQueue
	siem         *siem.Exporter
	servers      []*gosmtp.Server
}

// SetSIEM attaches a CEF syslog exporter (optional; call before Start).
func (s *Server) SetSIEM(e *siem.Exporter) {
	if s != nil {
		s.siem = e
	}
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store, eng *sieve.Engine, tlsMgr *tlsutil.Manager, gate ha.Gate, writers ha.WriterGate) *Server {
	if gate == nil {
		gate = ha.AlwaysLeader{}
	}
	srv := &Server{
		cfg: cfg, log: log, store: store, authn: authn, mailstore: ms, sieve: eng, tls: tlsMgr, ha: gate,
		fenceWriters: cfg.HA.FenceWriters(),
		writers:      writers,
	}
	sc, err := scan.New(scan.Config{
		Enabled:          cfg.Scan.Enabled,
		Backend:          cfg.Scan.Backend,
		Action:           scan.Action(cfg.Scan.Action),
		QuarantineFolder: cfg.Scan.QuarantineFolder,
		Timeout:          cfg.Scan.Timeout,
		FailOpen:         cfg.Scan.FailOpen,
		ClamAVAddress:    cfg.Scan.ClamAV.Address,
		ExecCommand:      cfg.Scan.Exec.Command,
		ICAPURL:          cfg.Scan.ICAP.URL,
	})
	if err != nil {
		log.Error("scan config invalid; scanning disabled", "err", err)
	} else if sc != nil {
		srv.scan = &scanPolicy{
			scanner: sc, action: scan.Action(cfg.Scan.Action), folder: cfg.Scan.QuarantineFolder,
			failOpen: cfg.Scan.FailOpen, log: log,
		}
		if srv.scan.action == "" {
			srv.scan.action = scan.ActionQuarantine
		}
		if srv.scan.folder == "" {
			srv.scan.folder = "Quarantine"
		}
		log.Info("smtp virus scan enabled", "backend", sc.Name(), "action", srv.scan.action)
	}
	sp, err := spam.New(spam.Config{
		DNSBLEnabled: cfg.Spam.DNSBL.Enabled, DNSBLZones: cfg.Spam.DNSBL.Zones, DNSBLScore: cfg.Spam.DNSBL.Score,
		Enabled:         cfg.Spam.Enabled,
		Backend:         cfg.Spam.Backend,
		URL:             cfg.Spam.URL,
		Password:        cfg.Spam.Password,
		Timeout:         cfg.Spam.Timeout,
		FailOpen:        cfg.Spam.FailOpen,
		Folder:          cfg.Spam.Folder,
		FollowRspamd:    cfg.Spam.FollowRspamd,
		RejectAbove:     cfg.Spam.RejectAbove,
		QuarantineAbove: cfg.Spam.QuarantineAbove,
		TagAbove:        cfg.Spam.TagAbove,
	})
	if err != nil {
		log.Error("spam config invalid; spam disabled", "err", err)
	} else if sp != nil {
		srv.spam = &spamPolicy{
			checker: sp,
			cfg: spam.Config{
				DNSBLEnabled: cfg.Spam.DNSBL.Enabled, DNSBLZones: cfg.Spam.DNSBL.Zones, DNSBLScore: cfg.Spam.DNSBL.Score,
				Enabled:         cfg.Spam.Enabled,
				Backend:         cfg.Spam.Backend,
				URL:             cfg.Spam.URL,
				Password:        cfg.Spam.Password,
				Timeout:         cfg.Spam.Timeout,
				FailOpen:        cfg.Spam.FailOpen,
				Folder:          cfg.Spam.Folder,
				FollowRspamd:    cfg.Spam.FollowRspamd,
				RejectAbove:     cfg.Spam.RejectAbove,
				QuarantineAbove: cfg.Spam.QuarantineAbove,
				TagAbove:        cfg.Spam.TagAbove,
			},
			log: log,
		}
		log.Info("smtp spam check enabled", "backend", sp.Name(), "folder", srv.spam.cfg.Folder)
	}
	if cfg.DKIMVerify.Enabled {
		authserv := cfg.DKIMVerify.AuthservID
		if authserv == "" {
			authserv = cfg.Server.Hostname
		}
		action := cfg.DKIMVerify.Action
		if action == "" {
			action = "tag"
		}
		srv.dkimVerify = &dkimVerifyPolicy{
			action:           action,
			requireSignature: cfg.DKIMVerify.RequireSignature,
			failOpen:         cfg.DKIMVerify.FailOpen,
			authservID:       authserv,
			log:              log,
		}
		log.Info("smtp dkim verify enabled", "action", action, "authserv_id", authserv)
	}
	if cfg.SPF.Enabled {
		authserv := cfg.SPF.AuthservID
		if authserv == "" {
			authserv = cfg.Server.Hostname
		}
		action := cfg.SPF.Action
		if action == "" {
			action = "tag"
		}
		srv.spf = &spfPolicy{action: action, failOpen: cfg.SPF.FailOpen, authservID: authserv, log: log}
		log.Info("smtp spf check enabled", "action", action)
	}
	if cfg.IPRev.Enabled {
		authserv := cfg.IPRev.AuthservID
		if authserv == "" {
			authserv = cfg.Server.Hostname
		}
		action := cfg.IPRev.Action
		if action == "" {
			action = "tag"
		}
		srv.iprev = &iprevPolicy{action: action, failOpen: cfg.IPRev.FailOpen, authservID: authserv, log: log}
		log.Info("smtp iprev check enabled", "action", action)
	}
	if cfg.Helo.Enabled {
		authserv := cfg.Helo.AuthservID
		if authserv == "" {
			authserv = cfg.Server.Hostname
		}
		action := cfg.Helo.Action
		if action == "" {
			action = "tag"
		}
		srv.helo = &heloPolicy{action: action, requireFQDN: cfg.Helo.RequireFQDN, authservID: authserv, log: log}
		log.Info("smtp helo check enabled", "action", action, "require_fqdn", cfg.Helo.RequireFQDN)
	}
	if cfg.Greylist.Enabled {
		srv.greylist = newGreylistPolicy(store, cfg.Greylist.Delay, cfg.Greylist.PassTTL, cfg.Greylist.IPv4Net, log)
		log.Info("smtp greylist enabled", "delay", cfg.Greylist.Delay.String(), "pass_ttl", cfg.Greylist.PassTTL.String(), "ipv4_net", cfg.Greylist.IPv4Net)
	}
	if cfg.DMARC.Enabled {
		authserv := cfg.DMARC.AuthservID
		if authserv == "" {
			authserv = cfg.Server.Hostname
		}
		action := cfg.DMARC.Action
		if action == "" {
			action = "tag"
		}
		srv.dmarc = &dmarcPolicy{
			action: action, failOpen: cfg.DMARC.FailOpen, arcTrust: cfg.DMARC.ARCTrust,
			authservID: authserv, log: log,
		}
		log.Info("smtp dmarc check enabled", "action", action, "arc_trust", cfg.DMARC.ARCTrust)
		if cfg.DMARC.Report.Enabled || cfg.DMARC.Report.Failure {
			srv.dmarcReport = newDMARCReporter(store, cfg.DMARC.Report.OrgName, cfg.DMARC.Report.Contact, cfg.Server.Hostname, cfg.DMARC.Report.Interval, cfg.DMARC.Report.Enabled, cfg.DMARC.Report.Failure, log)
			if cfg.DMARC.Report.Enabled {
				srv.dmarc.recorder = srv.dmarcReport
			}
			if cfg.DMARC.Report.Failure {
				srv.dmarc.failures = srv.dmarcReport
			}
		}
	}
	if cfg.ARC.Enabled {
		ap, err := newARCPolicy(
			cfg.ARC.Verify, cfg.ARC.Seal, cfg.ARC.Action, cfg.ARC.FailOpen,
			cfg.ARC.AuthservID, cfg.ARC.Domain, cfg.ARC.Selector, cfg.ARC.PrivateKeyFile, log,
		)
		if err != nil {
			log.Error("arc config invalid; arc disabled", "err", err)
		} else {
			srv.arc = ap
			log.Info("smtp arc enabled", "verify", ap.verify, "seal", ap.seal, "action", ap.action)
		}
	}
	if cfg.SMTP.TLSRPT.Enabled {
		srv.tlsrpt = newTLSReporter(cfg.SMTP.TLSRPT.OrgName, cfg.SMTP.TLSRPT.Contact, cfg.Server.Hostname, cfg.SMTP.TLSRPT.Interval, log)
	}
	return srv
}

func (s *Server) outbound() (outboundSender, *dkimSigner, error) {
	sig, err := newDKIMSigner(s.cfg.SMTP.DKIM)
	if err != nil {
		return nil, nil, err
	}
	if rel := newOutboundRelay(s.cfg.SMTP.Relay, s.cfg.SMTP.WriteTimeout); rel != nil {
		return rel, sig, nil
	}
	if s.cfg.SMTP.OutboundDirect {
		var sts *stsResolver
		if s.cfg.SMTP.MTASTS.Enabled {
			sts = newSTSResolver(s.cfg.SMTP.MTASTS.Timeout, s.cfg.SMTP.MTASTS.FailOpen, s.log)
			s.log.Info("smtp mta-sts enabled", "fail_open", s.cfg.SMTP.MTASTS.FailOpen)
		}
		var dane *daneResolver
		if s.cfg.SMTP.DANE.Enabled {
			dane = newDANEResolver(s.cfg.SMTP.DANE.Timeout, s.cfg.SMTP.DANE.FailOpen, s.log)
			s.log.Info("smtp dane enabled", "fail_open", s.cfg.SMTP.DANE.FailOpen)
		}
		return newDirectSender(s.cfg.SMTP.WriteTimeout, s.cfg.Server.Hostname, sts, dane, s.tlsrpt, s.log), sig, nil
	}
	return nil, sig, nil
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

	out, dkimSig, err := s.outbound()
	if err != nil {
		return fmt.Errorf("smtp dkim: %w", err)
	}
	if out != nil {
		mode := "direct-mx"
		if s.cfg.SMTP.Relay.Host != "" {
			mode = "relay:" + s.cfg.SMTP.Relay.Host
		}
		s.log.Info("smtp outbound enabled", "mode", mode, "dkim", dkimSig != nil)
	}
	qc := s.cfg.SMTP.Queue
	var oq *OutboundQueue
	if out != nil && qc.Enabled {
		oq = newOutboundQueue(s.store, out, dkimSig, s.sieve, s.cfg.Server.Hostname, QueueConfig{
			Enabled: true, Workers: qc.Workers, PollInterval: qc.PollInterval,
			MaxAttempts: qc.MaxAttempts, BatchSize: qc.BatchSize,
		}, s.log)
		oq.SetHA(s.ha, s.fenceWriters)
		oq.SetWriters(s.writers)
		oq.SetSIEM(s.siem)
		s.queue = oq
		oq.Start(ctx)
	}
	if s.dmarcReport != nil {
		s.dmarcReport.SetOutbound(oq, out)
		s.dmarcReport.Start(ctx)
	}
	if s.tlsrpt != nil {
		s.tlsrpt.SetOutbound(oq, out)
		s.tlsrpt.Start(ctx)
	}
	rl := s.cfg.SMTP.RateLimit
	ipLim := newRateLimiter(rl.PerIP, rl.Window)
	userLim := newRateLimiter(rl.PerUser, rl.Window)
	if ipLim != nil || userLim != nil {
		s.log.Info("smtp rate limits", "per_ip", rl.PerIP, "per_user", rl.PerUser, "window", rl.Window.String())
	}

	for _, spec := range specs {
		be := &backend{
			log:          s.log,
			store:        s.store,
			authn:        s.authn,
			mailstore:    s.mailstore,
			sieve:        s.sieve,
			hostname:     s.cfg.Server.Hostname,
			maxSize:      s.cfg.SMTP.MaxSize,
			requireAuth:  spec.requireAuth,
			ha:           s.ha,
			fenceWriters: s.fenceWriters,
			writers:      s.writers,
			scan:         s.scan,
			spam:         s.spam,
			dkimVerify:   s.dkimVerify,
			spf:          s.spf,
			iprev:        s.iprev,
			helo:         s.helo,
			greylist:     s.greylist,
			dmarc:        s.dmarc,
			arc:          s.arc,
			out:          out,
			dkim:         dkimSig,
			queue:        oq,
			siem:         s.siem,
			ipLimit:      ipLim,
			userLimit:    userLim,
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
	log          *slog.Logger
	store        storage.Driver
	authn        *auth.Layer
	mailstore    *mailstore.Store
	sieve        *sieve.Engine
	hostname     string
	maxSize      int64
	requireAuth  bool
	ha           ha.Gate
	fenceWriters bool
	writers      ha.WriterGate
	scan         *scanPolicy
	spam         *spamPolicy
	dkimVerify   *dkimVerifyPolicy
	spf          *spfPolicy
	iprev        *iprevPolicy
	helo         *heloPolicy
	greylist     *greylistPolicy
	dmarc        *dmarcPolicy
	arc          *arcPolicy
	out          outboundSender
	dkim         *dkimSigner
	queue        *OutboundQueue
	siem         *siem.Exporter
	ipLimit      *rateLimiter
	userLimit    *rateLimiter
}

func (b *backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	return &session{
		backend: b,
		remote:  c.Conn().RemoteAddr().String(),
		helo:    c.Hostname(),
	}, nil
}

func (b *backend) writeMailLog(e *storage.MailLogEntry) {
	if b == nil || b.store == nil || e == nil {
		return
	}
	if b.siem != nil {
		b.siem.EmitMail(e.Event, e.Direction, e.Peer, e.MailFrom, e.RcptTo, e.MessageID, e.Detail, e.Size)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := b.store.InsertMailLog(ctx, e); err != nil && b.log != nil {
			b.log.Warn("mail log write failed", "err", err)
		}
	}()
}

type session struct {
	backend  *backend
	remote   string
	helo     string
	user     *storage.User
	from     string
	toLocal  []string
	toRemote []string
	opts     *gosmtp.MailOptions
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
			s.emitAuth(err == nil, username, err)
			if err != nil {
				return authFailed
			}
			s.user = u
			return nil
		}), nil
	case sasl.Login:
		return newLoginServer(func(username, password string) error {
			u, err := s.backend.authn.Authenticate(context.Background(), username, password)
			s.emitAuth(err == nil, username, err)
			if err != nil {
				return authFailed
			}
			s.user = u
			return nil
		}), nil
	case sasl.OAuthBearer:
		return sasl.NewOAuthBearerServer(func(opts sasl.OAuthBearerOptions) *sasl.OAuthBearerError {
			u, err := s.backend.authn.AuthenticateToken(context.Background(), opts.Username, opts.Token)
			s.emitAuth(err == nil, opts.Username, err)
			if err != nil {
				return &sasl.OAuthBearerError{Status: "invalid_token", Schemes: "bearer"}
			}
			s.user = u
			return nil
		}), nil
	case auth.XOAuth2:
		return auth.NewXOAuth2Server(func(username, token string) error {
			u, err := s.backend.authn.AuthenticateToken(context.Background(), username, token)
			s.emitAuth(err == nil, username, err)
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

func (s *session) emitAuth(ok bool, username string, err error) {
	if s == nil || s.backend == nil || s.backend.siem == nil {
		return
	}
	detail := "smtp"
	if err != nil {
		detail = err.Error()
	}
	s.backend.siem.EmitAuth(ok, username, stripPort(s.remote), detail)
}

func (s *session) Mail(from string, opts *gosmtp.MailOptions) error {
	if s.backend.requireAuth && s.user == nil {
		return &gosmtp.SMTPError{Code: 530, EnhancedCode: gosmtp.EnhancedCode{5, 7, 0}, Message: "Authentication required"}
	}
	// HA fencing: mx = unauthenticated only; writers = all SMTP writers.
	if s.backend.ha != nil && !s.backend.ha.IsLeader() {
		if s.backend.fenceWriters || !s.backend.requireAuth {
			return &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 3, 2}, Message: "Not HA leader; try later"}
		}
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
	// External recipient: authenticated submission + outbound (relay or direct MX).
	if s.user != nil && s.backend.out != nil {
		s.toRemote = append(s.toRemote, to)
		return nil
	}
	return &gosmtp.SMTPError{Code: 550, EnhancedCode: gosmtp.EnhancedCode{5, 1, 1}, Message: "User unknown"}
}

func (s *session) Data(r io.Reader) error {
	if len(s.toLocal) == 0 && len(s.toRemote) == 0 {
		return &gosmtp.SMTPError{Code: 554, EnhancedCode: gosmtp.EnhancedCode{5, 5, 0}, Message: "No valid recipients"}
	}
	if ip := stripPort(s.remote); s.backend.ipLimit != nil && !s.backend.ipLimit.Allow(ip) {
		return &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 7, 0}, Message: "Rate limit exceeded"}
	}
	if s.user != nil && s.backend.userLimit != nil && !s.backend.userLimit.Allow(s.user.Email) {
		return &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 7, 0}, Message: "User rate limit exceeded"}
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
				s.logMailEvent("rejected", "inbound", rcpt, msgid, int64(len(data)), smtpErr.Message)
				return smtpErr
			}
			s.backend.log.Error("delivery failed", "rcpt", rcpt, "err", err)
			s.logMailEvent("failed", "inbound", rcpt, msgid, int64(len(data)), err.Error())
			return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Delivery failed"}
		}
		s.logMailEvent("delivered", "inbound", rcpt, msgid, int64(len(data)), "")
	}
	if len(s.toRemote) > 0 {
		outData := data
		if s.backend.dkim != nil {
			signed, err := s.backend.dkim.Sign(outData)
			if err != nil {
				s.backend.log.Error("dkim sign failed", "err", err)
				return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "DKIM signing failed"}
			}
			outData = signed
		}
		from := normalizeAddr(s.from)
		if s.backend.queue != nil {
			if err := s.backend.queue.Enqueue(ctx, from, s.toRemote, outData, msgid); err != nil {
				s.backend.log.Error("outbound enqueue failed", "err", err)
				return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Queue failed"}
			}
			for _, rcpt := range s.toRemote {
				s.logMailEvent("queued", "outbound", rcpt, msgid, int64(len(outData)), "")
			}
		} else if s.backend.out != nil {
			if err := s.backend.out.Send(from, s.toRemote, outData); err != nil {
				s.backend.log.Error("outbound delivery failed", "err", err, "recipients", len(s.toRemote))
				for _, rcpt := range s.toRemote {
					s.logMailEvent("failed", "outbound", rcpt, msgid, int64(len(outData)), err.Error())
				}
				return &gosmtp.SMTPError{Code: 451, EnhancedCode: gosmtp.EnhancedCode{4, 3, 0}, Message: "Outbound delivery failed"}
			}
			for _, rcpt := range s.toRemote {
				s.logMailEvent("sent", "outbound", rcpt, msgid, int64(len(outData)), "")
			}
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

func (s *session) logMailEvent(event, direction, rcpt, msgid string, size int64, detail string) {
	if s == nil || s.backend == nil {
		return
	}
	from := normalizeAddr(s.from)
	rcpt = normalizeAddr(rcpt)
	domain := storage.DomainOfEmail(rcpt)
	if direction == "outbound" {
		domain = storage.DomainOfEmail(from)
	}
	tenantID := ""
	if s.user != nil {
		tenantID = s.user.TenantID
		if domain == "" {
			domain = storage.DomainOfEmail(s.user.Email)
		}
	}
	if tenantID == "" && domain != "" {
		if d, err := s.backend.store.GetDomainByName(context.Background(), domain); err == nil && d != nil {
			tenantID = d.TenantID
		}
	}
	if len(detail) > 500 {
		detail = detail[:500]
	}
	s.backend.writeMailLog(&storage.MailLogEntry{
		TenantID:  tenantID,
		Domain:    domain,
		Event:     event,
		Direction: direction,
		Peer:      stripPort(s.remote),
		MailFrom:  from,
		RcptTo:    rcpt,
		MessageID: msgid,
		Size:      size,
		Detail:    detail,
	})
}

func (s *session) deliver(ctx context.Context, rcpt string, data []byte, msgid string) error {
	u, err := s.backend.store.ResolveRecipient(ctx, rcpt)
	if err != nil {
		return err
	}
	if s.backend.writers != nil {
		if err := s.backend.writers.AllowWrite(ctx, u.ID); err != nil {
			return &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 3, 2}, Message: "Not writer for recipient; try later"}
		}
	}
	// Inbound mail auth (helo → iprev → SPF → DKIM → ARC verify → DMARC → ARC seal) for unauthenticated MX only.
	if s.user == nil {
		if s.backend.greylist != nil {
			if err := s.backend.greylist.check(stripPort(s.remote), s.from, rcpt); err != nil {
				return err
			}
		}
		var (
			spfRes      = spfNoneResult()
			dkimDomains []string
			arcCV       = "none"
			authErr     error
		)
		if s.backend.helo != nil {
			data, authErr = s.backend.helo.apply(s.helo, data)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
		if s.backend.iprev != nil {
			data, authErr = s.backend.iprev.apply(stripPort(s.remote), data)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
		if s.backend.spf != nil {
			heloName := s.helo
			if heloName == "" {
				heloName = s.backend.hostname
			}
			data, spfRes, authErr = s.backend.spf.apply(stripPort(s.remote), heloName, s.from, data)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
		if s.backend.dkimVerify != nil {
			data, dkimDomains, authErr = s.backend.dkimVerify.apply(data)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
		if s.backend.arc != nil {
			data, arcCV, authErr = s.backend.arc.applyVerify(ctx, data)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
		if s.backend.dmarc != nil {
			data, authErr = s.backend.dmarc.apply(ctx, data, s.from, stripPort(s.remote), spfRes, dkimDomains, arcCV)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
		if s.backend.arc != nil {
			data, authErr = s.backend.arc.applySeal(ctx, data)
			if authErr != nil {
				var smtpErr *gosmtp.SMTPError
				if errors.As(authErr, &smtpErr) {
					return smtpErr
				}
				return authErr
			}
		}
	}
	if s.backend.scan != nil {
		var scanErr error
		data, scanErr = s.backend.scan.apply(ctx, u, data, msgid, s.deliverQuarantine)
		if scanErr != nil {
			if errors.Is(scanErr, errQuarantined) {
				return nil
			}
			var smtpErr *gosmtp.SMTPError
			if errors.As(scanErr, &smtpErr) {
				return smtpErr
			}
			if errors.Is(scanErr, storage.ErrQuotaExceeded) {
				return &gosmtp.SMTPError{Code: 552, EnhancedCode: gosmtp.EnhancedCode{5, 2, 2}, Message: "Mailbox full"}
			}
			return scanErr
		}
	}
	if s.backend.spam != nil {
		authUser := ""
		if s.user != nil {
			authUser = s.user.Email
		}
		meta := spam.Meta{
			From:     s.from,
			To:       rcpt,
			IP:       stripPort(s.remote),
			User:     authUser,
			Hostname: s.backend.hostname,
		}
		var spamErr error
		data, spamErr = s.backend.spam.apply(ctx, u, meta, data, msgid, s.deliverFolder)
		if spamErr != nil {
			if errors.Is(spamErr, errQuarantined) {
				return nil
			}
			var smtpErr *gosmtp.SMTPError
			if errors.As(spamErr, &smtpErr) {
				return smtpErr
			}
			if errors.Is(spamErr, storage.ErrQuotaExceeded) {
				return &gosmtp.SMTPError{Code: 552, EnhancedCode: gosmtp.EnhancedCode{5, 2, 2}, Message: "Mailbox full"}
			}
			return spamErr
		}
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
	msg := &storage.Message{
		MailboxID:    mb.ID,
		Size:         size,
		Flags:        "",
		InternalDate: time.Now().UTC(),
		FilePath:     rel,
		MessageID:    msgid,
	}
	mailsearch.ApplyHeaders(msg, data)
	inserted, err := s.backend.store.InsertMessage(ctx, msg)
	if err != nil {
		return err
	}
	_ = mailsearch.Index(ctx, s.backend.store, inserted.ID, data)
	return nil
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

func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
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
