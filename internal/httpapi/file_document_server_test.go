package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const documentTestSecret = "onlyoffice-test-secret-at-least-32-bytes"

func documentConfigFor(t *testing.T, f *fileFixture, who int, path string) (map[string]any, string) {
	t.Helper()
	session := officeSessionFor(t, f, who, path)
	endpoint, err := url.Parse(session["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	page := f.call(-1, "GET", endpoint.RequestURI(), "", 200)
	match := regexp.MustCompile(`const config=(.*);config.events=`).FindStringSubmatch(page.Body.String())
	if len(match) != 2 {
		t.Fatal("missing editor config", page.Body.String())
	}
	var config map[string]any
	if err = json.Unmarshal([]byte(match[1]), &config); err != nil {
		t.Fatal(err)
	}
	signed, err := jwt.Parse(config["token"].(string), func(*jwt.Token) (any, error) { return []byte(documentTestSecret), nil }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !signed.Valid {
		t.Fatal("invalid editor config signature", err)
	}
	return config, endpoint.RequestURI()
}

func callbackFor(t *testing.T, f *fileFixture, config map[string]any, status int, download string, code int) *httptest.ResponseRecorder {
	t.Helper()
	doc := config["document"].(map[string]any)
	cb := map[string]any{"key": doc["key"], "status": status, "url": download, "filetype": doc["fileType"]}
	signed, err := signDocumentConfig(map[string]any{"payload": cb}, f.s.cfg.Files.OfficeJWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(config["editorConfig"].(map[string]any)["callbackUrl"].(string))
	return f.call(-1, "POST", endpoint.RequestURI(), `{}`, code, "Authorization", "Bearer "+signed)
}

func TestDocumentServerSaveCollaborationAndPermissions(t *testing.T) {
	for _, provider := range []string{"onlyoffice", "r7"} {
		t.Run(provider, func(t *testing.T) {
			f := newFileFixture(t)
			saved := "force saved"
			documentServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirect" {
					http.Redirect(w, r, "https://example.net/private", 302)
					return
				}
				if r.URL.Path != "/saved" {
					http.NotFound(w, r)
					return
				}
				w.Write([]byte(saved))
			}))
			defer documentServer.Close()
			old := officeHTTPClient
			officeHTTPClient = documentServer.Client()
			defer func() { officeHTTPClient = old }()
			f.s.cfg.Files.OfficeURL = documentServer.URL
			f.s.cfg.Files.OfficeProvider = provider
			f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
			f.call(0, "PUT", "/api/v1/files/content?path=test.docx", "initial", 200)
			f.call(0, "POST", "/api/v1/files/access", `{"path":"test.docx","email":"reader@example.com","rights":"read"}`, 200)
			f.call(0, "POST", "/api/v1/files/access", `{"path":"test.docx","email":"editor@example.com","rights":"write"}`, 200)
			own, ownPage := documentConfigFor(t, f, 0, "test.docx")
			reader, _ := documentConfigFor(t, f, 1, "test.docx")
			editor, _ := documentConfigFor(t, f, 2, "test.docx")
			doc := own["document"].(map[string]any)
			pageURL, _ := url.Parse(ownPage)
			if doc["key"] == pageURL.Query().Get("access_token") {
				t.Fatal("collaboration key leaks owner credentials")
			}
			if reader["document"].(map[string]any)["key"] != doc["key"] || editor["document"].(map[string]any)["key"] != doc["key"] {
				t.Fatal("coediting key differs")
			}
			if reader["document"].(map[string]any)["permissions"].(map[string]any)["edit"] != false || reader["editorConfig"].(map[string]any)["mode"] != "view" {
				t.Fatal("reader can edit")
			}
			content, _ := url.Parse(doc["url"].(string))
			if got := f.call(-1, "GET", content.RequestURI(), "", 200).Body.String(); got != "initial" {
				t.Fatal(got)
			}
			f.call(-1, "GET", "/office/document/content?access_token="+strings.Repeat("0", 64), "", 401)
			callback, _ := url.Parse(own["editorConfig"].(map[string]any)["callbackUrl"].(string))
			f.call(-1, "POST", callback.RequestURI(), `{"key":"forged","status":2}`, 401)
			f.call(0, "PUT", "/api/v1/files/content?path=test.docx", "conflict", 423)
			f.call(0, "PUT", "/dav/files/test.docx", "conflict", 423)
			callbackFor(t, f, reader, 6, documentServer.URL+"/saved", 403)
			callbackFor(t, f, own, 6, "https://example.net/private", 400)
			callbackFor(t, f, own, 6, documentServer.URL+"/redirect", 502)
			if got := callbackFor(t, f, editor, 6, documentServer.URL+"/saved", 200).Body.String(); !strings.Contains(got, `"error":0`) {
				t.Fatal(got)
			}
			if got := f.call(0, "GET", "/api/v1/files/content?path=test.docx", "", 200).Body.String(); got != saved {
				t.Fatal(got)
			}
			if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=test.docx", "", 200))) != 1 {
				t.Fatal("missing version")
			}
			joined, _ := documentConfigFor(t, f, 0, "test.docx")
			if joined["document"].(map[string]any)["key"] != doc["key"] {
				t.Fatal("force save split coediting generation")
			}
			f.call(0, "DELETE", "/api/v1/files/access", `{"path":"test.docx","grantee_id":"`+f.users[2]+`"}`, 200)
			callbackFor(t, f, editor, 6, documentServer.URL+"/saved", 401)
			// A body token is also accepted. Unsigned fields cannot override it.
			bodyToken, _ := signDocumentConfig(map[string]any{"key": doc["key"], "status": 1}, documentTestSecret)
			f.call(-1, "POST", callback.RequestURI(), `{"token":"`+bodyToken+`","status":2,"url":"https://example.net/private"}`, 200)
			saved = "final saved"
			callbackFor(t, f, own, 2, documentServer.URL+"/saved", 200)
			callbackFor(t, f, own, 2, documentServer.URL+"/saved", 200)
			f.call(-1, "GET", ownPage, "", 410)
			if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=test.docx", "", 200))) != 2 {
				t.Fatal("duplicate final callback changed versions")
			}
			reopened, _ := documentConfigFor(t, f, 0, "test.docx")
			if reopened["document"].(map[string]any)["key"] == doc["key"] {
				t.Fatal("reopened document uses stale cache key")
			}
			callbackFor(t, f, reopened, 4, "", 200)
			f.call(0, "PUT", "/api/v1/files/content?path=test.docx", "after close", 200)
		})
	}
}

func TestDocumentServerRevocationAndFormats(t *testing.T) {
	f := newFileFixture(t)
	f.s.cfg.Files.OfficeURL = "https://office.example.com"
	f.s.cfg.Files.OfficeProvider = "onlyoffice"
	f.call(0, "PUT", "/api/v1/files/content?path=test.xlsx", "sheet", 200)
	f.call(0, "POST", "/api/v1/files/office", `{"path":"test.xlsx"}`, 503)
	f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
	config, page := documentConfigFor(t, f, 0, "test.xlsx")
	if config["documentType"] != "cell" {
		t.Fatal("wrong spreadsheet editor")
	}
	endpoint, _ := url.Parse(page)
	token := endpoint.Query().Get("access_token")
	f.call(-1, "GET", "/wopi/files/"+officeFileID(f.users[0], "test.xlsx")+"?access_token="+token, "", 401)
	f.s.cfg.Files.OfficeJWTSecret = strings.Repeat("changed", 8)
	f.call(-1, "GET", page, "", 401)
	f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
	f.s.cfg.Files.OfficeProvider = "r7"
	f.call(-1, "GET", page, "", 401)
	rotated, _ := documentConfigFor(t, f, 0, "test.xlsx")
	callbackFor(t, f, rotated, 4, "", 200)
	f.s.cfg.Files.OfficeProvider = "onlyoffice"
	sess := f.s.officeSessions[token]
	sess.Expires = time.Now().Add(-time.Minute)
	f.s.officeSessions[token] = sess
	f.call(-1, "GET", page, "", 401)
	f.call(0, "PUT", "/api/v1/files/content?path=test.odg", "drawing", 200)
	f.call(0, "POST", "/api/v1/files/office", `{"path":"test.odg"}`, 400)
}
