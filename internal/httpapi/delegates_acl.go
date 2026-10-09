package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleDelegates(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	ownerID := au.ID
	if requested := r.URL.Query().Get("owner_id"); requested != "" && requested != au.ID {
		admin, ok := s.requireAdmin(w, r)
		if !ok {
			return
		}
		owner, err := s.store.GetUserByID(r.Context(), requested)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "owner not found"})
			return
		}
		if !s.adminCanManageDomain(r, admin, owner.DomainID) {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		ownerID = requested
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/mail/delegates"), "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		mine, err := s.store.ListMailboxDelegates(r.Context(), ownerID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot list delegates"})
			return
		}
		serialized := serializeDelegates(mine)
		for i, d := range mine {
			if user, err := s.store.GetUserByID(r.Context(), d.DelegateID); err == nil {
				serialized[i]["email"] = user.Email
			}
		}
		forMe, _ := s.store.ListDelegationsFor(r.Context(), au.ID)
		writeJSON(w, http.StatusOK, map[string]any{
			"delegates":  serialized,
			"acting_for": serializeDelegates(forMe),
		})

	case r.Method == http.MethodPost && path == "":
		var req struct {
			Email           string `json:"email"`
			CanRead         *bool  `json:"can_read"`
			CanSendAs       bool   `json:"can_send_as"`
			CanSendOnBehalf bool   `json:"can_send_on_behalf"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		delegate, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		if delegate.ID == ownerID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delegate to self"})
			return
		}
		canRead := true
		if req.CanRead != nil {
			canRead = *req.CanRead
		}
		d := &storage.MailboxDelegate{
			OwnerID: ownerID, DelegateID: delegate.ID,
			CanRead: canRead, CanSendAs: req.CanSendAs, CanSendOnBehalf: req.CanSendOnBehalf,
		}
		if err := s.store.UpsertMailboxDelegate(r.Context(), d); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "delegate_id": delegate.ID})

	case r.Method == http.MethodDelete && path != "":
		if err := s.store.DeleteMailboxDelegate(r.Context(), ownerID, path); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func serializeDelegates(list []*storage.MailboxDelegate) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{
			"owner_id": d.OwnerID, "delegate_id": d.DelegateID,
			"can_read": d.CanRead, "can_send_as": d.CanSendAs, "can_send_on_behalf": d.CanSendOnBehalf,
		})
	}
	return out
}

func (s *Server) handleMailboxACL(w http.ResponseWriter, r *http.Request, au *authUser, mailboxID string) {
	mb, err := s.store.GetMailboxByID(r.Context(), mailboxID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if mb.UserID != au.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "owner required"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/mail/mailboxes/"+mailboxID+"/acl")
	path = strings.Trim(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		entries, err := s.store.ListMailboxACL(r.Context(), mailboxID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			email := ""
			if u, err := s.store.GetUserByID(r.Context(), e.GranteeUserID); err == nil {
				email = u.Email
			}
			out = append(out, map[string]any{"user_id": e.GranteeUserID, "email": email, "rights": e.Rights})
		}
		writeJSON(w, http.StatusOK, map[string]any{"acl": out})

	case r.Method == http.MethodPut && path == "":
		var req struct {
			Email  string `json:"email"`
			Rights string `json:"rights"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		u, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		rights := strings.TrimSpace(req.Rights)
		if rights == "" {
			rights = "lr"
		}
		if err := s.store.SetMailboxACL(r.Context(), mailboxID, u.ID, rights); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case r.Method == http.MethodDelete && path != "":
		if err := s.store.DeleteMailboxACL(r.Context(), mailboxID, path); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleCalendarACL(w http.ResponseWriter, r *http.Request, au *authUser, calendarID string) {
	cal, err := s.store.GetCalendarByID(r.Context(), au.ID, calendarID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	_ = cal
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/calendar/calendars/"+calendarID+"/acl")
	path = strings.Trim(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		entries, err := s.store.ListCalendarACL(r.Context(), calendarID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			email := ""
			if u, err := s.store.GetUserByID(r.Context(), e.GranteeUserID); err == nil {
				email = u.Email
			}
			out = append(out, map[string]any{"user_id": e.GranteeUserID, "email": email, "rights": e.Rights})
		}
		writeJSON(w, http.StatusOK, map[string]any{"acl": out})

	case r.Method == http.MethodPut && path == "":
		var req struct {
			Email  string `json:"email"`
			Rights string `json:"rights"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		u, err := s.store.GetUserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		rights := strings.TrimSpace(req.Rights)
		if rights == "" {
			rights = "read"
		}
		if rights != "read" && rights != "write" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rights must be read or write"})
			return
		}
		if u.ID == au.ID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner already has access"})
			return
		}
		if err := s.store.SetCalendarACL(r.Context(), calendarID, u.ID, rights); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case r.Method == http.MethodDelete && path != "":
		if err := s.store.DeleteCalendarACL(r.Context(), calendarID, path); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
