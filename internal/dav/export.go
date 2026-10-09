package dav

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/storage"
)

// CalendarExport provides authenticated collection snapshots alongside CalDAV.
func calendarExports(store storage.Driver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suffix := ""
		for _, ext := range []string{".ics", ".xml"} {
			if strings.HasSuffix(r.URL.Path, ext) {
				suffix = ext
			}
		}
		email, name, href, ok := parseCalPath(strings.TrimSuffix(r.URL.Path, suffix))
		if suffix == "" || !ok || name == "" || href != "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		u, ok := userFrom(r.Context())
		if !ok || !strings.EqualFold(email, u.Email) {
			http.NotFound(w, r)
			return
		}
		cal, err := store.GetCalendarByName(r.Context(), u.ID, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		objects, err := store.ListCalendarObjects(r.Context(), cal.ID)
		if err != nil {
			http.Error(w, "calendar unavailable", 500)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		var buf bytes.Buffer
		if suffix == ".xml" {
			type prop struct {
				Data string `xml:"urn:ietf:params:xml:ns:caldav calendar-data"`
			}
			type propstat struct {
				Prop   prop   `xml:"DAV: prop"`
				Status string `xml:"DAV: status"`
			}
			type response struct {
				Href     string   `xml:"DAV: href"`
				Propstat propstat `xml:"DAV: propstat"`
			}
			result := struct {
				XMLName   xml.Name   `xml:"DAV: multistatus"`
				Responses []response `xml:"DAV: response"`
			}{}
			for _, o := range objects {
				result.Responses = append(result.Responses, response{Href: calObjectPath(u.Email, cal.Name, o.HrefName), Propstat: propstat{Prop: prop{Data: o.Data}, Status: "HTTP/1.1 200 OK"}})
			}
			buf.WriteString(xml.Header)
			if err = xml.NewEncoder(&buf).Encode(result); err != nil {
				http.Error(w, "export failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		} else {
			calendar := ical.NewCalendar()
			calendar.Props.SetText(ical.PropVersion, "2.0")
			calendar.Props.SetText(ical.PropProductID, "-//Tayga//Calendar Export//EN")
			seen := map[string]bool{}
			for _, o := range objects {
				parsed, e := ical.NewDecoder(strings.NewReader(o.Data)).Decode()
				if e != nil {
					http.Error(w, "invalid calendar object", 500)
					return
				}
				for _, child := range parsed.Children {
					if child.Name == "VTIMEZONE" {
						id, _ := child.Props.Text("TZID")
						if seen[id] {
							continue
						}
						seen[id] = true
					}
					calendar.Children = append(calendar.Children, child)
				}
			}
			if err = ical.NewEncoder(&buf).Encode(calendar); err != nil {
				http.Error(w, "export failed", 500)
				return
			}
			w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		}
		if r.Method == "GET" {
			w.Write(buf.Bytes())
		}
	})
}
