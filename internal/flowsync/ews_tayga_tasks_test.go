package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strconv"
	"strings"
	"testing"
)

func TestTaygaEWSTasksRoundTripAndReplay(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	raw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:tb-task\r\nSUMMARY:Тестовая задача\r\nDESCRIPTION:Описание\r\nSTATUS:IN-PROCESS\r\nPERCENT-COMPLETE:25\r\nDUE;VALUE=DATE:20261103\r\nDTSTART:20261101T090000Z\r\nPRIORITY:2\r\nCATEGORIES:Работа,Планы\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Напоминание\r\nEND:VALARM\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	createID := "cb340024-321a-4850-921d-1bb397e0f9a4"
	create := `<m:CreateItem><m:SavedItemFolderId><t:FolderId Id="` + systemFolderID(u.ID, "tasks") + `"/></m:SavedItemFolderId><m:Items><t:Task>` + taygaProperty("TaygaVTodo", raw) + taygaProperty("TaygaCreateId", createID) + `</t:Task></m:Items></m:CreateItem>`
	call := func(body, code string) *protocolNode {
		t.Helper()
		r := calendarSOAP(t, h, ctx, u, body)
		if r.find("ResponseCode").Text != code {
			t.Fatal(r.render())
		}
		return r
	}
	r := call(create, "NoError")
	id := extractAttr(r.render(), "ItemId", "Id")
	if !strings.HasPrefix(id, "task:") {
		t.Fatal(id)
	}
	original, err := st.GetTask(ctx, u.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	if original.Title != "Тестовая задача" || original.PercentComplete != 25 || original.Due != "2026-11-03T00:00:00Z" {
		t.Fatal(original)
	}
	if strings.Contains(taskXML(*original, u.ID), "BEGIN:VTODO") {
		t.Fatal("private sync resource leaked to ordinary task XML")
	}
	get := func(id string) string {
		return `<m:GetItem><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape><t:AdditionalProperties>` + taygaProperty("TaygaVTodo", "") + `</t:AdditionalProperties></m:ItemShape><m:ItemIds><t:ItemId Id="` + id + `"/></m:ItemIds></m:GetItem>`
	}
	r = call(get("task-create:"+createID), "NoError")
	for _, want := range []string{"BEGIN:VALARM", "DUE;VALUE=DATE:20261103", "PRIORITY:2", "CATEGORIES:Работа,Планы"} {
		if !strings.Contains(r.render(), want) {
			t.Fatalf("missing %s: %s", want, r.render())
		}
	}
	original.Title = "Из веба"
	original.SetStatus(storage.TaskCompleted)
	if err = st.SaveTask(ctx, u.ID, original); err != nil {
		t.Fatal(err)
	}
	r = call(create, "NoError") // Lost create response must not overwrite the web change.
	if !strings.Contains(r.render(), "Из веба") || !strings.Contains(r.render(), "STATUS:COMPLETED") {
		t.Fatal(r.render())
	}
	tasks, _ := st.ListTasks(ctx, u.ID)
	if len(tasks) != 1 {
		t.Fatal(tasks)
	}
	update := func(key, data string) string {
		return `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="` + id + `" ChangeKey="` + key + `"/><t:Updates><t:SetItemField><t:Task>` + data + `</t:Task></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
	}
	call(update("1", taygaProperty("TaygaVTodo", raw)), "ErrorIrresolvableConflict")
	key := strconv.FormatInt(original.Revision, 10)
	call(update(key, taygaProperty("TaygaVTodo", strings.Replace(raw, "UID:tb-task", "UID:different", 1))), "ErrorInvalidPropertySet")
	call(update(key, taygaProperty("TaygaVTodo", strings.Replace(raw, "END:VTODO", "RRULE:FREQ=DAILY\r\nEND:VTODO", 1))), "ErrorInvalidPropertySet")
	r = call(update(key, taygaProperty("TaygaVTodo", raw)), "NoError")
	key = extractAttr(r.render(), "ItemId", "ChangeKey")
	other := *u
	other.ID = "other"
	denied := calendarSOAP(t, h, ctx, &other, get(id))
	if !strings.Contains(denied.render(), "ErrorItemNotFound") {
		t.Fatal(denied.render())
	}
	call(update("1", taygaProperty("TaygaDelete", "true")), "ErrorIrresolvableConflict")
	call(update(key, taygaProperty("TaygaDelete", "true")), "NoError")
	call(create, "ErrorItemNotFound") // A create retry must not resurrect a deletion.
	call(get("task-create:"+createID), "ErrorItemNotFound")
	tasks, _ = st.ListTasks(ctx, u.ID)
	if len(tasks) != 0 {
		t.Fatal(tasks)
	}
}
func TestTaygaTaskMetadataReflectsOtherClients(t *testing.T) {
	task := &storage.Task{ID: "task:1", Title: "Task", Status: storage.TaskWaitingOnOthers, PercentComplete: 42.5, Due: "2026-11-03T09:00:00Z"}
	raw, err := taskTodo(task)
	if err != nil {
		t.Fatal(err)
	}
	if err = applyTodoTask(task, raw); err != nil {
		t.Fatal(err)
	}
	if task.Status != storage.TaskWaitingOnOthers || task.PercentComplete != 42.5 {
		t.Fatal(task)
	}
	task.Title = "Updated"
	task.SetStatus(storage.TaskCompleted)
	task.CompletedAt = "2026-10-30T10:00:00Z"
	raw, err = taskTodo(task)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"SUMMARY:Updated", "COMPLETED:20261030T100000Z", "PERCENT-COMPLETE:100"} {
		if !strings.Contains(raw, v) {
			t.Fatal(raw)
		}
	}
}

func TestTaygaTaskAssigneePermissions(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	member, err := st.CreateUser(ctx, &storage.User{TenantID: owner.TenantID, DomainID: owner.DomainID, Email: "task-member@example.com", LocalPart: "task-member", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	task := &storage.Task{Title: "Assigned", AssigneeID: member.ID}
	if err = st.SaveTask(ctx, owner.ID, task); err != nil {
		t.Fatal(err)
	}
	h := &ewsHandler{store: st, ms: eas.ms}
	edit := func(transform func(string) string, code string) {
		t.Helper()
		current, e := st.GetTask(ctx, member.ID, task.ID)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := taskTodo(current)
		if e != nil {
			t.Fatal(e)
		}
		body := `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="` + task.ID + `" ChangeKey="` + strconv.FormatInt(current.Revision, 10) + `"/><t:Updates><t:SetItemField><t:Task>` + taygaProperty("TaygaVTodo", transform(raw)) + `</t:Task></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
		r := calendarSOAP(t, h, ctx, member, body)
		if r.find("ResponseCode").Text != code {
			t.Fatal(r.render())
		}
	}
	progress := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "STATUS:NEEDS-ACTION", "STATUS:IN-PROCESS"), "PERCENT-COMPLETE:0", "PERCENT-COMPLETE:40")
	}
	edit(progress, "ErrorAccessDenied")
	if _, err = st.AssignTask(ctx, member.ID, task.ID, "accept", "", task.Revision); err != nil {
		t.Fatal(err)
	}
	edit(progress, "NoError")
	edit(func(s string) string { return strings.Replace(s, "SUMMARY:Assigned", "SUMMARY:Not allowed", 1) }, "ErrorAccessDenied")
	current, _ := st.GetTask(ctx, owner.ID, task.ID)
	if current.PercentComplete != 40 || current.Title != "Assigned" {
		t.Fatal(current)
	}
	if err = st.DeleteTaskVersion(ctx, member.ID, task.ID, current.Revision); err != storage.ErrUnauthorized {
		t.Fatal(err)
	}
}
