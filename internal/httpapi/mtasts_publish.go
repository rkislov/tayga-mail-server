package httpapi

import (
	"fmt"
	"net/http"
	"strings"
)

// handleMTASTSPolicy serves RFC 8461 policy at /.well-known/mta-sts.txt.
// Point mta-sts.<domain> A/AAAA (and TLS cert SAN) at this host.
func (s *Server) handleMTASTSPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pub := s.cfg.SMTP.MTASTS.Publish
	if !pub.Enabled {
		http.NotFound(w, r)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "version: STSv1\n")
	fmt.Fprintf(&b, "mode: %s\n", pub.Mode)
	for _, mx := range pub.MX {
		mx = strings.TrimSpace(mx)
		if mx == "" {
			continue
		}
		fmt.Fprintf(&b, "mx: %s\n", strings.ToLower(strings.TrimSuffix(mx, ".")))
	}
	fmt.Fprintf(&b, "max_age: %d\n", pub.MaxAge)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", min(pub.MaxAge, 86400)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(b.String()))
}
