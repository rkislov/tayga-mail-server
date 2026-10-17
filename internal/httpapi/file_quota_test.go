// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"context"
	"encoding/json"
	"github.com/tayga/tms/internal/storage"
	"testing"
)

func TestFileQuotaRetainsHistoryAndAllWriters(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, e := f.s.store.GetUserByID(ctx, f.users[0])
	if e != nil {
		t.Fatal(e)
	}
	sc, e := f.s.store.CreateServiceClass(ctx, &storage.ServiceClass{TenantID: u.TenantID, Name: "small", Config: `{"file_quota_bytes":10}`})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.store.UpdateUserServiceClass(ctx, u.ID, sc.ID); e != nil {
		t.Fatal(e)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=one.txt", "12345", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=one.txt", "67890", 200)
	var usage fileStorageUsage
	json.Unmarshal(f.call(0, "GET", "/api/v1/files/library/usage", "", 200).Body.Bytes(), &usage)
	if usage.Used != 10 || usage.Live != 5 || usage.Versions != 5 {
		t.Fatal(usage)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=two.txt", "x", 507)
	f.call(0, "DELETE", "/api/v1/files?path=one.txt", "", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=two.txt", "x", 507)
	trash := revisions(t, f.call(0, "GET", "/api/v1/files/library/trash", "", 200))
	f.call(0, "POST", "/api/v1/files/library/restore", `{"kind":"trash","id":"`+trash[0].ID+`"}`, 200)
	versions := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions", "", 200))
	f.call(0, "DELETE", "/api/v1/files/library/versions", `{"id":"`+versions[0].ID+`"}`, 200)
	f.call(0, "PUT", "/dav/files/dav.txt", "12345", 201)
	f.call(0, "PUT", "/dav/files/blocked.txt", "x", 507)
	f.call(0, "POST", "/api/v1/files/uploads", `{"path":"blocked.txt","size":1}`, 507)
	json.Unmarshal(f.call(0, "GET", "/api/v1/files/library/usage", "", 200).Body.Bytes(), &usage)
	if usage.Used != 10 || usage.Trash != 0 || usage.Versions != 0 {
		t.Fatal(usage)
	}
}

func TestFileQuotaRechecksCompetingUploadsAndSharedOwner(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	u, _ := f.s.store.GetUserByID(ctx, f.users[0])
	sc, err := f.s.store.CreateServiceClass(ctx, &storage.ServiceClass{TenantID: u.TenantID, Name: "tiny", Config: `{"file_quota_bytes":5}`})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserServiceClass(ctx, u.ID, sc.ID); err != nil {
		t.Fatal(err)
	}
	a := beginUpload(t, f, "shared/a", 4)
	b := beginUpload(t, f, "shared/b", 4)
	for _, id := range []string{a.ID, b.ID} {
		f.call(0, "PATCH", "/api/v1/files/uploads/"+id, "1234", 200, "Upload-Offset", "0")
	}
	f.call(0, "POST", "/api/v1/files/uploads/"+a.ID+"/complete", "", 200)
	f.call(0, "POST", "/api/v1/files/uploads/"+b.ID+"/complete", "", 507)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"shared","email":"editor@example.com","rights":"write"}`, 200)
	f.call(2, "PUT", "/api/v1/files/content?path=shared/c&owner_id="+u.ID, "12", 507)
	f.call(2, "PUT", "/api/v1/files/content?path=shared/c&owner_id="+u.ID, "1", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=personal", "unlimited editor storage", 200)
}
