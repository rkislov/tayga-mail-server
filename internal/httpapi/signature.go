package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

type mailSignature struct {
	Text        string `json:"text"`
	NewMessages bool   `json:"new_messages"`
	Replies     bool   `json:"replies"`
}

func (s *Server) handleSignature(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	key := "user.signature." + u.ID
	switch r.Method {
	case http.MethodGet:
		raw, _, err := s.store.GetSetting(r.Context(), key)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		out := mailSignature{}
		if raw != "" {
			if err = json.Unmarshal([]byte(raw), &out); err != nil {
				writeJSON(w, 500, map[string]string{"error": "invalid stored signature"})
				return
			}
		}
		writeJSON(w, 200, out)
	case http.MethodPut:
		var value mailSignature
		if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&value); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid signature"})
			return
		}
		if len(value.Text) > 10000 || strings.ContainsRune(value.Text, '\x00') {
			writeJSON(w, 400, map[string]string{"error": "signature is too long or invalid"})
			return
		}
		raw, _ := json.Marshal(value)
		if err = s.store.PutSetting(r.Context(), key, string(raw)); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, value)
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
