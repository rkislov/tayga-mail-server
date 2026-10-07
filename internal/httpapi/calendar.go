package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func (s *Server) calendarAccessible(r *http.Request, au *authUser, calendarID string) (*storage.Calendar, error) {
	if cal, err := s.store.GetCalendarByID(r.Context(), au.ID, calendarID); err == nil {
		return cal, nil
	}
	rights, err := s.store.CalendarRightsForUser(r.Context(), calendarID, au.ID)
	if err != nil || rights == "" {
		return nil, storage.ErrNotFound
	}
	shared, err := s.store.ListSharedCalendars(r.Context(), au.ID)
	if err != nil {
		return nil, err
	}
	for _, c := range shared {
		if c.ID == calendarID {
			return c, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	au, err := s.userFromBearer(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/calendar"), "/")
	parts := splitPath(path)

	switch {
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "calendars":
		_ = s.store.EnsureDAVDefaults(r.Context(), au.ID)
		cals, err := s.store.ListCalendars(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(cals))
		seen := map[string]struct{}{}
		for _, c := range cals {
			seen[c.ID] = struct{}{}
			out = append(out, map[string]any{
				"id": c.ID, "name": c.Name, "display_name": c.DisplayName, "description": c.Description, "shared": false,
			})
		}
		if shared, err := s.store.ListSharedCalendars(r.Context(), au.ID); err == nil {
			for _, c := range shared {
				if _, ok := seen[c.ID]; ok {
					continue
				}
				out = append(out, map[string]any{
					"id": c.ID, "name": c.Name, "display_name": c.DisplayName + " (shared)", "description": c.Description, "shared": true,
				})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"calendars": out})

	case len(parts) >= 3 && parts[0] == "calendars" && parts[2] == "acl":
		s.handleCalendarACL(w, r, au, parts[1])

	case r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "calendars" && parts[2] == "events":
		cal, err := s.calendarAccessible(r, au, parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		objs, err := s.store.ListCalendarObjects(r.Context(), cal.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		from, to := parseTimeRange(r)
		out := make([]map[string]any, 0, len(objs))
		for _, o := range objs {
			ev := parseSimpleICal(o.Data)
			if from != nil && o.DTEnd != nil && o.DTEnd.Before(*from) {
				continue
			}
			if to != nil && o.DTStart != nil && o.DTStart.After(*to) {
				continue
			}
			item := map[string]any{
				"id": o.ID, "calendar_id": o.CalendarID, "uid": o.UID,
				"summary": ev.Summary, "location": ev.Location, "description": ev.Description,
			}
			if o.DTStart != nil {
				item["start"] = o.DTStart.UTC().Format(time.RFC3339)
			} else if ev.Start != "" {
				item["start"] = ev.Start
			}
			if o.DTEnd != nil {
				item["end"] = o.DTEnd.UTC().Format(time.RFC3339)
			} else if ev.End != "" {
				item["end"] = ev.End
			}
			out = append(out, item)
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": out})

	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "events":
		o, err := s.store.GetCalendarObjectByID(r.Context(), parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		cal, err := s.calendarAccessible(r, au, o.CalendarID)
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		ev := parseSimpleICal(o.Data)
		writeJSON(w, http.StatusOK, map[string]any{
			"id": o.ID, "calendar_id": cal.ID, "uid": o.UID,
			"summary": ev.Summary, "location": ev.Location, "description": ev.Description,
			"start": ev.Start, "end": ev.End, "ics": o.Data,
		})

	case (r.Method == http.MethodPost || r.Method == http.MethodPut) && len(parts) == 3 && parts[0] == "calendars" && parts[2] == "events":
		cal, err := s.store.GetCalendarByID(r.Context(), au.ID, parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			ID          string `json:"id"`
			UID         string `json:"uid"`
			Summary     string `json:"summary"`
			Location    string `json:"location"`
			Description string `json:"description"`
			Start       string `json:"start"`
			End         string `json:"end"`
			AllDay      bool   `json:"all_day"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Summary == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "summary required"})
			return
		}
		uid := req.UID
		if uid == "" {
			uid = storage.NewID()
		}
		href := uid + ".ics"
		start := parseFlexibleTime(req.Start)
		end := parseFlexibleTime(req.End)
		ics := buildSimpleVEVENT(uid, req.Summary, req.Location, req.Description, start, end, req.AllDay)
		obj := &storage.CalendarObject{
			CalendarID: cal.ID, UID: uid, HrefName: href,
			Data: ics, Size: int64(len(ics)), Component: "VEVENT",
			DTStart: start, DTEnd: end, ETag: storage.ContentETag(ics),
		}
		if req.ID != "" {
			obj.ID = req.ID
		}
		saved, err := s.store.UpsertCalendarObject(r.Context(), obj)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": saved.ID, "uid": saved.UID, "calendar_id": cal.ID})

	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "events":
		o, err := s.store.GetCalendarObjectByID(r.Context(), parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if _, err := s.store.GetCalendarByID(r.Context(), au.ID, o.CalendarID); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if err := s.store.DeleteCalendarObject(r.Context(), o.CalendarID, o.HrefName); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func parseTimeRange(r *http.Request) (*time.Time, *time.Time) {
	var from, to *time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		if t := parseFlexibleTime(v); t != nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t := parseFlexibleTime(v); t != nil {
			to = t
		}
	}
	return from, to
}

func parseFlexibleTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02T15:04",
		"2006-01-02", "20060102T150405Z", "20060102",
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}

type simpleEvent struct {
	Summary, Location, Description, Start, End string
}

func parseSimpleICal(data string) simpleEvent {
	ev := simpleEvent{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimRight(line, "\r")
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "SUMMARY:"):
			ev.Summary = icsUnescape(line[len("SUMMARY:"):])
		case strings.HasPrefix(upper, "LOCATION:"):
			ev.Location = icsUnescape(line[len("LOCATION:"):])
		case strings.HasPrefix(upper, "DESCRIPTION:"):
			ev.Description = icsUnescape(line[len("DESCRIPTION:"):])
		case strings.HasPrefix(upper, "DTSTART"):
			if i := strings.IndexByte(line, ':'); i >= 0 {
				ev.Start = strings.TrimSpace(line[i+1:])
			}
		case strings.HasPrefix(upper, "DTEND"):
			if i := strings.IndexByte(line, ':'); i >= 0 {
				ev.End = strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ev
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func icsUnescape(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return strings.TrimSpace(s)
}

func buildSimpleVEVENT(uid, summary, location, desc string, start, end *time.Time, allDay bool) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Web//EN\r\nBEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	if summary != "" {
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(summary))
	}
	if location != "" {
		fmt.Fprintf(&b, "LOCATION:%s\r\n", icsEscape(location))
	}
	if desc != "" {
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", icsEscape(desc))
	}
	if start != nil {
		if allDay {
			fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", start.Format("20060102"))
		} else {
			fmt.Fprintf(&b, "DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z"))
		}
	}
	if end != nil {
		if allDay {
			fmt.Fprintf(&b, "DTEND;VALUE=DATE:%s\r\n", end.Format("20060102"))
		} else {
			fmt.Fprintf(&b, "DTEND:%s\r\n", end.UTC().Format("20060102T150405Z"))
		}
	}
	b.WriteString("END:VEVENT\r\nEND:VCALENDAR\r\n")
	return b.String()
}
