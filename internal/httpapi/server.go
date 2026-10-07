package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
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
	"github.com/tayga/tms/internal/storage"
)

// Server exposes health, metrics, auth API, and the embedded Web UI.
type Server struct {
	log    *slog.Logger
	cfg    *config.Config
	store  storage.Driver
	authn  *auth.Layer
	ms     *mailstore.Store
	addr   string
	server *http.Server
}

func New(cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store) *Server {
	return &Server{cfg: cfg, addr: cfg.HTTP.Listen, log: log, store: store, authn: authn, ms: ms}
}

// Handler builds the HTTP mux (health, auth, DAV, FlowSync, admin UI).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
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
	mux.HandleFunc("/api/v1/admin/users", s.handleAdminUsers)
	mux.HandleFunc("/api/v1/admin/users/", s.handleAdminUsers)

	dav.Mount(mux, s.store, s.authn)
	flowsync.Mount(mux, s.cfg, s.log, s.store, s.authn, s.ms)

	mux.Handle("/", frontend.Handler())
	return mux
}

func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.log.Info("http listening", "addr", s.addr)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("http serve stopped", "err", err)
		}
	}()
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
