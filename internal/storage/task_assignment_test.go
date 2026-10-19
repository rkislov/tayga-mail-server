package storage

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage/migrations"
)

func TestTaskAssignmentLifecycleIsolationAndDurability(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "tasks.db")}}
	st, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { st.Close() }()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	org, err := st.CreateTenant(ctx, "Org")
	check(err)
	other, err := st.CreateTenant(ctx, "Other")
	check(err)
	domain, err := st.CreateDomain(ctx, org.ID, "org.test")
	check(err)
	domain2, err := st.CreateDomain(ctx, org.ID, "branch.test")
	check(err)
	domain3, err := st.CreateDomain(ctx, other.ID, "other.test")
	check(err)
	mk := func(d *Domain, name string) *User {
		t.Helper()
		u, e := st.CreateUser(ctx, &User{TenantID: d.TenantID, DomainID: d.ID, Email: name + "@" + d.Name, LocalPart: name, DisplayName: "Сотрудник " + name, Enabled: true})
		check(e)
		return u
	}
	owner, assignee, next, stranger, foreign := mk(domain, "author"), mk(domain2, "assignee"), mk(domain, "next"), mk(domain, "stranger"), mk(domain3, "external")
	disabled := mk(domain, "disabled")
	check(st.UpdateUserEnabled(ctx, disabled.ID, false))
	check(st.UpdateUserRoles(ctx, foreign.ID, "global_admin"))
	candidates, total, err := st.TaskAssignees(ctx, owner.ID, "СОТРУДНИК", 1, 1)
	check(err)
	if total != 3 || len(candidates) != 1 {
		t.Fatal(candidates, total)
	}
	candidates, total, err = st.TaskAssignees(ctx, owner.ID, "%", 25, 0)
	check(err)
	if total != 0 {
		t.Fatal("wildcard search", candidates)
	}
	for _, id := range []string{foreign.ID, disabled.ID, owner.ID, "missing"} {
		task := Task{Title: "Forbidden", AssigneeID: id}
		if err = st.SaveTask(ctx, owner.ID, &task); err == nil {
			t.Fatal("invalid assignee", id)
		}
	}
	rows, err := st.ListTasks(ctx, owner.ID)
	check(err)
	if len(rows) != 0 {
		t.Fatal("partial create")
	}
	task := Task{Title: "Prepare pilot", Body: "Private description", Due: "2026-11-01T09:45:00Z", AssigneeID: assignee.ID}
	check(st.SaveTask(ctx, owner.ID, &task))
	get := func(user string) *Task { t.Helper(); v, e := st.GetTask(ctx, user, task.ID); check(e); return v }
	for _, u := range []*User{stranger, foreign} {
		if _, err = st.GetTask(ctx, u.ID, task.ID); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if _, _, err = st.TaskHistory(ctx, u.ID, task.ID, 10, 0); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	pending := get(assignee.ID)
	if pending.CanManage || pending.CanUpdate || pending.OwnerID != owner.ID || pending.AssignmentState != "pending" {
		t.Fatal(pending)
	}
	pending.SetStatus(TaskCompleted)
	if err = st.SaveTask(ctx, assignee.ID, pending); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("pending write", err)
	}
	if err = st.DeleteTask(ctx, assignee.ID, task.ID); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("assignee delete", err)
	}
	if _, err = st.AssignTask(ctx, assignee.ID, task.ID, "assign", next.ID, task.Revision); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	accepted, err := st.AssignTask(ctx, assignee.ID, task.ID, "accept", "", task.Revision)
	check(err)
	if !accepted.CanUpdate || accepted.CanManage {
		t.Fatal(accepted)
	}
	// A stale response must not undo acceptance or insert another history event.
	if _, err = st.AssignTask(ctx, assignee.ID, task.ID, "decline", "", task.Revision); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for _, field := range []string{"title", "body", "due", "properties"} {
		update := *accepted
		switch field {
		case "title":
			update.Title = "Injected"
		case "body":
			update.Body = "Injected"
		case "due":
			update.Due = ""
		case "properties":
			update.EWSProperties = `{"item:Importance":"High"}`
		}
		if err = st.SaveTask(ctx, assignee.ID, &update); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(field, err)
		}
	}
	accepted.SetPercentComplete(45)
	check(st.SaveTask(ctx, assignee.ID, accepted))
	if got := get(owner.ID); got.PercentComplete != 45 || got.Due != task.Due {
		t.Fatal(got)
	}
	stale := *accepted
	accepted.SetStatus(TaskCompleted)
	check(st.SaveTask(ctx, assignee.ID, accepted))
	if err = st.SaveTask(ctx, assignee.ID, &stale); !errors.Is(err, ErrNotFound) {
		t.Fatal("stale progress", err)
	}
	notices, total, unread, err := st.TaskNotifications(ctx, owner.ID, 100, 0)
	check(err)
	if total != 3 || unread != 3 || notices[0].Action != "complete" {
		t.Fatal(notices, total, unread)
	}
	history, total, err := st.TaskHistory(ctx, owner.ID, task.ID, 100, 0)
	check(err)
	if total != 4 || history[0].Action != "complete" || history[0].ActorID != assignee.ID {
		t.Fatal(history, total)
	}
	// Reopen and hand the same task to another employee; old access disappears.
	current := get(owner.ID)
	current.SetStatus(TaskNotStarted)
	check(st.SaveTask(ctx, owner.ID, current))
	current, err = st.AssignTask(ctx, owner.ID, task.ID, "assign", next.ID, current.Revision)
	check(err)
	if _, err = st.GetTask(ctx, assignee.ID, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked access", err)
	}
	if err = st.SaveTask(ctx, assignee.ID, &stale); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked write", err)
	}
	history, total, err = st.TaskHistory(ctx, next.ID, task.ID, 10, 0)
	check(err)
	if total != 1 || history[0].Action != "assign" || history[0].Detail.AssigneeName != next.DisplayName || history[0].Detail.AssigneeEmail != next.Email {
		t.Fatal("new assignee saw previous history", history)
	}
	_, err = st.AssignTask(ctx, next.ID, task.ID, "decline", "", current.Revision)
	check(err)
	if _, err = st.GetTask(ctx, next.ID, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("declined still visible", err)
	}
	current = get(owner.ID)
	current, err = st.AssignTask(ctx, owner.ID, task.ID, "assign", next.ID, current.Revision)
	check(err)
	// A tenant move immediately closes the existing grant and hides notifications.
	_, err = st.DB().Exec(`UPDATE users SET tenant_id=?,domain_id=? WHERE id=?`, other.ID, domain3.ID, next.ID)
	check(err)
	if _, err = st.GetTask(ctx, next.ID, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("tenant move leaked", err)
	}
	if n, _, _, e := st.TaskNotifications(ctx, next.ID, 100, 0); e != nil || len(n) != 0 {
		t.Fatal(n, e)
	}
	_, err = st.DB().Exec(`UPDATE users SET tenant_id=?,domain_id=? WHERE id=?`, org.ID, domain.ID, next.ID)
	check(err)
	current, err = st.AssignTask(ctx, owner.ID, task.ID, "revoke", "", current.Revision)
	check(err)
	// Notification acknowledgement is private, and survives a process restart.
	if err = st.ReadTaskNotification(ctx, stranger.ID, notices[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	check(st.ReadTaskNotification(ctx, owner.ID, notices[0].ID))
	check(st.Close())
	st, err = Open(ctx, cfg)
	check(err)
	notices, _, unread, err = st.TaskNotifications(ctx, owner.ID, 100, 0)
	check(err)
	if unread != 3 {
		t.Fatal("read state lost", unread, notices)
	} // accept, progress, decline; complete acknowledged
	current = get(owner.ID)
	if current.AssigneeID != "" || current.AssignmentState != "" {
		t.Fatal(current)
	}
	// A failed outbox write rolls back progress, history, and assignment together.
	current, err = st.AssignTask(ctx, owner.ID, task.ID, "assign", assignee.ID, current.Revision)
	check(err)
	_, err = st.DB().Exec(`CREATE TRIGGER fail_task_notice BEFORE INSERT ON task_notifications BEGIN SELECT RAISE(ABORT,'test outbox failure'); END`)
	check(err)
	before := *current
	current.Title = "Must roll back"
	if err = st.SaveTask(ctx, owner.ID, current); err == nil {
		t.Fatal("outbox failure ignored")
	}
	if got := get(owner.ID); got.Title != before.Title || got.Revision != before.Revision {
		t.Fatal("partial write", got)
	}
	_, err = st.DB().Exec(`DROP TRIGGER fail_task_notice`)
	check(err)
	check(st.DeleteTask(ctx, owner.ID, task.ID))
	if _, err = st.GetTask(ctx, assignee.ID, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	notices, _, _, err = st.TaskNotifications(ctx, assignee.ID, 100, 0)
	check(err)
	if notices[0].Action != "delete" || strings.Contains(notices[0].Action, "Private") {
		t.Fatal(notices)
	}
}

func TestTaskAssignmentMigrationKeepsExistingTasks(t *testing.T) {
	ctx := context.Background()
	cfg := config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "migration.db")}}
	driver, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { driver.Close() }()
	tenant, err := driver.CreateTenant(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	domain, err := driver.CreateDomain(ctx, tenant.ID, "legacy.test")
	if err != nil {
		t.Fatal(err)
	}
	user, err := driver.CreateUser(ctx, &User{TenantID: tenant.ID, DomainID: domain.ID, Email: "u@legacy.test", LocalPart: "u", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	task := Task{Title: "Existing", Status: TaskWaitingOnOthers, PercentComplete: 55, Due: "2026-11-01T10:00:00Z"}
	if err = driver.SaveTask(ctx, user.ID, &task); err != nil {
		t.Fatal(err)
	}
	raw, err := migrations.FS.ReadFile("048_task_assignment.sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = execScript(ctx, driver.DB(), strings.Split(string(raw), "-- +goose Down")[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = driver.DB().Exec(`DELETE FROM schema_migrations WHERE version='048_task_assignment.sqlite'`); err != nil {
		t.Fatal(err)
	}
	driver.Close()
	driver, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := driver.GetTask(ctx, user.ID, task.ID)
	if err != nil || got.Title != task.Title || got.Status != task.Status || got.PercentComplete != 55 || got.AssigneeID != "" || got.Revision != 1 || got.Due != task.Due {
		t.Fatal(got, err)
	}
	history, total, err := driver.TaskHistory(ctx, user.ID, task.ID, 25, 0)
	if err != nil || total != 0 || len(history) != 0 {
		t.Fatal("migration invented activity", history, err)
	}
	got.Title = "Updated after migration"
	if err = driver.SaveTask(ctx, user.ID, got); err != nil {
		t.Fatal(err)
	}
	history, total, err = driver.TaskHistory(ctx, user.ID, task.ID, 25, 0)
	if err != nil || total != 1 || history[0].Revision != 2 {
		t.Fatal(history, err)
	}
}
