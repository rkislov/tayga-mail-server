package httpapi

import (
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func visualLaunch(t *testing.T, f *fileFixture, path, id string) (string, string) {
	t.Helper()
	var result struct {
		Mode  string
		Left  string `json:"left_url"`
		Right string `json:"right_url"`
	}
	if err := json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/compare", compareRequest(path, id, "current", "visual"), 200).Body.Bytes(), &result); err != nil || result.Mode != "visual" || result.Left == result.Right {
		t.Fatal(result, err)
	}
	return result.Left, result.Right
}
func TestRenderedComparisonPDFPermissionsAndRevocation(t *testing.T) {
	f := newFileFixture(t)
	rev := makeCompareHistory(t, f, "pages.pdf", "%PDF-1.4\nold", "%PDF-1.4\nnew")
	left, right := visualLaunch(t, f, "pages.pdf", rev.ID)
	for i, endpoint := range []string{left, right} {
		response := f.call(-1, "GET", endpoint, "", 200)
		want := "old"
		if i == 1 {
			want = "new"
		}
		if !strings.HasSuffix(response.Body.String(), want) || response.Header().Get("Content-Type") != "application/pdf" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(response)
		}
	}
	f.call(-1, "POST", left, "", 405)
	f.call(-1, "GET", strings.Replace(left, "/pdf?", "/editor?", 1), "", 403)
	f.call(-1, "GET", strings.Replace(left, "side=left", "side=other", 1), "", 400)
	f.call(1, "POST", "/api/v1/files/library/compare?owner_id="+f.users[0], compareRequest("pages.pdf", rev.ID, "current", "visual"), 403)
	f.call(0, "PUT", "/api/v1/files/content?path=pages.pdf", "%PDF-1.4\nchanged", 200)
	f.call(-1, "GET", left, "", 409)
	left, _ = visualLaunch(t, f, "pages.pdf", rev.ID)
	f.call(0, "DELETE", "/api/v1/files/library/versions", `{"id":"`+rev.ID+`"}`, 200)
	f.call(-1, "GET", left, "", 404)
	rev = makeCompareHistory(t, f, "broken.pdf", "not pdf", "also not pdf")
	left, _ = visualLaunch(t, f, "broken.pdf", rev.ID)
	f.call(-1, "GET", left, "", 422)
}
func TestRenderedComparisonConverterSecurity(t *testing.T) {
	f := newFileFixture(t)
	rev := makeCompareHistory(t, f, "slides.pptx", "old slides", "new slides")
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("slides.pptx", rev.ID, "current", "visual"), 503)
	var host string
	target := "/result"
	mutate := false
	downloads := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/converter":
			var payload map[string]any
			if json.NewDecoder(r.Body).Decode(&payload) != nil {
				t.Error("invalid request")
				w.WriteHeader(400)
				return
			}
			signed, err := jwt.Parse(payload["token"].(string), func(*jwt.Token) (any, error) { return []byte(documentTestSecret), nil }, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil || !signed.Valid {
				t.Error("unsigned conversion")
			}
			claims := signed.Claims.(jwt.MapClaims)
			if claims["outputtype"] != "pdf" || claims["url"] != payload["url"] || payload["async"] != false {
				t.Error(payload)
			}
			source, _ := url.Parse(payload["url"].(string))
			if got := f.call(-1, "GET", source.RequestURI(), "", 200).Body.String(); got != "old slides" && got != "new slides" {
				t.Error(got)
			}
			if mutate {
				f.call(0, "PUT", "/api/v1/files/content?path=slides.pptx", "changed during conversion", 200)
			}
			address := host + target
			if strings.HasPrefix(target, "https:") {
				address = target
			}
			json.NewEncoder(w).Encode(map[string]any{"endConvert": true, "fileUrl": address})
		case "/result":
			downloads++
			w.Write([]byte("%PDF-1.4\nrendered"))
		case "/redirect":
			http.Redirect(w, r, host+"/result", 302)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	host = server.URL
	old := fileRenderClient
	fileRenderClient = server.Client()
	fileRenderClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	defer func() { fileRenderClient = old }()
	f.s.cfg.Files.OfficeProvider = "onlyoffice"
	f.s.cfg.Files.OfficeURL = host
	f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
	left, _ := visualLaunch(t, f, "slides.pptx", rev.ID)
	if !strings.Contains(f.call(-1, "GET", left, "", 200).Body.String(), "rendered") {
		t.Fatal("missing PDF")
	}
	target = "https://elsewhere.invalid/private"
	f.call(-1, "GET", left, "", 502)
	if downloads != 1 {
		t.Fatal("untrusted output downloaded")
	}
	target = "/redirect"
	f.call(-1, "GET", left, "", 502)
	if downloads != 1 {
		t.Fatal("redirect followed")
	}
	target = "/result"
	mutate = true
	f.call(-1, "GET", left, "", 409)
}

func TestRenderedComparisonCollabora(t *testing.T) {
	for _, provider := range []string{"", "collabora"} {
		t.Run("provider="+provider, func(t *testing.T) {
			f := newFileFixture(t)
			rev := makeCompareHistory(t, f, "document.odt", "old document", "new document")
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/cool/convert-to/pdf" {
					t.Error(r.URL.Path)
					w.WriteHeader(404)
					return
				}
				reader, err := r.MultipartReader()
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				part, err := reader.NextPart()
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				data, err := io.ReadAll(part)
				if err != nil || part.FormName() != "data" || part.FileName() != "comparison.odt" || (string(data) != "old document" && string(data) != "new document") {
					t.Error(part, data, err)
				}
				if _, err = reader.NextPart(); err != io.EOF {
					t.Error("multipart did not end", err)
				}
				w.Write([]byte("%PDF-1.4\nconverted document"))
			}))
			defer server.Close()
			old := fileRenderClient
			fileRenderClient = server.Client()
			defer func() { fileRenderClient = old }()
			f.s.cfg.Files.OfficeProvider = provider
			f.s.cfg.Files.OfficeURL = server.URL
			var capability map[string]any
			json.Unmarshal(f.call(0, "GET", "/api/v1/files/library/versions?path=document.odt", "", 200).Body.Bytes(), &capability)
			if capability["visual_compare_enabled"] != true || capability["office_compare_enabled"] != false {
				t.Fatal(capability)
			}
			left, right := visualLaunch(t, f, "document.odt", rev.ID)
			f.call(-1, "GET", left, "", 200)
			f.call(-1, "GET", right, "", 200)
			f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("document.odt", rev.ID, "current", "office"), 503)
		})
	}
}

func TestRenderedComparisonBusyAndExpired(t *testing.T) {
	f := newFileFixture(t)
	rev := makeCompareHistory(t, f, "test.pdf", "%PDF-1.4\nold", "%PDF-1.4\nnew")
	left, _ := visualLaunch(t, f, "test.pdf", rev.ID)
	for i := 0; i < cap(fileRenderSlots); i++ {
		fileRenderSlots <- struct{}{}
	}
	f.call(-1, "GET", left, "", 429)
	for i := 0; i < cap(fileRenderSlots); i++ {
		<-fileRenderSlots
	}
	parsed, _ := url.Parse(left)
	token := parsed.Query().Get("access_token")
	f.s.fileMu.Lock()
	sess := f.s.fileComparisons[token]
	sess.Expires = time.Now().Add(-time.Second)
	f.s.fileComparisons[token] = sess
	f.s.fileMu.Unlock()
	f.call(-1, "GET", left, "", 401)
	left, _ = visualLaunch(t, f, "test.pdf", rev.ID)
	f.s.cfg.Files.OfficeJWTSecret = "revoked"
	f.call(-1, "GET", left, "", 401)
}
