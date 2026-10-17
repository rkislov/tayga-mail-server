package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func beginUpload(t *testing.T, f *fileFixture, path string, size int) resumableUpload {
	t.Helper()
	var u resumableUpload
	w := f.call(0, "POST", "/api/v1/files/uploads", fmt.Sprintf(`{"path":%q,"size":%d}`, path, size), 201)
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	return u
}
func TestResumableUploadRecoveryIsolationAndConflict(t *testing.T) {
	f := newFileFixture(t)
	u := beginUpload(t, f, "dir/a.txt", 6)
	route := "/api/v1/files/uploads/" + u.ID
	f.call(1, "GET", route, "", 404)
	f.call(1, "DELETE", route, "", 404)
	f.call(0, "PATCH", route, "abc", 200, "Upload-Offset", "0")
	f.call(0, "PATCH", route, "abc", 409, "Upload-Offset", "0")
	// Simulate a crash during the next chunk; acknowledged prefix must survive.
	root, _ := f.s.fileStateRoot(f.users[0], "resumable")
	file := filepath.Join(root, u.ID, "content")
	fd, _ := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	fd.WriteString("broken")
	fd.Close()
	var state struct {
		Upload resumableUpload
		Chunks []uploadChunk
	}
	json.Unmarshal(f.call(0, "GET", route, "", 200).Body.Bytes(), &state)
	if state.Upload.Offset != 3 || len(state.Chunks) != 1 || state.Chunks[0].SHA256 != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal(state)
	}
	f.call(0, "PATCH", route, "def", 200, "Upload-Offset", "3")
	f.call(0, "POST", route+"/complete", "", 200)
	f.call(0, "POST", route+"/complete", "", 200)
	if got := f.call(0, "GET", "/api/v1/files/content?path=dir/a.txt", "", 200).Body.String(); got != "abcdef" {
		t.Fatal(got)
	}
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=dir/a.txt", "", 200))) != 0 {
		t.Fatal("idempotent commit created a version")
	}
	v := beginUpload(t, f, "dir/a.txt", 3)
	r := "/api/v1/files/uploads/" + v.ID
	f.call(0, "PATCH", r, "new", 200, "Upload-Offset", "0")
	f.call(0, "PUT", "/api/v1/files/content?path=dir/a.txt", "concurrent edit", 200)
	f.call(0, "POST", r+"/complete", "", 409)
	if got := f.call(0, "GET", "/api/v1/files/content?path=dir/a.txt", "", 200).Body.String(); got != "concurrent edit" {
		t.Fatal(got)
	}
	f.call(0, "DELETE", r, "", 200)
	f.call(0, "GET", r, "", 404)
}
func TestResumableUploadBoundsExpiryAndEmptyFile(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "POST", "/api/v1/files/uploads", `{"path":"a","size":10737418241}`, 413)
	f.call(0, "POST", "/api/v1/files/uploads", `{"path":"../a","size":1}`, 400)
	u := beginUpload(t, f, "empty", 0)
	route := "/api/v1/files/uploads/" + u.ID
	f.call(0, "POST", route+"/complete", "", 200)
	v := beginUpload(t, f, "b", 3)
	r := "/api/v1/files/uploads/" + v.ID
	f.call(0, "PATCH", r, "four", 400, "Upload-Offset", "0")
	f.call(0, "POST", r+"/complete", "", 400)
	root, _ := f.s.fileStateRoot(f.users[0], "resumable")
	dir := filepath.Join(root, v.ID)
	rec, _ := readUploadRecord(dir)
	rec.Expires = time.Now().Add(-time.Minute)
	saveUploadRecord(dir, rec)
	f.call(0, "GET", r, "", 410)
	for i := 0; i < 4; i++ {
		beginUpload(t, f, fmt.Sprint(i), 1)
	}
	f.call(0, "POST", "/api/v1/files/uploads", `{"path":"too-many","size":1}`, 429)
}
func TestResumableCommitCrashAndRevokedAccess(t *testing.T) {
	f := newFileFixture(t)
	u := beginUpload(t, f, "recover", 3)
	r := "/api/v1/files/uploads/" + u.ID
	f.call(0, "PATCH", r, "abc", 200, "Upload-Offset", "0")
	root, _ := f.s.fileStateRoot(f.users[0], "resumable")
	dir := filepath.Join(root, u.ID)
	dest, _ := f.s.filesRoot(f.users[0])
	if err := os.Link(filepath.Join(dir, "content"), filepath.Join(dest, "recover")); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", r+"/complete", "", 200)
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=recover", "", 200))) != 0 {
		t.Fatal("crash recovery added version")
	}
	f.call(0, "POST", "/api/v1/files/mkdir", `{"path":"shared"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"shared","email":"editor@example.com","rights":"write"}`, 200)
	var share resumableUpload
	json.Unmarshal(f.call(2, "POST", "/api/v1/files/uploads", `{"path":"shared/file","owner_id":"`+f.users[0]+`","size":3}`, 201).Body.Bytes(), &share)
	sr := "/api/v1/files/uploads/" + share.ID
	f.call(2, "PATCH", sr, "abc", 200, "Upload-Offset", "0")
	f.call(0, "DELETE", "/api/v1/files?path=shared", "", 200)
	f.call(2, "POST", sr+"/complete", "", 403)
	f.call(2, "DELETE", sr, "", 200)
	if strings.Contains(f.call(0, "GET", "/api/v1/files/uploads", "", 200).Body.String(), share.ID) {
		t.Fatal("other user's session exposed")
	}
}
