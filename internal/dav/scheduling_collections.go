// SPDX-License-Identifier: Apache-2.0
package dav

import (
	"encoding/xml"
	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/storage"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func schedulingCollectionProperties(email, kind string) map[xml.Name]*davXML {
	values := map[xml.Name]*davXML{}
	add := func(n *davXML) { values[n.Name] = n }
	add(xn("DAV:", "resourcetype", xn("DAV:", "collection"), xn(calNS, "schedule-"+kind)))
	add(xt("DAV:", "displayname", "Scheduling "+kind))
	add(xn("DAV:", "current-user-principal", xt("DAV:", "href", calPrincipal(email))))
	privileges := xn("DAV:", "current-user-privilege-set", xn("DAV:", "privilege", xn("DAV:", "read")))
	if kind == "inbox" {
		add(xn(calNS, "schedule-default-calendar-URL", xt("DAV:", "href", calCollection(email, "default"))))
		privileges.Children = append(privileges.Children, xn("DAV:", "privilege", xn("DAV:", "unbind")))
		reports := xn("DAV:", "supported-report-set")
		for _, name := range []string{"calendar-query", "calendar-multiget"} {
			reports.Children = append(reports.Children, xn("DAV:", "supported-report", xn("DAV:", "report", xn(calNS, name))))
		}
		add(reports)
	} else {
		privileges.Children = append(privileges.Children, xn("DAV:", "privilege", xn(calNS, "schedule-send-freebusy")))
	}
	add(privileges)
	return values
}
func schedulingMessageProperties(m storage.CalendarSchedulingMessage) map[xml.Name]*davXML {
	out := map[xml.Name]*davXML{}
	for _, n := range []*davXML{xn("DAV:", "resourcetype"), xt("DAV:", "getetag", m.ETag), xt("DAV:", "getcontenttype", "text/calendar; charset=utf-8"), xt("DAV:", "getcontentlength", strconv.Itoa(len(m.Data))), xt("DAV:", "getlastmodified", m.CreatedAt.UTC().Format(http.TimeFormat)), xt(calNS, "calendar-data", m.Data)} {
		out[n.Name] = n
	}
	return out
}
func serveSchedulingCollection(store storage.Driver, w http.ResponseWriter, r *http.Request, u *storage.User, parts []string) {
	kind := parts[1]
	collection := len(parts) == 2
	if len(parts) > 3 || kind == "outbox" && !collection {
		http.NotFound(w, r)
		return
	}
	allowed := "OPTIONS, PROPFIND, POST"
	if kind == "inbox" {
		allowed = "OPTIONS, PROPFIND, REPORT, GET, HEAD, DELETE"
	}
	w.Header().Set("Allow", allowed)
	if r.Method == "OPTIONS" {
		w.Header().Set("DAV", "1, 3, calendar-access, calendar-auto-schedule")
		w.WriteHeader(200)
		return
	}
	if r.Method == "POST" && kind == "outbox" {
		serveSchedulingFreeBusy(store, w, r, u)
		return
	}
	if r.Method != "PROPFIND" && r.Method != "REPORT" && !(kind == "inbox" && !collection && (r.Method == "GET" || r.Method == "HEAD" || r.Method == "DELETE")) {
		http.Error(w, "method not allowed", 405)
		return
	}
	var messages []storage.CalendarSchedulingMessage
	if kind == "inbox" {
		var err error
		messages, err = store.ListCalendarSchedulingInbox(r.Context(), u.ID)
		if err != nil {
			http.Error(w, "inbox unavailable", 500)
			return
		}
	}
	var selected *storage.CalendarSchedulingMessage
	if !collection {
		for i := range messages {
			if parts[2] == messages[i].ID+".ics" {
				selected = &messages[i]
				break
			}
		}
		if selected == nil {
			http.NotFound(w, r)
			return
		}
	}
	if r.Method == "GET" || r.Method == "HEAD" {
		w.Header().Set("ETag", selected.ETag)
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("Content-Length", strconv.Itoa(len(selected.Data)))
		if match := webdav.ConditionalMatch(r.Header.Get("If-None-Match")); match.IsSet() {
			ok, err := match.MatchETag(strings.Trim(selected.ETag, `"`))
			if err != nil {
				http.Error(w, "invalid condition", 400)
				return
			}
			if ok {
				w.WriteHeader(304)
				return
			}
		}
		if r.Method == "GET" {
			_, _ = io.WriteString(w, selected.Data)
		} else {
			w.WriteHeader(200)
		}
		return
	}
	if r.Method == "DELETE" {
		if match := webdav.ConditionalMatch(r.Header.Get("If-Match")); match.IsSet() {
			ok, err := match.MatchETag(strings.Trim(selected.ETag, `"`))
			if err != nil || !ok {
				http.Error(w, "precondition failed", 412)
				return
			}
		}

		if err := store.DeleteCalendarSchedulingMessage(r.Context(), u.ID, selected.ID, selected.ETag); err != nil {
			http.Error(w, "precondition failed", 412)
			return
		}
		w.WriteHeader(204)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		http.Error(w, "request too large", 413)
		return
	}
	var root *davXML
	if len(strings.TrimSpace(string(raw))) > 0 {
		root, err = parseDAVXML(raw)
		if err != nil {
			http.Error(w, "invalid XML", 400)
			return
		}
	}
	req := propertyRequest(root)
	out := xn("DAV:", "multistatus")
	if r.Method == "PROPFIND" {
		if depth := r.Header.Get("Depth"); depth != "" && depth != "0" && depth != "1" {
			http.Error(w, "finite depth required", 403)
			return
		}
		if selected != nil {
			out.Children = append(out.Children, propertyResponse(r.URL.Path, req, schedulingMessageProperties(*selected)))
		} else {
			out.Children = append(out.Children, propertyResponse(calPrincipal(u.Email)+kind+"/", req, schedulingCollectionProperties(u.Email, kind)))
			if r.Header.Get("Depth") != "0" {
				for _, m := range messages {
					out.Children = append(out.Children, propertyResponse(calInbox(u.Email)+m.ID+".ics", req, schedulingMessageProperties(m)))
				}
			}
		}
	} else {
		if !collection || kind != "inbox" || root == nil {
			http.Error(w, "invalid report", 400)
			return
		}
		switch root.Name {
		case xml.Name{Space: calNS, Local: "calendar-multiget"}:
			count := 0
			for _, href := range root.Children {
				if href.Name != (xml.Name{Space: "DAV:", Local: "href"}) {
					continue
				}
				count++
				if count > 1000 {
					http.Error(w, "too many hrefs", 400)
					return
				}
				target, e := url.Parse(href.Text)
				if e != nil {
					http.Error(w, "invalid href", 400)
					return
				}
				found := false
				if target.Host == "" || target.Host == r.Host {
					for _, m := range messages {
						if target.Path == calInbox(u.Email)+m.ID+".ics" {
							out.Children = append(out.Children, propertyResponse(target.Path, req, schedulingMessageProperties(m)))
							found = true
							break
						}
					}
				}
				if !found {
					out.Children = append(out.Children, xn("DAV:", "response", xt("DAV:", "href", href.Text), xt("DAV:", "status", "HTTP/1.1 404 Not Found")))
				}
			}
		case xml.Name{Space: calNS, Local: "calendar-query"}:
			filter := root.child(calNS, "filter")
			if filter == nil || len(filter.Children) != 1 {
				http.Error(w, "filter required", 400)
				return
			}
			comp, e := inboxComponentFilter(filter.Children[0])
			if e != nil {
				http.Error(w, "unsupported filter", 400)
				return
			}
			for _, m := range messages {
				cal, e := ical.NewDecoder(strings.NewReader(m.Data)).Decode()
				if e != nil {
					http.Error(w, "invalid inbox data", 500)
					return
				}
				matched, e := caldav.Filter(&caldav.CalendarQuery{CompFilter: comp}, []caldav.CalendarObject{{Data: cal}})
				if e != nil {
					http.Error(w, "invalid filter", 400)
					return
				}
				if len(matched) > 0 {
					out.Children = append(out.Children, propertyResponse(calInbox(u.Email)+m.ID+".ics", req, schedulingMessageProperties(m)))
				}
			}
		default:
			http.Error(w, "unsupported report", 400)
			return
		}
	}
	writeDAVXML(w, 207, out)
}

func inboxComponentFilter(n *davXML) (caldav.CompFilter, error) {
	out := caldav.CompFilter{Name: n.attr("name")}
	if n.Name != (xml.Name{Space: calNS, Local: "comp-filter"}) || out.Name == "" {
		return out, errInboxFilter
	}
	for _, c := range n.Children {
		switch c.Name {
		case xml.Name{Space: calNS, Local: "comp-filter"}:
			child, err := inboxComponentFilter(c)
			if err != nil {
				return out, err
			}
			out.Comps = append(out.Comps, child)
		case xml.Name{Space: calNS, Local: "is-not-defined"}:
			out.IsNotDefined = true
		case xml.Name{Space: calNS, Local: "time-range"}:
			var err error
			out.Start, err = parseInboxTime(c.attr("start"))
			if err != nil {
				return out, err
			}
			out.End, err = parseInboxTime(c.attr("end"))
			if err != nil {
				return out, err
			}
		default:
			return out, errInboxFilter
		}
	}
	return out, nil
}

var errInboxFilter = &filterError{}

type filterError struct{}

func (*filterError) Error() string { return "unsupported inbox filter" }
func parseInboxTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse("20060102T150405Z", value)
}
