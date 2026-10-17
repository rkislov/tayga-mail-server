package httpapi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func mockOffice(t *testing.T, f *fileFixture) {
	t.Helper()
	var base string
	demo := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hosting/discovery" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `<wopi-discovery><net-zone><app name="writer"><action name="edit" ext="odt" urlsrc="%s/browser/editor?&amp;&lt;lang=UI_LLCC&amp;&gt;"/><action name="view" ext="odt" urlsrc="%s/browser/editor?"/></app></net-zone></wopi-discovery>`, base, base)
	}))
	base = demo.URL
	t.Cleanup(demo.Close)
	old := officeHTTPClient
	officeHTTPClient = demo.Client()
	t.Cleanup(func() { officeHTTPClient = old })
	f.s.cfg.Files.OfficeURL = base
}
func officeSessionFor(t *testing.T, f *fileFixture, who int, path string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"path": path, "owner_id": f.users[0]})
	w := f.call(who, "POST", "/api/v1/files/office", string(body), 200)
	var data map[string]any
	if json.Unmarshal(w.Body.Bytes(), &data) != nil {
		t.Fatal("session JSON")
	}
	return data
}
func TestOfficeWOPISaveLocksSharingAndRevocation(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "POST", "/api/v1/files/office", `{"path":"test.odt"}`, 503)
	mockOffice(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=test.odt", "document", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"test.odt","email":"reader@example.com","rights":"read"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"test.odt","email":"editor@example.com","rights":"write"}`, 200)
	own, reader, editor := officeSessionFor(t, f, 0, "test.odt"), officeSessionFor(t, f, 1, "test.odt"), officeSessionFor(t, f, 2, "test.odt")
	if own["file_id"] != editor["file_id"] || reader["writable"] != false {
		t.Fatal("collaboration identity/permissions")
	}
	launch, _ := url.Parse(own["url"].(string))
	if launch.Query().Get("WOPISrc") != "https://files.example.com/wopi/files/"+own["file_id"].(string) {
		t.Fatal("bad WOPISrc")
	}
	endpoint := func(session map[string]any, content bool) string {
		p := "/wopi/files/" + session["file_id"].(string)
		if content {
			p += "/contents"
		}
		return p + "?access_token=" + session["access_token"].(string)
	}
	f.call(-1, "GET", "/wopi/files/"+own["file_id"].(string), "", 401)
	f.call(-1, "GET", strings.Replace(endpoint(own, false), own["file_id"].(string), strings.Repeat("f", 64), 1), "", 404)
	info := f.call(-1, "GET", endpoint(reader, false), "", 200)
	if !strings.Contains(info.Body.String(), `"ReadOnly":true`) {
		t.Fatal(info.Body.String())
	}
	f.call(-1, "POST", endpoint(reader, true), "forbidden", 403, "X-WOPI-Override", "PUT")
	f.call(-1, "POST", endpoint(own, true), "without lock", 409, "X-WOPI-Override", "PUT")
	f.call(-1, "POST", endpoint(own, false), "", 200, "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "same-document")
	f.call(-1, "POST", endpoint(editor, false), "", 409, "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "other-lock")
	f.call(0, "PUT", "/api/v1/files/content?path=test.odt", "racing", 423)
	f.call(0, "PUT", "/dav/files/test.odt", "racing", 423)
	f.call(-1, "POST", endpoint(editor, true), "saved through WOPI", 200, "X-WOPI-Override", "PUT", "X-WOPI-Lock", "same-document")
	if f.call(0, "GET", "/api/v1/files/content?path=test.odt", "", 200).Body.String() != "saved through WOPI" {
		t.Fatal("save lost")
	}
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=test.odt", "", 200))) != 1 {
		t.Fatal("no version for office save")
	}
	f.call(-1, "POST", endpoint(own, false), "", 200, "X-WOPI-Override", "REFRESH_LOCK", "X-WOPI-Lock", "same-document")
	f.call(-1, "POST", endpoint(own, false), "", 200, "X-WOPI-Override", "UNLOCK", "X-WOPI-Lock", "same-document")
	var meta struct{ LastModifiedTime string }
	json.Unmarshal(f.call(-1, "GET", endpoint(own, false), "", 200).Body.Bytes(), &meta)
	f.call(-1, "POST", endpoint(own, true), "timestamp save", 200, "X-WOPI-Override", "PUT", "X-COOL-WOPI-Timestamp", meta.LastModifiedTime)
	f.call(-1, "POST", endpoint(own, true), "stale", 409, "X-WOPI-Override", "PUT", "X-COOL-WOPI-Timestamp", meta.LastModifiedTime)
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"test.odt","grantee_id":"`+f.users[2]+`"}`, 200)
	f.call(-1, "GET", endpoint(editor, true), "", 404)
	f.call(0, "POST", "/api/v1/files/move", `{"from":"test.odt","to":"renamed.odt"}`, 200)
	f.call(-1, "GET", endpoint(own, true), "", 401)
}
func TestOfficeProofSignatures(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := rsa.GenerateKey(rand.Reader, 2048)
	b64 := base64.StdEncoding.EncodeToString
	proof := officeProofKeys{Modulus: b64(key.N.Bytes()), Exponent: b64(big.NewInt(int64(key.E)).Bytes()), OldModulus: b64(old.N.Bytes()), OldExponent: b64(big.NewInt(int64(old.E)).Bytes())}
	now := time.Now()
	ticks := now.Unix()*10000000 + 621355968000000000
	address := "https://files.example.com/wopi/files/123?access_token=test"
	r := httptest.NewRequest("GET", address, nil)
	sign := func(k *rsa.PrivateKey, n int64) string {
		hash := sha256.Sum256(officeProofData("test", address, n))
		sig, e := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, hash[:])
		if e != nil {
			t.Fatal(e)
		}
		return b64(sig)
	}
	r.Header.Set("X-WOPI-TimeStamp", strconv.FormatInt(ticks, 10))
	r.Header.Set("X-WOPI-Proof", sign(key, ticks))
	if !proof.valid(r, "test", address, now) {
		t.Fatal("valid proof rejected")
	}
	if proof.valid(r, "other", address, now) || proof.valid(r, "test", address+"&other=1", now) {
		t.Fatal("tampering accepted")
	}
	r.Header.Set("X-WOPI-Proof", sign(old, ticks))
	if !proof.valid(r, "test", address, now) {
		t.Fatal("rotated proof rejected")
	}
	r.Header.Del("X-WOPI-Proof")
	r.Header.Set("X-WOPI-ProofOld", sign(key, ticks))
	if !proof.valid(r, "test", address, now) {
		t.Fatal("previous proof with current key rejected")
	}
	r.Header.Set("X-WOPI-ProofOld", sign(old, ticks))
	if proof.valid(r, "test", address, now) {
		t.Fatal("old key + old proof accepted")
	}
	r.Header.Set("X-WOPI-Proof", sign(key, ticks))
	if proof.valid(r, "test", address, now.Add(21*time.Minute)) {
		t.Fatal("expired proof accepted")
	}
}
