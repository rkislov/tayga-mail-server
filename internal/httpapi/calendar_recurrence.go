package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func (s *Server) handleCalendarRecurrence(w http.ResponseWriter, r *http.Request, au *authUser, objectID string) {
	if r.Method != http.MethodPut {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		ETag string                  `json:"etag"`
		Rule *calutil.RecurrenceRule `json:"rule"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || req.ETag == "" || req.Rule == nil {
		writeJSON(w, 400, map[string]string{"error": "etag and recurrence rule required"})
		return
	}
	o, err := s.store.GetCalendarObjectByID(r.Context(), objectID)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	cal, err := s.calendarAccessible(r, au, o.CalendarID)
	if err != nil || !calendarCanWrite(s, r, au, o.CalendarID) {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	p, err := calutil.ParseICS(o.Data)
	if err != nil {
		writeJSON(w, 409, map[string]string{"error": "invalid stored event"})
		return
	}
	if p.Organizer.Email != "" && !strings.EqualFold(p.Organizer.Email, au.Email) {
		writeJSON(w, 403, map[string]string{"error": "only the organizer can change the series"})
		return
	}
	if o.ETag != req.ETag || strings.EqualFold(p.Status, "CANCELLED") {
		writeJSON(w, 409, map[string]string{"error": "event changed; refresh before saving"})
		return
	}
	if _, err = calutil.WithRecurrence(o.Data, *req.Rule, calutil.NextSequence(o.Data)); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	saved, err := s.store.ChangeCalendarRecurrence(r.Context(), au.ID, o.ID, req.ETag, *req.Rule)
	if writeCalendarBookingError(w, err) {
		return
	}
	if err != nil {
		status := 500
		if err == storage.ErrCalendarConflict || err == storage.ErrNotFound {
			status = 409
		}
		if err == storage.ErrUnauthorized {
			status = 403
		}
		if status == 500 {
			s.log.Error("calendar recurrence update failed", "error", err)
		}
		writeJSON(w, status, map[string]string{"error": "cannot save series; refresh the calendar"})
		return
	}
	p, _ = calutil.ParseICS(saved.Data)
	writeJSON(w, 200, calendarEventJSON(saved, p, cal, au))
}

// Availability conflicts expose no data about other meetings.
func writeCalendarBookingError(w http.ResponseWriter, err error) bool {
	var booking *storage.CalendarBookingError
	if !errors.As(err, &booking) {
		return false
	}
	writeJSON(w, 409, map[string]any{"error": booking.Error(), "resource": booking.Resource, "start": booking.Start, "reason": booking.Reason})
	return true
}
