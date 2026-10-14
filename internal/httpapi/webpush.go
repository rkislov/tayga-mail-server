package httpapi

import (
	"encoding/json"
	push "github.com/SherClockHolmes/webpush-go"
	"github.com/tayga/tms/internal/webpush"
	"net/http"
)

func (s *Server) SetWebPush(p *webpush.Service) { s.webpush = p }
func (s *Server) handleWebPush(w http.ResponseWriter, r *http.Request) {
	user, err := s.userFromBearer(r)
	if err != nil || user == nil {
		writeAuthError(w, err)
		return
	}
	if s.webpush == nil {
		writeJSON(w, 503, map[string]string{"error": "push unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]string{"public_key": s.webpush.PublicKey()})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.WriteHeader(405)
		return
	}
	var sub push.Subscription
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&sub) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid subscription"})
		return
	}
	if r.Method == http.MethodPost {
		err = s.webpush.Subscribe(r.Context(), user.ID, sub)
	} else {
		err = s.webpush.Unsubscribe(r.Context(), user.ID, sub.Endpoint)
	}
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(204)
}
