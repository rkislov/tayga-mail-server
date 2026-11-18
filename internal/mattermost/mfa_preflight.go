package mattermost

import (
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

// MFA discovery precedes login in native clients. It never establishes a session
// or changes the second-factor checks in LoginWithPassword/CompleteMFA.
func (s *API) mfaRequired(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LoginID string `json:"login_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	login := strings.ToLower(strings.TrimSpace(req.LoginID))
	if login == "" || len(login) > 254 {
		s.failure(w, chat.ErrInvalid)
		return
	}
	required := false
	u, err := s.store.GetUserByEmail(r.Context(), login)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		s.failure(w, err)
		return
	}
	if err == nil && u.Enabled && u.AuthSource != "resource" {
		policy, e := s.policy.Resolve(r.Context(), userScope(u))
		if e != nil {
			s.failure(w, e)
			return
		}
		if policy.Enabled {
			mfa, e := s.store.GetUserMFA(r.Context(), u.ID)
			if e != nil && !errors.Is(e, storage.ErrNotFound) {
				s.failure(w, e)
				return
			}
			required = e == nil && mfa.TOTPEnabled
			n, e := s.store.CountWebAuthnCredentials(r.Context(), u.ID)
			if e != nil {
				s.failure(w, e)
				return
			}
			required = required || n > 0
		}
	}
	write(w, 200, map[string]bool{"mfa_required": required})
}
