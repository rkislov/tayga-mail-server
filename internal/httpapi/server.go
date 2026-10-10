package httpapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/dav"
	"github.com/tayga/tms/internal/flowsync"
	"github.com/tayga/tms/internal/frontend"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/migrate"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/settings"
	"github.com/tayga/tms/internal/siem"
	"github.com/tayga/tms/internal/storage"
	"github.com/tayga/tms/internal/tlsutil"
	"github.com/tayga/tms/internal/xmpp"
)

// Server exposes health, metrics, auth API, and the embedded Web UI.
type Server struct {
	flowSubmit flowsync.SubmitFunc
	log        *slog.Logger
	cfg        *config.Config
	hub        *settings.Hub
	store      storage.Driver
	authn      *auth.Layer
	ms         *mailstore.Store
	tls        *tlsutil.Manager
	xmpp       xmpp.Gateway
	migrate    *migrate.Service
	notify     *notify.Hub
	siem       *siem.Exporter
	servers    []*http.Server
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store, tlsMgr *tlsutil.Manager, hub *settings.Hub) *Server {
	return &Server{cfg: cfg, log: log, store: store, authn: authn, ms: ms, tls: tlsMgr, hub: hub}
}

// SetSIEM attaches a CEF syslog exporter (optional).
func (s *Server) SetSIEM(e *siem.Exporter) {
	if s != nil {
		s.siem = e
	}
}

// cfgLive returns DB-merged config when the settings hub is present.
func (s *Server) cfgLive() *config.Config {
	if s.hub != nil {
		if c := s.hub.Config(); c != nil {
			return c
		}
	}
	return s.cfg
}

// Handler builds the HTTP mux (health, auth, DAV, FlowSync, admin UI).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/.well-known/mta-sts.txt", s.handleMTASTSPolicy)
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/mfa/verify", s.handleMFAVerify)
	mux.HandleFunc("/api/v1/auth/token", s.handleToken)
	mux.HandleFunc("/api/v1/auth/mfa/setup", s.handleMFASetup)
	mux.HandleFunc("/api/v1/auth/mfa/confirm", s.handleMFAConfirm)
	mux.HandleFunc("/api/v1/auth/mfa/disable", s.handleMFADisable)
	mux.HandleFunc("/api/v1/auth/webauthn/register/begin", s.handleWebAuthnRegisterBegin)
	mux.HandleFunc("/api/v1/auth/webauthn/register/finish", s.handleWebAuthnRegisterFinish)
	mux.HandleFunc("/api/v1/auth/webauthn/login/begin", s.handleWebAuthnLoginBegin)
	mux.HandleFunc("/api/v1/auth/webauthn/login/finish", s.handleWebAuthnLoginFinish)
	mux.HandleFunc("/api/v1/auth/webauthn/credentials", s.handleWebAuthnCredentials)
	mux.HandleFunc("/api/v1/auth/webauthn/credentials/", s.handleWebAuthnCredentials)
	mux.HandleFunc("/api/v1/auth/oidc/", s.handleOIDCRoutes)
	mux.HandleFunc("/api/v1/me", s.handleMe)
	mux.HandleFunc("/api/v1/me/avatar", s.handleAvatar)
	mux.HandleFunc("/api/v1/me/password", s.handleChangePassword)
	mux.HandleFunc("/api/v1/mail/signature", s.handleSignature)
	mux.HandleFunc("/api/v1/mail/image-preferences", s.handleMailImages)
	mux.HandleFunc("/api/v1/mail/", s.handleMail)
	mux.HandleFunc("/api/v1/mail", s.handleMail)
	mux.HandleFunc("/api/v1/calendar/", s.handleCalendar)
	mux.HandleFunc("/api/v1/calendar", s.handleCalendar)
	mux.HandleFunc("/api/v1/notes/", s.handleNotes)
	mux.HandleFunc("/api/v1/notes", s.handleNotes)
	mux.HandleFunc("/api/v1/contacts/", s.handleContacts)
	mux.HandleFunc("/api/v1/contacts", s.handleContacts)
	mux.HandleFunc("/api/v1/files/shares", s.handleFileShares)
	mux.HandleFunc("/api/v1/files/shares/", s.handleFileShares)
	mux.HandleFunc("/api/v1/files/", s.handleFiles)
	mux.HandleFunc("/api/v1/files", s.handleFiles)
	mux.HandleFunc("/calendar/public/", s.handlePublicCalendar)
	mux.HandleFunc("/s/", s.handlePublicShare)
	mux.HandleFunc("/api/v1/sieve/scripts", s.handleSieveScripts)
	mux.HandleFunc("/api/v1/sieve/scripts/", s.handleSieveScripts)
	mux.HandleFunc("/api/v1/admin/service-classes", s.handleAdminCoS)
	mux.HandleFunc("/api/v1/admin/service-classes/", s.handleAdminCoS)
	mux.HandleFunc("/api/v1/admin/devices", s.handleAdminDevices)
	mux.HandleFunc("/api/v1/admin/devices/", s.handleAdminDevices)
	mux.HandleFunc("/api/v1/admin/sessions", s.handleAdminSessions)
	mux.HandleFunc("/api/v1/admin/sessions/", s.handleAdminSessions)
	mux.HandleFunc("/api/v1/admin/users", s.handleAdminUsers)
	mux.HandleFunc("/api/v1/admin/users/", s.handleAdminUsers)
	mux.HandleFunc("/api/v1/admin/resources", s.handleAdminResources)
	mux.HandleFunc("/api/v1/admin/resources/", s.handleAdminResources)
	mux.HandleFunc("/api/v1/admin/tenant", s.handleAdminTenant)
	mux.HandleFunc("/api/v1/admin/tenants", s.handleAdminTenants)
	mux.HandleFunc("/api/v1/admin/tenants/", s.handleAdminTenants)
	mux.HandleFunc("/api/v1/admin/domains", s.handleAdminDomains)
	mux.HandleFunc("/api/v1/admin/domains/", s.handleAdminDomains)
	mux.HandleFunc("/api/v1/admin/tls", s.handleAdminTLS)
	mux.HandleFunc("/api/v1/admin/certs", s.handleAdminCerts)
	mux.HandleFunc("/api/v1/admin/certs/", s.handleAdminCerts)
	mux.HandleFunc("/api/v1/admin/status", s.handleAdminStatus)
	mux.HandleFunc("/api/v1/admin/outbound", s.handleAdminOutbound)
	mux.HandleFunc("/api/v1/admin/outbound/", s.handleAdminOutbound)
	mux.HandleFunc("/api/v1/admin/quarantine", s.handleAdminQuarantine)
	mux.HandleFunc("/api/v1/admin/quarantine/", s.handleAdminQuarantine)
	mux.HandleFunc("/api/v1/admin/backup", s.handleAdminBackup)
	mux.HandleFunc("/api/v1/admin/backup/restore", s.handleAdminBackupRestore)
	mux.HandleFunc("/api/v1/admin/dkim/generate", s.handleAdminDKIM)
	mux.HandleFunc("/api/v1/admin/settings", s.handleAdminSettings)
	mux.HandleFunc("/api/v1/admin/settings/", s.handleAdminSettings)
	mux.HandleFunc("/api/v1/admin/bots", s.handleAdminBots)
	mux.HandleFunc("/api/v1/admin/bots/", s.handleAdminBots)
	mux.HandleFunc("/api/v1/bots/xmpp", s.handleBotXMPP)
	mux.HandleFunc("/api/v1/bots/xmpp/", s.handleBotXMPP)
	mux.HandleFunc("/api/v1/chat/", s.handleChat)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/migration", s.handleMigration)
	mux.HandleFunc("/api/v1/migration/", s.handleMigration)
	mux.HandleFunc("/api/v1/admin/migration", s.handleAdminMigration)
	mux.HandleFunc("/api/v1/admin/mail-log", s.handleAdminMailLog)
	mux.HandleFunc("/api/v1/notifications", s.handleNotifications)
	mux.HandleFunc("/api/v1/notifications/", s.handleNotifications)

	dav.Mount(mux, s.store, s.authn)
	flowsync.Mount(mux, s.cfg, s.log, s.store, s.authn, s.ms, s.flowSubmit)

	mux.Handle("/", frontend.Handler())
	if s.tls != nil {
		return s.tls.HTTPChallengeHandler(mux)
	}
	return mux
}

func (s *Server) Start(ctx context.Context) error {
	var tlsCfg *tls.Config
	if s.tls != nil {
		tlsCfg = s.tls.HTTPConfig()
	} else {
		var err error
		tlsCfg, err = tlsutil.LoadHTTP(s.cfg)
		if err != nil {
			return err
		}
	}
	handler := s.Handler()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, srv := range s.servers {
			_ = srv.Shutdown(shutdownCtx)
		}
	}()

	httpsAddr := s.cfg.HTTP.TLSListen
	httpAddr := s.cfg.HTTP.Listen

	if httpsAddr != "" {
		if tlsCfg == nil {
			s.log.Warn("http.tls_listen set but TLS certs missing; skipping HTTPS", "addr", httpsAddr)
		} else {
			srv := &http.Server{
				Addr:              httpsAddr,
				Handler:           handler,
				TLSConfig:         tlsCfg,
				ReadHeaderTimeout: 5 * time.Second,
			}
			s.servers = append(s.servers, srv)
			ln, err := net.Listen("tcp", httpsAddr)
			if err != nil {
				return err
			}
			s.log.Info("https listening", "addr", httpsAddr)
			go func() {
				if err := srv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
					s.log.Error("https serve stopped", "err", err)
				}
			}()
		}
	}

	if httpAddr != "" {
		var h http.Handler = handler
		if s.cfg.HTTP.RedirectHTTPToTLS && httpsAddr != "" && tlsCfg != nil {
			h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				target := s.cfg.HTTP.PublicURL
				if target == "" {
					host := r.Host
					if h, _, err := net.SplitHostPort(host); err == nil {
						host = h
					}
					target = "https://" + host
					if _, port, err := net.SplitHostPort(httpsAddr); err == nil && port != "443" {
						target += ":" + port
					}
				}
				http.Redirect(w, r, strings.TrimRight(target, "/")+r.URL.RequestURI(), http.StatusMovedPermanently)
			})
		}
		srv := &http.Server{
			Addr:              httpAddr,
			Handler:           h,
			ReadHeaderTimeout: 5 * time.Second,
		}
		s.servers = append(s.servers, srv)
		s.log.Info("http listening", "addr", httpAddr)
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.log.Error("http serve stopped", "err", err)
			}
		}()
	}

	if len(s.servers) == 0 {
		return nil
	}
	return nil
}

func (s *Server) handleOIDCRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/auth/oidc/")
	if strings.HasSuffix(path, "/start") || strings.HasSuffix(path, "/start/") {
		s.handleOIDCStart(w, r)
		return
	}
	if strings.HasSuffix(path, "/callback") || strings.HasSuffix(path, "/callback/") || path == "callback" {
		s.handleOIDCCallback(w, r)
		return
	}
	// Domain-scoped callback: /api/v1/auth/oidc/{domain}/callback
	if strings.Contains(path, "/callback") {
		s.handleOIDCCallback(w, r)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) SetFlowSyncSubmit(submit flowsync.SubmitFunc) { s.flowSubmit = submit }
