package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/fetchmail"
)

func (s *Server) SetMailCollector(collector *fetchmail.Service) { s.collector = collector }
func (s *Server) handleMailSources(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if s.collector == nil {
		writeJSON(w, 503, map[string]string{"error": "mail collection unavailable"})
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/mail-sources"), "/"), "/")
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	fail := func(err error) {
		status := 400
		if errors.Is(err, fetchmail.ErrNotFound) {
			status = 404
		}
		if errors.Is(err, fetchmail.ErrBusy) {
			status = 409
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
	}
	if id == "" && r.Method == http.MethodGet {
		sources, e := s.collector.List(r.Context(), u.ID)
		if e != nil {
			fail(e)
			return
		}
		writeJSON(w, 200, map[string]any{"sources": sources})
		return
	}
	if action == "run" && id != "" && r.Method == http.MethodPost {
		if e := s.collector.RunNow(r.Context(), u.ID, id); e != nil {
			fail(e)
			return
		}
		writeJSON(w, 202, map[string]string{"status": "scheduled"})
		return
	}
	if action == "" && id != "" && id != "test" && r.Method == http.MethodDelete {
		if e := s.collector.Delete(r.Context(), u.ID, id); e != nil {
			fail(e)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "deleted"})
		return
	}
	testing := r.Method == http.MethodPost && ((id == "test" && action == "") || action == "test")
	if testing || (id == "" && r.Method == http.MethodPost) || (id != "" && action == "" && r.Method == http.MethodPut) {
		var req fetchmail.Settings
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid source settings"})
			return
		}
		if testing {
			if id == "test" {
				id = ""
			}
			if e := s.collector.Test(r.Context(), u.ID, id, req); e != nil {
				fail(e)
				return
			}
			writeJSON(w, 200, map[string]string{"status": "connected"})
			return
		}
		source, e := s.collector.Save(r.Context(), u.ID, id, req)
		if e != nil {
			fail(e)
			return
		}
		writeJSON(w, 200, source)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
