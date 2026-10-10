// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// Mount registers original FlowSync endpoints (ActiveSync- and EWS-compatible).
func Mount(mux *http.ServeMux, cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store) {
	if cfg == nil || !cfg.FlowSync.Enabled {
		log.Info("flowsync disabled")
		return
	}
	public := strings.TrimRight(cfg.HTTP.PublicURL, "/")
	if public == "" {
		public = "http://127.0.0.1" + cfg.HTTP.Listen
	}

	ad := &autodiscover{publicURL: public, hostname: cfg.Server.Hostname, cfg: cfg, store: store}
	eas := &easHandler{store: store, ms: ms}
	ews := &ewsHandler{store: store, ms: ms, publicURL: public}

	mux.Handle("/Autodiscover/Autodiscover.xml", logRequests(log, "autodiscover", http.HandlerFunc(ad.handlePOX), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/autodiscover/autodiscover.xml", logRequests(log, "autodiscover", http.HandlerFunc(ad.handlePOX), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/Autodiscover/", logRequests(log, "autodiscover", http.HandlerFunc(ad.handlePOX), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/autodiscover/autodiscover.json", logRequests(log, "autodiscover", http.HandlerFunc(ad.handleJSON), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/autodiscover/autodiscover.json/", logRequests(log, "autodiscover", http.HandlerFunc(ad.handleJSON), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/.well-known/autoconfig/mail/config-v1.1.xml", logRequests(log, "autodiscover", http.HandlerFunc(ad.handleMozilla), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/mail/config-v1.1.xml", logRequests(log, "autodiscover", http.HandlerFunc(ad.handleMozilla), cfg.FlowSync.ProtocolDebug))

	mux.Handle("/Microsoft-Server-ActiveSync", logRequests(log, "activesync", authMiddleware(authn, eas), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/Microsoft-Server-ActiveSync/", logRequests(log, "activesync", authMiddleware(authn, eas), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/EWS/Exchange.asmx", logRequests(log, "ews", authMiddleware(authn, ews), cfg.FlowSync.ProtocolDebug))
	mux.Handle("/ews/exchange.asmx", logRequests(log, "ews", authMiddleware(authn, ews), cfg.FlowSync.ProtocolDebug))

	log.Info("flowsync enabled",
		"engine", "FlowSync/1.0",
		"activesync", "/Microsoft-Server-ActiveSync",
		"ews", "/EWS/Exchange.asmx",
		"autodiscover", "/Autodiscover/Autodiscover.xml",
		"mozilla_autoconfig", "/.well-known/autoconfig/mail/config-v1.1.xml",
		"license", "Apache-2.0",
	)
}
