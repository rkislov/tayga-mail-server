package flowsync

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestSharedTaskEWSAndActiveSync(t *testing.T) {
	ctx, st, owner, h, _, _ := syncFixture(t)
	recipient, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, Email: "recipient@example.com", LocalPart: "recipient", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	dev, err := st.EnsureFlowSyncDevice(ctx, recipient.ID, "recipient-phone", "test")
	if err != nil {
		t.Fatal(err)
	}
	task := storage.Task{Title: "Shared", Body: "Description", AssigneeID: recipient.ID}
	if err = st.SaveTask(ctx, owner.ID, &task); err != nil {
		t.Fatal(err)
	}
	ews := &ewsHandler{store: st, ms: h.ms}
	syncKey := "0"
	sync := func(extra string) *protocolNode {
		t.Helper()
		out, e := h.syncCollections(ctx, recipient, dev, []byte(taskSyncRequest(recipient.ID, syncKey, extra)))
		if e != nil {
			t.Fatal(e)
		}
		c := taskResponse(t, out)
		syncKey = c.value("SyncKey")
		return c
	}
	update := func(field, value string) string {
		t.Helper()
		out, e := ews.updateItem(ctx, recipient, `<UpdateItem><ItemChanges><ItemChange><ItemId Id="`+task.ID+`"/><Updates><SetItemField><FieldURI FieldURI="`+field+`"/><Task><`+strings.Split(field, ":")[1]+`>`+value+`</`+strings.Split(field, ":")[1]+`></Task></SetItemField></Updates></ItemChange></ItemChanges></UpdateItem>`)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	sync("")
	if got := sync(""); got.find("Add") == nil || got.find("Subject").Text != "Shared" {
		t.Fatal(got.render())
	}
	// Neither protocol can bypass acceptance.
	if got := update("task:PercentComplete", "25"); !strings.Contains(got, "ErrorAccessDenied") {
		t.Fatal(got)
	}
	got := sync(taskOp("Change", task.ID, `<t:Complete>1</t:Complete>`))
	if got.find("Responses").child("Change").value("Status") == "1" {
		t.Fatal(got.render())
	}
	_, err = st.AssignTask(ctx, recipient.ID, task.ID, "accept", "", task.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := update("item:Subject", "Forbidden rename"); !strings.Contains(got, "ErrorAccessDenied") {
		t.Fatal(got)
	}
	if got := update("task:PercentComplete", "40"); !strings.Contains(got, "NoError") {
		t.Fatal(got)
	}
	got = sync(taskOp("Change", task.ID, `<t:Complete>1</t:Complete><t:DateCompleted>2026-10-19T10:00:00.000Z</t:DateCompleted>`))
	if got.find("Responses").child("Change").value("Status") != "1" {
		t.Fatal(got.render())
	}
	saved, err := st.GetTask(ctx, owner.ID, task.ID)
	if err != nil || !saved.Completed || saved.PercentComplete != 100 {
		t.Fatal(saved, err)
	}
	notices, _, _, err := st.TaskNotifications(ctx, owner.ID, 25, 0)
	if err != nil || len(notices) != 3 || notices[0].Action != "complete" {
		t.Fatal(notices, err)
	}
	if got, e := ews.deleteItem(ctx, recipient, fmt.Sprintf(`<DeleteItem><ItemIds><ItemId Id="%s"/></ItemIds></DeleteItem>`, task.ID)); e != nil || !strings.Contains(got, "ErrorAccessDenied") {
		t.Fatal(got, e)
	}
	// Revocation causes a normal synchronization tombstone, not a folder reset.
	if _, err = st.AssignTask(ctx, owner.ID, task.ID, "revoke", "", saved.Revision); err != nil {
		t.Fatal(err)
	}
	got = sync("")
	if got.find("Delete") == nil || got.find("Delete").value("ServerId") != task.ID {
		t.Fatal(got.render())
	}
	if got := update("task:PercentComplete", "30"); !strings.Contains(got, "ErrorItemNotFound") {
		t.Fatal(got)
	}
	if saved, err = st.GetTask(ctx, owner.ID, task.ID); err != nil || !saved.Completed {
		t.Fatal(saved, err)
	}
}

func TestAssignedFlaggedMailTaskKeepsAuthorMailboxPrivate(t *testing.T) {
	ctx, st, owner, h, _, _ := syncFixture(t)
	ews := &ewsHandler{store: st, ms: h.ms}
	if err := ews.ensureStandardMailFolders(ctx, owner); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, owner.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: "task-test", InternalDate: time.Now(), Subject: "Mail follow-up"})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.UpdateMessageFlags(ctx, msg.ID, `\Seen \Flagged`); err != nil {
		t.Fatal(err)
	}
	recipient, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, Email: "worker@example.com", LocalPart: "worker", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListTasks(ctx, owner.ID)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	task, err := st.AssignTask(ctx, owner.ID, rows[0].ID, "assign", recipient.ID, rows[0].Revision)
	if err != nil {
		t.Fatal(err)
	}
	task, err = st.AssignTask(ctx, recipient.ID, task.ID, "accept", "", task.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if task.SourceMessageID != "" {
		t.Fatal("author message reference leaked")
	}
	if err = task.SetStatus(storage.TaskCompleted); err != nil {
		t.Fatal(err)
	}
	if err = st.SaveTask(ctx, recipient.ID, task); err != nil {
		t.Fatal(err)
	}
	msg, err = st.GetMessageByID(ctx, msg.ID)
	if err != nil || storage.HasFlag(msg.Flags, `\Flagged`) || !storage.HasFlag(msg.Flags, `\Seen`) {
		t.Fatal(msg, err)
	}
	// Reflagging from mail reopens the shared task and notifies the assignee.
	if err = st.UpdateMessageFlags(ctx, msg.ID, `\Seen \Flagged`); err != nil {
		t.Fatal(err)
	}
	task, err = st.GetTask(ctx, recipient.ID, task.ID)
	if err != nil || task.Completed {
		t.Fatal(task, err)
	}
	notices, _, _, err := st.TaskNotifications(ctx, recipient.ID, 25, 0)
	if err != nil || notices[0].Action != "reopen" {
		t.Fatal(notices, err)
	}
	history, _, err := st.TaskHistory(ctx, owner.ID, task.ID, 25, 0)
	if err != nil || history[0].Action != "reopen" {
		t.Fatal(history, err)
	}
}
