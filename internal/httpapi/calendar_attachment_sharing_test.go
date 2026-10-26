package httpapi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestInvitedCalendarAttachmentHTTPAccess(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	cal, err := f.s.store.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/calendar/calendars/" + cal.ID + "/events"
	response := f.call(0, "POST", base, `{"uid":"shared-boarding","summary":"Flight","start":"2026-10-23T10:00:00Z","end":"2026-10-23T11:00:00Z","attendees":[{"email":"reader@example.com"}]}`, 200)
	var created struct{ ID string }
	if err = json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	invites, err := f.s.store.ListCalendarInvitesForUser(ctx, f.users[1], false)
	if err != nil || len(invites) != 1 {
		t.Fatal(invites, err)
	}
	data := []byte("%PDF-1.7\nboard\n")
	root := filepath.Join(f.s.ms.Root, "calendar-attachments")
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "boarding.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := f.s.store.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: created.ID, UserID: f.users[0], Filename: "boarding.pdf", ContentType: "application/pdf", Size: int64(len(data)), StoragePath: "boarding.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	url := "/api/v1/calendar/attachments/" + file.ID
	copyURL := "/api/v1/calendar/events/" + invites[0].CalendarObjectID
	response = f.call(1, "GET", copyURL+"/attachments", "", 200)
	if !strings.Contains(response.Body.String(), `"read_only":true`) || !strings.Contains(response.Body.String(), file.ID) {
		t.Fatal(response.Body.String())
	}
	response = f.call(1, "GET", copyURL, "", 200)
	if !strings.Contains(response.Body.String(), file.ID) {
		t.Fatal("event details omit shared files")
	}
	response = f.call(1, "GET", url, "", 200)
	if response.Body.String() != string(data) {
		t.Fatal("wrong content")
	}
	response = f.call(1, "GET", url, "", 206, "Range", "bytes=0-3")
	if response.Body.String() != "%PDF" {
		t.Fatal("range corrupted")
	}
	f.call(2, "GET", url, "", 403)
	f.call(1, "DELETE", url, "", 403)
	f.call(0, "GET", url, "", 200)
	// Declining the invitation revokes the web link, too.
	f.call(1, "POST", "/api/v1/calendar/invites/"+invites[0].ID+"/reply", `{"action":"decline"}`, 200)
	f.call(1, "GET", url, "", 403)
	f.call(0, "GET", url, "", 200)
}
