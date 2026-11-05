// SPDX-License-Identifier: Apache-2.0
package dav

import (
	"bytes"
	"context"
	"encoding/xml"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type calScheduleMatchKey struct{}
type scheduleMetadataKey struct{}
type scheduleMetadata map[string]string

func metadataPath(p string) string { return strings.TrimRight(p, "/") }
func recordScheduleMetadata(ctx context.Context, p string, o *storage.CalendarObject) {
	data, ok := ctx.Value(scheduleMetadataKey{}).(scheduleMetadata)
	if !ok {
		return
	}
	parsed, err := calutil.ParseICS(o.Data)
	if err == nil && parsed.Organizer.Email != "" && len(parsed.Attendees) > 0 {
		data[metadataPath(p)] = o.ScheduleTag
	}
}
func calInbox(email string) string  { return calPrincipal(email) + "inbox/" }
func calOutbox(email string) string { return calPrincipal(email) + "outbox/" }

type scheduleWriter struct {
	http.ResponseWriter
	request  *http.Request
	metadata scheduleMetadata
	written  bool
}

func (w *scheduleWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.written = true
	if status >= 200 && status < 300 && (w.request.Method == "GET" || w.request.Method == "HEAD" || w.request.Method == "PUT") {
		if tag := w.metadata[metadataPath(w.request.URL.Path)]; tag != "" {
			w.Header().Set("Schedule-Tag", tag)
		}
	}
	if w.request.Method == "OPTIONS" {
		w.Header().Set("DAV", w.Header().Get("DAV")+", calendar-auto-schedule")
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *scheduleWriter) Write(p []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}

type davCapture struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *davCapture) Header() http.Header { return w.header }
func (w *davCapture) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *davCapture) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	return w.body.Write(p)
}

func calendarScheduling(store storage.Driver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFrom(r.Context())
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		parts := splitPath(strings.TrimPrefix(r.URL.Path, calPrefix))
		if len(parts) > 0 && !strings.EqualFold(parts[0], u.Email) {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method == "PUT" || r.Method == "DELETE" {
			reply := r.Header.Get("Schedule-Reply")
			if reply != "" && reply != "T" && reply != "F" {
				http.Error(w, "invalid Schedule-Reply", 400)
				return
			}
			tag := r.Header.Get("If-Schedule-Tag-Match")
			if len(r.Header.Values("If-Schedule-Tag-Match")) > 1 || tag != "" && (len(tag) < 2 || tag[0] != '"' || tag[len(tag)-1] != '"' || strings.ContainsAny(tag[1:len(tag)-1], "\"\r\n,")) {
				http.Error(w, "invalid schedule tag", 400)
				return
			}
			ctx := storage.WithCalDAVScheduling(r.Context(), r.Method != "DELETE" || reply != "F")
			ctx = context.WithValue(ctx, calScheduleMatchKey{}, tag)
			r = r.WithContext(ctx)
		}
		metadata := scheduleMetadata{}
		r = r.WithContext(context.WithValue(r.Context(), scheduleMetadataKey{}, metadata))
		writer := &scheduleWriter{ResponseWriter: w, request: r, metadata: metadata}
		if len(parts) >= 2 && (parts[1] == "inbox" || parts[1] == "outbox") {
			serveSchedulingCollection(store, writer, r, u, parts)
			return
		}
		if r.Method != "PROPFIND" && r.Method != "REPORT" {
			next.ServeHTTP(writer, r)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil {
			http.Error(w, "request too large", 413)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		var req *davXML
		if len(bytes.TrimSpace(raw)) > 0 {
			req, err = parseDAVXML(raw)
			if err != nil {
				http.Error(w, "invalid XML", 400)
				return
			}
		}
		capture := &davCapture{header: make(http.Header)}
		next.ServeHTTP(capture, r)
		for key, values := range capture.header {
			w.Header()[key] = values
		}
		w.Header().Del("Content-Length")
		if capture.code != 207 {
			writer.WriteHeader(capture.code)
			_, _ = writer.Write(capture.body.Bytes())
			return
		}
		result, err := parseDAVXMLLimit(capture.body.Bytes(), 500000)
		if err != nil {
			http.Error(w, "invalid DAV response", 500)
			return
		}
		requested := propertyRequest(req)
		for _, response := range result.Children {
			href := response.child("DAV:", "href")
			if href == nil {
				continue
			}
			p, err := url.PathUnescape(href.Text)
			if err != nil {
				continue
			}
			values := map[xml.Name]*davXML{}
			if metadataPath(p) == metadataPath(calPrincipal(u.Email)) {
				for name, value := range principalSchedulingProperties(u.Email) {
					values[name] = value
				}
			}
			if tag := metadata[metadataPath(p)]; tag != "" {
				values[xml.Name{Space: calNS, Local: "schedule-tag"}] = xt(calNS, "schedule-tag", tag)
			}
			_, name, object, parsed := parseCalPath(p)
			if parsed && name != "" && object == "" {
				values[xml.Name{Space: calNS, Local: "schedule-calendar-transp"}] = xn(calNS, "schedule-calendar-transp", xn(calNS, "opaque"))
			}
			augmentSchedulingProperties(response, requested, values)
		}
		if r.Method == "PROPFIND" && metadataPath(r.URL.Path) == metadataPath(calPrincipal(u.Email)) && r.Header.Get("Depth") != "0" {
			for _, kind := range []string{"inbox", "outbox"} {
				result.Children = append(result.Children, propertyResponse(calPrincipal(u.Email)+kind+"/", requested, schedulingCollectionProperties(u.Email, kind)))
			}
		}
		writeDAVXML(w, 207, result)
	})
}
func principalSchedulingProperties(email string) map[xml.Name]*davXML {
	out := map[xml.Name]*davXML{}
	for name, href := range map[string]string{"schedule-inbox-URL": calInbox(email), "schedule-outbox-URL": calOutbox(email)} {
		out[xml.Name{Space: calNS, Local: name}] = xn(calNS, name, xt("DAV:", "href", href))
	}
	out[xml.Name{Space: calNS, Local: "calendar-user-address-set"}] = xn(calNS, "calendar-user-address-set", xt("DAV:", "href", "mailto:"+email))
	out[xml.Name{Space: calNS, Local: "calendar-user-type"}] = xt(calNS, "calendar-user-type", "INDIVIDUAL")
	return out
}
func augmentSchedulingProperties(response *davXML, req davPropertyRequest, values map[xml.Name]*davXML) {
	wanted := map[xml.Name]bool{}
	for _, name := range req.Properties {
		wanted[name] = true
	}
	prop := xn("DAV:", "prop")
	for name, value := range values {
		if !wanted[name] && !req.All && !req.Names {
			continue
		}
		if name.Local == "schedule-tag" && req.All && !wanted[name] {
			continue
		}
		for _, ps := range response.Children {
			if p := ps.child("DAV:", "prop"); p != nil {
				kept := p.Children[:0]
				for _, c := range p.Children {
					if c.Name != name {
						kept = append(kept, c)
					}
				}
				p.Children = kept
			}
		}
		if req.Names {
			value = xn(name.Space, name.Local)
		}
		prop.Children = append(prop.Children, value)
	}
	kept := response.Children[:0]
	for _, ps := range response.Children {
		p := ps.child("DAV:", "prop")
		if p == nil || len(p.Children) > 0 {
			kept = append(kept, ps)
		}
	}
	response.Children = kept
	if len(prop.Children) > 0 {
		response.Children = append(response.Children, xn("DAV:", "propstat", prop, xt("DAV:", "status", "HTTP/1.1 200 OK")))
	}
}
