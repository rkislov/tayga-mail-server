package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestDocumentTemplatesCreatePackagesAndOfficeRoundTrip(t *testing.T) {
	f := newFileFixture(t)
	f.call(-1, "GET", "/api/v1/files/templates", "", 401)
	var catalog struct {
		Templates     []officeTemplate
		OfficeEnabled bool `json:"office_enabled"`
	}
	if err := json.Unmarshal(f.call(0, "GET", "/api/v1/files/templates", "", 200).Body.Bytes(), &catalog); err != nil || len(catalog.Templates) != 4 || catalog.OfficeEnabled {
		t.Fatal(catalog, err)
	}
	for _, item := range catalog.Templates {
		payload, _ := json.Marshal(map[string]string{"template_id": item.ID, "name": item.ID})
		var result struct{ Path, Name, ETag string }
		if err := json.Unmarshal(f.call(0, "POST", "/api/v1/files/templates", string(payload), 201).Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Name != item.ID+item.Extension || result.ETag == "" {
			t.Fatal(result)
		}
		data := f.call(0, "GET", "/dav/files/"+result.Path, "", 200).Body.Bytes()
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		entries := map[string]bool{}
		for _, entry := range archive.File {
			entries[entry.Name] = true
			if strings.HasSuffix(entry.Name, ".xml") || strings.HasSuffix(entry.Name, ".rels") {
				stream, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(stream)
				stream.Close()
				if err != nil {
					t.Fatal(err)
				}
				decoder := xml.NewDecoder(bytes.NewReader(raw))
				for {
					token, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(item.ID, entry.Name, err)
					}
					if start, ok := token.(xml.StartElement); ok && start.Name.Local == "Relationship" {
						for _, attr := range start.Attr {
							if attr.Name.Local == "TargetMode" && attr.Value == "External" {
								t.Fatal("starter contains external relationship", entry.Name)
							}
						}
					}
				}
			}
		}
		if !entries["[Content_Types].xml"] || !entries["_rels/.rels"] {
			t.Fatal("invalid office package", entries)
		}
		main := map[string]string{".docx": "word/document.xml", ".xlsx": "xl/workbook.xml", ".pptx": "ppt/presentation.xml"}[item.Extension]
		if !entries[main] {
			t.Fatal("main part missing", main)
		}
		f.call(0, "POST", "/api/v1/files/templates", string(payload), 409)
	}
	// The newly created document enters the existing WOPI edit/save lifecycle.
	mockOffice(t, f)
	session := officeSessionFor(t, f, 0, "document.docx")
	if session["writable"] != true {
		t.Fatal(session)
	}
	endpoint := wopiSessionURL(session)
	f.call(-1, "GET", endpoint, "", 200)
	f.call(-1, "POST", endpoint, "", 200, "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "created-document")
	updated, err := officeTemplates.ReadFile("file_templates/meeting-minutes.docx")
	if err != nil {
		t.Fatal(err)
	}
	f.call(-1, "POST", strings.Replace(endpoint, "?", "/contents?", 1), string(updated), 200, "X-WOPI-Override", "PUT", "X-WOPI-Lock", "created-document")
	if !bytes.Equal(f.call(0, "GET", "/dav/files/document.docx", "", 200).Body.Bytes(), updated) {
		t.Fatal("created document did not save through WOPI")
	}
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=document.docx", "", 200))) != 1 {
		t.Fatal("office save lost history")
	}
}
func TestDocumentTemplateCopiesReadableSourceAndEnforcesDestination(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=templates/base.txt", "Do not change original", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"templates/base.txt","email":"reader@example.com","rights":"read"}`, 200)
	request := `{"source_path":"templates/base.txt","source_owner_id":"` + f.users[0] + `","owner_id":"","name":"copy.txt"}`
	f.call(1, "POST", "/api/v1/files/templates?owner_id="+f.users[0], request, 201)
	if f.call(1, "GET", "/dav/files/copy.txt", "", 200).Body.String() != "Do not change original" {
		t.Fatal("copy differs")
	}
	if f.call(0, "GET", "/dav/files/templates/base.txt", "", 200).Body.String() != "Do not change original" {
		t.Fatal("template changed")
	}
	f.call(2, "POST", "/api/v1/files/templates", request, 403)
	f.call(0, "POST", "/api/v1/files/mkdir", `{"path":"team"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"reader@example.com","rights":"read"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"editor@example.com","rights":"write"}`, 200)
	shared := `{"template_id":"document","directory":"team","name":"shared.docx","owner_id":"` + f.users[0] + `"}`
	f.call(1, "POST", "/api/v1/files/templates", shared, 403)
	f.call(2, "POST", "/api/v1/files/templates", shared, 201)
	f.call(0, "GET", "/dav/files/team/shared.docx", "", 200)
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team","grantee_id":"`+f.users[2]+`"}`, 200)
	f.call(2, "POST", "/api/v1/files/templates", strings.Replace(shared, "shared.docx", "revoked.docx", 1), 403)
}
func TestDocumentTemplatesValidationAndQuota(t *testing.T) {
	f := newFileFixture(t)
	for _, body := range []string{`{}`, `{"template_id":"document","name":""}`, `{"template_id":"document","name":"../escape.docx"}`, `{"template_id":"document","name":"bad.xlsx"}`, `{"template_id":"document","source_path":"a.txt","name":"bad.docx"}`, `{"source_path":"bad.exe","name":"bad.exe"}`} {
		f.call(0, "POST", "/api/v1/files/templates", body, 400)
	}
	f.call(0, "POST", "/api/v1/files/templates", `{"template_id":"unknown","name":"a.docx"}`, 404)
	f.call(0, "POST", "/api/v1/files/templates", `{"template_id":"document","directory":"missing","name":"a.docx"}`, 404)
	f.call(0, "POST", "/api/v1/files/templates", `{"template_id":"document","directory":"../outside","name":"a.docx"}`, 400)
	ctx := context.Background()
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	sc, err := f.s.store.CreateServiceClass(ctx, &storage.ServiceClass{TenantID: owner.TenantID, Name: "templates-small", Config: `{"file_quota_bytes":1}`})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserServiceClass(ctx, owner.ID, sc.ID); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/files/templates", `{"template_id":"document","name":"no-space.docx"}`, 507)
	f.call(0, "GET", "/dav/files/no-space.docx", "", 404)
}
func TestDocumentTemplatesConcurrentNameAndSymlink(t *testing.T) {
	f := newFileFixture(t)
	body := `{"template_id":"document","name":"once.docx"}`
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/v1/files/templates", strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+f.tokens[0])
			w := httptest.NewRecorder()
			f.s.Handler().ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for code := range codes {
		got[code]++
	}
	if got[201] != 1 || got[409] != 1 {
		t.Fatal(got)
	}
	root, err := f.s.filesRoot(f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err = os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/files/templates", `{"source_path":"link/secret.txt","name":"leak.txt"}`, 400)
	f.call(0, "POST", "/api/v1/files/templates", `{"template_id":"document","directory":"link","name":"escape.docx"}`, 400)
}
