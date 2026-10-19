// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"fmt"
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
	if err = task.SetPercentComplete(45); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveTask(ctx, user.ID, &task); err != nil {
		t.Fatal(err)
	}
	// Repeated reconciliation of the unchanged mail flag must retain progress.
	tasks, err = store.ListTasks(ctx, user.ID)
	if err != nil || len(tasks) != 1 || tasks[0].PercentComplete != 45 || tasks[0].Status != storage.TaskInProgress || tasks[0].Revision != task.Revision {
		t.Fatalf("%+v %v", tasks, err)
	}
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
	if len(tasks) != 1 || !tasks[0].Completed || tasks[0].PercentComplete != 100 || tasks[0].Status != storage.TaskCompleted {
		t.Fatal(tasks)
	}
	completed := tasks[0]
	completed.EWSProperties = `{"task:CompleteDate":"<t:CompleteDate>2026-10-19T00:00:00Z</t:CompleteDate>","item:Importance":"<t:Importance>High</t:Importance>"}`
	if err = store.SaveTask(ctx, user.ID, &completed); err != nil {
		t.Fatal(err)
	}
	if err = store.UpdateEWSMailMetadata(ctx, msg.ID, `\Flagged`, "{}"); err != nil {
		t.Fatal(err)
	}
	tasks, _ = store.ListTasks(ctx, user.ID)
	if len(tasks) != 1 || tasks[0].Completed || tasks[0].ID != task.ID || tasks[0].Status != storage.TaskNotStarted || tasks[0].PercentComplete != 0 {
		t.Fatal(tasks)
	}
	if strings.Contains(tasks[0].EWSProperties, "CompleteDate") || !strings.Contains(tasks[0].EWSProperties, "High") {
		t.Fatal(tasks[0].EWSProperties)
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

func TestEWSTaskProgressAndPropertyOrder(t *testing.T) {
	ctx, store, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: store, ms: eas.ms}
	created, err := h.createItem(ctx, user, `<CreateItem><SavedItemFolderId><DistinguishedFolderId Id="tasks"/></SavedItemFolderId><Items><Task><Subject>Progress</Subject><Status>InProgress</Status><PercentComplete>25.5</PercentComplete></Task></Items></CreateItem>`)
	if err != nil || !strings.Contains(created, "NoError") {
		t.Fatalf("%s %v", created, err)
	}
	requireXML(t, created)
	id := extractAttr(created, "ItemId", "Id")
	initial, err := h.syncEWSItems(ctx, user, `<SyncFolderItems><SyncFolderId><DistinguishedFolderId Id="tasks"/></SyncFolderId></SyncFolderItems>`)
	if err != nil || extractTag(initial, "PercentComplete") != "25.5" {
		t.Fatalf("%s %v", initial, err)
	}
	syncState := extractTag(initial, "SyncState")
	field := func(name, value string) string {
		return `<SetItemField><FieldURI FieldURI="task:` + name + `"/><Task><` + name + `>` + value + `</` + name + `></Task></SetItemField>`
	}
	run := func(updates string) string {
		t.Helper()
		out, err := h.updateItem(ctx, user, `<UpdateItem><ItemChanges><ItemChange><ItemId Id="`+id+`"/><Updates>`+updates+`</Updates></ItemChange></ItemChanges></UpdateItem>`)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	for _, test := range []struct {
		updates, status string
		percent         float64
		date            bool
	}{
		{field("Status", "WaitingOnOthers"), storage.TaskWaitingOnOthers, 25.5, false},
		{field("Status", "Deferred"), storage.TaskDeferred, 25.5, false},
		{field("PercentComplete", "100.0"), storage.TaskCompleted, 100, false},
		{field("PercentComplete", "60"), storage.TaskInProgress, 60, false},
		{field("CompleteDate", "2026-10-19T00:00:00Z"), storage.TaskCompleted, 100, true},
		{field("Status", "NotStarted"), storage.TaskNotStarted, 0, false},
		{field("PercentComplete", "100") + field("CompleteDate", "2026-10-19T00:00:00Z") + field("Status", "NotStarted"), storage.TaskNotStarted, 0, false},
		{field("Status", "NotStarted") + field("CompleteDate", "2026-10-19T00:00:00Z"), storage.TaskCompleted, 100, true},
		{field("Status", "InProgress") + field("Status", "Completed"), storage.TaskCompleted, 100, false},
	} {
		out := run(test.updates)
		if !strings.Contains(out, "NoError") {
			t.Fatal(out)
		}
		saved, err := h.getTask(ctx, user, id)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Status != test.status || saved.PercentComplete != test.percent || saved.Completed != (test.status == storage.TaskCompleted) {
			t.Fatalf("%+v expected %+v", saved, test)
		}
		xml := taskXML(*saved, user.ID)
		requireXML(t, xml)
		if (extractTag(xml, "CompleteDate") != "") != test.date || !strings.Contains(xml, "<t:Status>"+test.status+"</t:Status>") || !strings.Contains(xml, fmt.Sprintf("<t:PercentComplete>%v</t:PercentComplete>", test.percent)) {
			t.Fatal(xml)
		}
	}
	saved, _ := h.getTask(ctx, user, id)
	for _, invalid := range []string{field("Status", "Unknown"), field("PercentComplete", "NaN"), field("PercentComplete", "Inf"), field("PercentComplete", "-1"), field("PercentComplete", "101")} {
		out := run(field("Status", "InProgress") + invalid)
		if !strings.Contains(out, "ErrorInvalidPropertySet") {
			t.Fatal(out)
		}
		after, _ := h.getTask(ctx, user, id)
		if after.Revision != saved.Revision || after.Status != saved.Status {
			t.Fatal("partial progress saved", after)
		}
	}
	delta, err := h.syncEWSItems(ctx, user, `<SyncFolderItems><SyncFolderId><DistinguishedFolderId Id="tasks"/></SyncFolderId><SyncState>`+syncState+`</SyncState></SyncFolderItems>`)
	if err != nil || !strings.Contains(delta, "<t:Update>") || extractTag(delta, "PercentComplete") != "100" {
		t.Fatalf("%s %v", delta, err)
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
	updates := `<DeleteItemField><FieldURI FieldURI="task:Recurrence"/></DeleteItemField><DeleteItemField><FieldURI FieldURI="item:ReminderDueBy"/></DeleteItemField><DeleteItemField><FieldURI FieldURI="item:ReminderIsSet"/></DeleteItemField><SetItemField><FieldURI FieldURI="item:Subject"/><Item><Subject>Outlook title</Subject></Item></SetItemField><SetItemField><FieldURI FieldURI="item:Importance"/><Item><Importance>High</Importance></Item></SetItemField><SetItemField><FieldURI FieldURI="item:Sensitivity"/><Item><Sensitivity>Private</Sensitivity></Item></SetItemField><SetItemField><FieldURI FieldURI="task:Status"/><Task><Status>Completed</Status></Task></SetItemField><SetItemField><FieldURI FieldURI="task:CompleteDate"/><Task><CompleteDate>2026-10-13T14:13:00Z</CompleteDate></Task></SetItemField><SetItemField>` + extended + `<Task><ExtendedProperty>` + extended + `<Value>1</Value></ExtendedProperty></Task></SetItemField>`
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
	web := *saved
	web.EWSProperties = ""
	if err := store.SaveTask(ctx, user.ID, &web); err != nil {
		t.Fatal(err)
	}
	saved, _ = h.getTask(ctx, user, task.ID)
	if !strings.Contains(saved.EWSProperties, "34072") {
		t.Fatal("web update lost EWS properties")
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
