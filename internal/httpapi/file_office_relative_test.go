package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"

	"github.com/tayga/tms/internal/storage"
)

func wopiSessionURL(session map[string]any) string {
	return "/wopi/files/" + session["file_id"].(string) + "?access_token=" + session["access_token"].(string)
}
func relativeResult(t *testing.T, w *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var result struct{ Name, Url string }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(result.Url)
	if err != nil || address.Scheme != "https" || address.Query().Get("access_token") == "" {
		t.Fatal(result, err)
	}
	return result.Name, address.RequestURI()
}
func wopiUTF7(name string) string {
	units := utf16.Encode([]rune(name))
	data := make([]byte, len(units)*2)
	for i, value := range units {
		binary.BigEndian.PutUint16(data[i*2:], value)
	}
	return "+" + base64.RawStdEncoding.EncodeToString(data) + "-"
}
func TestWOPIPutRelativeExactSuggestedAndTokenScope(t *testing.T) {
	f := newFileFixture(t)
	mockOffice(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=docs/source.odt", "source", 200)
	session := officeSessionFor(t, f, 0, "docs/source.odt")
	endpoint := wopiSessionURL(session)
	info := f.call(-1, "GET", endpoint, "", 200)
	if !strings.Contains(info.Body.String(), `"UserCanNotWriteRelative":false`) {
		t.Fatal(info.Body.String())
	}
	f.call(-1, "POST", endpoint, "", 200, "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "source-lock")
	result := f.call(-1, "POST", endpoint, "new contents", 200, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", wopiUTF7("Копия.odt"), "X-WOPI-Size", "12")
	name, address := relativeResult(t, result)
	if name != "Копия.odt" {
		t.Fatal(name)
	}
	if f.call(-1, "GET", strings.Replace(address, "?", "/contents?", 1), "", 200).Body.String() != "new contents" {
		t.Fatal("wrong copy")
	}
	if f.call(0, "GET", "/dav/files/docs/source.odt", "", 200).Body.String() != "source" {
		t.Fatal("source changed")
	}
	target, _ := url.Parse(address)
	source, _ := url.Parse(endpoint)
	f.call(-1, "GET", target.Path+"?access_token="+source.Query().Get("access_token"), "", 404)
	f.call(-1, "GET", source.Path+"?access_token="+target.Query().Get("access_token"), "", 404)
	f.call(-1, "POST", endpoint, "collision", 409, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", wopiUTF7("Копия.odt"))
	for _, want := range []string{"source (1).odt", "source (2).odt"} {
		result = f.call(-1, "POST", endpoint, "copy", 200, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-SuggestedTarget", ".odt", "X-WOPI-OverwriteRelativeTarget", "true")
		name, _ = relativeResult(t, result)
		if name != want {
			t.Fatal(want, name)
		}
	}
	result = f.call(-1, "POST", endpoint, "sanitized", 200, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-SuggestedTarget", "../invalid?.odt")
	name, _ = relativeResult(t, result)
	if strings.ContainsAny(name, "/\\?") {
		t.Fatal(name)
	}
}
func TestWOPIPutRelativeOverwriteLocksAndVersions(t *testing.T) {
	f := newFileFixture(t)
	mockOffice(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=source.odt", "source", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=target.odt", "old", 200)
	session := officeSessionFor(t, f, 0, "source.odt")
	targetSession := officeSessionFor(t, f, 0, "target.odt")
	endpoint := wopiSessionURL(session)
	f.call(-1, "POST", wopiSessionURL(targetSession), "", 200, "X-WOPI-Override", "LOCK", "X-WOPI-Lock", "target-lock")
	conflict := f.call(-1, "POST", endpoint, "new", 409, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "target.odt", "X-WOPI-OverwriteRelativeTarget", "true")
	if conflict.Header().Get("X-WOPI-Lock") != "target-lock" {
		t.Fatal(conflict.Header())
	}
	f.call(-1, "POST", wopiSessionURL(targetSession), "", 200, "X-WOPI-Override", "UNLOCK", "X-WOPI-Lock", "target-lock")
	lock := f.call(0, "LOCK", "/dav/files/target.odt", `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`, 200, "Depth", "0").Header().Get("Lock-Token")
	f.call(-1, "POST", endpoint, "new", 409, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "target.odt", "X-WOPI-OverwriteRelativeTarget", "true")
	f.call(0, "UNLOCK", "/dav/files/target.odt", "", 204, "Lock-Token", lock)
	f.call(-1, "POST", endpoint, "new", 200, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "target.odt", "X-WOPI-OverwriteRelativeTarget", "true")
	if f.call(0, "GET", "/dav/files/target.odt", "", 200).Body.String() != "new" {
		t.Fatal("overwrite failed")
	}
	history := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=target.odt", "", 200))
	if len(history) != 1 {
		t.Fatal(history)
	}
	f.call(-1, "POST", endpoint, "destroy source", 409, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "source.odt", "X-WOPI-OverwriteRelativeTarget", "true")
}
func TestWOPIPutRelativePermissionsAndValidation(t *testing.T) {
	f := newFileFixture(t)
	mockOffice(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=team/source.odt", "source", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team/source.odt","email":"editor@example.com","rights":"write"}`, 200)
	session := officeSessionFor(t, f, 2, "team/source.odt")
	endpoint := wopiSessionURL(session)
	if info := f.call(-1, "GET", endpoint, "", 200); !strings.Contains(info.Body.String(), `"UserCanNotWriteRelative":true`) {
		t.Fatal(info.Body.String())
	}
	f.call(-1, "POST", endpoint, "denied", 501, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "copy.odt")
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"editor@example.com","rights":"write"}`, 200)
	f.call(-1, "POST", endpoint, "shared copy", 200, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "copy.odt")
	for _, headers := range [][]string{{}, {"X-WOPI-RelativeTarget", "a.odt", "X-WOPI-SuggestedTarget", "b.odt"}, {"X-WOPI-RelativeTarget", "../outside.odt"}, {"X-WOPI-RelativeTarget", "a/b.odt"}, {"X-WOPI-RelativeTarget", "bad+name.odt"}, {"X-WOPI-RelativeTarget", " spaced.odt"}, {"X-WOPI-RelativeTarget", "copy2.odt", "X-WOPI-Size", "99"}} {
		all := append([]string{"X-WOPI-Override", "PUT_RELATIVE"}, headers...)
		f.call(-1, "POST", endpoint, "invalid", 400, all...)
	}
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team","grantee_id":"`+f.users[2]+`"}`, 200)
	f.call(-1, "POST", endpoint, "revoked", 501, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "revoked.odt")
}
func TestDecodeWOPITarget(t *testing.T) {
	for _, name := range []string{"Копия.odt", "😀.odt", "文件.docx"} {
		got, err := decodeWOPITarget(wopiUTF7(name))
		if err != nil || got != name {
			t.Fatal(got, err)
		}
	}
	for input, want := range map[string]string{"a+-b.odt": "a+b.odt"} {
		got, err := decodeWOPITarget(input)
		if err != nil || got != want {
			t.Fatal(got, err)
		}
	}
	implicit := strings.TrimSuffix(wopiUTF7("Копия"), "-") + ".odt"
	if got, err := decodeWOPITarget(implicit); err != nil || got != "Копия.odt" {
		t.Fatal(got, err)
	}
}

// Revoke midway through streaming the body, after the initial permission check.
type wopiRelativeBody struct {
	reader *strings.Reader
	first  func()
}

func (b *wopiRelativeBody) Read(p []byte) (int, error) {
	if b.first != nil {
		fn := b.first
		b.first = nil
		fn()
	}
	return b.reader.Read(p)
}
func (b *wopiRelativeBody) Close() error { return nil }

func TestWOPIPutRelativeRevocationDuringUpload(t *testing.T) {
	f := newFileFixture(t)
	mockOffice(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=team/source.odt", "source", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"editor@example.com","rights":"write"}`, 200)
	session := officeSessionFor(t, f, 2, "team/source.odt")
	r := httptest.NewRequest("POST", wopiSessionURL(session), nil)
	r.Header.Set("X-WOPI-Override", "PUT_RELATIVE")
	r.Header.Set("X-WOPI-RelativeTarget", "revoked.odt")
	r.Body = &wopiRelativeBody{reader: strings.NewReader("new"), first: func() {
		f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team","grantee_id":"`+f.users[2]+`"}`, 200)
	}}
	r.ContentLength = 3
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, r)
	if w.Code != 501 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.call(0, "GET", "/dav/files/team/revoked.odt", "", 404)
	if f.call(0, "GET", "/dav/files/team/source.odt", "", 200).Body.String() != "source" {
		t.Fatal("source lost")
	}
}
func TestWOPIPutRelativeConcurrentSuggestedAndQuota(t *testing.T) {
	f := newFileFixture(t)
	mockOffice(t, f)
	f.call(0, "PUT", "/api/v1/files/content?path=source.odt", "src", 200)
	session := officeSessionFor(t, f, 0, "source.odt")
	endpoint := wopiSessionURL(session)
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", endpoint, strings.NewReader("copy"))
			r.Header.Set("X-WOPI-Override", "PUT_RELATIVE")
			r.Header.Set("X-WOPI-SuggestedTarget", "copy.odt")
			w := httptest.NewRecorder()
			f.s.Handler().ServeHTTP(w, r)
			responses <- w
		}()
	}
	wg.Wait()
	close(responses)
	names := map[string]bool{}
	for w := range responses {
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		name, _ := relativeResult(t, w)
		names[name] = true
	}
	if len(names) != 2 || !names["copy.odt"] || !names["copy (1).odt"] {
		t.Fatal(names)
	}
	ctx := context.Background()
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	sc, err := f.s.store.CreateServiceClass(ctx, &storage.ServiceClass{TenantID: owner.TenantID, Name: "relative-quota", Config: `{"file_quota_bytes":11}`})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserServiceClass(ctx, owner.ID, sc.ID); err != nil {
		t.Fatal(err)
	}
	f.call(-1, "POST", endpoint, "over", 507, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "over.odt")
	f.call(-1, "POST", endpoint, "over", 507, "X-WOPI-Override", "PUT_RELATIVE", "X-WOPI-RelativeTarget", "copy.odt", "X-WOPI-OverwriteRelativeTarget", "true")
	if f.call(0, "GET", "/dav/files/copy.odt", "", 200).Body.String() != "copy" {
		t.Fatal("quota failure lost target")
	}
	f.call(0, "GET", "/dav/files/over.odt", "", 404)
}
