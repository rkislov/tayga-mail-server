package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
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
	case r.Method == http.MethodPut && path == "default":
		var req struct {
			CalendarID string `json:"calendar_id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if _, err := s.store.GetCalendarByID(r.Context(), au.ID, req.CalendarID); err != nil {
			writeJSON(w, 404, map[string]string{"error": "owned calendar required"})
			return
		}
		if err := s.store.PutSetting(r.Context(), "calendar.default."+au.ID, req.CalendarID); err != nil {
			writeJSON(w, 500, map[string]string{"error": "cannot save default"})
			return
		}
		writeJSON(w, 200, map[string]string{"default_calendar_id": req.CalendarID})
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "calendars":
		_ = s.store.EnsureDAVDefaults(r.Context(), au.ID)
		cals, err := s.store.ListCalendars(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defaultID, _, _ := s.store.GetSetting(r.Context(), "calendar.default."+au.ID)
		validDefault := false
		for _, c := range cals {
			if c.ID == defaultID {
				validDefault = true
			}
		}
		if !validDefault {
			for _, c := range cals {
				if c.Name == "default" {
					defaultID = c.ID
					break
				}
			}
		}
		out := make([]map[string]any, 0, len(cals))
		seen := map[string]struct{}{}
		for _, c := range cals {
			seen[c.ID] = struct{}{}
			out = append(out, map[string]any{
				"id": c.ID, "name": c.Name, "display_name": c.DisplayName, "description": c.Description, "color": c.Color,
				"is_default": c.ID == defaultID, "shared": false, "owned": true, "writable": true, "deletable": !builtinCalendar(c.Name), "busy_only": c.Name == "personal",
			})
		}
		if shared, err := s.store.ListSharedCalendars(r.Context(), au.ID); err == nil {
			for _, c := range shared {
				if _, ok := seen[c.ID]; ok {
					continue
				}
				out = append(out, map[string]any{
					"id": c.ID, "name": c.Name, "display_name": c.DisplayName, "description": c.Description, "color": c.Color,
					"busy_only": c.Name == "personal", "shared": true, "owned": false, "writable": calendarCanWrite(s, r, au, c.ID), "deletable": false,
				})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"calendars": out, "default_calendar_id": defaultID})

	case r.Method == http.MethodPost && len(parts) == 1 && parts[0] == "calendars":
		var req struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		display := strings.TrimSpace(req.DisplayName)
		if display == "" {
			display = strings.TrimSpace(req.Name)
		}
		if display == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "display_name required"})
			return
		}
		name := sanitizeCalendarName(req.Name)
		if name == "" {
			name = sanitizeCalendarName(display)
		}
		if name == "" || name == "default" {
			name = "cal-" + storage.NewID()[:8]
		}
		if _, err := s.store.GetCalendarByName(r.Context(), au.ID, name); err == nil {
			name = name + "-" + storage.NewID()[:6]
		}
		cal, err := s.store.CreateCalendar(r.Context(), &storage.Calendar{
			UserID: au.ID, Name: name, DisplayName: display, Description: strings.TrimSpace(req.Description),
		})
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": cal.ID, "name": cal.Name, "display_name": cal.DisplayName, "description": cal.Description,
			"shared": false, "owned": true, "deletable": true,
		})

	case (r.Method == http.MethodGet || r.Method == http.MethodPatch) && len(parts) == 2 && parts[0] == "calendars":
		s.handleCalendarProperties(w, r, au, parts[1])

	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "calendars":
		cal, err := s.store.GetCalendarByID(r.Context(), au.ID, parts[1])
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		if builtinCalendar(cal.Name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "default calendar cannot be deleted"})
			return
		}
		if err := s.store.DeleteCalendar(r.Context(), au.ID, cal.Name); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	case len(parts) >= 3 && parts[0] == "calendars" && parts[2] == "acl":
		s.handleCalendarACL(w, r, au, parts[1])

	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "places":
		s.handleCalendarPlaces(w, r)

	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "freebusy":
		s.handleCalendarFreeBusy(w, r, au)

	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "resources":
		s.handleCalendarResourcesList(w, r, au)

	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] == "invites":
		s.handleCalendarInvitesList(w, r, au)

	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "invites" && parts[2] == "reply":
		s.handleCalendarInviteReply(w, r, au, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "invites" && parts[2] == "decision":
		s.handleCalendarProposalDecision(w, r, au, parts[1])

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
		if r.URL.Query().Get("expand") == "true" {
			s.expandedCalendarEvents(w, r, au, cal, objs)
			return
		}
		out := make([]map[string]any, 0, len(objs))
		for _, o := range objs {
			ev, _ := calutil.ParseICS(o.Data)
			if from != nil && o.DTEnd != nil && o.DTEnd.Before(*from) {
				continue
			}
			if to != nil && o.DTStart != nil && o.DTStart.After(*to) {
				continue
			}
			item := calendarEventJSON(o, ev, cal, au)
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
		ev, _ := calutil.ParseICS(o.Data)
		item := calendarEventJSON(o, ev, cal, au)
		item["calendar_id"] = cal.ID
		if rid := r.URL.Query().Get("recurrence_id"); rid != "" {
			instance, err := calutil.GetOccurrence(o.Data, rid)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "occurrence not found"})
				return
			}
			item = occurrenceJSON(o, instance, cal, au)
		}
		if cal.Name == "personal" && cal.UserID != au.ID {
			item["ics"], _ = calutil.BusyOnlyICS(o.Data, o.ID)
			writeJSON(w, http.StatusOK, item)
			return
		}
		item["ics"] = o.Data
		if atts := s.attachmentsForEventJSON(r, o.ID); len(atts) > 0 {
			item["attachments"] = atts
		}
		writeJSON(w, http.StatusOK, item)

	case len(parts) == 3 && parts[0] == "events" && parts[2] == "attachments":
		s.handleCalendarEventAttachments(w, r, au, parts[1])
	case len(parts) == 3 && parts[0] == "events" && parts[2] == "occurrences":
		s.handleCalendarOccurrence(w, r, au, parts[1])
	case len(parts) == 3 && parts[0] == "events" && parts[2] == "recurrence":
		s.handleCalendarRecurrence(w, r, au, parts[1])

	case len(parts) == 2 && parts[0] == "attachments":
		s.handleCalendarAttachmentByID(w, r, au, parts[1])

	case (r.Method == http.MethodPost || r.Method == http.MethodPut) && len(parts) == 3 && parts[0] == "calendars" && parts[2] == "events":
		cal, err := s.calendarAccessible(r, au, parts[1])
		if err != nil || !calendarCanWrite(s, r, au, parts[1]) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			ID          string                  `json:"id"`
			UID         string                  `json:"uid"`
			Summary     string                  `json:"summary"`
			Location    string                  `json:"location"`
			Geo         string                  `json:"geo"`
			Description string                  `json:"description"`
			Start       string                  `json:"start"`
			End         string                  `json:"end"`
			AllDay      *bool                   `json:"all_day"`
			ETag        string                  `json:"etag"`
			Recurrence  *calutil.RecurrenceRule `json:"recurrence"`
			Timezone    string                  `json:"timezone"`
			Attendees   []struct {
				Email string `json:"email"`
				Name  string `json:"name"`
				Kind  string `json:"kind"` // individual|resource (optional hint)
			} `json:"attendees"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if !validEventGeo(req.Geo) {
			writeJSON(w, 400, map[string]string{"error": "invalid geo coordinates"})
			return
		}
		if req.ID != "" && req.Recurrence != nil {
			writeJSON(w, 400, map[string]string{"error": "use the recurrence endpoint with an etag to change a series"})
			return
		}
		if req.Summary == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "summary required"})
			return
		}
		uid := req.UID
		sequence := 0
		var previous *calutil.Event
		var previousObject *storage.CalendarObject
		if req.ID != "" {
			old, e := s.store.GetCalendarObjectByID(r.Context(), req.ID)
			if e != nil || old.CalendarID != cal.ID {
				writeJSON(w, 403, map[string]string{"error": "forbidden"})
				return
			}
			uid = old.UID
			previousObject = old
			if req.ETag != "" && req.ETag != old.ETag {
				writeJSON(w, 409, map[string]string{"error": "event changed; refresh before saving"})
				return
			}
			parsed, parseErr := calutil.ParseICS(old.Data)
			if parseErr != nil {
				writeJSON(w, 409, map[string]string{"error": "invalid stored event"})
				return
			}
			previous = &parsed.Event
			if parsed.Organizer.Email != "" && !strings.EqualFold(parsed.Organizer.Email, au.Email) {
				writeJSON(w, 403, map[string]string{"error": "only the organizer can update a meeting"})
				return
			}
			sequence = calutil.NextSequence(old.Data)
		}
		if uid == "" {
			uid = storage.NewID()
		}
		href := uid + ".ics"
		if req.ID != "" {
			href = previousObject.HrefName
		}
		start := parseFlexibleTime(req.Start)
		end := parseFlexibleTime(req.End)
		if start == nil || end == nil || !end.After(*start) {
			writeJSON(w, 400, map[string]string{"error": "end must be after start"})
			return
		}
		user, err := s.store.GetUserByID(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
			return
		}
		allDay := previous != nil && previous.AllDay
		if req.AllDay != nil {
			allDay = *req.AllDay
		}
		ev := calutil.Event{
			UID: uid, Summary: req.Summary, Location: req.Location, Geo: req.Geo, Description: req.Description,
			Start: start, End: end, AllDay: allDay, Sequence: sequence, Status: "CONFIRMED",
			Organizer: calutil.Attendee{Email: user.Email, Name: user.DisplayName},
		}
		if previous != nil {
			updated := *previous
			updated.Summary, updated.Location, updated.Geo, updated.Description = ev.Summary, ev.Location, ev.Geo, ev.Description
			updated.Start, updated.End, updated.AllDay = ev.Start, ev.End, ev.AllDay
			updated.Sequence, updated.Status, updated.Organizer = ev.Sequence, ev.Status, ev.Organizer
			updated.Attendees = nil
			ev = updated
		}
		seenAtt := map[string]struct{}{}
		for _, a := range req.Attendees {
			email := strings.ToLower(strings.TrimSpace(a.Email))
			if email == "" || email == strings.ToLower(user.Email) {
				continue
			}
			if _, ok := seenAtt[email]; ok {
				continue
			}
			seenAtt[email] = struct{}{}
			name := strings.TrimSpace(a.Name)
			role := "REQ-PARTICIPANT"
			if previous != nil {
				for _, attendee := range previous.Attendees {
					if strings.EqualFold(attendee.Email, email) && attendee.Role != "" {
						role = attendee.Role
					}
				}
			}
			cutype := "INDIVIDUAL"
			partstat := "NEEDS-ACTION"
			rsvp := true
			if res, rerr := s.store.GetCalendarResourceByEmail(r.Context(), email); rerr == nil && res != nil && res.Enabled {
				cutype = "RESOURCE"
				if name == "" {
					name = res.DisplayName
				}
				if res.AutoAccept {
					// The storage transaction validates and accepts the entire booking.
					partstat = "NEEDS-ACTION"
					rsvp = false
				}
			} else if strings.EqualFold(a.Kind, "resource") {
				cutype = "RESOURCE"
				rsvp = false
			}
			ev.Attendees = append(ev.Attendees, calutil.Attendee{
				Email: email, Name: name,
				PartStat: partstat, RSVP: rsvp, Role: role, CUType: cutype,
			})
		}
		var ics string
		if len(ev.Attendees) > 0 {
			ics, err = calutil.BuildRequestICS(ev)
		} else {
			ics, err = calutil.BuildSimpleVEVENT(ev)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if req.Recurrence != nil {
			if req.Recurrence.Frequency != "" {
				ics, err = calutil.SetRecurrenceTimezone(ics, req.Timezone)
			}
			if err == nil {
				ics, err = calutil.WithRecurrence(ics, *req.Recurrence, sequence)
			}
			if err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
			parsed, parseErr := calutil.ParseICS(ics)
			if parseErr != nil {
				writeJSON(w, 400, map[string]string{"error": parseErr.Error()})
				return
			}
			ev = parsed.Event
		}
		obj := &storage.CalendarObject{
			CalendarID: cal.ID, UID: uid, HrefName: href,
			Data: ics, Size: int64(len(ics)), Component: "VEVENT",
			DTStart: start, DTEnd: end, ETag: storage.ContentETag(ics),
		}
		if req.ID != "" {
			obj.ID = req.ID
			obj.ExpectedETag = previousObject.ETag
		} else {
			obj.CreateOnly = true
		}
		saved, err := s.store.SaveCalendarMeeting(r.Context(), au.ID, obj)
		if writeCalendarBookingError(w, err) {
			return
		}
		if err == storage.ErrUnauthorized {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		if err == storage.ErrCalendarConflict {
			writeJSON(w, 409, map[string]string{"error": "event changed; refresh before saving"})
			return
		}
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
		cal, accessErr := s.calendarAccessible(r, au, o.CalendarID)
		if accessErr != nil || !calendarCanWrite(s, r, au, o.CalendarID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if expected := r.Header.Get("If-Match"); expected != "" && expected != o.ETag {
			writeJSON(w, 409, map[string]string{"error": "event changed; refresh before deleting"})
			return
		}
		if err = s.store.DeleteCalendarMeeting(r.Context(), au.ID, o); err != nil {
			status := 500
			if err == storage.ErrUnauthorized {
				status = 403
			}
			if err == storage.ErrCalendarConflict {
				status = 409
			}
			writeJSON(w, status, map[string]string{"error": "event changed or could not be deleted"})
			return
		}
		if root, err := s.calendarAttachRoot(); err == nil {
			_ = os.RemoveAll(filepath.Join(root, cal.UserID, o.ID))
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func eventToJSON(o *storage.CalendarObject, ev calutil.ParseResult) map[string]any {
	item := map[string]any{
		"id": o.ID, "calendar_id": o.CalendarID, "uid": o.UID,
		"summary": ev.Summary, "location": ev.Location, "geo": ev.Geo, "description": ev.Description,
		"sequence": ev.Sequence, "status": ev.Status, "all_day": ev.AllDay, "etag": o.ETag,
	}
	if o.DTStart != nil {
		item["start"] = o.DTStart.UTC().Format(time.RFC3339)
	} else if ev.Start != nil {
		item["start"] = ev.Start.UTC().Format(time.RFC3339)
	}
	if o.DTEnd != nil {
		item["end"] = o.DTEnd.UTC().Format(time.RFC3339)
	} else if ev.End != nil {
		item["end"] = ev.End.UTC().Format(time.RFC3339)
	}
	if ev.Organizer.Email != "" {
		item["organizer"] = map[string]any{"email": ev.Organizer.Email, "name": ev.Organizer.Name}
	}
	atts := make([]map[string]any, 0, len(ev.Attendees))
	for _, a := range ev.Attendees {
		atts = append(atts, map[string]any{
			"email": a.Email, "name": a.Name, "partstat": a.PartStat, "role": a.Role,
			"kind": strings.ToLower(a.CUType),
		})
	}
	if len(atts) > 0 {
		item["attendees"] = atts
	}
	return item
}

func (s *Server) handleCalendarFreeBusy(w http.ResponseWriter, r *http.Request, au *authUser) {
	from, to := parseTimeRange(r)
	if from == nil || to == nil || !to.After(*from) || to.Sub(*from) > 31*24*time.Hour {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to required"})
		return
	}
	emailsRaw := r.URL.Query().Get("emails")
	emails := normalizeAddrs(strings.Split(emailsRaw, ","))
	if len(emails) == 0 || len(emails) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "emails required"})
		return
	}
	users := make([]map[string]any, 0, len(emails))
	for _, email := range emails {
		entry := map[string]any{"email": email, "busy": []any{}, "available": "unknown", "type": "external"}
		u, err := s.store.ResolveRecipient(r.Context(), email)
		if err != nil {
			users = append(users, entry)
			continue
		}
		busy, err := s.store.FreeBusyForUser(r.Context(), u.ID, *from, *to)
		if err != nil {
			users = append(users, entry)
			continue
		}
		intervals := make([]map[string]string, 0, len(busy))
		for _, b := range busy {
			intervals = append(intervals, map[string]string{
				"start": b.Start.UTC().Format(time.RFC3339),
				"end":   b.End.UTC().Format(time.RFC3339),
			})
		}
		entry["busy"] = intervals
		entry["available"] = "local"
		entry["type"] = "user"
		if res, rerr := s.store.GetCalendarResourceByUserID(r.Context(), u.ID); rerr == nil && res != nil {
			entry["type"] = "resource"
			entry["kind"] = res.Kind
			entry["display_name"] = res.DisplayName
		}
		users = append(users, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users, "from": from.UTC().Format(time.RFC3339), "to": to.UTC().Format(time.RFC3339)})
}

func (s *Server) handleCalendarResourcesList(w http.ResponseWriter, r *http.Request, au *authUser) {
	user, err := s.store.GetUserByID(r.Context(), au.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
		return
	}
	list, err := s.store.ListCalendarResourcesByDomain(r.Context(), user.DomainID, true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, res := range list {
		out = append(out, resourceToJSON(res))
	}
	writeJSON(w, http.StatusOK, map[string]any{"resources": out})
}

func (s *Server) handleCalendarInvitesList(w http.ResponseWriter, r *http.Request, au *authUser) {
	pending := r.URL.Query().Get("all") != "1"
	invites, err := s.store.ListCalendarInvitesForUser(r.Context(), au.ID, pending)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(invites))
	for _, inv := range invites {
		item := map[string]any{
			"id": inv.ID, "event_uid": inv.EventUID, "calendar_object_id": inv.CalendarObjectID,
			"summary": inv.Summary, "partstat": inv.PartStat, "attendee_email": inv.AttendeeEmail,
			"organizer_user_id": inv.OrganizerUserID,
			"created_at":        inv.CreatedAt.UTC().Format(time.RFC3339),
		}
		if inv.ProposedStart != nil {
			item["proposed_start"] = inv.ProposedStart.UTC().Format(time.RFC3339)
		}
		if inv.ProposedEnd != nil {
			item["proposed_end"] = inv.ProposedEnd.UTC().Format(time.RFC3339)
		}
		if inv.CalendarObjectID != "" {
			if o, err := s.store.GetCalendarObjectByID(r.Context(), inv.CalendarObjectID); err == nil {
				ev, _ := calutil.ParseICS(o.Data)
				item["etag"] = o.ETag
				if o.DTStart != nil {
					item["start"] = o.DTStart.UTC().Format(time.RFC3339)
				} else if ev.Start != nil {
					item["start"] = ev.Start.UTC().Format(time.RFC3339)
				}
				if o.DTEnd != nil {
					item["end"] = o.DTEnd.UTC().Format(time.RFC3339)
				} else if ev.End != nil {
					item["end"] = ev.End.UTC().Format(time.RFC3339)
				}
				if ev.Organizer.Email != "" {
					item["organizer"] = map[string]any{"email": ev.Organizer.Email, "name": ev.Organizer.Name}
				}
			}
		}
		out = append(out, item)
	}
	proposals, err := s.store.ListCalendarProposals(r.Context(), au.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "cannot load calendar proposals"})
		return
	}
	proposalJSON := make([]map[string]any, 0, len(proposals))
	for _, proposal := range proposals {
		inv, o := proposal.Invite, proposal.Object
		proposalJSON = append(proposalJSON, map[string]any{
			"id": inv.ID, "event_id": o.ID, "etag": o.ETag, "revision": inv.UpdatedAt.UTC().Format(time.RFC3339Nano),
			"summary": inv.Summary, "attendee": inv.AttendeeEmail, "start": o.DTStart, "end": o.DTEnd,
			"proposed_start": inv.ProposedStart, "proposed_end": inv.ProposedEnd,
			"recurring": strings.Contains(strings.ToUpper(o.Data), "RRULE:") || strings.Contains(strings.ToUpper(o.Data), "RDATE:"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"invites": out, "proposals": proposalJSON})
}

func (s *Server) handleCalendarProposalDecision(w http.ResponseWriter, r *http.Request, au *authUser, inviteID string) {
	var req struct {
		Action   string `json:"action"`
		EventID  string `json:"event_id"`
		Revision string `json:"revision"`
		ETag     string `json:"etag"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || (req.Action != "accept" && req.Action != "decline") || req.EventID == "" || req.Revision == "" || req.ETag == "" {
		writeJSON(w, 400, map[string]string{"error": "action, event_id, revision and etag required"})
		return
	}
	decision, err := s.store.DecideCalendarProposal(r.Context(), au.ID, inviteID, req.EventID, req.Revision, req.ETag, req.Action == "accept")
	if writeCalendarBookingError(w, err) {
		return
	}
	if err != nil {
		status := 500
		if err == storage.ErrUnauthorized {
			status = 403
		}
		if err == storage.ErrCalendarConflict || err == storage.ErrNotFound {
			status = 409
		}
		if status == 500 {
			s.log.Error("calendar proposal decision failed", "error", err)
		}
		writeJSON(w, status, map[string]string{"error": "proposal changed or is no longer available; refresh the calendar"})
		return
	}
	writeJSON(w, 200, map[string]string{"id": inviteID, "event_id": decision.Object.ID, "action": req.Action})
}

func (s *Server) handleCalendarInviteReply(w http.ResponseWriter, r *http.Request, au *authUser, inviteID string) {
	inv, err := s.store.GetCalendarInvite(r.Context(), inviteID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if inv.AttendeeUserID != au.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req struct {
		Action  string `json:"action"` // accept|decline|tentative|counter
		Start   string `json:"start"`
		End     string `json:"end"`
		Comment string `json:"comment"`
		ETag    string `json:"etag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	partstat := ""
	switch action {
	case "accept":
		partstat = "ACCEPTED"
	case "decline":
		partstat = "DECLINED"
	case "tentative":
		partstat = "TENTATIVE"
	case "counter":
		partstat = "TENTATIVE"
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid action"})
		return
	}
	var proposedStart, proposedEnd *time.Time
	if action == "counter" {
		proposedStart = parseFlexibleTime(req.Start)
		proposedEnd = parseFlexibleTime(req.End)
		if proposedStart == nil || proposedEnd == nil || !proposedEnd.After(*proposedStart) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "counter requires end after start"})
			return
		}
	}
	inv, err = s.store.ReplyCalendarInvite(r.Context(), au.ID, inv.ID, partstat, proposedStart, proposedEnd, req.ETag, req.Comment)
	if err != nil {
		status := http.StatusConflict
		if err != storage.ErrCalendarConflict && err != storage.ErrNotFound && err != storage.ErrUnauthorized {
			s.log.Error("calendar reply failed", "error", err)
			writeJSON(w, 500, map[string]string{"error": "cannot save calendar reply"})
			return
		}
		if err == storage.ErrUnauthorized {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{"error": "invitation changed or is no longer available; refresh the calendar"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": inv.ID, "partstat": inv.PartStat, "action": action})
}

func buildIMIPMIME(fromEmail, fromName string, to []string, subject, text, ics, method string) []byte {
	return calutil.IMIPMessage(fromEmail, fromName, strings.Join(to, ", "), subject, text, ics, method, storage.NewID(), time.Now().UTC())
}

func sanitizeCalendarName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '_' || r == '-' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 48 {
		out = out[:48]
		out = strings.Trim(out, "-")
	}
	return out
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

func calendarEventJSON(o *storage.CalendarObject, ev calutil.ParseResult, cal *storage.Calendar, au *authUser) map[string]any {
	item := eventToJSON(o, ev)
	item["recurrence"] = calutil.ReadRecurrence(o.Data)
	item["recurrence_organizer"] = ev.Organizer.Email == "" || strings.EqualFold(ev.Organizer.Email, au.Email)
	if cal.Name == "personal" && cal.UserID != au.ID {
		for _, key := range []string{"description", "location", "geo", "attendees", "organizer", "recurrence", "recurrence_organizer"} {
			delete(item, key)
		}
		item["uid"] = o.ID
		item["summary"] = "Busy"
		item["busy_only"] = true
	}
	return item
}
