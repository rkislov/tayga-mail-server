package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/notify"
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

	case len(parts) == 2 && parts[0] == "attachments":
		s.handleCalendarAttachmentByID(w, r, au, parts[1])

	case (r.Method == http.MethodPost || r.Method == http.MethodPut) && len(parts) == 3 && parts[0] == "calendars" && parts[2] == "events":
		cal, err := s.calendarAccessible(r, au, parts[1])
		if err != nil || !calendarCanWrite(s, r, au, parts[1]) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var req struct {
			ID          string `json:"id"`
			UID         string `json:"uid"`
			Summary     string `json:"summary"`
			Location    string `json:"location"`
			Geo         string `json:"geo"`
			Description string `json:"description"`
			Start       string `json:"start"`
			End         string `json:"end"`
			AllDay      bool   `json:"all_day"`
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
		if req.Summary == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "summary required"})
			return
		}
		uid := req.UID
		if req.ID != "" {
			old, e := s.store.GetCalendarObjectByID(r.Context(), req.ID)
			if e != nil || old.CalendarID != cal.ID {
				writeJSON(w, 403, map[string]string{"error": "forbidden"})
				return
			}
			uid = old.UID
		}
		if uid == "" {
			uid = storage.NewID()
		}
		href := uid + ".ics"
		start := parseFlexibleTime(req.Start)
		end := parseFlexibleTime(req.End)
		user, err := s.store.GetUserByID(r.Context(), au.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "user lookup failed"})
			return
		}
		ev := calutil.Event{
			UID: uid, Summary: req.Summary, Location: req.Location, Geo: req.Geo, Description: req.Description,
			Start: start, End: end, AllDay: req.AllDay, Sequence: 0, Status: "CONFIRMED",
			Organizer: calutil.Attendee{Email: user.Email, Name: user.DisplayName},
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
			cutype := "INDIVIDUAL"
			partstat := "NEEDS-ACTION"
			rsvp := true
			if res, rerr := s.store.GetCalendarResourceByEmail(r.Context(), email); rerr == nil && res != nil && res.Enabled {
				cutype = "RESOURCE"
				if name == "" {
					name = res.DisplayName
				}
				if res.AutoAccept {
					// conflict → decline; free → accept (applied in fan-out)
					partstat = "NEEDS-ACTION"
					rsvp = false
				}
			} else if strings.EqualFold(a.Kind, "resource") {
				cutype = "RESOURCE"
				rsvp = false
			}
			ev.Attendees = append(ev.Attendees, calutil.Attendee{
				Email: email, Name: name,
				PartStat: partstat, RSVP: rsvp, Role: "REQ-PARTICIPANT", CUType: cutype,
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
		if len(ev.Attendees) > 0 {
			s.fanOutMeetingInvites(r, user, saved, ev)
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
		if atts, _ := s.store.ListCalendarAttachments(r.Context(), o.ID); len(atts) > 0 {
			if root, err := s.calendarAttachRoot(); err == nil {
				_ = os.RemoveAll(filepath.Join(root, cal.UserID, o.ID))
			}
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

func eventToJSON(o *storage.CalendarObject, ev calutil.ParseResult) map[string]any {
	item := map[string]any{
		"id": o.ID, "calendar_id": o.CalendarID, "uid": o.UID,
		"summary": ev.Summary, "location": ev.Location, "geo": ev.Geo, "description": ev.Description,
		"sequence": ev.Sequence, "status": ev.Status,
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
	writeJSON(w, http.StatusOK, map[string]any{"invites": out})
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
		if proposedStart == nil || proposedEnd == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start and end required for counter"})
			return
		}
	}
	if err := s.store.UpdateCalendarInvitePartStat(r.Context(), inv.ID, partstat, proposedStart, proposedEnd); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	inv.PartStat = partstat
	inv.ProposedStart = proposedStart
	inv.ProposedEnd = proposedEnd

	user, _ := s.store.GetUserByID(r.Context(), au.ID)
	org, _ := s.store.GetUserByID(r.Context(), inv.OrganizerUserID)
	s.applyInviteReplyToCalendars(r, inv, user, org, partstat, proposedStart, proposedEnd, req.Comment)
	s.deliverInviteReply(r, inv, user, org, action, partstat, proposedStart, proposedEnd, req.Comment)

	writeJSON(w, http.StatusOK, map[string]any{"id": inv.ID, "partstat": partstat, "action": action})
}

func (s *Server) fanOutMeetingInvites(r *http.Request, organizer *storage.User, obj *storage.CalendarObject, ev calutil.Event) {
	for i, a := range ev.Attendees {
		if res, err := s.store.GetCalendarResourceByEmail(r.Context(), a.Email); err == nil && res != nil && res.Enabled {
			partstat := s.bookCalendarResource(r, organizer, res, obj, ev)
			ev.Attendees[i].PartStat = partstat
			ev.Attendees[i].CUType = "RESOURCE"
			ev.Attendees[i].RSVP = false
			_, _ = s.store.UpsertCalendarInvite(r.Context(), &storage.CalendarInvite{
				OrganizerUserID:  organizer.ID,
				AttendeeEmail:    a.Email,
				AttendeeUserID:   res.UserID,
				EventUID:         ev.UID,
				CalendarObjectID: obj.ID,
				Summary:          ev.Summary,
				PartStat:         partstat,
			})
			continue
		}
		attUser, _ := s.store.ResolveRecipient(r.Context(), a.Email)
		attUID := ""
		if attUser != nil {
			attUID = attUser.ID
		}
		inv, err := s.store.UpsertCalendarInvite(r.Context(), &storage.CalendarInvite{
			OrganizerUserID:  organizer.ID,
			AttendeeEmail:    a.Email,
			AttendeeUserID:   attUID,
			EventUID:         ev.UID,
			CalendarObjectID: obj.ID,
			Summary:          ev.Summary,
			PartStat:         "NEEDS-ACTION",
		})
		if err != nil {
			continue
		}
		if attUser != nil {
			s.deliverLocalInvite(r, organizer, attUser, obj, ev, inv)
		} else {
			s.deliverExternalInvite(r, organizer, a.Email, ev)
		}
	}
	// Persist PARTSTAT updates for resources on organizer's copy.
	if ics, err := calutil.BuildRequestICS(ev); err == nil && len(ev.Attendees) > 0 {
		obj.Data = ics
		obj.Size = int64(len(ics))
		obj.ETag = storage.ContentETag(ics)
		_, _ = s.store.UpsertCalendarObject(r.Context(), obj)
	}
}

func (s *Server) bookCalendarResource(r *http.Request, organizer *storage.User, res *storage.CalendarResource, src *storage.CalendarObject, ev calutil.Event) string {
	partstat := "NEEDS-ACTION"
	conflict := false
	if src.DTStart != nil && src.DTEnd != nil {
		busy, err := s.store.FreeBusyForUser(r.Context(), res.UserID, *src.DTStart, *src.DTEnd)
		if err == nil {
			for _, b := range busy {
				if b.End.After(*src.DTStart) && b.Start.Before(*src.DTEnd) {
					conflict = true
					break
				}
			}
		}
	}
	if conflict {
		partstat = "DECLINED"
	} else if res.AutoAccept {
		partstat = "ACCEPTED"
	}
	if partstat == "DECLINED" {
		return partstat
	}
	cal, err := s.store.EnsureCalendar(r.Context(), res.UserID, "default", res.DisplayName)
	if err != nil {
		return "DECLINED"
	}
	copyEv := ev
	for i := range copyEv.Attendees {
		if strings.EqualFold(copyEv.Attendees[i].Email, res.Email) {
			copyEv.Attendees[i].PartStat = partstat
			copyEv.Attendees[i].CUType = "RESOURCE"
			copyEv.Attendees[i].RSVP = false
		}
	}
	ics, err := calutil.BuildSimpleVEVENT(copyEv)
	if err != nil {
		ics = src.Data
	}
	_, _ = s.store.UpsertCalendarObject(r.Context(), &storage.CalendarObject{
		CalendarID: cal.ID, UID: ev.UID, HrefName: ev.UID + ".ics",
		Data: ics, Size: int64(len(ics)), Component: "VEVENT",
		DTStart: src.DTStart, DTEnd: src.DTEnd, ETag: storage.ContentETag(ics),
	})
	_ = organizer
	return partstat
}

func (s *Server) deliverLocalInvite(r *http.Request, organizer, attendee *storage.User, src *storage.CalendarObject, ev calutil.Event, inv *storage.CalendarInvite) {
	_ = s.store.EnsureDAVDefaults(r.Context(), attendee.ID)
	cal, err := s.store.EnsureCalendar(r.Context(), attendee.ID, "default", "Calendar")
	if err != nil {
		return
	}
	copyEv := ev
	ics, err := calutil.BuildRequestICS(copyEv)
	if err != nil {
		ics = src.Data
	}
	attObj := &storage.CalendarObject{
		CalendarID: cal.ID, UID: ev.UID, HrefName: ev.UID + ".ics",
		Data: ics, Size: int64(len(ics)), Component: "VEVENT",
		DTStart: src.DTStart, DTEnd: src.DTEnd, ETag: storage.ContentETag(ics),
	}
	saved, err := s.store.UpsertCalendarObject(r.Context(), attObj)
	if err == nil && inv != nil {
		inv.CalendarObjectID = saved.ID
		_, _ = s.store.UpsertCalendarInvite(r.Context(), inv)
	}
	when := calutil.FormatWhen(src.DTStart, src.DTEnd)
	if s.notify != nil {
		s.notify.Publish(attendee.ID, notify.Event{
			Kind:  notify.KindCalendar,
			Title: ev.Summary,
			Body:  organizer.Email + " · " + when,
			Href:  "calendar?invite=" + inv.ID,
		})
	}
	subject := "Invitation: " + ev.Summary
	text := fmt.Sprintf("%s invited you to: %s\nWhen: %s\n", organizer.DisplayName, ev.Summary, when)
	if organizer.DisplayName == "" {
		text = fmt.Sprintf("%s invited you to: %s\nWhen: %s\n", organizer.Email, ev.Summary, when)
	}
	raw := buildIMIPMIME(organizer.Email, organizer.DisplayName, []string{attendee.Email}, subject, text, ics, "REQUEST")
	s.deliverLocalMail(r, organizer, attendee, subject, raw)
}

func (s *Server) deliverExternalInvite(r *http.Request, organizer *storage.User, email string, ev calutil.Event) {
	ics, err := calutil.BuildRequestICS(ev)
	if err != nil {
		return
	}
	when := calutil.FormatWhen(ev.Start, ev.End)
	subject := "Invitation: " + ev.Summary
	text := fmt.Sprintf("%s invited you to: %s\nWhen: %s\n", organizer.Email, ev.Summary, when)
	raw := buildIMIPMIME(organizer.Email, organizer.DisplayName, []string{email}, subject, text, ics, "REQUEST")
	msgid := fmt.Sprintf("<%d.%s@tayga>", time.Now().UnixNano(), storage.NewID()[:8])
	_, _ = s.store.EnqueueOutbound(r.Context(), &storage.OutboundItem{
		EnvelopeFrom: organizer.Email,
		EnvelopeTo:   email,
		MessageID:    msgid,
		Data:         raw,
		MaxAttempts:  8,
	})
}

func (s *Server) applyInviteReplyToCalendars(r *http.Request, inv *storage.CalendarInvite, attendee, organizer *storage.User, partstat string, pStart, pEnd *time.Time, comment string) {
	if inv.CalendarObjectID == "" {
		return
	}
	o, err := s.store.GetCalendarObjectByID(r.Context(), inv.CalendarObjectID)
	if err != nil {
		return
	}
	parsed, _ := calutil.ParseICS(o.Data)
	ev := parsed.Event
	for i := range ev.Attendees {
		if strings.EqualFold(ev.Attendees[i].Email, inv.AttendeeEmail) {
			ev.Attendees[i].PartStat = partstat
			ev.Attendees[i].RSVP = false
		}
	}
	if pStart != nil {
		ev.Start = pStart
	}
	if pEnd != nil {
		ev.End = pEnd
	}
	ev.Comment = comment
	ev.Sequence++
	ics, err := calutil.BuildSimpleVEVENT(ev)
	if err != nil {
		return
	}
	o.Data = ics
	o.Size = int64(len(ics))
	o.ETag = storage.ContentETag(ics)
	if pStart != nil {
		o.DTStart = pStart
	}
	if pEnd != nil {
		o.DTEnd = pEnd
	}
	_, _ = s.store.UpsertCalendarObject(r.Context(), o)

	// Also update organizer's copy by UID if accessible.
	if organizer != nil {
		cals, _ := s.store.ListCalendars(r.Context(), organizer.ID)
		for _, c := range cals {
			objs, _ := s.store.ListCalendarObjects(r.Context(), c.ID)
			for _, oo := range objs {
				if oo.UID != inv.EventUID {
					continue
				}
				op, _ := calutil.ParseICS(oo.Data)
				oe := op.Event
				for i := range oe.Attendees {
					if strings.EqualFold(oe.Attendees[i].Email, inv.AttendeeEmail) {
						oe.Attendees[i].PartStat = partstat
					}
				}
				if pStart != nil {
					oe.Start = pStart
					oo.DTStart = pStart
				}
				if pEnd != nil {
					oe.End = pEnd
					oo.DTEnd = pEnd
				}
				oe.Sequence++
				if data, err := calutil.BuildSimpleVEVENT(oe); err == nil {
					oo.Data = data
					oo.Size = int64(len(data))
					oo.ETag = storage.ContentETag(data)
					_, _ = s.store.UpsertCalendarObject(r.Context(), oo)
				}
			}
		}
	}
	_ = attendee
}

func (s *Server) deliverInviteReply(r *http.Request, inv *storage.CalendarInvite, attendee, organizer *storage.User, action, partstat string, pStart, pEnd *time.Time, comment string) {
	if organizer == nil || attendee == nil {
		return
	}
	ev := calutil.Event{
		UID: inv.EventUID, Summary: inv.Summary, Sequence: 1,
		Organizer: calutil.Attendee{Email: organizer.Email, Name: organizer.DisplayName},
		Start:     pStart, End: pEnd, Comment: comment,
	}
	if pStart == nil || pEnd == nil {
		if o, err := s.store.GetCalendarObjectByID(r.Context(), inv.CalendarObjectID); err == nil {
			ev.Start = o.DTStart
			ev.End = o.DTEnd
			parsed, _ := calutil.ParseICS(o.Data)
			if ev.Start == nil {
				ev.Start = parsed.Start
			}
			if ev.End == nil {
				ev.End = parsed.End
			}
			ev.Location = parsed.Location
			ev.Description = parsed.Description
		}
	}
	responder := calutil.Attendee{
		Email: attendee.Email, Name: attendee.DisplayName, PartStat: partstat, Role: "REQ-PARTICIPANT",
	}
	var ics string
	var method string
	if action == "counter" {
		method = "COUNTER"
		ics, _ = calutil.BuildCounterICS(ev, responder)
	} else {
		method = "REPLY"
		ics, _ = calutil.BuildReplyICS(ev, responder)
	}
	if ics == "" {
		return
	}
	subject := "Re: " + inv.Summary + " (" + strings.ToLower(partstat) + ")"
	text := fmt.Sprintf("%s responded: %s\n", attendee.Email, partstat)
	if comment != "" {
		text += comment + "\n"
	}
	raw := buildIMIPMIME(attendee.Email, attendee.DisplayName, []string{organizer.Email}, subject, text, ics, method)
	s.deliverLocalMail(r, attendee, organizer, subject, raw)
	if s.notify != nil {
		s.notify.Publish(organizer.ID, notify.Event{
			Kind:  notify.KindCalendar,
			Title: inv.Summary,
			Body:  attendee.Email + ": " + partstat,
			Href:  "calendar",
		})
	}
}

func (s *Server) deliverLocalMail(r *http.Request, fromUser, toUser *storage.User, subject string, raw []byte) {
	if s.ms == nil || toUser == nil || fromUser == nil {
		return
	}
	msgid := fmt.Sprintf("<%d.%s@tayga>", time.Now().UnixNano(), storage.NewID()[:8])
	// ensure Message-ID present
	if !strings.Contains(string(raw), "Message-ID:") {
		raw = append([]byte(fmt.Sprintf("Message-ID: %s\r\n", msgid)), raw...)
	}
	root := s.ms.UserRoot(toUser.Email)
	mb, err := s.store.EnsureMailbox(r.Context(), toUser.ID, "INBOX", root)
	if err != nil {
		return
	}
	if rel, size, derr := s.ms.Deliver(toUser.Email, "INBOX", raw); derr == nil {
		im := &storage.Message{
			MailboxID: mb.ID, Size: size, Flags: "",
			InternalDate: time.Now().UTC(), FilePath: rel, MessageID: msgid,
		}
		mailsearch.ApplyHeaders(im, raw)
		if inserted, ierr := s.store.InsertMessage(r.Context(), im); ierr == nil {
			_ = mailsearch.Index(r.Context(), s.store, inserted.ID, raw)
		}
		if s.notify != nil {
			s.notify.PublishMail(toUser.ID, "INBOX", subject, fromUser.Email)
		}
	}
}

func buildIMIPMIME(fromEmail, fromName string, to []string, subject, text, ics, method string) []byte {
	var b strings.Builder
	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}
	boundary := "tayga-cal-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	msgid := fmt.Sprintf("<%d.%s@tayga>", time.Now().UnixNano(), storage.NewID()[:8])
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: %s\r\n", msgid)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", boundary, text)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/calendar; charset=utf-8; method=%s\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s", boundary, method, ics)
	if !strings.HasSuffix(ics, "\n") {
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
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
	if cal.Name == "personal" && cal.UserID != au.ID {
		for _, key := range []string{"description", "location", "geo", "attendees", "organizer"} {
			delete(item, key)
		}
		item["uid"] = o.ID
		item["summary"] = "Busy"
		item["busy_only"] = true
	}
	return item
}
