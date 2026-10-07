package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleAdminMailLog(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireDomainAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	query := storage.MailLogQuery{
		Q:      q,
		Limit:  limit,
		Offset: offset,
	}

	scope := "global"
	if s.isGlobalAdminUser(admin) {
		// Global admin sees the full mail log (all domains / tenants).
		scope = "global"
	} else {
		scope = "domain"
		names, err := s.adminDomainNames(r, admin)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if len(names) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []storage.MailLogEntry{},
				"scope": scope,
				"q":     q,
			})
			return
		}
		query.Domains = names
		query.TenantID = admin.TenantID
	}

	items, err := s.store.SearchMailLog(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		items = []storage.MailLogEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"scope":  scope,
		"q":      q,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) adminDomainNames(r *http.Request, admin *storage.User) ([]string, error) {
	ids, err := s.store.ListDomainAdminDomains(r.Context(), admin.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var names []string
	for _, id := range ids {
		d, err := s.store.GetDomainByID(r.Context(), id)
		if err != nil || d == nil {
			continue
		}
		n := strings.ToLower(strings.TrimSpace(d.Name))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	// Fallback: domain of the admin's own mailbox.
	if len(names) == 0 {
		if n := storage.DomainOfEmail(admin.Email); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}
