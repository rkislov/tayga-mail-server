package notify

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestTaskOutboxRetriesAndOfflineInbox(t *testing.T) {
	ctx := context.Background()
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "tasks.db")}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	org, _ := st.CreateTenant(ctx, "tasks")
	d, _ := st.CreateDomain(ctx, org.ID, "example.test")
	owner, _ := st.CreateUser(ctx, &storage.User{TenantID: org.ID, DomainID: d.ID, Email: "o@example.test", LocalPart: "o", Enabled: true})
	assignee, _ := st.CreateUser(ctx, &storage.User{TenantID: org.ID, DomainID: d.ID, Email: "a@example.test", LocalPart: "a", Enabled: true})
	task := storage.Task{Title: "Not a push payload", Body: "Secret content", AssigneeID: assignee.ID}
	if err = st.SaveTask(ctx, owner.ID, &task); err != nil {
		t.Fatal(err)
	}
	hub := NewHub()
	loop := &TaskLoop{Store: st, Hub: hub}
	var ids []string
	hub.SetSink(func(user string, event Event) error {
		ids = append(ids, event.ID)
		if user != assignee.ID || event.Kind != KindTask || event.Href != "tasks" || event.Title == task.Title {
			t.Fatal(event)
		}
		return errors.New("offline push queue")
	})
	if err = loop.drain(ctx); err == nil {
		t.Fatal("sink error ignored")
	}
	pending, err := st.PendingTaskNotifications(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatal(pending, err)
	}
	hub.SetSink(func(user string, event Event) error { ids = append(ids, event.ID); return nil })
	if err = loop.drain(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatal("unstable retry", ids)
	}
	pending, err = st.PendingTaskNotifications(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatal(pending, err)
	}
	notices, total, unread, err := st.TaskNotifications(ctx, assignee.ID, 25, 0)
	if err != nil || total != 1 || unread != 1 || len(notices) != 1 {
		t.Fatal(notices, err)
	}
	// Read offline notifications do not produce late push alerts.
	task.Title = "Updated"
	if err = st.SaveTask(ctx, owner.ID, &task); err != nil {
		t.Fatal(err)
	}
	notices, _, _, _ = st.TaskNotifications(ctx, assignee.ID, 25, 0)
	if err = st.ReadTaskNotification(ctx, assignee.ID, notices[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = loop.drain(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatal("read notification was pushed")
	}
}
