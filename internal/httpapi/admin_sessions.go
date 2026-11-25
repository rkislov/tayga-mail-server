package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/tayga/tms/internal/auth"
	"github.com/tayga/tms/internal/chat"
)

func (s *Server) handleAdminSessions(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/sessions"), "/")
	w.Header().Set("Cache-Control", "no-store")
	ctx := r.Context()
	token := auth.ParseBearerToken(r.Header.Get("Authorization"))
	current, _ := s.store.GetOAuthTokenByAccess(ctx, token)
	currentHash := sha256.Sum256([]byte("Web\x00" + token))
	currentWebID := hex.EncodeToString(currentHash[:])
	isCurrent := func(id string) bool { return current != nil && id == current.ID }
	messenger := chat.New(s.store)
	if r.Method == http.MethodGet && id == "" {
		type sessionRow struct {
			auth.SessionInfo
			IsCurrent bool `json:"is_current,omitempty"`
		}
		out := []sessionRow{}
		live := make(map[string]auth.SessionInfo)
		matched := make(map[string]bool)
		for _, observed := range s.authn.Sessions() {
			live[observed.ID] = observed
		}
		if s.mattermostAPI != nil {
			sessions, err := messenger.ActiveSessions(ctx)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "cannot list Mattermost sessions"})
				return
			}
			for _, item := range sessions {
				user, err := s.store.GetUserByID(ctx, item.UserID)
				if err != nil || item.DomainID != user.DomainID || item.TenantID != user.TenantID || !s.adminCanManageDomain(r, admin, user.DomainID) {
					continue
				}
				observed := live[item.ObservedID]
				matched[item.ObservedID] = true
				out = append(out, sessionRow{
					SessionInfo: auth.SessionInfo{ID: "mm-" + item.ID, UserID: user.ID, Email: user.Email, Protocol: "Mattermost", Address: observed.Address, Agent: observed.Agent, Started: time.UnixMilli(item.CreatedAt), Seen: time.UnixMilli(item.LastActivityAt)},
					IsCurrent:   isCurrent(item.ID),
				})
			}
		}
		for _, item := range live {
			if matched[item.ID] || strings.EqualFold(item.Protocol, "XMPP") {
				continue
			}
			user, err := s.store.GetUserByID(r.Context(), item.UserID)
			if err == nil && s.adminCanManageDomain(r, admin, user.DomainID) {
				out = append(out, sessionRow{SessionInfo: item, IsCurrent: item.Protocol == "Web" && item.ID == currentWebID})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Seen.After(out[j].Seen) })
		writeJSON(w, 200, map[string]any{"sessions": out})
		return
	}
	if r.Method == http.MethodDelete && id != "" {
		if sessionID, ok := strings.CutPrefix(id, "mm-"); ok {
			item, err := messenger.Session(ctx, sessionID)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "session not found"})
				return
			}
			user, err := s.store.GetUserByID(ctx, item.UserID)
			if err != nil || item.DomainID != user.DomainID || item.TenantID != user.TenantID || !s.adminCanManageDomain(r, admin, user.DomainID) {
				writeJSON(w, 404, map[string]string{"error": "session not found"})
				return
			}
			if isCurrent(sessionID) {
				writeJSON(w, 409, map[string]string{"error": "cannot end the current session"})
				return
			}
			if err := s.store.DeleteOAuthToken(ctx, sessionID); err != nil {
				writeJSON(w, 500, map[string]string{"error": "cannot close session"})
				return
			}
			s.authn.ForgetSession(item.ObservedID)
			writeJSON(w, 200, map[string]string{"status": "closed"})
			return
		}
		if id == currentWebID {
			writeJSON(w, 409, map[string]string{"error": "cannot end the current session"})
			return
		}
		owner := s.authn.SessionOwner(id)
		user, err := s.store.GetUserByID(r.Context(), owner)
		if err != nil || !s.adminCanManageDomain(r, admin, user.DomainID) {
			writeJSON(w, 404, map[string]string{"error": "session not found"})
			return
		}
		if err = s.authn.CloseSession(id); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot close session"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "closed"})
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
