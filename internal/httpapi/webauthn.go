package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type webauthnSessionBody struct {
	SessionID  string          `json:"session_id"`
	Name       string          `json:"name"`
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential"`
}

type webauthnChallengeBody struct {
	Challenge string `json:"challenge"`
}

func (s *Server) handleWebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if s.authn.WebAuthn == nil || !s.authn.WebAuthn.Enabled() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webauthn disabled"})
		return
	}
	su, err := s.store.GetUserByID(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	opts, sid, err := s.authn.WebAuthn.BeginRegistration(r.Context(), su)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	// opts is *protocol.CredentialCreation with json:"publicKey" — browsers use options.publicKey.
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sid,
		"options":    opts,
	})
}

func (s *Server) handleWebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if s.authn.WebAuthn == nil || !s.authn.WebAuthn.Enabled() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webauthn disabled"})
		return
	}
	var req webauthnSessionBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.SessionID == "" || len(req.Credential) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id and credential required"})
		return
	}
	su, err := s.store.GetUserByID(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	cred, err := s.authn.WebAuthn.FinishRegistration(r.Context(), su, req.SessionID, req.Name, req.Credential)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   cred.ID,
		"name": cred.Name,
	})
}

func (s *Server) handleWebAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.authn.WebAuthn == nil || !s.authn.WebAuthn.Enabled() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webauthn disabled"})
		return
	}
	var req webauthnChallengeBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	opts, sid, err := s.authn.WebAuthn.BeginLogin(r.Context(), req.Challenge)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sid,
		"options":    opts,
	})
}

func (s *Server) handleWebAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.authn.WebAuthn == nil || !s.authn.WebAuthn.Enabled() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webauthn disabled"})
		return
	}
	var req webauthnSessionBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	pair, err := s.authn.WebAuthn.FinishLogin(r.Context(), req.Challenge, req.SessionID, req.Credential)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func (s *Server) handleWebAuthnCredentials(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/auth/webauthn/credentials")
	path = strings.Trim(path, "/")

	if s.authn.WebAuthn == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webauthn disabled"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		if path != "" {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		list, err := s.authn.WebAuthn.ListCredentials(r.Context(), u.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"credentials": list})
	case http.MethodDelete:
		if path == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "credential id required"})
			return
		}
		if err := s.authn.WebAuthn.DeleteCredential(r.Context(), u.ID, path); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
