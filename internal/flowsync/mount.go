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

// Mount registers proprietary FlowSync endpoints (ActiveSync- and EWS-compatible).
func Mount(mux *http.ServeMux, cfg *config.Config, log *slog.Logger, store storage.Driver, authn *auth.Layer, ms *mailstore.Store) {
	if cfg == nil || !cfg.FlowSync.Enabled {
		log.Info("flowsync disabled")
		return
	}
	public := strings.TrimRight(cfg.HTTP.PublicURL, "/")
	if public == "" {
		public = "http://127.0.0.1" + cfg.HTTP.Listen
	}

	ad := &autodiscover{publicURL: public, hostname: cfg.Server.Hostname}
	eas := &easHandler{store: store, ms: ms}
	ews := &ewsHandler{store: store, ms: ms, publicURL: public}

	mux.HandleFunc("/Autodiscover/Autodiscover.xml", ad.handlePOX)
	mux.HandleFunc("/autodiscover/autodiscover.xml", ad.handlePOX)
	mux.HandleFunc("/autodiscover/autodiscover.json/", ad.handleJSON)

	mux.Handle("/Microsoft-Server-ActiveSync", authMiddleware(authn, eas))
	mux.Handle("/Microsoft-Server-ActiveSync/", authMiddleware(authn, eas))
	mux.Handle("/EWS/Exchange.asmx", authMiddleware(authn, ews))
	mux.Handle("/ews/exchange.asmx", authMiddleware(authn, ews))

	log.Info("flowsync enabled",
		"engine", "FlowSync/1.0",
		"activesync", "/Microsoft-Server-ActiveSync",
		"ews", "/EWS/Exchange.asmx",
		"proprietary", true,
	)
}
