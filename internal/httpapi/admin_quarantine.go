package httpapi

import (
	"bufio"
	"bytes"
	"errors"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleAdminQuarantine(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/quarantine")
	path = strings.Trim(path, "/")

	switch {
	case path == "" && r.Method == http.MethodGet:
		s.listQuarantine(w, r, admin)
	case path != "" && r.Method == http.MethodDelete:
		s.deleteQuarantine(w, r, admin, path)
	case path != "" && r.Method == http.MethodPost && strings.HasSuffix(path, "/release"):
		id := strings.TrimSuffix(path, "/release")
		id = strings.Trim(id, "/")
		s.releaseQuarantine(w, r, admin, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func quarantineFolders(r *http.Request) []string {
	raw := r.URL.Query().Get("folder")
	if raw == "" {
		return []string{"Quarantine", "Junk"}
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"Quarantine", "Junk"}
	}
	return out
}

func (s *Server) listQuarantine(w http.ResponseWriter, r *http.Request, admin *storage.User) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}
	folders := quarantineFolders(r)
	users, err := s.store.ListUsersByTenant(r.Context(), admin.TenantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type item struct {
		ID        string `json:"id"`
		UserID    string `json:"user_id"`
		Email     string `json:"email"`
		Folder    string `json:"folder"`
		UID       int64  `json:"uid"`
		Size      int64  `json:"size"`
		MessageID string `json:"message_id"`
		From      string `json:"from"`
		Subject   string `json:"subject"`
		Date      string `json:"date"`
	}
	out := make([]item, 0, limit)
	for _, u := range users {
		for _, folder := range folders {
			mb, err := s.store.GetMailbox(r.Context(), u.ID, folder)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					continue
				}
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			msgs, err := s.store.ListMessages(r.Context(), mb.ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for i := len(msgs) - 1; i >= 0 && len(out) < limit; i-- {
				m := msgs[i]
				it := item{
					ID: m.ID, UserID: u.ID, Email: u.Email, Folder: folder,
					UID: m.UID, Size: m.Size, MessageID: m.MessageID,
					Date: m.InternalDate.UTC().Format(time.RFC3339),
				}
				if s.ms != nil && m.FilePath != "" {
					if raw, rerr := s.ms.Read(m.FilePath); rerr == nil {
						it.From, it.Subject = parseMailHeaders(raw)
					}
				}
				out = append(out, it)
			}
			if len(out) >= limit {
				break
			}
		}
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "folders": folders})
}

func (s *Server) quarantineOwned(w http.ResponseWriter, r *http.Request, admin *storage.User, id string) (*storage.Message, *storage.User, *storage.Mailbox, bool) {
	msg, err := s.store.GetMessageByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return nil, nil, nil, false
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return nil, nil, nil, false
	}
	mb, err := s.store.GetMailboxByID(r.Context(), msg.MailboxID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return nil, nil, nil, false
	}
	owner := s.adminTenantUser(w, r, admin, mb.UserID)
	if owner == nil {
		return nil, nil, nil, false
	}
	switch strings.ToLower(mb.Name) {
	case "quarantine", "junk":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is not in Quarantine/Junk"})
		return nil, nil, nil, false
	}
	return msg, owner, mb, true
}

func (s *Server) deleteQuarantine(w http.ResponseWriter, r *http.Request, admin *storage.User, id string) {
	msg, _, _, ok := s.quarantineOwned(w, r, admin, id)
	if !ok {
		return
	}
	if s.ms != nil && msg.FilePath != "" {
		_ = s.ms.Delete(msg.FilePath)
	}
	if err := s.store.DeleteMessage(r.Context(), msg.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) releaseQuarantine(w http.ResponseWriter, r *http.Request, admin *storage.User, id string) {
	msg, owner, _, ok := s.quarantineOwned(w, r, admin, id)
	if !ok {
		return
	}
	inbox, err := s.store.GetMailbox(r.Context(), owner.ID, "INBOX")
	if errors.Is(err, storage.ErrNotFound) {
		path := ""
		if s.ms != nil {
			path, _ = s.ms.EnsureFolder(owner.Email, "INBOX")
		}
		inbox, err = s.store.EnsureMailbox(r.Context(), owner.ID, "INBOX", path)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := s.store.MoveMessage(r.Context(), msg.ID, inbox.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "released", "folder": "INBOX"})
}

func parseMailHeaders(raw []byte) (from, subject string) {
	tr := textproto.NewReader(bufio.NewReader(bytes.NewReader(raw)))
	hdr, err := tr.ReadMIMEHeader()
	if err != nil {
		return "", ""
	}
	return hdr.Get("From"), hdr.Get("Subject")
}
