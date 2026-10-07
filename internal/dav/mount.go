package dav

import (
	"net/http"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/storage"
)

// Mount registers CalDAV/CardDAV and well-known discovery on mux.
func Mount(mux *http.ServeMux, store storage.Driver, authn *auth.Layer) {
	calH := &caldav.Handler{Backend: &calBackend{store: store}, Prefix: calPrefix}
	cardH := &carddav.Handler{Backend: &cardBackend{store: store}, Prefix: cardPrefix}

	mux.Handle(calPrefix+"/", AuthMiddleware(authn, calH))
	mux.Handle(cardPrefix+"/", AuthMiddleware(authn, cardH))

	mux.HandleFunc("/.well-known/caldav", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, calPrefix+"/", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("/.well-known/carddav", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, cardPrefix+"/", http.StatusPermanentRedirect)
	})

	mux.HandleFunc(calPrefix, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == calPrefix {
			http.Redirect(w, r, calPrefix+"/", http.StatusPermanentRedirect)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc(cardPrefix, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == cardPrefix {
			http.Redirect(w, r, cardPrefix+"/", http.StatusPermanentRedirect)
			return
		}
		http.NotFound(w, r)
	})
}
