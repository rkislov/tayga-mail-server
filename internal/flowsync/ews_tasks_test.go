// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestFlaggedMailTasksLifecycle(t *testing.T) {
	ctx, store, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: store, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, user); err != nil {
		t.Fatal(err)
	}
	mb, _ := store.GetMailbox(ctx, user.ID, "INBOX")
	msg, err := store.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: "task-test", InternalDate: time.Now(), Subject: "Follow up"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = store.UpdateMessageFlags(ctx, msg.ID, `\Seen \Flagged`); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := store.ListTasks(ctx, user.ID)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("%+v %v", tasks, err)
	}
	task := tasks[0]
	other := *user
	other.ID = "not-owner"
	if _, err = h.getTask(ctx, &other, task.ID); err == nil {
		t.Fatal("foreign task visible")
	}
	list, err := h.findItem(ctx, user, `<FindItem><ParentFolderIds><DistinguishedFolderId Id="tasks"/></ParentFolderIds></FindItem>`)
	if err != nil || !strings.Contains(list, task.ID) {
		t.Fatalf("%s %v", list, err)
	}
	requireXML(t, list)
	sync, err := h.syncEWSItems(ctx, user, `<SyncFolderItems><SyncFolderId><DistinguishedFolderId Id="tasks"/></SyncFolderId></SyncFolderItems>`)
	if err != nil || !strings.Contains(sync, task.ID) {
		t.Fatalf("%s %v", sync, err)
	}
	response, err := h.updateItem(ctx, user, `<UpdateItem><ItemChanges><ItemChange><ItemId Id="`+task.ID+`"/><Updates><SetItemField><FieldURI FieldURI="task:Status"/><Task><Status>Completed</Status></Task></SetItemField></Updates></ItemChange></ItemChanges></UpdateItem>`)
	if err != nil || !strings.Contains(response, "NoError") {
		t.Fatalf("%s %v", response, err)
	}
	saved, _ := store.GetMessageByID(ctx, msg.ID)
	if storage.HasFlag(saved.Flags, `\Flagged`) || !storage.HasFlag(saved.Flags, `\Seen`) {
		t.Fatal(saved.Flags)
	}
	tasks, _ = store.ListTasks(ctx, user.ID)
	if len(tasks) != 1 || !tasks[0].Completed {
		t.Fatal(tasks)
	}
	if err = store.UpdateEWSMailMetadata(ctx, msg.ID, `\Flagged`, "{}"); err != nil {
		t.Fatal(err)
	}
	tasks, _ = store.ListTasks(ctx, user.ID)
	if len(tasks) != 1 || tasks[0].Completed || tasks[0].ID != task.ID {
		t.Fatal(tasks)
	}
	stale := tasks[0]
	changed := stale
	changed.Title = "Renamed"
	if err = store.SaveTask(ctx, user.ID, &changed); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveTask(ctx, user.ID, &stale); err == nil {
		t.Fatal("stale update accepted")
	}
	if err = store.DeleteTask(ctx, "not-owner", task.ID); err == nil {
		t.Fatal("foreign task deleted")
	}
	if err = store.DeleteTask(ctx, user.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	tasks, _ = store.ListTasks(ctx, user.ID)
	if len(tasks) != 0 {
		t.Fatal("deleted task recreated", tasks)
	}
	created, err := h.createItem(ctx, user, `<CreateItem><SavedItemFolderId><DistinguishedFolderId Id="tasks"/></SavedItemFolderId><Items><Task><Subject>Standalone</Subject><DueDate>2026-10-20T12:00:00Z</DueDate></Task></Items></CreateItem>`)
	if err != nil || !strings.Contains(created, "NoError") {
		t.Fatalf("%s %v", created, err)
	}
	requireXML(t, created)
	tasks, _ = store.ListTasks(ctx, user.ID)
	if len(tasks) != 1 || tasks[0].Title != "Standalone" {
		t.Fatal(tasks)
	}
}

func TestOutlookTaskCommonAndExtendedUpdates(t *testing.T) {
	ctx, store, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: store, ms: eas.ms}
	task := &storage.Task{Title: "Original"}
	if err := store.SaveTask(ctx, user.ID, task); err != nil {
		t.Fatal(err)
	}
	extended := `<ExtendedFieldURI DistinguishedPropertySetId="Common" PropertyId="34072" PropertyType="Integer"/>`
	updates := `<SetItemField><FieldURI FieldURI="item:Subject"/><Item><Subject>Outlook title</Subject></Item></SetItemField><SetItemField><FieldURI FieldURI="item:Importance"/><Item><Importance>High</Importance></Item></SetItemField><SetItemField><FieldURI FieldURI="item:Sensitivity"/><Item><Sensitivity>Private</Sensitivity></Item></SetItemField><SetItemField><FieldURI FieldURI="task:Status"/><Task><Status>Completed</Status></Task></SetItemField><SetItemField><FieldURI FieldURI="task:CompleteDate"/><Task><CompleteDate>2026-10-13T14:13:00Z</CompleteDate></Task></SetItemField><SetItemField>` + extended + `<Task><ExtendedProperty>` + extended + `<Value>1</Value></ExtendedProperty></Task></SetItemField>`
	run := func(updates string) string {
		t.Helper()
		out, err := h.updateItem(ctx, user, `<UpdateItem><ItemChanges><ItemChange><ItemId Id="`+task.ID+`"/><Updates>`+updates+`</Updates></ItemChange></ItemChanges></UpdateItem>`)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := run(updates); !strings.Contains(out, "NoError") {
		t.Fatal(out)
	}
	saved, err := h.getTask(ctx, user, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != "Outlook title" || !saved.Completed {
		t.Fatal(saved)
	}
	out := taskXML(*saved, user.ID)
	requireXML(t, out)
	for _, want := range []string{"High", "Private", "2026-10-13T14:13:00Z", "34072"} {
		if !strings.Contains(out, want) {
			t.Fatal(out)
		}
	}
	revision := saved.Revision
	if out := run(`<SetItemField><FieldURI FieldURI="item:Subject"/><Item><Subject>Should roll back</Subject></Item></SetItemField><SetItemField><FieldURI FieldURI="task:Status"/><Task><Subject>Completed</Subject></Task></SetItemField>`); !strings.Contains(out, "ErrorInvalidPropertySet") {
		t.Fatal(out)
	}
	saved, _ = h.getTask(ctx, user, task.ID)
	if saved.Title != "Outlook title" || saved.Revision != revision {
		t.Fatal("partial update persisted", saved)
	}
	if out := run(`<DeleteItemField><FieldURI FieldURI="task:CompleteDate"/></DeleteItemField><DeleteItemField>` + extended + `</DeleteItemField>`); !strings.Contains(out, "NoError") {
		t.Fatal(out)
	}
	saved, _ = h.getTask(ctx, user, task.ID)
	if strings.Contains(saved.EWSProperties, "34072") || strings.Contains(saved.EWSProperties, "CompleteDate") {
		t.Fatal(saved.EWSProperties)
	}
}
