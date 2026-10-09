package httpapi

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

const maxCalAttachmentBytes = 32 << 20 // 32 MiB

func (s *Server) calendarAttachRoot() (string, error) {
	if s.ms == nil {
		return "", errString("mailstore unavailable")
	}
	root := filepath.Join(s.ms.Root, "calendar-attachments")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	return root, nil
}

func (s *Server) attachmentPublicURL(id string) string {
	base := ""
	if s.cfg != nil {
		base = strings.TrimRight(s.cfg.HTTP.PublicURL, "/")
	}
	path := "/api/v1/calendar/attachments/" + url.PathEscape(id)
	if base == "" {
		return path
	}
	return base + path
}

func attachmentToJSON(a *storage.CalendarAttachment) map[string]any {
	return map[string]any{
		"id":                 a.ID,
		"calendar_object_id": a.CalendarObjectID,
		"filename":           a.Filename,
		"content_type":       a.ContentType,
		"size":               a.Size,
		"created_at":         a.CreatedAt.UTC().Format(time.RFC3339),
		"url":                "/api/v1/calendar/attachments/" + a.ID,
	}
}

func (s *Server) eventOwnedOrAccessible(r *http.Request, au *authUser, objectID string) (*storage.CalendarObject, error) {
	o, err := s.store.GetCalendarObjectByID(r.Context(), objectID)
	if err != nil {
		return nil, err
	}
	if cal, err := s.calendarAccessible(r, au, o.CalendarID); err != nil || (cal.Name == "personal" && cal.UserID != au.ID) {
		return nil, storage.ErrNotFound
	}
	return o, nil
}

func (s *Server) handleCalendarEventAttachments(w http.ResponseWriter, r *http.Request, au *authUser, eventID string) {
	o, err := s.eventOwnedOrAccessible(r, au, eventID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListCalendarAttachments(r.Context(), o.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]map[string]any, 0, len(list))
		for _, a := range list {
			out = append(out, attachmentToJSON(a))
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachments": out})

	case http.MethodPost:
		// only calendar owner can attach
		if _, err := s.store.GetCalendarByID(r.Context(), au.ID, o.CalendarID); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if s.ms == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
			return
		}
		if err := r.ParseMultipartForm(maxCalAttachmentBytes); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart form required (file)"})
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file required"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxCalAttachmentBytes+1))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read failed"})
			return
		}
		if int64(len(data)) > maxCalAttachmentBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file too large"})
			return
		}
		filename := filepath.Base(hdr.Filename)
		if filename == "" || filename == "." {
			filename = "file"
		}
		ct := hdr.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		attID := storage.NewID()
		rel := filepath.ToSlash(filepath.Join(au.ID, o.ID, attID+"_"+sanitizeAttachName(filename)))
		root, err := s.calendarAttachRoot()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := os.WriteFile(abs, data, 0o640); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		saved, err := s.store.CreateCalendarAttachment(r.Context(), &storage.CalendarAttachment{
			ID: attID, CalendarObjectID: o.ID, UserID: au.ID,
			Filename: filename, ContentType: ct, Size: int64(len(data)), StoragePath: rel,
		})
		if err != nil {
			_ = os.Remove(abs)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = s.syncEventAttachmentsICS(r, o)
		writeJSON(w, http.StatusCreated, attachmentToJSON(saved))

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleCalendarAttachmentByID(w http.ResponseWriter, r *http.Request, au *authUser, attachID string) {
	a, err := s.store.GetCalendarAttachment(r.Context(), attachID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	o, err := s.eventOwnedOrAccessible(r, au, a.CalendarObjectID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if s.ms == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
			return
		}
		root, err := s.calendarAttachRoot()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(a.StoragePath))
		f, err := os.Open(abs)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "file missing"})
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "file missing"})
			return
		}
		ct := a.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(a.Filename, `"`, "")+`"`)
		http.ServeContent(w, r, a.Filename, st.ModTime(), f)

	case http.MethodDelete:
		if _, err := s.store.GetCalendarByID(r.Context(), au.ID, o.CalendarID); err != nil && a.UserID != au.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if s.ms != nil {
			if root, err := s.calendarAttachRoot(); err == nil {
				_ = os.Remove(filepath.Join(root, filepath.FromSlash(a.StoragePath)))
			}
		}
		if err := s.store.DeleteCalendarAttachment(r.Context(), a.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = s.syncEventAttachmentsICS(r, o)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func sanitizeAttachName(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "file"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func (s *Server) syncEventAttachmentsICS(r *http.Request, o *storage.CalendarObject) error {
	list, err := s.store.ListCalendarAttachments(r.Context(), o.ID)
	if err != nil {
		return err
	}
	parsed, _ := calutil.ParseICS(o.Data)
	ev := parsed.Event
	ev.Attachments = nil
	for _, a := range list {
		ev.Attachments = append(ev.Attachments, calutil.Attachment{
			URI: s.attachmentPublicURL(a.ID), Filename: a.Filename, ContentType: a.ContentType,
		})
	}
	var ics string
	if len(ev.Attendees) > 0 || ev.Organizer.Email != "" {
		ics, err = calutil.BuildRequestICS(ev)
		if err != nil {
			ics, err = calutil.BuildSimpleVEVENT(ev)
		}
	} else {
		ics, err = calutil.BuildSimpleVEVENT(ev)
	}
	if err != nil {
		return err
	}
	o.Data = ics
	o.Size = int64(len(ics))
	o.ETag = storage.ContentETag(ics)
	_, err = s.store.UpsertCalendarObject(r.Context(), o)
	return err
}

func (s *Server) attachmentsForEventJSON(r *http.Request, objectID string) []map[string]any {
	list, err := s.store.ListCalendarAttachments(r.Context(), objectID)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, attachmentToJSON(a))
	}
	return out
}
