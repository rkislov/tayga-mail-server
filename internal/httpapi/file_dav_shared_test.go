package httpapi

import (
	"context"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestSharedDAVDiscoveryReadWriteAndRevocation(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=team/docs/report.txt", "one", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=team/secret.txt", "secret", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=private.txt", "private", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team/docs","email":"reader@example.com","rights":"read"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team/docs","email":"editor@example.com","rights":"write"}`, 200)
	prefix := "/dav/shared/owner@example.com"
	index := f.call(1, "PROPFIND", "/dav/shared/", "", 207, "Depth", "1").Body.String()
	if !strings.Contains(index, "owner@example.com") || strings.Contains(index, "reader@example.com") {
		t.Fatal(index)
	}
	for _, rel := range []string{"/", "/team/"} {
		xml := f.call(1, "PROPFIND", prefix+rel, "", 207, "Depth", "1").Body.String()
		if strings.Contains(xml, "private.txt") || strings.Contains(xml, "secret.txt") {
			t.Fatal("unshared name leaked", xml)
		}
	}
	if f.call(1, "GET", prefix+"/team/docs/report.txt", "", 200).Body.String() != "one" {
		t.Fatal("wrong shared content")
	}
	f.call(1, "GET", prefix+"/team/secret.txt", "", 403)
	f.call(1, "PUT", prefix+"/team/docs/report.txt", "denied", 403)
	f.call(1, "MKCOL", prefix+"/team/docs/denied/", "", 403)
	f.call(1, "LOCK", prefix+"/team/docs/report.txt", `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`, 403)
	key := f.call(2, "GET", prefix+"/team/docs/report.txt", "", 200).Header().Get("ETag")
	f.call(2, "PUT", prefix+"/team/docs/report.txt", "two", 201, "If-Match", key)
	f.call(2, "PUT", prefix+"/team/docs/report.txt", "stale", 412, "If-Match", key)
	if f.call(0, "GET", "/api/v1/files/content?path=team/docs/report.txt", "", 200).Body.String() != "two" {
		t.Fatal("DAV and web diverged")
	}
	f.call(2, "MKCOL", prefix+"/team/docs/sub/", "", 201)
	f.call(2, "PUT", prefix+"/team/docs/sub/new.txt", "new", 201)
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team/docs","grantee_id":"`+f.users[1]+`"}`, 200)
	f.call(1, "GET", prefix+"/team/docs/report.txt", "", 403)
	xml := f.call(1, "PROPFIND", "/dav/shared/", "", 207, "Depth", "1").Body.String()
	if strings.Contains(xml, "owner@example.com") {
		t.Fatal("revoked owner remains", xml)
	}
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team/docs","email":"editor@example.com","rights":"read"}`, 200)
	f.call(2, "PUT", prefix+"/team/docs/report.txt", "denied", 403)
	f.call(2, "GET", prefix+"/team/docs/report.txt", "", 200)
}

func TestSharedDAVLocksCopyMoveTrashAndIsolation(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=team/a.txt", "one", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=private/secret.txt", "secret", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"editor@example.com","rights":"write"}`, 200)
	prefix := "/dav/shared/owner@example.com/team/"
	lock := f.call(0, "LOCK", "/dav/files/team/a.txt", `<D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`, 200, "Depth", "0").Header().Get("Lock-Token")
	f.call(2, "PUT", prefix+"a.txt", "blocked", 423)
	f.call(2, "PUT", prefix+"a.txt", "two", 201, "If", "("+lock+")")
	f.call(0, "UNLOCK", "/dav/files/team/a.txt", "", 204, "Lock-Token", lock)
	f.call(2, "COPY", prefix+"a.txt", "", 201, "Destination", prefix+"b.txt")
	f.call(2, "MOVE", prefix+"b.txt", "", 201, "Destination", prefix+"c.txt")
	f.call(2, "COPY", prefix+"a.txt", "", 403, "Destination", "/dav/shared/owner@example.com/private/new.txt")
	f.call(2, "MOVE", prefix+"c.txt", "", 403, "Destination", "/dav/shared/owner@example.com/private/secret.txt", "Overwrite", "T")
	f.call(2, "COPY", prefix+"a.txt", "", 403, "Destination", "/dav/shared/editor@example.com/stolen.txt")
	// Existing targets survive rejected overwrites, including their ACL/history.
	f.call(2, "COPY", prefix+"a.txt", "", 403, "Destination", prefix+"c.txt", "Overwrite", "T")
	if f.call(0, "GET", "/dav/files/team/c.txt", "", 200).Body.String() != "two" {
		t.Fatal("overwrite lost destination")
	}
	f.call(2, "DELETE", prefix+"c.txt", "", 204)
	if len(revisions(t, f.call(0, "GET", "/api/v1/files/library/trash", "", 200))) != 1 {
		t.Fatal("shared deletion bypassed owner trash")
	}
	f.call(2, "GET", "/dav/shared/owner@example.com/team/%2e%2e/private/secret.txt", "", 403)
	f.call(2, "PUT", "/dav/shared/owner@example.com/team/%2e%2e/private/new.txt", "denied", 403)
}

func TestSharedDAVRevokedDuringUploadCannotCommit(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=team/a.txt", "original", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"editor@example.com","rights":"write"}`, 200)
	root, err := f.s.filesRoot(f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	fs := &personalDAV{srv: f.s, owner: f.users[0], grantee: f.users[2], root: root, upload: &davUploadState{}}
	file, err := fs.OpenFile(context.Background(), "/team/a.txt", os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("must not commit")); err != nil {
		t.Fatal(err)
	}
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team","grantee_id":"`+f.users[2]+`"}`, 200)
	if err = file.Close(); err == nil {
		t.Fatal("revoked upload committed")
	}
	if f.call(0, "GET", "/dav/files/team/a.txt", "", 200).Body.String() != "original" {
		t.Fatal("revocation lost file")
	}
	// A direct request never bypasses the shared namespace checks.
	request := httptest.NewRequest("PUT", "/dav/shared/owner@example.com/team/a.txt", strings.NewReader("denied"))
	request.Header.Set("Authorization", "Bearer "+f.tokens[2])
	response := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestSharedDAVOwnerQuotaAndConcurrentWrites(t *testing.T) {
	f := newFileFixture(t)
	f.call(0, "PUT", "/api/v1/files/content?path=team/a.txt", "one", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"editor@example.com","rights":"write"}`, 200)
	ctx := context.Background()
	root, err := f.s.filesRoot(f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	fs := &personalDAV{srv: f.s, owner: f.users[0], grantee: f.users[2], root: root, upload: &davUploadState{}}
	a, err := fs.OpenFile(ctx, "team/a.txt", os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.OpenFile(ctx, "team/a.txt", os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		t.Fatal(err)
	}
	a.Write([]byte("two"))
	b.Write([]byte("lost"))
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	if err = b.Close(); err == nil {
		t.Fatal("concurrent write accepted")
	}
	if f.call(0, "GET", "/dav/files/team/a.txt", "", 200).Body.String() != "two" {
		t.Fatal("concurrent upload lost file")
	}
	owner, _ := f.s.store.GetUserByID(ctx, f.users[0])
	sc, err := f.s.store.CreateServiceClass(ctx, &storage.ServiceClass{TenantID: owner.TenantID, Name: "shared-quota", Config: `{"file_quota_bytes":6}`})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserServiceClass(ctx, owner.ID, sc.ID); err != nil {
		t.Fatal(err)
	}
	f.call(2, "PUT", "/dav/shared/owner@example.com/team/over.txt", "x", 507)
	f.call(2, "PUT", "/dav/shared/owner@example.com/team/a.txt", "over", 507)
	if f.call(0, "GET", "/dav/files/team/a.txt", "", 200).Body.String() != "two" {
		t.Fatal("quota failure lost file")
	}
	f.call(2, "PUT", "/dav/files/personal.txt", "editor has independent storage", 201)
}
