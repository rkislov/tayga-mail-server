package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func compareRequest(path, left, right, mode string) string {
	data, _ := json.Marshal(map[string]string{"path": path, "left": left, "right": right, "mode": mode})
	return string(data)
}
func makeCompareHistory(t *testing.T, f *fileFixture, path, old, current string) fileRevision {
	t.Helper()
	f.call(0, "PUT", "/api/v1/files/content?path="+url.QueryEscape(path), old, 200)
	f.call(0, "PUT", "/api/v1/files/content?path="+url.QueryEscape(path), current, 200)
	versions := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path="+url.QueryEscape(path), "", 200))
	if len(versions) != 1 {
		t.Fatal(versions)
	}
	return versions[0]
}
func TestFileComparisonTextAndHistoricalPair(t *testing.T) {
	f := newFileFixture(t)
	rev := makeCompareHistory(t, f, "notes.txt", "keep\nold\nend\n", "keep\nnew\nend\n")
	request := compareRequest("notes.txt", rev.ID, "current", "text")
	var result struct {
		Added, Removed int
		Lines          []documentDiffLine
		LeftETag       string `json:"left_etag"`
		RightETag      string `json:"right_etag"`
	}
	response := f.call(0, "POST", "/api/v1/files/library/compare", request, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Added != 1 || result.Removed != 1 || len(result.Lines) != 4 || result.LeftETag == "" || result.RightETag == "" {
		t.Fatal(result, err)
	}
	if result.Lines[1].Kind != "delete" || result.Lines[1].Text != "old" || result.Lines[2].Kind != "insert" || result.Lines[2].Text != "new" {
		t.Fatal(result)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=notes.txt", "third\n", 200)
	versions := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=notes.txt", "", 200))
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("notes.txt", rev.ID, versions[0].ID, "text"), 200)
	if f.call(0, "GET", "/dav/files/notes.txt", "", 200).Body.String() != "third\n" {
		t.Fatal("comparison changed live content")
	}
	f.call(1, "POST", "/api/v1/files/library/compare?owner_id="+f.users[0], request, 403)
	f.call(1, "POST", "/api/v1/files/library/compare", request, 404)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("other.txt", rev.ID, "current", "text"), 404)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("notes.txt", rev.ID, rev.ID, "text"), 400)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("../notes.txt", rev.ID, "current", "text"), 400)
	f.call(-1, "POST", "/api/v1/files/library/compare", request, 401)
}
func compareZip(t *testing.T, name, body string) string {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	part, err := archive.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte(body))
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	return data.String()
}
func TestFileComparisonDocxOdtLimitsAndMalformed(t *testing.T) {
	for _, ext := range []string{"docx", "odt"} {
		t.Run(ext, func(t *testing.T) {
			f := newFileFixture(t)
			var part, pattern string
			if ext == "docx" {
				part = "word/document.xml"
				pattern = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p><w:p><w:r><w:t>unchanged</w:t></w:r></w:p></w:body></w:document>`
			} else {
				part = "content.xml"
				pattern = `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"><office:body><office:text><text:p>%s</text:p><text:p>unchanged</text:p></office:text></office:body></office:document-content>`
			}
			path := "document." + ext
			rev := makeCompareHistory(t, f, path, compareZip(t, part, fmt.Sprintf(pattern, "Старая строка")), compareZip(t, part, fmt.Sprintf(pattern, "Новая строка")))
			response := f.call(0, "POST", "/api/v1/files/library/compare", compareRequest(path, rev.ID, "current", "text"), 200)
			var result struct{ Added, Removed int }
			json.Unmarshal(response.Body.Bytes(), &result)
			if result.Added != 1 || result.Removed != 1 {
				t.Fatal(response.Body.String())
			}
			f.call(0, "PUT", "/api/v1/files/content?path="+path, "broken package", 200)
			f.call(0, "POST", "/api/v1/files/library/compare", compareRequest(path, rev.ID, "current", "text"), 422)
		})
	}
	f := newFileFixture(t)
	rev := makeCompareHistory(t, f, "large.txt", "old", strings.Repeat("line\n", maxCompareLines+1))
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("large.txt", rev.ID, "current", "text"), 422)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("photo.pdf", rev.ID, "current", "text"), 415)
	f.call(0, "PUT", "/api/v1/files/content?path=large.txt", string([]byte{0xff, 0xfe, 0}), 200)
	f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("large.txt", rev.ID, "current", "text"), 422)
}
func parseComparisonConfig(t *testing.T, html string) (map[string]any, map[string]any) {
	t.Helper()
	match := regexp.MustCompile(`const config=(.*);const revised=(.*);const showError=`).FindStringSubmatch(html)
	if len(match) != 3 {
		t.Fatal("missing compare config", html)
	}
	var config, revised map[string]any
	if json.Unmarshal([]byte(match[1]), &config) != nil || json.Unmarshal([]byte(match[2]), &revised) != nil {
		t.Fatal("invalid compare config")
	}
	return config, revised
}
func TestFileComparisonOfficeReadOnlySignaturesAndRevocation(t *testing.T) {
	for _, provider := range []string{"onlyoffice", "r7"} {
		t.Run(provider, func(t *testing.T) {
			f := newFileFixture(t)
			f.s.cfg.Files.OfficeProvider = provider
			f.s.cfg.Files.OfficeURL = "https://office.example.com"
			f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
			rev := makeCompareHistory(t, f, "file.docx", "old content", "current content")
			request := compareRequest("file.docx", rev.ID, "current", "office")
			var launch struct {
				URL string `json:"url"`
			}
			json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/compare", request, 200).Body.Bytes(), &launch)
			address, _ := url.Parse(launch.URL)
			response := f.call(-1, "GET", address.RequestURI(), "", 200)
			config, revised := parseComparisonConfig(t, response.Body.String())
			for _, object := range []map[string]any{config, revised} {
				token, err := jwt.Parse(object["token"].(string), func(*jwt.Token) (any, error) { return []byte(documentTestSecret), nil }, jwt.WithValidMethods([]string{"HS256"}))
				if err != nil || !token.Valid {
					t.Fatal(err)
				}
			}
			doc := config["document"].(map[string]any)
			permissions := doc["permissions"].(map[string]any)
			if permissions["edit"] != false || permissions["review"] != false || config["editorConfig"].(map[string]any)["mode"] != "view" || revised["c"] != "compare" {
				t.Fatal(config, revised)
			}
			if _, ok := config["editorConfig"].(map[string]any)["callbackUrl"]; ok {
				t.Fatal("comparison has write callback")
			}
			oldURL, _ := url.Parse(doc["url"].(string))
			newURL, _ := url.Parse(revised["url"].(string))
			if f.call(-1, "GET", oldURL.RequestURI(), "", 200).Body.String() != "old content" || f.call(-1, "GET", newURL.RequestURI(), "", 200).Body.String() != "current content" {
				t.Fatal("wrong compare sides")
			}
			f.call(-1, "POST", newURL.RequestURI(), "overwrite", 405)
			f.call(-1, "GET", "/office/document/content?access_token="+address.Query().Get("access_token"), "", 401)
			f.call(0, "PUT", "/api/v1/files/content?path=file.docx", "concurrent edit", 200)
			f.call(-1, "GET", oldURL.RequestURI(), "", 409)
			f.call(-1, "GET", address.RequestURI(), "", 409)
			json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/compare", request, 200).Body.Bytes(), &launch)
			address, _ = url.Parse(launch.URL)
			f.s.cfg.Files.OfficeJWTSecret = strings.Repeat("n", 32)
			f.call(-1, "GET", address.RequestURI(), "", 401)
			f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
			token := address.Query().Get("access_token")
			f.s.fileMu.Lock()
			sess := f.s.fileComparisons[token]
			sess.Expires = time.Now().Add(-time.Second)
			f.s.fileComparisons[token] = sess
			f.s.fileMu.Unlock()
			f.call(-1, "GET", address.RequestURI(), "", 401)
		})
	}
}
func TestFileComparisonDeletedRevisionAndConfiguration(t *testing.T) {
	f := newFileFixture(t)
	rev := makeCompareHistory(t, f, "test.txt", "old", "new")
	request := compareRequest("test.txt", rev.ID, "current", "office")
	f.call(0, "POST", "/api/v1/files/library/compare", request, 503)
	f.s.cfg.Files.OfficeProvider = "onlyoffice"
	f.s.cfg.Files.OfficeURL = "https://office.example.com"
	f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
	var result struct {
		URL string `json:"url"`
	}
	json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/compare", request, 200).Body.Bytes(), &result)
	address, _ := url.Parse(result.URL)
	f.call(0, "DELETE", "/api/v1/files/library/versions", `{"id":"`+rev.ID+`"}`, 200)
	f.call(-1, "GET", address.RequestURI(), "", 404)
	f.call(-1, "GET", "/office/compare/editor?access_token="+strings.Repeat("0", 64), "", 401)
}

func TestDocumentLineDiffReconstructsVersions(t *testing.T) {
	cases := []struct {
		left, right    []string
		added, removed int
	}{
		{[]string{}, []string{}, 0, 0},
		{[]string{"one"}, []string{"one"}, 0, 0},
		{[]string{"one", "two"}, []string{"one", "new", "two"}, 1, 0},
		{[]string{"a", "b", "a"}, []string{"a", "a"}, 0, 1},
		{[]string{"old"}, []string{"new"}, 1, 1},
	}
	for _, c := range cases {
		diff, added, removed := documentLineDiff(c.left, c.right)
		if added != c.added || removed != c.removed {
			t.Fatal(diff, added, removed)
		}
		a, b := []string{}, []string{}
		for _, line := range diff {
			if line.Kind != "insert" {
				a = append(a, line.Text)
			}
			if line.Kind != "delete" {
				b = append(b, line.Text)
			}
		}
		if strings.Join(a, "\n") != strings.Join(c.left, "\n") || strings.Join(b, "\n") != strings.Join(c.right, "\n") {
			t.Fatal("diff lost text", diff)
		}
	}
}
func TestFileComparisonODFSpacesAndComments(t *testing.T) {
	f := newFileFixture(t)
	body := `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"><office:body><office:text><text:p>Word<text:s text:c="3"/>next<office:annotation><text:p>hidden comment</text:p></office:annotation></text:p></office:text></office:body></office:document-content>`
	old := compareZip(t, "content.xml", body)
	current := compareZip(t, "content.xml", strings.Replace(body, "hidden comment", "different comment", 1))
	rev := makeCompareHistory(t, f, "spaces.odt", old, current)
	response := f.call(0, "POST", "/api/v1/files/library/compare", compareRequest("spaces.odt", rev.ID, "current", "text"), 200)
	var result struct {
		Added, Removed int
		Lines          []documentDiffLine
	}
	json.Unmarshal(response.Body.Bytes(), &result)
	if result.Added != 0 || result.Removed != 0 || len(result.Lines) != 1 || result.Lines[0].Text != "Word   next" {
		t.Fatal(response.Body.String())
	}
}
