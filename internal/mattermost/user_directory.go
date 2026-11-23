package mattermost

import (
	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type userSearch struct {
	Term           string `json:"term"`
	TeamID         string `json:"team_id"`
	NotInTeamID    string `json:"not_in_team_id"`
	InChannelID    string `json:"in_channel_id"`
	NotInChannelID string `json:"not_in_channel_id"`
	Limit          int    `json:"limit"`
}

// Directory discovery is bounded to the authenticated user's domain. Channel
// filters additionally require membership so private channel rosters stay private.
func (s *API) directoryUsers(w http.ResponseWriter, r *http.Request, u *storage.User, path string) bool {
	if path != "/users/search" && path != "/users/autocomplete" && path != "/users/known" && path != "/users/usernames" && !strings.HasPrefix(path, "/users/username/") && !strings.HasPrefix(path, "/users/email/") {
		return false
	}
	req := userSearch{Limit: 100}
	autocomplete := path == "/users/autocomplete"
	var names []string
	switch {
	case path == "/users/search" && r.Method == http.MethodPost:
		if !decode(w, r, &req) {
			return true
		}
		if req.Limit == 0 {
			req.Limit = 100
		}
	case autocomplete && r.Method == http.MethodGet:
		q := r.URL.Query()
		req.Term = q.Get("name")
		req.TeamID = q.Get("in_team")
		req.InChannelID = q.Get("in_channel")
		if raw := q.Get("limit"); raw != "" {
			var e error
			req.Limit, e = strconv.Atoi(raw)
			if e != nil {
				s.failure(w, chat.ErrInvalid)
				return true
			}
		}
	case path == "/users/known" && r.Method == http.MethodGet:
	case path == "/users/usernames" && r.Method == http.MethodPost:
		if !decode(w, r, &names) {
			return true
		}
		if names == nil || len(names) > 200 {
			s.failure(w, chat.ErrInvalid)
			return true
		}
	case (strings.HasPrefix(path, "/users/username/") || strings.HasPrefix(path, "/users/email/")) && r.Method == http.MethodGet:
	default:
		wrongMethod(w)
		return true
	}
	if req.Limit < 1 || req.Limit > 200 || len(req.Term) > 256 || (req.InChannelID != "" && req.NotInChannelID != "") {
		s.failure(w, chat.ErrInvalid)
		return true
	}
	if req.TeamID != "" && req.TeamID != chat.ID(u.TenantID) || req.NotInTeamID != "" && req.NotInTeamID != chat.ID(u.TenantID) {
		s.failure(w, storage.ErrNotFound)
		return true
	}
	members := map[string]bool{}
	filterChannel := req.InChannelID
	if filterChannel == "" {
		filterChannel = req.NotInChannelID
	}
	if filterChannel != "" {
		list, e := s.chat.Members(r.Context(), u, filterChannel)
		if e != nil {
			s.failure(w, e)
			return true
		}
		for _, m := range list {
			members[m.UserID] = true
		}
	}
	users, e := s.store.ListUsersByDomain(r.Context(), u.DomainID)
	if e != nil {
		s.failure(w, e)
		return true
	}
	sort.Slice(users, func(i, j int) bool {
		a, b := strings.ToLower(users[i].DisplayLabel()), strings.ToLower(users[j].DisplayLabel())
		if a == b {
			return users[i].ID < users[j].ID
		}
		return a < b
	})
	out, other, known := []any{}, []any{}, []string{}
	term := strings.ToLower(strings.TrimSpace(req.Term))
	for _, target := range users {
		if !s.federatedUserVisible(r.Context(), u, target) || target.TenantID != u.TenantID || target.AuthSource == "resource" {
			continue
		}
		model := userModel(target, target.ID == u.ID)
		matches := func(name string) bool {
			return strings.EqualFold(name, target.LocalPart) || strings.EqualFold(name, model["username"].(string))
		}
		if strings.HasPrefix(path, "/users/username/") {
			if matches(strings.TrimPrefix(path, "/users/username/")) {
				write(w, 200, model)
				return true
			}
			continue
		}
		if strings.HasPrefix(path, "/users/email/") {
			if strings.EqualFold(strings.TrimPrefix(path, "/users/email/"), target.Email) {
				write(w, 200, model)
				return true
			}
			continue
		}
		if names != nil {
			found := false
			for _, name := range names {
				if matches(name) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if term != "" && !strings.Contains(strings.ToLower(target.DisplayLabel()+" "+target.Email+" "+target.LocalPart), term) {
			continue
		}
		if req.NotInTeamID != "" {
			continue
		}
		inChannel := members[target.ID]
		if req.NotInChannelID != "" && inChannel {
			continue
		}
		if autocomplete && req.InChannelID != "" && !inChannel {
			if len(other) < req.Limit {
				other = append(other, model)
			}
			continue
		}
		if !autocomplete && req.InChannelID != "" && !inChannel {
			continue
		}
		if len(out) < req.Limit {
			out = append(out, model)
			known = append(known, chat.ID(target.ID))
		}
	}
	if strings.HasPrefix(path, "/users/username/") || strings.HasPrefix(path, "/users/email/") {
		s.failure(w, storage.ErrNotFound)
		return true
	}
	if autocomplete {
		write(w, 200, map[string]any{"users": out, "out_of_channel": other, "agents": []any{}})
	} else if path == "/users/known" {
		write(w, 200, known)
	} else {
		write(w, 200, out)
	}
	return true
}
