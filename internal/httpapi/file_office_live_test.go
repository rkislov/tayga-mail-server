package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in only: exposes a disposable synthetic document to an official demo.
// The listener serves WOPI and one random test page, never the API or real data.
func TestOfficePublicIntegration(t *testing.T) {
	office, public := os.Getenv("TAYGA_OFFICE_TEST_URL"), os.Getenv("TAYGA_PUBLIC_TEST_URL")
	if office == "" || public == "" {
		t.Skip("set TAYGA_OFFICE_TEST_URL and TAYGA_PUBLIC_TEST_URL for the manual public demo test")
	}
	f := newFileFixture(t)
	f.s.cfg.HTTP.PublicURL = public
	f.s.cfg.Files.OfficeURL = office
	var document bytes.Buffer
	zw := zip.NewWriter(&document)
	for _, part := range []struct{ name, body string }{
		{"mimetype", "application/vnd.oasis.opendocument.text"},
		{"META-INF/manifest.xml", `<?xml version="1.0"?><manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2"><manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/></manifest:manifest>`},
		{"content.xml", `<?xml version="1.0"?><office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" office:version="1.2"><office:body><office:text><text:p>Tayga synthetic document. No personal data.</text:p></office:text></office:body></office:document-content>`},
	} {
		w, e := zw.Create(part.name)
		if e != nil {
			t.Fatal(e)
		}
		w.Write([]byte(part.body))
	}
	zw.Close()
	f.call(0, "PUT", "/api/v1/files/content?path=integration-test.odt", document.String(), 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"integration-test.odt","email":"editor@example.com","rights":"write"}`, 200)
	sessions := []map[string]any{officeSessionFor(t, f, 0, "integration-test.odt"), officeSessionFor(t, f, 2, "integration-test.odt")}
	for _, session := range sessions {
		// HTML forms must contain decimal milliseconds, not Go's float64
		// exponent formatting after decoding JSON into map[string]any.
		session["access_token_ttl"] = fmt.Sprintf("%.0f", session["access_token_ttl"])
	}
	var key [24]byte
	if _, e := rand.Read(key[:]); e != nil {
		t.Fatal(e)
	}
	nonce := hex.EncodeToString(key[:])
	done := make(chan struct{}, 1)
	page := template.Must(template.New("test").Parse(`<!doctype html><html><head><meta name="referrer" content="no-referrer"><title>Tayga WOPI integration test</title></head><body style="margin:0"><iframe name="office" title="Office editor" style="width:100vw;height:100vh;border:0" allow="clipboard-read; clipboard-write; fullscreen"></iframe><form id="launch" target="office" method="post" action="{{.url}}"><input type="hidden" name="access_token" value="{{.access_token}}"><input type="hidden" name="access_token_ttl" value="{{.access_token_ttl}}"></form><script>document.getElementById('launch').submit()</script></body></html>`))
	mux := http.NewServeMux()
	mux.HandleFunc("/wopi/files/", func(w http.ResponseWriter, r *http.Request) {
		cw := &officeCaptureWriter{ResponseWriter: w}
		f.s.handleWOPI(cw, r)
		t.Logf("WOPI %s %s -> %d proof=%t", r.Method+" "+r.Header.Get("X-WOPI-Override"), r.URL.Path, cw.status, r.Header.Get("X-WOPI-Proof") != "")
	})
	mux.HandleFunc("/test/"+nonce, func(w http.ResponseWriter, r *http.Request) {
		who := 0
		if r.URL.Query().Get("user") == "2" {
			who = 1
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		page.Execute(w, sessions[who])
	})
	mux.HandleFunc("/done/"+nonce, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method", 405)
			return
		}
		select {
		case done <- struct{}{}:
		default:
		}
		w.WriteHeader(204)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:18439")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	info, _ := json.Marshal(map[string]string{"url": public + "/test/" + nonce, "done": public + "/done/" + nonce})
	if err = os.WriteFile(filepath.Join(os.TempDir(), "tayga-office-live.json"), info, 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("Ready: test page URL saved to temporary tayga-office-live.json (synthetic data only)")
	select {
	case <-done:
	case <-time.After(15 * time.Minute):
		t.Fatal("public editor test timed out")
	}
	root, _ := f.s.filesRoot(f.users[0])
	body, err := os.ReadFile(filepath.Join(root, "integration-test.odt"))
	if err != nil {
		t.Fatal(err)
	}
	archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		t.Fatal(e)
	}
	text := ""
	for _, file := range archive.File {
		if file.Name == "content.xml" {
			r, _ := file.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			text = string(b)
		}
	}
	if !strings.Contains(text, "WOPI-SAVE-OK") {
		t.Fatalf("editor did not save marker; synthetic document length=%d", len(body))
	}
	versions, err := f.s.fileRevisions(f.users[0], "versions", "integration-test.odt")
	if err != nil || len(versions) == 0 {
		t.Fatal("no previous version after save", err)
	}
	fmt.Println("Public Collabora save and version retention verified")
}

type officeCaptureWriter struct {
	http.ResponseWriter
	status int
}

func (w *officeCaptureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *officeCaptureWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}
