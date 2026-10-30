package storage

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage/migrations"
)

func TestFlowSyncTaskAtomicReceiptAndCompletionTime(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "receipt.db")}}
	st, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { st.Close() }()
	tenant, e := st.CreateTenant(ctx, "receipt")
	if e != nil {
		t.Fatal(e)
	}
	domain, e := st.CreateDomain(ctx, tenant.ID, "example.test")
	if e != nil {
		t.Fatal(e)
	}
	u, e := st.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, Email: "a@example.test", LocalPart: "a", AuthSource: "local", Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	d, e := st.EnsureFlowSyncDevice(ctx, u.ID, "phone", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = st.CreateFlowSyncTask(ctx, u.ID, d.ID, "create", &Task{Status: "invalid"}); e == nil {
		t.Fatal("invalid task accepted")
	}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := st.CreateFlowSyncTask(ctx, u.ID, d.ID, "create", &Task{Title: "Concurrent", Completed: true})
			ids <- id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id string
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("concurrent duplicate", id, got)
		}
		id = got
	}
	rows, e := st.ListTasks(ctx, u.ID)
	if e != nil || len(rows) != 1 {
		t.Fatal(rows, e)
	}
	completed := rows[0].CompletedAt
	if _, e = time.Parse(time.RFC3339Nano, completed); e != nil {
		t.Fatal(completed, e)
	}
	task := rows[0]
	task.Title = "Changed"
	if e = st.SaveTask(ctx, u.ID, &task); e != nil {
		t.Fatal(e)
	}
	if task.CompletedAt != completed {
		t.Fatal("completion time changed on title edit")
	}
	st.Close()
	st, e = Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	got, e := st.CreateFlowSyncTask(ctx, u.ID, d.ID, "create", &Task{Title: "Stale"})
	if e != nil || got != id {
		t.Fatal(got, e)
	}
	rows, e = st.ListTasks(ctx, u.ID)
	if e != nil || len(rows) != 1 || rows[0].Title != "Changed" || rows[0].CompletedAt != completed {
		t.Fatal(rows, e)
	}
	if _, e = st.CreateFlowSyncTask(ctx, "foreign", d.ID, "create", &Task{}); e != ErrUnauthorized {
		t.Fatal("foreign device allowed", e)
	}
	if e = st.DeleteTask(ctx, u.ID, id); e != nil {
		t.Fatal(e)
	}
	if got, e = st.CreateFlowSyncTask(ctx, u.ID, d.ID, "create", &Task{}); e != nil || got != id {
		t.Fatal(got, e)
	}
	if rows, e = st.ListTasks(ctx, u.ID); e != nil || len(rows) != 0 {
		t.Fatal("resurrected task", rows, e)
	}
}

func TestTaskProgressPersistenceAndLegacyClients(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "tasks.db")}}
	driver, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { driver.Close() }()
	tenant, err := driver.CreateTenant(ctx, "tasks")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := driver.CreateDomain(ctx, tenant.ID, "example.test")
	if err != nil {
		t.Fatal(err)
	}
	user, err := driver.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, Email: "tasks@example.test", LocalPart: "tasks", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	task := Task{Title: "Progress", Status: TaskInProgress, PercentComplete: 37.5}
	if err = driver.SaveTask(ctx, user.ID, &task); err != nil {
		t.Fatal(err)
	}
	read := func() Task {
		t.Helper()
		rows, err := driver.ListTasks(ctx, user.ID)
		if err != nil || len(rows) != 1 {
			t.Fatalf("rows=%+v err=%v", rows, err)
		}
		return rows[0]
	}
	driver.Close()
	driver, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := read()
	if got.Status != TaskInProgress || got.PercentComplete != 37.5 || got.Completed {
		t.Fatal(got)
	}
	// A legacy title edit with no progress fields must retain the EWS progress.
	legacy := Task{ID: got.ID, Revision: got.Revision, Title: "Renamed"}
	if err = driver.SaveTask(ctx, user.ID, &legacy); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Status != TaskInProgress || got.PercentComplete != 37.5 {
		t.Fatal(got)
	}
	for _, status := range []string{TaskWaitingOnOthers, TaskDeferred, TaskCompleted, TaskNotStarted} {
		if err = got.SetStatus(status); err != nil {
			t.Fatal(err)
		}
		if err = driver.SaveTask(ctx, user.ID, &got); err != nil {
			t.Fatal(err)
		}
		got = read()
		if got.Status != status || got.Completed != (status == TaskCompleted) {
			t.Fatal(got)
		}
	}
	// Cached web clients spread a GET response, then change just the checkbox.
	got.Completed = true
	if err = driver.SaveTask(ctx, user.ID, &got); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Status != TaskCompleted || got.PercentComplete != 100 {
		t.Fatal(got)
	}
	got.EWSProperties = `{"task:CompleteDate":"<t:CompleteDate>2026-10-19T00:00:00Z</t:CompleteDate>","item:Importance":"<t:Importance>High</t:Importance>"}`
	if err = driver.SaveTask(ctx, user.ID, &got); err != nil {
		t.Fatal(err)
	}
	got.Completed = false
	got.EWSProperties = ""
	if err = driver.SaveTask(ctx, user.ID, &got); err != nil {
		t.Fatal(err)
	}
	got = read()
	if got.Status != TaskNotStarted || got.PercentComplete != 0 || strings.Contains(got.EWSProperties, "CompleteDate") || !strings.Contains(got.EWSProperties, "High") {
		t.Fatal(got)
	}
	for _, bad := range []Task{
		{Status: "Unknown"}, {Status: TaskInProgress, PercentComplete: -1},
		{Status: TaskInProgress, PercentComplete: 101}, {Status: TaskInProgress, PercentComplete: math.NaN()},
		{Status: TaskInProgress, PercentComplete: math.Inf(1)},
	} {
		bad.ID, bad.Revision, bad.Title = got.ID, got.Revision, "Must not save"
		if err = driver.SaveTask(ctx, user.ID, &bad); err != ErrInvalidTaskProgress {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if after := read(); after.Revision != got.Revision || after.Title != got.Title {
		t.Fatal("invalid update persisted", after)
	}
	foreign := got
	if err = driver.SaveTask(ctx, "another-user", &foreign); err != ErrNotFound {
		t.Fatal(err)
	}
	stale := got
	got.Title = "Current"
	if err = driver.SaveTask(ctx, user.ID, &got); err != nil {
		t.Fatal(err)
	}
	stale.Status = TaskCompleted
	if err = driver.SaveTask(ctx, user.ID, &stale); err != ErrNotFound {
		t.Fatal("stale progress accepted", err)
	}

	// Restore the pre-046/047 schema on this disposable database, then use the real
	// startup migration to verify old completed/unfinished records are preserved.
	got.Completed = true
	if err = driver.SaveTask(ctx, user.ID, &got); err != nil {
		t.Fatal(err)
	}
	second := Task{Title: "Unfinished"}
	if err = driver.SaveTask(ctx, user.ID, &second); err != nil {
		t.Fatal(err)
	}
	st := driver.(*Store)
	raw047, err := migrations.FS.ReadFile("047_task_completion_time.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = execScript(ctx, st.db, strings.Split(string(raw047), "-- +goose Down")[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='047_task_completion_time.sqlite'`); err != nil {
		t.Fatal(err)
	}
	raw, err := migrations.FS.ReadFile("046_task_progress.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	down := strings.Split(string(raw), "-- +goose Down")[1]
	if err = execScript(ctx, st.db, down); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='046_task_progress.sqlite'`); err != nil {
		t.Fatal(err)
	}
	driver.Close()
	driver, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := driver.ListTasks(ctx, user.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, row := range rows {
		if row.ID == got.ID {
			if row.Status != TaskCompleted || row.PercentComplete != 100 || row.Revision != got.Revision {
				t.Fatal(row)
			}
			if _, err := time.Parse(time.RFC3339Nano, row.CompletedAt); err != nil {
				t.Fatal("missing migrated completion time", row, err)
			}
		} else if row.ID != second.ID || row.Status != TaskNotStarted || row.PercentComplete != 0 {
			t.Fatal(row)
		} else if row.CompletedAt != "" {
			t.Fatal("unfinished task received completion time", row)
		}
	}
}

func TestEWSTaskAtomicReceiptAndCompletionTime(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "receipt.db")}}
	if pg := bookingPostgresConfig(t); pg != nil {
		cfg = *pg
	}
	st, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { st.Close() }()
	tenant, e := st.CreateTenant(ctx, "receipt")
	if e != nil {
		t.Fatal(e)
	}
	domain, e := st.CreateDomain(ctx, tenant.ID, "example.test")
	if e != nil {
		t.Fatal(e)
	}
	u, e := st.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, Email: "a@example.test", LocalPart: "a", AuthSource: "local", Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = st.CreateEWSTask(ctx, u.ID, "create", &Task{Status: "invalid"}); e == nil {
		t.Fatal("invalid task accepted")
	}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := st.CreateEWSTask(ctx, u.ID, "create", &Task{Title: "Concurrent", Completed: true})
			ids <- id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id string
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("concurrent duplicate", id, got)
		}
		id = got
	}
	rows, e := st.ListTasks(ctx, u.ID)
	if e != nil || len(rows) != 1 {
		t.Fatal(rows, e)
	}
	completed := rows[0].CompletedAt
	if _, e = time.Parse(time.RFC3339Nano, completed); e != nil {
		t.Fatal(completed, e)
	}
	task := rows[0]
	task.Title = "Changed"
	if e = st.SaveTask(ctx, u.ID, &task); e != nil {
		t.Fatal(e)
	}
	if task.CompletedAt != completed {
		t.Fatal("completion time changed on title edit")
	}
	st.Close()
	st, e = Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	got, e := st.CreateEWSTask(ctx, u.ID, "create", &Task{Title: "Stale"})
	if e != nil || got != id {
		t.Fatal(got, e)
	}
	rows, e = st.ListTasks(ctx, u.ID)
	if e != nil || len(rows) != 1 || rows[0].Title != "Changed" || rows[0].CompletedAt != completed {
		t.Fatal(rows, e)
	}
	if e = st.DeleteTaskVersion(ctx, u.ID, id, 1); e != ErrTaskConflict {
		t.Fatal("stale deletion allowed", e)
	}
	if e = st.DeleteTaskVersion(ctx, u.ID, id, task.Revision); e != nil {
		t.Fatal(e)
	}
	if got, e = st.CreateEWSTask(ctx, u.ID, "create", &Task{}); e != nil || got != id {
		t.Fatal(got, e)
	}
	if rows, e = st.ListTasks(ctx, u.ID); e != nil || len(rows) != 0 {
		t.Fatal("resurrected task", rows, e)
	}
}
