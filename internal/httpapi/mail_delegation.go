package httpapi

import (
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) mailboxAllows(r *http.Request, au *authUser, mb *storage.Mailbox, right string) bool {
	if mb.UserID == au.ID {
		return true
	}
	rights, err := s.store.MailboxRightsForUser(r.Context(), mb.ID, au.ID)
	return err == nil && strings.Contains(rights, right)
}
