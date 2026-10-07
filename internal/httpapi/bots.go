package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/xmpp"
)

func (s *Server) SetXMPP(gw xmpp.Gateway) {
	s.xmpp = gw
}

func (s *Server) handleAdminBots(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireGlobalAdmin(w, r); !ok {
		return
	}
	if s.xmpp == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "xmpp unavailable"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/bots"), "/")
	switch {
	case path == "" && r.Method == http.MethodGet:
		list, err := s.xmpp.ListBots(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     s.xmpp.Status(),
			"bots":       list,
			"components": s.xmpp.ComponentDomains(),
		})
	case path == "" && r.Method == http.MethodPost:
		var req struct {
			UserID     string `json:"user_id"`
			Name       string `json:"name"`
			WebhookURL string `json:"webhook_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id and name required"})
			return
		}
		info, err := s.xmpp.CreateBot(r.Context(), req.UserID, req.Name, req.WebhookURL)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, info)
	case path != "" && r.Method == http.MethodDelete:
		if err := s.xmpp.DeleteBot(r.Context(), path); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleBotXMPP(w http.ResponseWriter, r *http.Request) {
	if s.xmpp == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "xmpp unavailable"})
		return
	}
	bot, err := s.botFromToken(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bot token"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/bots/xmpp"), "/")
	switch {
	case (path == "" || path == "me") && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, bot)
	case path == "send" && r.Method == http.MethodPost:
		var req struct {
			To   string `json:"to"`
			Body string `json:"body"`
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.To == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to required"})
			return
		}
		if err := s.xmpp.SendBotMessage(r.Context(), bot.Email, req.To, req.Body, req.Type); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
	case path == "inbox" && r.Method == http.MethodGet:
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		msgs, err := s.xmpp.PollBotInbox(r.Context(), bot.ID, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Server) botFromToken(r *http.Request) (*xmpp.BotInfo, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return nil, errors.New("missing bearer")
	}
	token := strings.TrimSpace(h[7:])
	bot, err := s.xmpp.LookupBotToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	return bot, nil
}
