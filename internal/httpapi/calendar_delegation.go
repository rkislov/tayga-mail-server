package httpapi

import (
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func calendarActsAsOrganizer(email string, cal *storage.Calendar, au *authUser) bool {
	return strings.EqualFold(email, au.Email) || cal.DelegateOrganizerEmail != "" && strings.EqualFold(email, cal.DelegateOrganizerEmail)
}
func calendarCanRespond(s *Server, r *http.Request, au *authUser, cal *storage.Calendar) bool {
	if cal.UserID == au.ID {
		return true
	}
	if cal.Name == "personal" {
		return false
	}
	d, err := s.store.GetMailboxDelegate(r.Context(), cal.UserID, au.ID)
	return err == nil && (d.CanRespondInvites || cal.Name == "default" && d.EWSCalendarPermission == "Editor")
}

func calendarOwnerLabel(s *Server, r *http.Request, ownerID string) string {
	owner, err := s.store.GetUserByID(r.Context(), ownerID)
	if err != nil {
		return ownerID
	}
	return owner.Email
}
