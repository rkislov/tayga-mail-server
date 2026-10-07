package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/notify"
)

func (s *Server) SetNotify(h *notify.Hub) {
	s.notify = h
}

func (s *Server) userFromBearerOrQuery(r *http.Request) (*authUser, error) {
	au, err := s.userFromBearer(r)
	if err == nil && au != nil {
		return au, nil
	}
	if tok := r.URL.Query().Get("access_token"); tok != "" {
		u, err2 := s.authn.AuthenticateToken(r.Context(), "", tok)
		if err2 == nil {
			return &authUser{ID: u.ID, Email: u.Email}, nil
		}
		return nil, err2
	}
	if err == nil {
		err = auth.ErrInvalidToken
	}
	return nil, err
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearerOrQuery(r)
	if err != nil || au == nil {
		writeAuthError(w, err)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/notifications"), "/")
	if path == "stream" {
		s.handleNotificationStream(w, r, au)
		return
	}
	if path != "" || r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.notify == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	evs := s.notify.Recent(au.ID, limit)
	if evs == nil {
		evs = []notify.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": evs})
}

func (s *Server) handleNotificationStream(w http.ResponseWriter, r *http.Request, au *authUser) {
	if s.notify == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notifications unavailable"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := s.notify.Subscribe(au.ID)
	defer unsub()

	_, _ = fmt.Fprintf(w, "event: ready\ndata: {\"user_id\":%q}\n\n", au.ID)
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: notify\ndata: %s\n\n", b)
			flusher.Flush()
		}
	}
}
