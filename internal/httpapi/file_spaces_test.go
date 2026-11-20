package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
)

func newSpaceFixture(t *testing.T) (*fileFixture, *storage.FileSpace, *storage.DirectoryGroup) {
	t.Helper()
	f := newFileFixture(t)
	ctx := context.Background()
	user, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.UpdateUserRoles(ctx, user.ID, storage.RoleTenantAdmin); err != nil {
		t.Fatal(err)
	}
	manager, err := f.s.store.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: user.DomainID, Name: "Managers", LocalPart: "space-managers", MemberIDs: []string{f.users[1]}})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := f.s.store.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: user.DomainID, Name: "Department", LocalPart: "space-writers"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.store.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: user.DomainID, Name: "Project subgroup", LocalPart: "space-children", ParentID: writer.ID, MemberIDs: []string{f.users[2]}})
	if err != nil {
		t.Fatal(err)
	}
	input := storage.FileSpace{TenantID: user.TenantID, Name: "Project Alpha", Kind: "project", QuotaBytes: 10, Groups: []storage.FileSpaceGroup{{GroupID: manager.ID, Rights: "manage"}, {GroupID: writer.ID, Rights: "write"}}}
	body, _ := json.Marshal(input)
	var space storage.FileSpace
	if err = json.Unmarshal(f.call(0, "POST", "/api/v1/files/spaces", string(body), 201).Body.Bytes(), &space); err != nil {
		t.Fatal(err)
	}
	return f, &space, writer
}
func TestFileSpacesIndependentOwnershipQuotaAndHistory(t *testing.T) {
	f, space, _ := newSpaceFixture(t)
	ctx := context.Background()
	suffix := "&owner_id=" + space.ID
	if _, err := f.s.store.GetUserByID(ctx, space.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("space must not be a user")
	}
	f.call(2, "PUT", "/api/v1/files/content?path=report.txt"+suffix, "first", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=report.txt"+suffix, "other", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=extra.txt"+suffix, "x", 507)
	var usage fileStorageUsage
	json.Unmarshal(f.call(2, "GET", "/api/v1/files/library/usage?owner_id="+space.ID, "", 200).Body.Bytes(), &usage)
	if usage.Used != 10 || usage.Limit != 10 || usage.Versions != 5 {
		t.Fatalf("space quota %+v", usage)
	}
	json.Unmarshal(f.call(2, "GET", "/api/v1/files/library/usage", "", 200).Body.Bytes(), &usage)
	if usage.Used != 0 {
		t.Fatal("charged personal quota")
	}
	f.call(2, "GET", "/api/v1/files/library/versions?path=report.txt"+suffix, "", 403)
	versions := revisions(t, f.call(1, "GET", "/api/v1/files/library/versions?path=report.txt"+suffix, "", 200))
	if len(versions) != 1 {
		t.Fatal("space versions missing")
	}
	f.call(1, "PUT", "/api/v1/files/library/retention?owner_id="+space.ID, `{"version_days":1,"trash_days":2,"keep_versions":0,"automatic":true}`, 200)
	saved, err := f.s.store.GetFileSpace(ctx, space.ID)
	if err != nil || saved.VersionDays != 1 || !saved.Automatic {
		t.Fatalf("policy %+v %v", saved, err)
	}
	// Remove the creator entirely; neither the owner nor links/files depend on them.
	if err = f.s.store.DeleteUser(ctx, f.users[0]); err != nil {
		t.Fatal(err)
	}
	if got := f.call(2, "GET", "/api/v1/files/content?path=report.txt"+suffix, "", 200).Body.String(); got != "other" {
		t.Fatal("creator owned data")
	}
	var link struct{ ID, Token string }
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/shares?owner_id="+space.ID, `{"path":"report.txt"}`, 201).Body.Bytes(), &link)
	f.call(-1, "GET", "/s/"+link.Token, "", 200)
	f.call(2, "POST", "/api/v1/files/shares?owner_id="+space.ID, `{"path":"report.txt"}`, 403)
	f.call(1, "DELETE", "/api/v1/files/library/versions?owner_id="+space.ID, `{"id":"`+versions[0].ID+`"}`, 200)
	f.call(2, "PUT", "/api/v1/files/content?path=extra.txt"+suffix, "hello", 200)
	f.call(1, "DELETE", "/api/v1/files?path=extra.txt"+suffix, "", 200)
	trash := revisions(t, f.call(1, "GET", "/api/v1/files/library/trash?owner_id="+space.ID, "", 200))
	if len(trash) != 1 {
		t.Fatal("trash missing")
	}
	f.call(1, "POST", "/api/v1/files/library/restore?owner_id="+space.ID, `{"kind":"trash","id":"`+trash[0].ID+`"}`, 200)
	card := readFileCard(t, f, 1, "path=report.txt&owner_id="+space.ID)
	if !card.CanManage || card.Rights != "manage" {
		t.Fatalf("manager card %+v", card)
	}
	card = readFileCard(t, f, 2, "path=report.txt&owner_id="+space.ID)
	if card.CanManage || len(card.Events) > 0 {
		t.Fatal("member leaked management")
	}
}
func TestFileSpacesGroupRevocationDAVAndTenantIsolation(t *testing.T) {
	f, space, writer := newSpaceFixture(t)
	ctx := context.Background()
	suffix := "&owner_id=" + space.ID
	f.call(2, "PROPFIND", "/dav/spaces/", "", 207, "Depth", "1")
	f.call(2, "PUT", "/dav/spaces/"+space.ID+"/dav.txt", "abc", 201)
	f.call(2, "GET", "/dav/spaces/"+space.ID+"/dav.txt", "", 200)
	// Revoke a nested group's parent grant while a web upload is in flight.
	started, resume := make(chan struct{}), make(chan struct{})
	req := httptest.NewRequest("PUT", "/api/v1/files/content?path=slow.txt"+suffix, &pausedReader{started: started, resume: resume, data: strings.NewReader("small")})
	req.Header.Set("Authorization", "Bearer "+f.tokens[2])
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { f.s.Handler().ServeHTTP(response, req); close(done) }()
	<-started
	fresh, _ := f.s.store.GetFileSpace(ctx, space.ID)
	kept := []storage.FileSpaceGroup{}
	for _, g := range fresh.Groups {
		if g.GroupID != writer.ID {
			kept = append(kept, g)
		}
	}
	fresh.Groups = kept
	body, _ := json.Marshal(fresh)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	close(resume)
	<-done
	if response.Code != 403 {
		t.Fatalf("revoked upload %d %s", response.Code, response.Body.String())
	}
	f.call(2, "GET", "/api/v1/files/content?path=dav.txt"+suffix, "", 403)
	f.call(2, "GET", "/dav/spaces/"+space.ID+"/dav.txt", "", 403)
	fresh, _ = f.s.store.GetFileSpace(ctx, space.ID)
	fresh.Groups = append(fresh.Groups, storage.FileSpaceGroup{GroupID: writer.ID, Rights: "read"})
	body, _ = json.Marshal(fresh)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	f.call(2, "GET", "/dav/spaces/"+space.ID+"/dav.txt", "", 200)
	f.call(2, "PUT", "/dav/spaces/"+space.ID+"/dav.txt", "x", 403)
	f.call(2, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 403)
	// Another tenant's users and groups cannot enter this space by default or ACL.
	tenant, _ := f.s.store.CreateTenant(ctx, "Other tenant")
	domain, _ := f.s.store.CreateDomain(ctx, tenant.ID, "other.example")
	outsider, err := f.s.store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "other@other.example", LocalPart: "other", Enabled: true, Roles: storage.RoleTenantAdmin})
	if err != nil {
		t.Fatal(err)
	}
	otherGroup, err := f.s.store.SaveDirectoryGroup(ctx, &storage.DirectoryGroup{DomainID: domain.ID, Name: "Other", LocalPart: "other-group", MemberIDs: []string{outsider.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if allowed := f.s.fileAccessAllowedContext(ctx, space.ID, outsider.ID, "dav.txt", false); allowed {
		t.Fatal("cross-tenant access")
	}
	fresh, _ = f.s.store.GetFileSpace(ctx, space.ID)
	fresh.Groups = append(fresh.Groups, storage.FileSpaceGroup{GroupID: otherGroup.ID, Rights: "manage"})
	body, _ = json.Marshal(fresh)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 400)
	// CAS prevents a stale manager from overwriting someone else's settings.
	fresh, _ = f.s.store.GetFileSpace(ctx, space.ID)
	body, _ = json.Marshal(fresh)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 409)
}
func TestFileSpacesSearchRequestsAndCleanup(t *testing.T) {
	f, space, _ := newSpaceFixture(t)
	ctx := context.Background()
	fresh, _ := f.s.store.GetFileSpace(ctx, space.ID)
	fresh.QuotaBytes = 0
	body, _ := json.Marshal(fresh)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	suffix := "&owner_id=" + space.ID
	f.call(2, "PUT", "/api/v1/files/content?path=doc.docx"+suffix, searchWord(t, "space confidential document"), 200)
	result := searchDocuments(t, f, 2, "confidential", "")
	if result.Total != 1 || result.Entries[0].Owner != space.ID {
		t.Fatalf("space search %+v", result)
	}
	f.call(1, "POST", "/api/v1/files/mkdir?owner_id="+space.ID, `{"path":"incoming"}`, 200)
	var request struct{ Token string }
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/shares?owner_id="+space.ID, `{"path":"incoming","rights":"upload","ttl_hours":24}`, 201).Body.Bytes(), &request)
	hub := notify.NewHub()
	f.s.SetNotify(hub)
	f.call(-1, "PUT", "/s/"+request.Token+"?name=external.txt", "received", 201)
	if len(hub.Recent(f.users[1], 20)) != 1 || len(hub.Recent(f.users[2], 20)) != 0 {
		t.Fatal("request must notify managers")
	}
	f.s.SetNotify(nil)
	var events struct{ Events []notify.Event }
	json.Unmarshal(f.call(1, "GET", "/api/v1/notifications", "", 200).Body.Bytes(), &events)
	if len(events.Events) != 1 {
		t.Fatal("manager notices not durable")
	}
	f.call(2, "PUT", "/api/v1/files/content?path=old.txt"+suffix, "before", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=old.txt"+suffix, "after", 200)
	versions, _ := f.s.fileRevisions(space.ID, "versions", "old.txt")
	if len(versions) != 1 {
		t.Fatal("missing version")
	}
	versions[0].At = time.Now().Add(-48 * time.Hour)
	f.s.rewriteRevision(space.ID, versions[0])
	f.call(1, "PUT", "/api/v1/files/library/retention?owner_id="+space.ID, `{"automatic":true,"version_days":1}`, 200)
	f.s.cleanFileRetention(ctx)
	versions, _ = f.s.fileRevisions(space.ID, "versions", "old.txt")
	if len(versions) != 0 {
		t.Fatal("space retention did not run")
	}
	root, _ := f.s.filesRoot(space.ID)
	data, err := os.ReadFile(filepath.Join(root, "old.txt"))
	if err != nil || string(data) != "after" {
		t.Fatal("live file removed")
	}
	// New principal persists separately in the DB, not in any employee's profile.
	principal := fmt.Sprint(space.ID)
	if strings.Contains(principal, "@") {
		t.Fatal("space is a mailbox")
	}
	f.call(2, "GET", "/api/v1/files/library/search?mode=content&scope=own&q="+url.QueryEscape("confidential")+"&owner_id="+space.ID, "", 200)
}

func TestFileSpacesOfficeSessionSurvivesCreatorAndRechecksMembership(t *testing.T) {
	f, space, writer := newSpaceFixture(t)
	ctx := context.Background()
	f.s.cfg.Files.OfficeProvider = "onlyoffice"
	f.s.cfg.Files.OfficeURL = "https://office.example.com"
	f.s.cfg.Files.OfficeJWTSecret = documentTestSecret
	f.call(2, "PUT", "/api/v1/files/content?path=doc.docx&owner_id="+space.ID, "initial", 200)
	var session struct {
		URL string `json:"url"`
	}
	json.Unmarshal(f.call(2, "POST", "/api/v1/files/office", `{"path":"doc.docx","owner_id":"`+space.ID+`"}`, 200).Body.Bytes(), &session)
	endpoint, err := url.Parse(session.URL)
	if err != nil {
		t.Fatal(err)
	}
	f.call(-1, "GET", endpoint.RequestURI(), "", 200)
	if err = f.s.store.DeleteUser(ctx, f.users[0]); err != nil {
		t.Fatal(err)
	}
	f.call(-1, "GET", endpoint.RequestURI(), "", 200)
	current, _ := f.s.store.GetFileSpace(ctx, space.ID)
	groups := []storage.FileSpaceGroup{}
	for _, grant := range current.Groups {
		if grant.GroupID != writer.ID {
			groups = append(groups, grant)
		}
	}
	current.Groups = groups
	body, _ := json.Marshal(current)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	f.call(-1, "GET", endpoint.RequestURI(), "", 401)
}
