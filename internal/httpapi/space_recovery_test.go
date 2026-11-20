package httpapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func recoveryFixture(t *testing.T) (*fileFixture, *storage.FileSpace) {
	t.Helper()
	f, space, _ := newSpaceFixture(t)
	space.QuotaBytes = 0
	body, _ := json.Marshal(space)
	if err := json.Unmarshal(f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200).Body.Bytes(), space); err != nil {
		t.Fatal(err)
	}
	return f, space
}
func recoveryPoint(t *testing.T, f *fileFixture, space, label string) spaceCheckpointInfo {
	t.Helper()
	var point spaceCheckpointInfo
	body, _ := json.Marshal(map[string]string{"label": label})
	if err := json.Unmarshal(f.call(1, "POST", "/api/v1/files/spaces/"+space+"/recovery/checkpoints", string(body), 201).Body.Bytes(), &point); err != nil {
		t.Fatal(err)
	}
	return point
}
func recoveryPreview(t *testing.T, f *fileFixture, space, point, mode, path string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"checkpoint_id": point, "mode": mode, "path": path})
	var result struct {
		Plan string `json:"plan_id"`
	}
	if err := json.Unmarshal(f.call(1, "POST", "/api/v1/files/spaces/"+space+"/recovery/preview", string(body), 200).Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Plan
}
func TestSpaceRecoveryFullMetadataAndSafety(t *testing.T) {
	f, space := recoveryFixture(t)
	suffix := "&owner_id=" + space.ID
	ctx := context.Background()
	f.call(2, "PUT", "/api/v1/files/content?path=docs/report.txt"+suffix, "first", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=docs/report.txt"+suffix, "second", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=removed.txt"+suffix, "trash", 200)
	f.call(2, "DELETE", "/api/v1/files?path=removed.txt"+suffix, "", 200)
	versions := revisions(t, f.call(1, "GET", "/api/v1/files/library/versions?path=docs/report.txt"+suffix, "", 200))
	f.call(1, "PATCH", "/api/v1/files/library/protection?owner_id="+space.ID, `{"id":"`+versions[0].ID+`","protected":true}`, 200)
	var link struct{ ID, Token string }
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/shares?owner_id="+space.ID, `{"path":"docs/report.txt"}`, 201).Body.Bytes(), &link)
	point := recoveryPoint(t, f, space.ID, "Known good")
	f.call(2, "PUT", "/api/v1/files/content?path=docs/report.txt"+suffix, "bad change", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=extra.txt"+suffix, "new file", 200)
	f.call(1, "DELETE", "/api/v1/files/shares/"+link.ID+"?owner_id="+space.ID, "", 200)
	current, _ := f.s.store.GetFileSpace(ctx, space.ID)
	current.Name = "Changed name"
	current.TrashDays = 17
	body, _ := json.Marshal(current)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	plan := recoveryPreview(t, f, space.ID, point.ID, "full", "")
	var restored struct {
		Safety string `json:"safety_checkpoint_id"`
	}
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+plan+`"}`, 200).Body.Bytes(), &restored)
	if restored.Safety == "" {
		t.Fatal("missing safety checkpoint")
	}
	if got := f.call(2, "GET", "/api/v1/files/content?path=docs/report.txt"+suffix, "", 200).Body.String(); got != "second" {
		t.Fatal("wrong restored content")
	}
	f.call(2, "GET", "/api/v1/files/content?path=extra.txt"+suffix, "", 404)
	f.call(-1, "GET", "/s/"+link.Token, "", 200)
	current, _ = f.s.store.GetFileSpace(ctx, space.ID)
	if current.Name != space.Name || current.TrashDays != space.TrashDays || current.Generation != 1 {
		t.Fatalf("metadata %+v", current)
	}
	versions = revisions(t, f.call(1, "GET", "/api/v1/files/library/versions?path=docs/report.txt"+suffix, "", 200))
	if len(versions) != 1 || !versions[0].Protected {
		t.Fatal("protected versions lost")
	}
	trash := revisions(t, f.call(1, "GET", "/api/v1/files/library/trash?owner_id="+space.ID, "", 200))
	if len(trash) != 1 {
		t.Fatal("trash lost")
	}
	// Undo restores the exact pre-restore state, while the generation keeps growing.
	undo := recoveryPreview(t, f, space.ID, restored.Safety, "full", "")
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+undo+`"}`, 200)
	if got := f.call(2, "GET", "/api/v1/files/content?path=extra.txt"+suffix, "", 200).Body.String(); got != "new file" {
		t.Fatal("safety undo failed")
	}
	current, _ = f.s.store.GetFileSpace(ctx, space.ID)
	if current.Generation != 2 {
		t.Fatal("generation was rewound")
	}
}
func TestSpaceRecoveryFolderQuotaAndStalePlan(t *testing.T) {
	f, space := recoveryFixture(t)
	suffix := "&owner_id=" + space.ID
	f.call(2, "PUT", "/api/v1/files/content?path=folder/a.txt"+suffix, "old", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=outside.txt"+suffix, "outside", 200)
	point := recoveryPoint(t, f, space.ID, "Folder baseline")
	f.call(2, "PUT", "/api/v1/files/content?path=folder/a.txt"+suffix, "new", 200)
	f.call(2, "PUT", "/api/v1/files/content?path=folder/b.txt"+suffix, "extra", 200)
	var link struct{ ID, Token string }
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/shares?owner_id="+space.ID, `{"path":"folder"}`, 201).Body.Bytes(), &link)
	stale := recoveryPreview(t, f, space.ID, point.ID, "folder", "folder")
	f.call(2, "PUT", "/api/v1/files/content?path=outside.txt"+suffix, "changed outside", 200)
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+stale+`"}`, 409)
	plan := recoveryPreview(t, f, space.ID, point.ID, "folder", "folder")
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+plan+`"}`, 200)
	if got := f.call(2, "GET", "/api/v1/files/content?path=folder/a.txt"+suffix, "", 200).Body.String(); got != "old" {
		t.Fatal("folder not restored")
	}
	f.call(2, "GET", "/api/v1/files/content?path=folder/b.txt"+suffix, "", 404)
	if got := f.call(2, "GET", "/api/v1/files/content?path=outside.txt"+suffix, "", 200).Body.String(); got != "changed outside" {
		t.Fatal("outside path changed")
	}
	f.call(-1, "GET", "/s/"+link.Token, "", 404)
	trash := revisions(t, f.call(1, "GET", "/api/v1/files/library/trash?owner_id="+space.ID, "", 200))
	if len(trash) != 1 || trash[0].Path != "folder" {
		t.Fatal("pre-rollback folder not preserved")
	}
	if trash[0].ActivityIDs["folder/b.txt"] == "" {
		t.Fatal("replaced folder lost activity identities in trash")
	}
	f.s.fileMu.Lock()
	activity, err := f.s.fileActivity(space.ID)
	if err != nil || activity.Paths["folder/b.txt"] != "" || activity.Paths["folder/a.txt"] == "" {
		f.s.fileMu.Unlock()
		t.Fatal("folder rollback retained deleted-path activity bindings")
	}
	f.s.fileMu.Unlock()
	f.call(2, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/preview", `{"checkpoint_id":"`+point.ID+`","mode":"full"}`, 403)
}
func TestSpaceRecoveryArchiveAndImport(t *testing.T) {
	f, space := recoveryFixture(t)
	suffix := "&owner_id=" + space.ID
	f.call(2, "PUT", "/api/v1/files/content?path=archive.txt"+suffix, "backed up", 200)
	point := recoveryPoint(t, f, space.ID, "Export")
	var ticket struct {
		URL string `json:"url"`
	}
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/download", `{"checkpoint_id":"`+point.ID+`"}`, 200).Body.Bytes(), &ticket)
	archive := f.call(-1, "GET", ticket.URL, "", 200).Body.Bytes()
	f.call(-1, "GET", ticket.URL, "", 403)
	f.call(2, "PUT", "/api/v1/files/content?path=archive.txt"+suffix, "changed", 200)
	imported := f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/import", string(archive), 201)
	var response struct {
		Preview struct {
			Plan string `json:"plan_id"`
		} `json:"preview"`
	}
	json.Unmarshal(imported.Body.Bytes(), &response)
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+response.Preview.Plan+`"}`, 200)
	if got := f.call(2, "GET", "/api/v1/files/content?path=archive.txt"+suffix, "", 200).Body.String(); got != "backed up" {
		t.Fatal("archive import")
	}
	bad := append([]byte{}, archive...)
	bad[len(bad)-5] ^= 0xff
	request := httptest.NewRequest("POST", "/api/v1/files/spaces/"+space.ID+"/recovery/import", bytes.NewReader(bad))
	request.Header.Set("Authorization", "Bearer "+f.tokens[1])
	result := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(result, request)
	if result.Code < 400 {
		t.Fatal("corrupt archive accepted")
	}
}
func TestSpaceRecoveryInvalidatesPendingUploads(t *testing.T) {
	f, space := recoveryFixture(t)
	suffix := "&owner_id=" + space.ID
	f.call(2, "PUT", "/api/v1/files/content?path=a.txt"+suffix, "old", 200)
	point := recoveryPoint(t, f, space.ID, "Before uploads")
	var upload resumableUpload
	json.Unmarshal(f.call(2, "POST", "/api/v1/files/uploads", `{"path":"new.txt","owner_id":"`+space.ID+`","size":3}`, 201).Body.Bytes(), &upload)
	f.call(2, "PATCH", "/api/v1/files/uploads/"+upload.ID, "new", 200, "Upload-Offset", "0")
	started, resume := make(chan struct{}), make(chan struct{})
	req := httptest.NewRequest("PUT", "/api/v1/files/content?path=raw.txt"+suffix, &pausedReader{started: started, resume: resume, data: strings.NewReader("raw")})
	req.Header.Set("Authorization", "Bearer "+f.tokens[2])
	result := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { f.s.Handler().ServeHTTP(result, req); close(done) }()
	<-started
	plan := recoveryPreview(t, f, space.ID, point.ID, "full", "")
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+plan+`"}`, 200)
	close(resume)
	<-done
	if result.Code != 409 {
		t.Fatalf("old raw upload %d %s", result.Code, result.Body.String())
	}
	f.call(2, "POST", "/api/v1/files/uploads/"+upload.ID+"/complete", "", 409)
	root, _ := f.s.filesRoot(space.ID)
	if _, err := os.Stat(filepath.Join(root, "raw.txt")); !os.IsNotExist(err) {
		t.Fatal("stale upload published")
	}
}
func TestSpaceRecoveryRedoAfterPartialSwap(t *testing.T) {
	f, space := recoveryFixture(t)
	ctx := context.Background()
	suffix := "&owner_id=" + space.ID
	f.call(2, "PUT", "/api/v1/files/content?path=file.txt"+suffix, "original", 200)
	point := recoveryPoint(t, f, space.ID, "Crash target")
	f.call(2, "PUT", "/api/v1/files/content?path=file.txt"+suffix, "changed", 200)
	target, _ := f.s.readSpaceCheckpoint(space.ID, point.ID)
	op := storage.NewID()
	root := filepath.Join(f.s.ms.Root, "space-recovery", "operations", op)
	os.MkdirAll(root, 0700)
	source := filepath.Join(f.s.checkpointRoot(space.ID), point.ID)
	if err := cloneRecoveryTree(ctx, filepath.Join(source, "files"), filepath.Join(root, "new-files"), false); err != nil {
		t.Fatal(err)
	}
	if err := cloneRecoveryTree(ctx, filepath.Join(source, "state"), filepath.Join(root, "new-state"), true); err != nil {
		t.Fatal(err)
	}
	intent := &spaceRestoreIntent{ID: op, Space: space.ID, Actor: f.users[1], Mode: "full", Status: "pending", At: time.Now().UTC(), HadFiles: true, HadState: true, State: target.State}
	writeFileStateJSON(filepath.Join(root, "intent.json"), intent)
	// Crash between the two renames of the live tree.
	if err := os.Rename(filepath.Join(f.s.ms.Root, "files", space.ID), filepath.Join(root, "old-files")); err != nil {
		t.Fatal(err)
	}
	if err := f.s.recoverPendingSpaceRestores(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.call(2, "GET", "/api/v1/files/content?path=file.txt"+suffix, "", 200).Body.String(); got != "original" {
		t.Fatal("redo failed")
	}
	generation, _ := f.s.store.GetFileSpace(ctx, space.ID)
	if generation.Generation != 1 {
		t.Fatal("redo generation")
	}
	if err := f.s.recoverPendingSpaceRestores(ctx); err != nil {
		t.Fatal(err)
	}
	again, _ := f.s.store.GetFileSpace(ctx, space.ID)
	if again.Generation != 1 {
		t.Fatal("redo applied twice")
	}
}

func TestSpaceRecoveryMissingSpaceAndTenantBinding(t *testing.T) {
	f, space := recoveryFixture(t)
	ctx := context.Background()
	f.call(2, "PUT", "/api/v1/files/content?path=data.txt&owner_id="+space.ID, "recovery data", 200)
	point := recoveryPoint(t, f, space.ID, "Portable")
	snapshot, err := f.s.readSpaceCheckpoint(space.ID, point.ID)
	if err != nil {
		t.Fatal(err)
	}
	var original bytes.Buffer
	if err = writeSpaceArchive(ctx, filepath.Join(f.s.checkpointRoot(space.ID), point.ID), snapshot, &original); err != nil {
		t.Fatal(err)
	}
	tenant, _ := f.s.store.CreateTenant(ctx, "Other recovery tenant")
	domain, _ := f.s.store.CreateDomain(ctx, tenant.ID, "recovery-other.example")
	source, _ := f.s.store.GetUserByID(ctx, f.users[0])
	user, err := f.s.store.CreateUser(ctx, &storage.User{TenantID: tenant.ID, DomainID: domain.ID, Email: "admin@recovery-other.example", LocalPart: "admin", PasswordHash: source.PasswordHash, Enabled: true, Roles: storage.RoleTenantAdmin, AuthSource: "local"})
	if err != nil {
		t.Fatal(err)
	}
	login, err := f.s.authn.LoginWithPassword(ctx, user.Email, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	f.users = append(f.users, user.ID)
	f.tokens = append(f.tokens, login.Tokens.AccessToken)
	forgedState, _ := storage.CloneFileSpaceSnapshot(snapshot.State)
	forgedState.Space.TenantID = tenant.ID
	forgedState.Space.Groups = []storage.FileSpaceGroup{}
	forgedState.Groups = []storage.FileSpaceGroupRef{}
	forgedState.Access = []storage.FileAccess{}
	forgedState.Users = []storage.FileSpaceUserRef{}
	forgedState.Shares = []*storage.FileShare{}
	forged := *snapshot
	forged.State = forgedState
	var changed bytes.Buffer
	if err = writeSpaceArchive(ctx, filepath.Join(f.s.checkpointRoot(space.ID), point.ID), &forged, &changed); err != nil {
		t.Fatal(err)
	}
	// Losing the DB row cannot make old checkpoints adoptable by another tenant.
	if _, err = f.s.store.(*storage.Store).DB().Exec("DELETE FROM file_spaces WHERE id=?", space.ID); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(f.s.ms.Root, "files", space.ID))
	os.RemoveAll(filepath.Join(f.s.ms.Root, "file-state", space.ID))
	f.call(3, "POST", "/api/v1/files/space-recovery/import", changed.String(), 403)
	var response struct {
		Preview struct {
			Plan string `json:"plan_id"`
		} `json:"preview"`
	}
	json.Unmarshal(f.call(0, "POST", "/api/v1/files/space-recovery/import", original.String(), 201).Body.Bytes(), &response)
	f.call(0, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+response.Preview.Plan+`"}`, 200)
	if got := f.call(2, "GET", "/api/v1/files/content?path=data.txt&owner_id="+space.ID, "", 200).Body.String(); got != "recovery data" {
		t.Fatal("missing-space restoration failed")
	}
}

func TestSpaceRecoveryReadGateAndQuota(t *testing.T) {
	f, space := recoveryFixture(t)
	suffix := "&owner_id=" + space.ID
	f.call(2, "PUT", "/api/v1/files/content?path=folder/a.txt"+suffix, "old content", 200)
	point := recoveryPoint(t, f, space.ID, "Before change")
	f.call(2, "PUT", "/api/v1/files/content?path=folder/a.txt"+suffix, "new", 200)
	plan := recoveryPreview(t, f, space.ID, point.ID, "full", "")
	gate := f.s.spaceReadLock(space.ID)
	gate.RLock()
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+plan+`"}`, 423)
	gate.RUnlock()
	current, _ := f.s.store.GetFileSpace(context.Background(), space.ID)
	current.QuotaBytes = 1
	body, _ := json.Marshal(current)
	f.call(1, "PUT", "/api/v1/files/spaces/"+space.ID, string(body), 200)
	var preview struct {
		Plan    string `json:"plan_id"`
		Blocked bool   `json:"quota_blocked"`
	}
	body, _ = json.Marshal(map[string]string{"checkpoint_id": point.ID, "mode": "folder", "path": "folder"})
	json.Unmarshal(f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/preview", string(body), 200).Body.Bytes(), &preview)
	if !preview.Blocked {
		t.Fatal("quota preview allowed oversized restore")
	}
	f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/apply", `{"plan_id":"`+preview.Plan+`"}`, 507)
	if got := f.call(2, "GET", "/api/v1/files/content?path=folder/a.txt"+suffix, "", 200).Body.String(); got != "new" {
		t.Fatal("blocked restore changed content")
	}
}

func TestSpaceRecoveryRejectsUnsafeArchives(t *testing.T) {
	f, space := recoveryFixture(t)
	for _, tc := range []struct {
		name, path string
		kind       byte
		duplicate  bool
	}{
		{"traversal", "../escaped", tar.TypeReg, false},
		{"absolute", "/escaped", tar.TypeReg, false},
		{"symlink", "files/link", tar.TypeSymlink, false},
		{"hardlink", "files/link", tar.TypeLink, false},
		{"duplicate", "files/a", tar.TypeReg, true},
		{"runtime", "state/uploads/token", tar.TypeReg, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var archive bytes.Buffer
			gz := gzip.NewWriter(&archive)
			tw := tar.NewWriter(gz)
			count := 1
			if tc.duplicate {
				count = 2
			}
			for i := 0; i < count; i++ {
				if err := tw.WriteHeader(&tar.Header{Name: tc.path, Typeflag: tc.kind, Linkname: "/etc/passwd", Mode: 0600}); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			f.call(1, "POST", "/api/v1/files/spaces/"+space.ID+"/recovery/import", archive.String(), 400)
		})
	}
}

func TestSpaceRecoveryAutomaticLimitKeepsManual(t *testing.T) {
	f, space := recoveryFixture(t)
	manual := recoveryPoint(t, f, space.ID, "Protected manual point")
	f.s.fileMu.Lock()
	defer f.s.fileMu.Unlock()
	for i := 0; i < 52; i++ {
		if _, err := f.s.captureSpaceCheckpoint(context.Background(), space.ID, "Automatic", true); err != nil {
			t.Fatal(err)
		}
	}
	points, err := f.s.listSpaceCheckpoints(space.ID)
	if err != nil {
		t.Fatal(err)
	}
	automatic := 0
	found := false
	for _, point := range points {
		if point.Automatic {
			automatic++
		}
		if point.ID == manual.ID {
			found = true
		}
	}
	if automatic != 50 || !found {
		t.Fatalf("automatic=%d manual kept=%v", automatic, found)
	}
}
