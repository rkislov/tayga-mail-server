// SPDX-License-Identifier: Apache-2.0
package dav

import (
	"bytes"
	"github.com/emersion/go-ical"
	"github.com/tayga/tms/internal/storage"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

func serveSchedulingFreeBusy(store storage.Driver, w http.ResponseWriter, r *http.Request, u *storage.User) {
	content, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || content != "text/calendar" {
		http.Error(w, "text/calendar required", 415)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		http.Error(w, "request too large", 413)
		return
	}
	cal, err := ical.NewDecoder(bytes.NewReader(raw)).Decode()
	if err != nil {
		schedulingError(w, 403, "valid-scheduling-message")
		return
	}
	method, _ := cal.Props.Text("METHOD")
	if method != "REQUEST" || len(cal.Children) != 1 || cal.Children[0].Name != "VFREEBUSY" {
		schedulingError(w, 403, "valid-scheduling-message")
		return
	}
	request := cal.Children[0]
	organizer := request.Props.Get("ORGANIZER")
	if organizer == nil || !strings.EqualFold(organizer.Value, "mailto:"+u.Email) {
		schedulingError(w, 403, "valid-organizer")
		return
	}
	startProp, endProp := request.Props.Get("DTSTART"), request.Props.Get("DTEND")
	if startProp == nil || endProp == nil {
		schedulingError(w, 403, "valid-scheduling-message")
		return
	}
	stamp := request.Props.Get("DTSTAMP")
	if stamp == nil || !strings.HasSuffix(startProp.Value, "Z") || !strings.HasSuffix(endProp.Value, "Z") {
		schedulingError(w, 403, "valid-scheduling-message")
		return
	}
	if _, err := stamp.DateTime(time.UTC); err != nil {
		schedulingError(w, 403, "valid-scheduling-message")
		return
	}
	from, e1 := startProp.DateTime(time.UTC)
	to, e2 := endProp.DateTime(time.UTC)
	uid, _ := request.Props.Text("UID")
	attendees := request.Props.Values("ATTENDEE")
	if e1 != nil || e2 != nil || !to.After(from) || to.Sub(from) > 366*24*time.Hour || uid == "" || len(uid) > 1024 || len(attendees) == 0 || len(attendees) > 100 {
		schedulingError(w, 403, "valid-scheduling-message")
		return
	}
	seen := map[string]bool{}
	for _, a := range attendees {
		key := strings.ToLower(a.Value)
		if seen[key] {
			schedulingError(w, 403, "valid-scheduling-message")
			return
		}
		seen[key] = true
	}
	out := xn(calNS, "schedule-response")
	for _, a := range attendees {
		response := xn(calNS, "response", xn(calNS, "recipient", xt("DAV:", "href", a.Value)))
		out.Children = append(out.Children, response)
		email := strings.TrimPrefix(strings.ToLower(a.Value), "mailto:")
		peer, err := store.GetUserByEmail(r.Context(), email)
		if !strings.HasPrefix(strings.ToLower(a.Value), "mailto:") || err != nil || !peer.Enabled || peer.TenantID != u.TenantID {
			response.Children = append(response.Children, xt(calNS, "request-status", "3.7;Invalid Calendar User"))
			continue
		}
		busy, err := store.FreeBusyForUser(r.Context(), peer.ID, from, to)
		if err != nil {
			http.Error(w, "freebusy unavailable", 500)
			return
		}
		result := ical.NewCalendar()
		result.Props.SetText("VERSION", "2.0")
		result.Props.SetText("PRODID", "-//Tayga//CalDAV scheduling//EN")
		result.Props.SetText("METHOD", "REPLY")
		component := ical.NewComponent("VFREEBUSY")
		component.Props.SetText("UID", uid)
		component.Props.SetDateTime("DTSTAMP", time.Now().UTC())
		component.Props.SetDateTime("DTSTART", from.UTC())
		component.Props.SetDateTime("DTEND", to.UTC())
		component.Props.Set(&ical.Prop{Name: "ORGANIZER", Value: "mailto:" + u.Email})
		component.Props.Add(&a)
		for _, interval := range busy {
			start, end := interval.Start, interval.End
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			if end.After(start) {
				p := ical.NewProp("FREEBUSY")
				p.Params.Set("FBTYPE", "BUSY")
				p.Value = start.UTC().Format("20060102T150405Z") + "/" + end.UTC().Format("20060102T150405Z")
				component.Props.Add(p)
			}
		}
		result.Children = append(result.Children, component)
		var data strings.Builder
		if err = ical.NewEncoder(&data).Encode(result); err != nil {
			http.Error(w, "freebusy encoding failed", 500)
			return
		}
		response.Children = append(response.Children, xt(calNS, "request-status", "2.0;Success"), xt(calNS, "calendar-data", data.String()))
	}
	writeDAVXML(w, 200, out)
}
