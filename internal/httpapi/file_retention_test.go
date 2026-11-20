package httpapi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileRetentionPolicyCleanupAndProtection(t *testing.T) {
	f := newFileFixture(t)
	owner := f.users[0]
	f.call(0, "PUT", "/api/v1/files/content?path=report.txt", "first", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=report.txt", "second", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=report.txt", "third", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=report.txt", "current", 200)
	versions, err := f.s.fileRevisions(owner, "versions", "")
	if err != nil || len(versions) != 3 {
		t.Fatalf("versions: %v %v", versions, err)
	}
	for i := range versions {
		versions[i].At = time.Now().Add(-time.Duration(10+i) * 24 * time.Hour)
		if err = f.s.rewriteRevision(owner, versions[i]); err != nil {
			t.Fatal(err)
		}
	}
	f.call(0, "PATCH", "/api/v1/files/library/protection", `{"id":"`+versions[1].ID+`","protected":true}`, 200)
	f.call(0, "DELETE", "/api/v1/files/library/versions", `{"id":"`+versions[1].ID+`"}`, 423)
	f.call(1, "POST", "/api/v1/files/library/cleanup/run?owner_id="+owner, "", 403)
	f.call(-1, "GET", "/api/v1/files/library/retention", "", 401)
	f.call(0, "PUT", "/api/v1/files/library/retention", `{"version_days":-1}`, 400)
	f.call(0, "PUT", "/api/v1/files/library/retention", `{"version_days":1,"trash_days":1,"keep_versions":1}`, 200)
	// Trash a whole directory: estimate content, never the directory inode size.
	f.call(0, "PUT", "/api/v1/files/content?path=folder/a.txt", "trash-data", 200)
	f.s.fileMu.Lock()
	trash, err := f.s.trashFile(owner, "folder")
	f.s.fileMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	_, dir, err := f.s.revision(owner, "trash", trash.ID)
	if err != nil {
		t.Fatal(err)
	}
	trash.At = time.Now().Add(-48 * time.Hour)
	if err = writeFileStateJSON(filepath.Join(dir, "metadata.json"), trash); err != nil {
		t.Fatal(err)
	}
	uploads, _ := f.s.fileStateRoot(owner, "uploads")
	if err = os.WriteFile(filepath.Join(uploads, "active"), []byte("pending"), 0600); err != nil {
		t.Fatal(err)
	}
	// A resumable session remains usable after cleanup.
	upload := beginUpload(t, f, "pending.txt", 6)
	route := "/api/v1/files/uploads/" + upload.ID
	f.call(0, "PATCH", route, "abc", 200, "Upload-Offset", "0")
	usageBefore, err := f.s.fileUsage(owner)
	if err != nil {
		t.Fatal(err)
	}
	var plan fileCleanupPlan
	json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/cleanup/preview", "", 200).Body.Bytes(), &plan)
	if len(plan.Entries) != 2 || plan.Bytes != int64(len("third")+len("trash-data")) {
		t.Fatalf("plan: %+v", plan)
	}
	// Preview leaves both retained records intact.
	if _, err = os.Stat(filepath.Join(dir, "content", "a.txt")); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/files/library/cleanup/run", "", 200)
	if _, err = os.Stat(filepath.Join(dir, "content")); !os.IsNotExist(err) {
		t.Fatalf("trash remains: %v", err)
	}
	usageAfter, err := f.s.fileUsage(owner)
	if err != nil {
		t.Fatal(err)
	}
	if usageBefore.Used-usageAfter.Used != plan.Bytes {
		t.Fatalf("quota relief %d, preview %d", usageBefore.Used-usageAfter.Used, plan.Bytes)
	}
	f.call(0, "PATCH", route, "def", 200, "Upload-Offset", "3")
	f.call(0, "POST", route+"/complete", "", 200)
	got, _ := f.s.fileRevisions(owner, "versions", "")
	if len(got) != 2 {
		t.Fatalf("kept %d", len(got))
	}
	root, _ := f.s.filesRoot(owner)
	data, err := os.ReadFile(filepath.Join(root, "report.txt"))
	if err != nil || string(data) != "current" {
		t.Fatalf("live %s %v", data, err)
	}
	if _, err = os.Stat(filepath.Join(uploads, "active")); err != nil {
		t.Fatal(err)
	}
	log, err := f.s.cleanupJournal(owner)
	if err != nil || len(log) != 2 || !log[0].Completed || !log[1].Completed {
		t.Fatalf("journal: %+v %v", log, err)
	}
	json.Unmarshal(f.call(0, "POST", "/api/v1/files/library/cleanup/run", "", 200).Body.Bytes(), &plan)
	if len(plan.Entries) != 0 {
		t.Fatal("repeat cleanup")
	}
}

func TestFileRetentionRecoveryAndAutomaticOptIn(t *testing.T) {
	f := newFileFixture(t)
	owner := f.users[0]
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "old", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "new", 200)
	versions, _ := f.s.fileRevisions(owner, "versions", "")
	rev := versions[0]
	rev.At = time.Now().Add(-48 * time.Hour)
	if err := f.s.rewriteRevision(owner, rev); err != nil {
		t.Fatal(err)
	}
	f.s.cleanFileRetention(context.Background())
	got, _ := f.s.fileRevisions(owner, "versions", "")
	if len(got) != 1 {
		t.Fatal("default must preserve data")
	}
	// Simulate a crash after durable intent and partial deletion, before completion.
	root, _ := f.s.fileStateRoot(owner, "cleanup")
	e := fileCleanupEntry{ID: rev.ID, Kind: "versions", Revision: rev, Bytes: 3, At: time.Now().UTC()}
	if err := writeFileStateJSON(filepath.Join(root, e.ID+".json"), e); err != nil {
		t.Fatal(err)
	}
	_, dir, _ := f.s.revision(owner, "versions", rev.ID)
	if err := os.Remove(filepath.Join(dir, "metadata.json")); err != nil {
		t.Fatal(err)
	}
	f.s.cleanFileRetention(context.Background())
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("recovery %v", err)
	}
	log, err := f.s.cleanupJournal(owner)
	if err != nil || len(log) != 1 || !log[0].Completed {
		t.Fatalf("recovered journal %+v %v", log, err)
	}
	// Re-run after content removal but before completion also succeeds.
	e.Completed = false
	writeFileStateJSON(filepath.Join(root, e.ID+".json"), e)
	if err = f.s.recoverFileCleanup(context.Background(), owner); err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "newest", 200)
	versions, _ = f.s.fileRevisions(owner, "versions", "")
	rev = versions[0]
	rev.At = time.Now().Add(-48 * time.Hour)
	f.s.rewriteRevision(owner, rev)
	f.call(0, "PUT", "/api/v1/files/library/retention", `{"automatic":true,"version_days":1}`, 200)
	f.s.cleanFileRetention(context.Background())
	versions, _ = f.s.fileRevisions(owner, "versions", "")
	if len(versions) != 0 {
		t.Fatal("automatic cleanup did not run")
	}
	// Cancellation must leave a fresh candidate untouched.
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "latest", 200)
	versions, _ = f.s.fileRevisions(owner, "versions", "")
	rev = versions[0]
	rev.At = time.Now().Add(-48 * time.Hour)
	f.s.rewriteRevision(owner, rev)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = f.s.executeFileCleanup(ctx, owner, fileRetentionPolicy{VersionDays: 1}); err == nil {
		t.Fatal("expected cancellation")
	}
	versions, _ = f.s.fileRevisions(owner, "versions", "")
	if len(versions) != 1 {
		t.Fatal("cancelled cleanup removed data")
	}
}
