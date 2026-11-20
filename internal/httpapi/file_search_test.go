package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fileContentSearchResult struct {
	Entries []struct {
		Owner, OwnerEmail         string
		Path, Name, Rights, Match string
		Shared                    bool
		Snippet                   *fileSearchSnippet
	} `json:"entries"`
	Total, Skipped, Empty int
	Truncated             bool
	PDFAvailable          bool `json:"pdf_search_available"`
}

func searchDOCX(t *testing.T, parts map[string]string) string {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for name, text := range parts {
		part, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return data.String()
}
func searchWord(t *testing.T, text string) string {
	t.Helper()
	var escaped bytes.Buffer
	xml.EscapeText(&escaped, []byte(text))
	return searchDOCX(t, map[string]string{"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + escaped.String() + `</w:t></w:r></w:p></w:body></w:document>`})
}
func searchDocuments(t *testing.T, f *fileFixture, user int, query, extra string) fileContentSearchResult {
	t.Helper()
	var raw struct {
		Entries []struct {
			Owner                     string `json:"owner_id"`
			OwnerEmail                string `json:"owner_email"`
			Path, Name, Rights, Match string
			Shared                    bool
			Snippet                   *fileSearchSnippet
		}
		Total        int
		Skipped      int `json:"skipped_documents"`
		Empty        int `json:"empty_documents"`
		Truncated    bool
		PDFAvailable bool `json:"pdf_search_available"`
	}
	w := f.call(user, "GET", "/api/v1/files/library/search?mode=content&q="+url.QueryEscape(query)+extra, "", 200)
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	out := fileContentSearchResult{Total: raw.Total, Skipped: raw.Skipped, Empty: raw.Empty, Truncated: raw.Truncated, PDFAvailable: raw.PDFAvailable}
	for _, e := range raw.Entries {
		out.Entries = append(out.Entries, struct {
			Owner, OwnerEmail         string
			Path, Name, Rights, Match string
			Shared                    bool
			Snippet                   *fileSearchSnippet
		}{e.Owner, e.OwnerEmail, e.Path, e.Name, e.Rights, e.Match, e.Shared, e.Snippet})
	}
	return out
}
func TestFileContentSearchPermissionsAndHistory(t *testing.T) {
	f := newFileFixture(t)
	word := searchWord(t, "Поставка оборудования согласована в ноябре. Straße")
	f.call(0, "PUT", "/api/v1/files/content?path=team/report.docx", word, 200)
	f.call(0, "PUT", "/api/v1/files/content?path=private/secret.docx", word, 200)
	f.call(0, "PUT", "/api/v1/files/content?path=private/corrupt.docx", "corrupt", 200)
	f.call(1, "PUT", "/api/v1/files/content?path=personal.docx", word, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"reader@example.com","rights":"read"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team/report.docx","email":"reader@example.com","rights":"write"}`, 200)
	result := searchDocuments(t, f, 1, "ПОСТАВКА  оборудования", "")
	if result.Total != 2 || result.Skipped != 0 || result.Empty != 0 {
		t.Fatalf("search %+v", result)
	}
	shared := 0
	for _, entry := range result.Entries {
		if entry.Snippet == nil || !strings.Contains(entry.Snippet.Match, "Поставка") {
			t.Fatalf("snippet %+v", entry)
		}
		if entry.Shared {
			shared++
			if entry.Path != "team/report.docx" || entry.Owner != f.users[0] || entry.Rights != "write" {
				t.Fatalf("shared %+v", entry)
			}
		}
	}
	if shared != 1 {
		t.Fatal("overlapping grants duplicate results")
	}
	own := searchDocuments(t, f, 1, "поставка", "&scope=own")
	if own.Total != 1 {
		t.Fatalf("own %+v", own)
	}
	if got := searchDocuments(t, f, 2, "поставка", ""); got.Total != 0 || got.Skipped != 0 {
		t.Fatal("private data/counters exposed")
	}
	folded := searchDocuments(t, f, 1, "STRASSE", "")
	if folded.Entries[0].Snippet.Match != "Straße" {
		t.Fatalf("Unicode folding %+v", folded)
	}
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team/report.docx","grantee_id":"`+f.users[1]+`"}`, 200)
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team","grantee_id":"`+f.users[1]+`"}`, 200)
	if got := searchDocuments(t, f, 1, "поставка", ""); got.Total != 1 {
		t.Fatal("cache bypassed revocation")
	}
	f.call(0, "PUT", "/api/v1/files/content?path=history.docx", searchWord(t, "историческийсекрет"), 200)
	f.call(0, "PUT", "/api/v1/files/content?path=history.docx", searchWord(t, "текущий текст"), 200)
	f.call(0, "PUT", "/api/v1/files/content?path=trash.docx", searchWord(t, "историческийсекрет"), 200)
	f.call(0, "DELETE", "/api/v1/files?path=trash.docx", "", 200)
	if got := searchDocuments(t, f, 0, "историческийсекрет", ""); got.Total != 0 {
		t.Fatal("trash/version content exposed")
	}
	f.call(-1, "GET", "/api/v1/files/library/search?mode=content&q=поставка", "", 401)
	f.call(1, "GET", "/api/v1/files/library/search?mode=content&q=x&owner_id="+f.users[0], "", 403)
	f.call(0, "GET", "/api/v1/files/library/search?mode=content&q=x&path=../private", "", 400)
	f.call(0, "GET", "/api/v1/files/library/search?mode=invalid&q=x", "", 400)
}
func TestFileContentSearchCacheAndFailures(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=cache.docx", searchWord(t, "oldword"), 200)
	if searchDocuments(t, f, 0, "oldword", "").Total != 1 {
		t.Fatal("initial text")
	}
	root, _ := f.s.filesRoot(f.users[0])
	path := filepath.Join(root, "cache.docx")
	before, _ := os.Stat(path)
	f.call(0, "PUT", "/api/v1/files/content?path=cache.docx", searchWord(t, "newword"), 200)
	os.Chtimes(path, before.ModTime(), before.ModTime())
	if searchDocuments(t, f, 0, "oldword", "").Total != 0 || searchDocuments(t, f, 0, "newword", "").Total != 1 {
		t.Fatal("stale text after atomic replacement")
	}
	f.call(0, "POST", "/api/v1/files/move", `{"from":"cache.docx","to":"renamed.docx"}`, 200)
	got := searchDocuments(t, f, 0, "newword", "")
	if got.Total != 1 || got.Entries[0].Path != "renamed.docx" {
		t.Fatal("move")
	}
	f.call(0, "PUT", "/api/v1/files/content?path=broken.docx", "broken", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=scan.pdf", "not pdf", 200)
	got = searchDocuments(t, f, 0, "newword", "")
	if got.Total != 1 || got.Skipped != 2 {
		t.Fatalf("skips %+v", got)
	}
	t.Setenv("PATH", t.TempDir())
	f.s.fileSearchCache = nil
	got = searchDocuments(t, f, 0, "newword", "")
	if got.PDFAvailable || got.Total != 1 || got.Skipped != 2 {
		t.Fatalf("missing PDF converter %+v", got)
	}
	if err := os.Symlink(path, filepath.Join(root, "symlink.docx")); err != nil {
		t.Fatal(err)
	}
	if got := searchDocuments(t, f, 0, "newword", ""); got.Total != 1 {
		t.Fatal("symlink indexed")
	}
}
func TestFileContentSearchPDF(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=ticket.pdf", string(ticketTestPDF()), 200)
	got := searchDocuments(t, f, 0, "moscow", "")
	if got.Total != 1 || got.Entries[0].Snippet.Match != "Moscow" {
		t.Fatalf("PDF search %+v", got)
	}
}
func TestFileContentExtractionBoundaries(t *testing.T) {
	parts := map[string]string{"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>split</w:t></w:r><w:r><w:t>word</w:t></w:r></w:p><w:del><w:r><w:t>deletedsecret</w:t></w:r></w:del></w:body></w:document>`, "word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:r><w:t>headertext</w:t></w:r></w:p></w:hdr>`}
	file := filepath.Join(t.TempDir(), "sample.docx")
	os.WriteFile(file, []byte(searchDOCX(t, parts)), 0600)
	fd, _ := os.Open(file)
	text, err := extractFileSearchText(context.Background(), fd, file)
	fd.Close()
	if err != nil || !strings.Contains(text, "splitword") || !strings.Contains(text, "headertext") || strings.Contains(text, "deletedsecret") {
		t.Fatalf("DOCX text %q %v", text, err)
	}
	os.WriteFile(file, []byte(searchWord(t, strings.Repeat("x", maxFileSearchText+1))), 0600)
	fd, _ = os.Open(file)
	_, err = extractFileSearchText(context.Background(), fd, file)
	fd.Close()
	if err == nil {
		t.Fatal("unbounded extraction")
	}
	for _, sample := range []struct{ Text, Query, Match string }{{"Поставка оборудования", "поставка", "Поставка"}, {"Straße", "strasse", "Straße"}, {"longprefix AΣ test", "aσ", "AΣ"}} {
		snippet, found := contentSearchSnippet(normalizeFileSearchText(sample.Text), foldFileSearchText(sample.Query))
		if !found || snippet.Match != sample.Match {
			t.Fatalf("snippet %+v %v", snippet, found)
		}
	}
}
func TestFileContentSearchRevalidatesInFlight(t *testing.T) {
	for _, change := range []string{"revoke", "replace"} {
		t.Run(change, func(t *testing.T) {
			f := newFileFixture(t)
			f.call(0, "PUT", "/api/v1/files/content?path=slow.pdf", "original", 200)
			f.call(0, "POST", "/api/v1/files/access", `{"path":"slow.pdf","email":"reader@example.com","rights":"read"}`, 200)
			folder := t.TempDir()
			started, release := filepath.Join(folder, "started"), filepath.Join(folder, "release")
			binary := fmt.Sprintf("#!/bin/sh\n/bin/cat >/dev/null\nprintf ready > '%s'\nwhile [ ! -f '%s' ]; do /bin/sleep 0.01; done\nprintf 'sensitive needle'\n", started, release)
			os.WriteFile(filepath.Join(folder, "pdftotext"), []byte(binary), 0700)
			t.Setenv("PATH", folder)
			req := httptest.NewRequest("GET", "/api/v1/files/library/search?mode=content&q=needle", nil)
			req.Header.Set("Authorization", "Bearer "+f.tokens[1])
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { f.s.Handler().ServeHTTP(response, req); close(done) }()
			defer os.WriteFile(release, []byte("done"), 0600)
			deadline := time.Now().Add(2 * time.Second)
			for {
				if _, err := os.Stat(started); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("extractor did not start")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if change == "revoke" {
				f.call(0, "DELETE", "/api/v1/files/access", `{"path":"slow.pdf","grantee_id":"`+f.users[1]+`"}`, 200)
			} else {
				f.call(0, "PUT", "/api/v1/files/content?path=slow.pdf", "replaced", 200)
			}
			os.WriteFile(release, []byte("done"), 0600)
			<-done
			if change == "revoke" && response.Code != 409 {
				t.Fatalf("revocation %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "sensitive") || strings.Contains(response.Body.String(), `"path":"slow.pdf"`) {
				t.Fatalf("stale snippet leaked %s", response.Body.String())
			}
		})
	}
}

func TestFileContentSearchPaginationAndNameMode(t *testing.T) {
	f := newFileFixture(t)
	root, _ := f.s.filesRoot(f.users[0])
	if err := os.MkdirAll(filepath.Join(root, "folder"), 0750); err != nil {
		t.Fatal(err)
	}
	data := searchWord(t, "unique body phrase")
	for i := 0; i < 52; i++ {
		if err := os.WriteFile(filepath.Join(root, "folder", fmt.Sprintf("report-%02d.docx", i)), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var page struct {
		Total   int
		Entries []struct{ Path, Match string }
	}
	w := f.call(0, "GET", "/api/v1/files/library/search?mode=name&scope=own&q=report-&page=2", "", 200)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 52 || len(page.Entries) != 2 || page.Entries[0].Path != "folder/report-50.docx" {
		t.Fatalf("page %+v", page)
	}
	if page.Entries[0].Match != "name" {
		t.Fatal("wrong mode")
	}
	if got := searchDocuments(t, f, 0, "unique body phrase", "&scope=own&path=folder"); got.Total != 52 {
		t.Fatalf("scoped %+v", got)
	}
	f.s.fileSearchMu.Lock()
	defer f.s.fileSearchMu.Unlock()
	size := 0
	for _, e := range f.s.fileSearchCache {
		size += len(e.Text)
	}
	if len(f.s.fileSearchCache) > 64 || size > 32<<20 {
		t.Fatal("cache limits")
	}
}
