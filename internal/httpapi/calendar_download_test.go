package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http/httptest"
	"net/mail"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func TestExternalCalendarUploadCreatesAnonymousDownload(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	st := f.s.store.(*storage.Store)
	if err := st.SetCalendarAttachmentLinks(f.s.cfg.HTTP.PublicURL, bytes.Repeat([]byte{17}, 32)); err != nil {
		t.Fatal(err)
	}
	cal, err := st.EnsureCalendar(ctx, f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	response := f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", `{"uid":"external-download","summary":"Flight","start":"2026-10-23T10:00:00Z","end":"2026-10-23T11:00:00Z","attendees":[{"email":"guest@another.example"}]}`, 200)
	var event struct{ ID string }
	if err = json.Unmarshal(response.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	job, err := st.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CommitCalendarDelivery(ctx, job, nil); err != nil {
		t.Fatal(err)
	}
	if err = st.CompleteCalendarDelivery(ctx, job); err != nil {
		t.Fatal(err)
	}
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "Посадочный талон.pdf")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("%PDF-1.7\nboarding pass\n")
	part.Write(data)
	writer.Close()
	req := httptest.NewRequest("POST", "/api/v1/calendar/events/"+event.ID+"/attachments", &form)
	req.Header.Set("Authorization", "Bearer "+f.tokens[0])
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response = httptest.NewRecorder()
	f.s.Handler().ServeHTTP(response, req)
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	var file struct{ ID string }
	if err = json.Unmarshal(response.Body.Bytes(), &file); err != nil {
		t.Fatal(err)
	}
	job, err = st.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal("upload failed to queue invitation update", err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(job.Data))
	if err != nil {
		t.Fatal(err)
	}
	_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	parts := multipart.NewReader(msg.Body, params["boundary"])
	var link string
	for {
		part, err := parts.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		kind, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if kind != "text/calendar" {
			continue
		}
		raw, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		p, err := calutil.ParseICS(string(raw))
		if err != nil || len(p.Attachments) != 1 {
			t.Fatal("missing link", err)
		}
		link = p.Attachments[0].URI
	}
	if link == "" {
		t.Fatal("no personalized download link")
	}
	w := f.call(-1, "GET", link, "", 200)
	if !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("anonymous download invalid")
	}
	_, disposition, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	if err != nil || disposition["filename"] != "Посадочный талон.pdf" {
		t.Fatal("filename corrupted", err)
	}
	if w = f.call(-1, "HEAD", link, "", 200); w.Body.Len() != 0 {
		t.Fatal("HEAD includes content")
	}
	if w = f.call(-1, "GET", link, "", 206, "Range", "bytes=0-3"); w.Body.String() != "%PDF" {
		t.Fatal("range corrupted")
	}
	f.call(-1, "POST", link, "", 405)
	f.call(-1, "GET", link+"x", "", 404)
	f.call(-1, "GET", "/api/v1/calendar/attachments/"+file.ID, "", 401)
	f.call(0, "DELETE", "/api/v1/calendar/attachments/"+file.ID, "", 200)
	f.call(-1, "GET", link, "", 404)
	obj, _ := st.GetCalendarObjectByID(ctx, event.ID)
	if strings.Contains(obj.Data, storage.CalendarDownloadPath) {
		t.Fatal("stored organizer event contains bearer link")
	}
}
