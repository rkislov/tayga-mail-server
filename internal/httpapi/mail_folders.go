package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/tayga/tms/internal/storage"
)

func validFolderName(name string) bool {
	if name == "" || len(name) > 200 || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\\`) {
		return false
	}
	for _, c := range name {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func (s *Server) folderName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid folder name"})
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if !validFolderName(name) {
		writeJSON(w, 400, map[string]string{"error": "invalid folder name"})
		return "", false
	}
	return name, true
}
func (s *Server) mailCreateFolder(w http.ResponseWriter, r *http.Request, au *authUser) {
	name, ok := s.folderName(w, r)
	if !ok {
		return
	}
	if storage.SystemMailboxRole(name) != "" {
		writeJSON(w, 409, map[string]string{"error": "reserved system folder"})
		return
	}
	if _, err := s.store.GetMailbox(r.Context(), au.ID, name); !errors.Is(err, storage.ErrNotFound) {
		writeJSON(w, 409, map[string]string{"error": "folder already exists"})
		return
	}
	path, err := s.ms.EnsureFolder(au.Email, name)
	if err == nil {
		_, err = s.store.CreateMailbox(r.Context(), au.ID, name, path)
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created"})
}
func (s *Server) mailChangeFolder(w http.ResponseWriter, r *http.Request, au *authUser, id string) {
	mb, code := s.mailboxOwned(r, au, id)
	if code != 0 {
		writeJSON(w, code, map[string]string{"error": http.StatusText(code)})
		return
	}
	if mb.UserID != au.ID || storage.SystemMailboxRole(mb.Name) != "" {
		writeJSON(w, 403, map[string]string{"error": "system and shared folders cannot be changed"})
		return
	}
	var err error
	if r.Method == http.MethodDelete {
		// Remove the index first: if that fails, leave the message files intact.
		err = s.store.DeleteMailbox(r.Context(), au.ID, mb.Name)
		if err == nil {
			err = s.ms.RemoveFolder(au.Email, mb.Name)
		}
	} else {
		name, ok := s.folderName(w, r)
		if !ok {
			return
		}
		if storage.SystemMailboxRole(name) != "" {
			writeJSON(w, 403, map[string]string{"error": "reserved system folder"})
			return
		}
		if name == mb.Name {
			writeJSON(w, 200, map[string]string{"status": "updated"})
			return
		}
		if _, e := s.store.GetMailbox(r.Context(), au.ID, name); !errors.Is(e, storage.ErrNotFound) {
			writeJSON(w, 409, map[string]string{"error": "folder already exists"})
			return
		}
		err = s.ms.RenameFolder(au.Email, mb.Name, name)
		if err == nil {
			path, e := s.ms.EnsureFolder(au.Email, name)
			err = e
			if err == nil {
				err = s.store.RenameMailbox(r.Context(), au.ID, mb.Name, name, path)
			}
			if err != nil {
				_ = s.ms.RenameFolder(au.Email, name, mb.Name)
			}
		}
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "updated"})
}
func (s *Server) mailFolderOrder(w http.ResponseWriter, r *http.Request, au *authUser) {
	var req struct {
		Order []string `json:"order"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid order"})
		return
	}
	seen := map[string]bool{}
	for i, id := range req.Order {
		mb, code := s.mailboxOwned(r, au, id)
		if code != 0 || seen[id] || (mb != nil && mb.UserID == au.ID && storage.SystemMailboxRole(mb.Name) == "inbox" && i != 0) {
			writeJSON(w, 400, map[string]string{"error": "invalid order; INBOX must be first"})
			return
		}
		seen[id] = true
	}
	raw, _ := json.Marshal(req.Order)
	if err := s.store.PutSetting(r.Context(), "ui.mailfolders."+au.ID, string(raw)); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved"})
}
