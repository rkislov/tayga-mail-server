package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"mime"
	"net/http"
	"regexp"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

var calendarColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func calendarCanWrite(s *Server, r *http.Request, au *authUser, id string) bool {
	rights, err := s.store.CalendarRightsForUser(r.Context(), id, au.ID)
	return err == nil && rights == "write"
}
func (s *Server) calendarProperties(c *storage.Calendar, owned bool) map[string]any {
	out := map[string]any{"id": c.ID, "display_name": c.DisplayName, "description": c.Description, "color": c.Color, "owned": owned, "deletable": owned && c.Name != "default"}
	if owned {
		out["public_enabled"] = c.PublicToken != ""
		if c.PublicToken != "" {
			out["public_url"] = strings.TrimRight(s.publicBase(), "/") + "/calendar/public/" + c.PublicToken + ".ics"
		}
	}
	return out
}
func (s *Server) handleCalendarProperties(w http.ResponseWriter, r *http.Request, au *authUser, id string) {
	c, err := s.calendarAccessible(r, au, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	owned := c.UserID == au.ID
	if r.Method == http.MethodGet {
		writeJSON(w, 200, s.calendarProperties(c, owned))
		return
	}
	if !owned {
		writeJSON(w, 403, map[string]string{"error": "only owner can manage calendar properties"})
		return
	}
	var req struct {
		DisplayName   *string `json:"display_name"`
		Description   *string `json:"description"`
		Color         *string `json:"color"`
		PublicEnabled *bool   `json:"public_enabled"`
		RotateLink    bool    `json:"rotate_link"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if req.DisplayName != nil {
		c.DisplayName = strings.TrimSpace(*req.DisplayName)
		if c.DisplayName == "" || len(c.DisplayName) > 200 {
			writeJSON(w, 400, map[string]string{"error": "name must contain 1–200 characters"})
			return
		}
	}
	if req.Description != nil {
		if len(*req.Description) > 4000 {
			writeJSON(w, 400, map[string]string{"error": "description too long"})
			return
		}
		c.Description = *req.Description
	}
	if req.Color != nil {
		if !calendarColorPattern.MatchString(*req.Color) {
			writeJSON(w, 400, map[string]string{"error": "invalid color"})
			return
		}
		c.Color = strings.ToLower(*req.Color)
	}
	if req.PublicEnabled != nil {
		if !*req.PublicEnabled {
			c.PublicToken = ""
		} else if c.PublicToken == "" || req.RotateLink {
			b := make([]byte, 32)
			if _, err = rand.Read(b); err != nil {
				writeJSON(w, 500, map[string]string{"error": "could not create link"})
				return
			}
			c.PublicToken = hex.EncodeToString(b)
		}
	}
	if err = s.store.UpdateCalendar(r.Context(), c); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, s.calendarProperties(c, true))
}

// The opaque link exposes a read-only subscription. Revocation immediately invalidates it.
func (s *Server) handlePublicCalendar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", 405)
		return
	}
	token := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/calendar/public/"), ".ics")
	if len(token) != 64 {
		http.NotFound(w, r)
		return
	}
	c, err := s.store.GetPublicCalendar(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	objects, err := s.store.ListCalendarObjects(r.Context(), c.ID)
	if err != nil {
		http.Error(w, "calendar unavailable", 500)
		return
	}
	calendar := ical.NewCalendar()
	calendar.Props.SetText(ical.PropVersion, "2.0")
	calendar.Props.SetText(ical.PropProductID, calutil.ProdID)
	calendar.Props.SetText("X-WR-CALNAME", c.DisplayName)
	calendar.Props.SetText("X-APPLE-CALENDAR-COLOR", c.Color)
	timezones := map[string]bool{}
	for _, obj := range objects {
		parsed, e := ical.NewDecoder(strings.NewReader(obj.Data)).Decode()
		if e != nil {
			http.Error(w, "calendar unavailable", 500)
			return
		}
		for _, child := range parsed.Children {
			if child.Name == "VTIMEZONE" {
				id, _ := child.Props.Text("TZID")
				if timezones[id] {
					continue
				}
				timezones[id] = true
			}
			calendar.Children = append(calendar.Children, child)
		}
	}
	var buf bytes.Buffer
	if len(calendar.Children) == 0 {
		buf.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:" + calutil.ProdID + "\r\nEND:VCALENDAR\r\n")
	} else if err = ical.NewEncoder(&buf).Encode(calendar); err != nil {
		http.Error(w, "calendar unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": c.DisplayName + ".ics"}))
	if r.Method != http.MethodHead {
		_, _ = w.Write(buf.Bytes())
	}
}
