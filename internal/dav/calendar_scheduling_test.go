package dav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestCalDAVSchedulingHTTP(t *testing.T) {
	ctx := context.Background()
	s, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "dav.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	tenant, _ := s.CreateTenant(ctx, "DAV")
	domain, _ := s.CreateDomain(ctx, tenant.ID, "example.com")
	users := []*storage.User{}
	for _, name := range []string{"organizer", "participant"} {
		u, err := s.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: name + "@example.com", LocalPart: name, Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.EnsureDAVDefaults(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		users = append(users, u)
	}
	handler := calendarRequestConditions(&caldav.Handler{Backend: &calBackend{store: s}, Prefix: calPrefix})
	path := calObjectPath(users[0].Email, "default", "meeting.ics")
	request := func(method, path, body, etag string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(withUser(r.Context(), users[0]))
		r.Header.Set("Content-Type", "text/calendar")
		if etag != "" {
			r.Header.Set("If-Match", etag)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//DAV client//EN\r\nBEGIN:VEVENT\r\nUID:dav-meeting\r\nDTSTAMP:20261020T100000Z\r\nDTSTART:20261021T100000Z\r\nDTEND:20261021T110000Z\r\nSUMMARY:Meeting\r\nORGANIZER:mailto:organizer@example.com\r\nATTENDEE:mailto:participant@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	w := request("PUT", path, ics, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("ETag") != "" {
		t.Fatal("validator for transformed request")
	}
	invites, err := s.ListCalendarInvitesForUser(ctx, users[1].ID, false)
	if err != nil || len(invites) != 1 {
		t.Fatal(invites, err)
	}
	job, err := s.ClaimCalendarDelivery(ctx)
	if err != nil || job.Recipient != users[1].Email {
		t.Fatal("missing queued invitation", err)
	}
	w = request("GET", path, "", "")
	etag, body := w.Header().Get("ETag"), w.Body.String()
	if w.Code != 200 || etag == "" {
		t.Fatal("GET failed", w.Code)
	}
	w = request("PUT", path, body, etag)
	if w.Code != http.StatusCreated {
		t.Fatalf("replay: %d %s", w.Code, w.Body.String())
	}
	if _, err = s.ClaimCalendarDelivery(ctx); err != storage.ErrNotFound {
		t.Fatal("replay sent duplicate mail", err)
	}
	w = request("PUT", path, strings.Replace(body, "SUMMARY:Meeting", "SUMMARY:Updated", 1), etag)
	if w.Code != http.StatusCreated {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	copy, _ := s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	p, _ := calutil.ParseICS(copy.Data)
	if p.Summary != "Updated" || p.Sequence != 1 {
		t.Fatal("participant not updated", p.Event)
	}
	for _, method := range []string{"PUT", "DELETE"} {
		w = request(method, path, body, etag)
		if w.Code != http.StatusPreconditionFailed {
			t.Fatalf("stale %s: %d %s", method, w.Code, w.Body.String())
		}
	}
	w = request("GET", path, "", "")
	w = request("DELETE", path, "", w.Header().Get("ETag"))
	if w.Code < 200 || w.Code >= 300 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	copy, _ = s.GetCalendarObjectByID(ctx, invites[0].CalendarObjectID)
	p, _ = calutil.ParseICS(copy.Data)
	if p.Status != "CANCELLED" {
		t.Fatal("participant not cancelled")
	}
	// Tasks remain ordinary DAV objects, without meeting fan-out.
	task := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:test\r\nBEGIN:VTODO\r\nUID:task\r\nDTSTAMP:20261020T100000Z\r\nSUMMARY:Task\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	w = request("PUT", strings.Replace(path, "meeting.ics", "task.ics", 1), task, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("task: %d %s", w.Code, w.Body.String())
	}
}
