package httpapi

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
)

type mailImagePreferences struct {
	Mode    string   `json:"mode"`
	Senders []string `json:"senders"`
}

func (s *Server) handleMailImages(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	key := "user.mail-images." + u.ID
	value := mailImagePreferences{Mode: "block", Senders: []string{}}
	switch r.Method {
	case http.MethodGet:
		raw, _, e := s.store.GetSetting(r.Context(), key)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "read failed"})
			return
		}
		if raw != "" {
			if json.Unmarshal([]byte(raw), &value) != nil {
				writeJSON(w, 500, map[string]string{"error": "invalid stored preferences"})
				return
			}
		}
	case http.MethodPut:
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&value) != nil || value.Mode != "block" && value.Mode != "always" || len(value.Senders) > 200 {
			writeJSON(w, 400, map[string]string{"error": "invalid image preferences"})
			return
		}
		normalized := []string{}
		seen := map[string]bool{}
		for _, sender := range value.Senders {
			a, e := mail.ParseAddress(sender)
			if e != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid sender"})
				return
			}
			email := strings.ToLower(a.Address)
			if !seen[email] {
				normalized = append(normalized, email)
				seen[email] = true
			}
		}
		value.Senders = normalized
		raw, _ := json.Marshal(value)
		if s.store.PutSetting(r.Context(), key, string(raw)) != nil {
			writeJSON(w, 500, map[string]string{"error": "save failed"})
			return
		}
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, 200, value)
}
