package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/auth"
)

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.xmpp == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "chat unavailable"})
		return
	}
	au, err := s.userFromBearer(r)
	if err != nil {
		// EventSource cannot set Authorization — allow ?access_token=
		if tok := r.URL.Query().Get("access_token"); tok != "" {
			u, err2 := s.authn.AuthenticateToken(r.Context(), "", tok)
			if err2 == nil {
				au = &authUser{ID: u.ID, Email: u.Email}
				err = nil
			}
		}
	}
	if err != nil || au == nil {
		if err == nil {
			err = auth.ErrInvalidToken
		}
		writeAuthError(w, err)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/chat"), "/")
	switch {
	case path == "roster" && r.Method == http.MethodGet:
		list, err := s.xmpp.ListChatRoster(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"roster": list, "me": au.Email})
	case path == "roster" && r.Method == http.MethodPost:
		var req struct {
			JID  string `json:"jid"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "jid required"})
			return
		}
		if err := s.xmpp.UpsertChatRoster(r.Context(), au.ID, req.JID, req.Name); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case path == "history" && r.Method == http.MethodGet:
		with := r.URL.Query().Get("with")
		if with == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "with required"})
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		msgs, err := s.xmpp.ChatHistory(r.Context(), au.Email, with, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
	case path == "send" && r.Method == http.MethodPost:
		var req struct {
			To   string `json:"to"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.To == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to required"})
			return
		}
		ev, err := s.xmpp.SendChatMessage(r.Context(), au.Email, req.To, req.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = s.xmpp.UpsertChatRoster(r.Context(), au.ID, req.To, "")
		writeJSON(w, http.StatusOK, ev)
	case path == "events" && r.Method == http.MethodGet:
		s.handleChatSSE(w, r, au.Email)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) handleChatSSE(w http.ResponseWriter, r *http.Request, email string) {
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

	ch, unsub := s.xmpp.SubscribeChat(email)
	defer unsub()

	_, _ = fmt.Fprintf(w, "event: ready\ndata: {\"me\":%q}\n\n", email)
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
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
			flusher.Flush()
		}
	}
}
