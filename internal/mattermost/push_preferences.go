package mattermost

import (
	"net/http"

	"github.com/tayga/tms/internal/chat"
	"github.com/tayga/tms/internal/storage"
)

func (s *API) pushPreferences(w http.ResponseWriter, r *http.Request, u *storage.User, parts []string) bool {
	if len(parts) != 3 || parts[2] != "notify_props" {
		return false
	}
	if parts[1] != "me" && parts[1] != chat.ID(u.ID) {
		s.failure(w, chat.ErrForbidden)
		return true
	}
	if r.Method != http.MethodPut {
		wrongMethod(w)
		return true
	}
	var props map[string]string
	if !decode(w, r, &props) {
		return true
	}
	mode, exists := props["push"]
	if !exists || (mode != "none" && mode != "mention" && mode != "all") {
		s.failure(w, chat.ErrInvalid)
		return true
	}
	if e := s.store.PutSetting(r.Context(), "mattermost.notify.push:"+u.ID, mode); e != nil {
		s.failure(w, e)
		return true
	}
	model := userModel(u, true)
	model["notify_props"].(map[string]string)["push"] = mode
	write(w, 200, model)
	return true
}
