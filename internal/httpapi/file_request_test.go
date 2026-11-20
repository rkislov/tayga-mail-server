package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
)

func newFileRequest(t *testing.T, f *fileFixture) (string, string) {
	t.Helper()
	f.call(0, "POST", "/api/v1/files/mkdir", `{"path":"incoming"}`, 200)
	var share struct{ ID, Token string }
	if err := json.Unmarshal(f.call(0, "POST", "/api/v1/files/shares", `{"path":"incoming","rights":"upload","ttl_hours":24}`, 201).Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	return share.ID, "/s/" + share.Token
}
func TestFileRequestUploadOnlyAndNotification(t *testing.T) {
	f := newFileFixture(t)
	hub := notify.NewHub()
	f.s.SetNotify(hub)
	_, link := newFileRequest(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=incoming/report.txt", "private original", 200)
	page := f.call(-1, "GET", link, "", 200)
	if !strings.Contains(page.Body.String(), "Отправить документы") || strings.Contains(page.Body.String(), "private original") || strings.Contains(page.Body.String(), "report.txt") || strings.Contains(page.Body.String(), "incoming") {
		t.Fatal("page leaks folder contents")
	}
	if page.Header().Get("Cache-Control") != "no-store" || page.Header().Get("Referrer-Policy") != "no-referrer" || !strings.Contains(page.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing public page headers")
	}
	for _, query := range []string{"?path=report.txt", "?path=missing.txt", "?archive=1", "?name=report.txt"} {
		f.call(-1, "GET", link+query, "", 403)
	}
	for _, method := range []string{"HEAD", "PROPFIND", "POST", "DELETE", "MOVE"} {
		f.call(-1, method, link, "", 405)
	}
	for _, name := range []string{"../report.txt", "a/b.txt", `a\b.txt`, "..", "", ".upload-hidden", "a\n.txt"} {
		f.call(-1, "PUT", link+"?name="+url.QueryEscape(name), "bad", 400)
	}
	f.call(-1, "PUT", link+"?path=report.txt&name=report.txt", "bad", 403)
	for i := 0; i < 2; i++ {
		response := f.call(-1, "PUT", link+"?name=report.txt", "external", 201)
		if strings.Contains(response.Body.String(), "incoming") || strings.Contains(response.Body.String(), "report") {
			t.Fatal("response exposes target path")
		}
	}
	root, _ := f.s.filesRoot(f.users[0])
	entries, err := os.ReadDir(filepath.Join(root, "incoming"))
	if err != nil || len(entries) != 3 {
		t.Fatalf("entries %v %v", entries, err)
	}
	original, _ := os.ReadFile(filepath.Join(root, "incoming", "report.txt"))
	if string(original) != "private original" {
		t.Fatal("overwritten")
	}
	for _, e := range entries {
		if e.Name() != "report.txt" {
			body, _ := os.ReadFile(filepath.Join(root, "incoming", e.Name()))
			if string(body) != "external" {
				t.Fatal("wrong content")
			}
		}
	}
	if got := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions", "", 200)); len(got) != 0 {
		t.Fatal("upload created versions")
	}
	notices := hub.Recent(f.users[0], 20)
	if len(notices) != 2 || notices[0].Kind != notify.KindFiles || notices[0].Href != "files" {
		t.Fatalf("notices %+v", notices)
	}
	if len(hub.Recent(f.users[1], 20)) != 0 {
		t.Fatal("wrong recipient")
	}
	// One persistent owner notice per upload, even if the live hub/cache is gone.
	f.s.SetNotify(nil)
	f.s.fileActivities = nil
	var result struct{ Events []notify.Event }
	json.Unmarshal(f.call(0, "GET", "/api/v1/notifications", "", 200).Body.Bytes(), &result)
	if len(result.Events) != 2 {
		t.Fatalf("persistent notifications %+v", result.Events)
	}
	json.Unmarshal(f.call(1, "GET", "/api/v1/notifications", "", 200).Body.Bytes(), &result)
	if len(result.Events) != 0 {
		t.Fatal("notification leak")
	}
	card := readFileCard(t, f, 0, "path=incoming")
	if card.Events[0].Action != "file_received" {
		t.Fatalf("folder activity %+v", card.Events)
	}
	activityRoot, _ := f.s.fileStateRoot(f.users[0], "activity")
	log, _ := os.ReadFile(filepath.Join(activityRoot, "events.jsonl"))
	if strings.Contains(string(log), strings.TrimPrefix(link, "/s/")) {
		t.Fatal("capability token in activity")
	}
}
func TestFileRequestConstraintsAndRevocation(t *testing.T) {
	for _, mode := range []string{"revoke", "expire", "move", "disable"} {
		t.Run(mode, func(t *testing.T) {
			f := newFileFixture(t)
			id, link := newFileRequest(t, f)
			started, resume := make(chan struct{}), make(chan struct{})
			request := httptest.NewRequest("PUT", link+"?name=document.pdf", &pausedReader{started: started, resume: resume, data: strings.NewReader("document")})
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { f.s.Handler().ServeHTTP(response, request); close(done) }()
			<-started
			switch mode {
			case "revoke":
				f.call(0, "DELETE", "/api/v1/files/shares/"+id, "", 200)
			case "move":
				f.call(0, "POST", "/api/v1/files/move", `{"from":"incoming","to":"moved"}`, 200)
			case "expire":
				_, err := f.s.store.(*storage.Store).DB().Exec("UPDATE file_shares SET expires_at=? WHERE id=?", time.Now().Add(-time.Hour), id)
				if err != nil {
					t.Fatal(err)
				}
			case "disable":
				_, err := f.s.store.(*storage.Store).DB().Exec("UPDATE users SET enabled=FALSE WHERE id=?", f.users[0])
				if err != nil {
					t.Fatal(err)
				}
			}
			close(resume)
			<-done
			if response.Code != 403 {
				t.Fatalf("%s: %d %s", mode, response.Code, response.Body.String())
			}
			root, _ := f.s.filesRoot(f.users[0])
			for _, folder := range []string{"incoming", "moved"} {
				entries, _ := os.ReadDir(filepath.Join(root, folder))
				if len(entries) != 0 {
					t.Fatalf("published after %s", mode)
				}
			}
		})
	}
	f := newFileFixture(t)
	id, link := newFileRequest(t, f)
	f.call(0, "POST", "/api/v1/files/shares", `{"path":"incoming","rights":"upload"}`, 400)
	f.call(0, "PUT", "/api/v1/files/content?path=file.txt", "x", 200)
	f.call(0, "POST", "/api/v1/files/shares", `{"path":"file.txt","rights":"upload","ttl_hours":24}`, 400)
	f.call(0, "POST", "/api/v1/files/shares", `{"path":"incoming","rights":"upload","ttl_hours":24,"max_downloads":1}`, 400)
	f.call(1, "DELETE", "/api/v1/files/shares/"+id, "", 404)
	if _, err := f.s.store.(*storage.Store).DB().Exec("UPDATE file_shares SET expires_at=? WHERE id=?", time.Now().Add(-time.Hour), id); err != nil {
		t.Fatal(err)
	}
	f.call(-1, "GET", link, "", 410)
	f.call(-1, "PUT", link+"?name=a.txt", "x", 410)
}
func TestFileRequestQuotaFailureAndConcurrentNames(t *testing.T) {
	f := newFileFixture(t)
	_, link := newFileRequest(t, f)
	ctx := context.Background()
	user, _ := f.s.store.GetUserByID(ctx, f.users[0])
	sc, err := f.s.store.CreateServiceClass(ctx, &storage.ServiceClass{TenantID: user.TenantID, Name: "request-quota", Config: `{"file_quota_bytes":4}`})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserServiceClass(ctx, user.ID, sc.ID); err != nil {
		t.Fatal(err)
	}
	f.call(-1, "PUT", link+"?name=big.txt", "12345", 507)
	hub := notify.NewHub()
	hub.SetSink(func(string, notify.Event) error { return errors.New("push unavailable") })
	f.s.SetNotify(hub)
	// Concurrent same-name uploads create separate immutable destinations.
	var wg sync.WaitGroup
	responses := make(chan int, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("PUT", link+"?name=same.txt", strings.NewReader("xx"))
			w := httptest.NewRecorder()
			f.s.Handler().ServeHTTP(w, r)
			responses <- w.Code
		}()
	}
	wg.Wait()
	close(responses)
	success, blocked := 0, 0
	for code := range responses {
		if code == 201 {
			success++
		} else if code == 507 {
			blocked++
		} else {
			t.Fatalf("code %d", code)
		}
	}
	if success != 2 || blocked != 1 {
		t.Fatalf("success %d quota %d", success, blocked)
	}
	root, _ := f.s.filesRoot(user.ID)
	entries, _ := os.ReadDir(filepath.Join(root, "incoming"))
	if len(entries) != 2 {
		t.Fatal("lost concurrent uploads")
	}
	if len(hub.Recent(user.ID, 20)) != 2 {
		t.Fatal("notification count")
	}
	if f.s.fileRequestUploads[user.ID] != 0 {
		t.Fatal("leaked upload slot")
	}
}
func TestFileRequestBoundsAndCancellation(t *testing.T) {
	f := newFileFixture(t)
	_, link := newFileRequest(t, f)
	f.s.fileRequestUploads = map[string]int{f.users[0]: 4}
	f.call(-1, "PUT", link+"?name=x.txt", "x", 429)
	f.s.fileRequestUploads[f.users[0]] = 0
	req := httptest.NewRequest("PUT", link+"?name=x.txt", strings.NewReader("x"))
	req.ContentLength = maxWebUploadBytes + 1
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, req)
	if w.Code != 413 {
		t.Fatalf("size %d", w.Code)
	}
	req = httptest.NewRequest("PUT", link+"?name=x.txt", strings.NewReader("x"))
	req.ContentLength = 3
	w = httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("incomplete %d", w.Code)
	}
	root, _ := f.s.filesRoot(f.users[0])
	entries, _ := os.ReadDir(filepath.Join(root, "incoming"))
	if len(entries) != 0 {
		t.Fatal("partial file published")
	}
	f.call(-1, "PUT", link+"?name=valid.txt", "ok", 201)
	f.call(-1, "GET", "/file-request-assets/file_request.js", "", 200)
	f.call(-1, "GET", "/file-request-assets/file_request.html", "", 404)
}
