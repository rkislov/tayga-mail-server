package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func occurrenceJSON(o *storage.CalendarObject, instance calutil.Occurrence, cal *storage.Calendar, au *authUser) map[string]any {
	copy := *o
	copy.DTStart, copy.DTEnd = instance.Start, instance.End
	item := calendarEventJSON(&copy, instance.ParseResult, cal, au)
	item["object_id"] = o.ID
	if instance.RecurrenceID != "" {
		item["id"] = o.ID + "@" + instance.RecurrenceID
		item["recurrence_id"] = instance.RecurrenceID
	}
	master, _ := calutil.ParseICS(o.Data)
	item["occurrence_organizer"] = master.Organizer.Email == "" || strings.EqualFold(master.Organizer.Email, au.Email)
	return item
}

func (s *Server) expandedCalendarEvents(w http.ResponseWriter, r *http.Request, au *authUser, cal *storage.Calendar, objects []*storage.CalendarObject) {
	from, to := parseTimeRange(r)
	if from == nil || to == nil || !to.After(*from) || to.Sub(*from) > 366*24*time.Hour {
		writeJSON(w, 400, map[string]string{"error": "expansion requires a window of at most 366 days"})
		return
	}
	out := []map[string]any{}
	warnings := []map[string]string{}
	for _, o := range objects {
		instances, err := calutil.ExpandOccurrences(o.Data, *from, *to)
		if err != nil {
			warnings = append(warnings, map[string]string{"id": o.ID, "error": err.Error()})
			continue
		}
		for _, instance := range instances {
			out = append(out, occurrenceJSON(o, instance, cal, au))
		}
	}
	writeJSON(w, 200, map[string]any{"events": out, "warnings": warnings})
}

func (s *Server) handleCalendarOccurrence(w http.ResponseWriter, r *http.Request, au *authUser, objectID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		RecurrenceID string  `json:"recurrence_id"`
		ETag         string  `json:"etag"`
		Action       string  `json:"action"`
		Start        string  `json:"start"`
		End          string  `json:"end"`
		Summary      *string `json:"summary"`
		Location     *string `json:"location"`
		Description  *string `json:"description"`
		Geo          *string `json:"geo"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if req.RecurrenceID == "" || req.ETag == "" || (req.Action != "update" && req.Action != "cancel") {
		writeJSON(w, 400, map[string]string{"error": "recurrence_id, etag and update/cancel action required"})
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
		writeJSON(w, 403, map[string]string{"error": "only the organizer can change a meeting occurrence"})
		return
	}
	if o.ETag != req.ETag || strings.EqualFold(p.Status, "CANCELLED") {
		writeJSON(w, 409, map[string]string{"error": "event changed; refresh before saving"})
		return
	}
	change := calutil.OccurrenceChange{Cancel: req.Action == "cancel", Summary: req.Summary, Location: req.Location, Description: req.Description, Geo: req.Geo}
	if !change.Cancel {
		change.Start, change.End = parseFlexibleTime(req.Start), parseFlexibleTime(req.End)
		if change.Start == nil || change.End == nil || !change.End.After(*change.Start) || req.Summary != nil && strings.TrimSpace(*req.Summary) == "" || req.Geo != nil && !validEventGeo(*req.Geo) {
			writeJSON(w, 400, map[string]string{"error": "valid times, summary and coordinates required"})
			return
		}
	}
	if _, _, _, err := calutil.ChangeOccurrence(o.Data, req.RecurrenceID, change); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	saved, err := s.store.ChangeCalendarOccurrence(r.Context(), au.ID, o.ID, req.ETag, req.RecurrenceID, change)
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
			s.log.Error("calendar occurrence update failed", "error", err)
		}
		writeJSON(w, status, map[string]string{"error": "cannot save occurrence; refresh the calendar"})
		return
	}
	instance, err := calutil.GetOccurrence(saved.Data, req.RecurrenceID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot read saved occurrence"})
		return
	}
	writeJSON(w, 200, occurrenceJSON(saved, instance, cal, au))
}
