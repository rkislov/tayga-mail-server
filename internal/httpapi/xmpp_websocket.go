package httpapi

import "net/http"

// Authentication happens inside XMPP SASL, never via a web cookie or URL token.
func (s *Server) handleXMPPWebSocket(w http.ResponseWriter, r *http.Request) {
	transport, ok := s.xmpp.(interface {
		ServeWebSocket(http.ResponseWriter, *http.Request)
	})
	if !ok {
		http.NotFound(w, r)
		return
	}
	transport.ServeWebSocket(w, r)
}
