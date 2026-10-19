// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func taskSyncRequest(user, key, extra string) string {
	return `<Sync xmlns="AirSync:"><Collections><Collection><Class>Tasks</Class><SyncKey>` + key + `</SyncKey><CollectionId>` + systemFolderID(user, "tasks") + `</CollectionId>` + extra + `</Collection></Collections></Sync>`
}
func taskOp(kind, id, fields string) string {
	key := "ServerId"
	if kind == "Add" {
		key = "ClientId"
	}
	data := ""
	if kind != "Delete" {
		data = `<ApplicationData xmlns:t="Tasks:">` + fields + `</ApplicationData>`
	}
	return `<Commands><` + kind + `><` + key + `>` + id + `</` + key + `>` + data + `</` + kind + `></Commands>`
}
func taskResponse(t *testing.T, raw string) *protocolNode {
	t.Helper()
	doc, err := parseProtocolXML([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := doc.find("Collection")
	if c == nil || c.value("Status") != "1" {
		t.Fatal(raw)
	}
	return c
}

func TestActiveSyncTasksTwoDevicesAndTransport(t *testing.T) {
	for _, binary := range []bool{false, true} {
		t.Run(fmt.Sprint(binary), func(t *testing.T) {
			ctx, st, user, h, a, _ := syncFixture(t)
			b, err := st.EnsureFlowSyncDevice(ctx, user.ID, "second", "test")
			if err != nil {
				t.Fatal(err)
			}
			keys := map[string]string{a.ID: "0", b.ID: "0"}
			call := func(dev *storage.FlowSyncDevice, extra string) *protocolNode {
				t.Helper()
				payload := []byte(taskSyncRequest(user.ID, keys[dev.ID], extra))
				if binary {
					payload, err = encodeWire(string(payload))
					if err != nil {
						t.Fatal(err)
					}
				}
				r := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId="+dev.DeviceID, bytes.NewReader(payload)).WithContext(withUser(ctx, user))
				r.Header.Set("MS-ASProtocolVersion", "14.1")
				if binary {
					r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
				} else {
					r.Header.Set("Content-Type", "text/xml")
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != 200 {
					t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
				}
				payload = w.Body.Bytes()
				if binary {
					payload, err = decodeWire(payload)
					if err != nil {
						t.Fatal(err)
					}
				}
				c := taskResponse(t, string(payload))
				keys[dev.ID] = c.value("SyncKey")
				return c
			}
			for _, d := range []*storage.FlowSyncDevice{a, b} {
				if call(d, "").find("ApplicationData") != nil {
					t.Fatal("handshake sent data")
				}
			}
			folders, _, err := h.folderSync(ctx, user, a, false)
			if err != nil {
				t.Fatal(err)
			}
			folderDoc, _ := parseProtocolXML([]byte(folders))
			found := false
			for _, f := range folderDoc.find("Changes").Children {
				if f.value("ServerId") == systemFolderID(user.ID, "tasks") {
					found = f.value("Type") == "7" && f.value("DisplayName") == "Tasks"
				}
			}
			if !found {
				t.Fatal("default Tasks folder missing")
			}
			fields := `<t:Subject>Мобильная задача</t:Subject><t:Complete>0</t:Complete><t:StartDate>2026-10-19T09:00:00.000Z</t:StartDate><t:UtcStartDate>2026-10-19T06:00:00.000Z</t:UtcStartDate><t:DueDate>2026-10-20T18:00:00.000Z</t:DueDate><t:UtcDueDate>2026-10-20T15:00:00.000Z</t:UtcDueDate><t:Importance>2</t:Importance><t:Sensitivity>2</t:Sensitivity><t:Categories><t:Category>Работа</t:Category></t:Categories><t:ReminderSet>1</t:ReminderSet><t:ReminderTime>2026-10-20T14:00:00.000Z</t:ReminderTime><Body xmlns="AirSyncBase:"><Type>2</Type><Data>&lt;p&gt;Описание&lt;/p&gt;</Data></Body>`
			created := call(a, `<GetChanges>0</GetChanges>`+taskOp("Add", "phone-1", fields))
			ack := created.find("Responses").child("Add")
			if ack.value("Status") != "1" {
				t.Fatal(created.render())
			}
			id := ack.value("ServerId")
			if call(a, "").find("Commands") != nil {
				t.Fatal("own Add echoed as new task")
			}
			received := call(b, `<Options><BodyPreference xmlns="AirSyncBase:"><Type>2</Type></BodyPreference></Options>`)
			add := received.find("Commands").child("Add")
			if add.value("ServerId") != id {
				t.Fatal(received.render())
			}
			data := add.child("ApplicationData")
			if data.value("DueDate") != "2026-10-20T18:00:00.000Z" || data.value("UtcDueDate") != "2026-10-20T15:00:00.000Z" || data.find("Body").value("Data") != "<p>Описание</p>" || data.value("ReminderSet") != "1" {
				t.Fatal(data.render())
			}
			saved, err := (&ewsHandler{store: st}).getTask(ctx, user, id)
			if err != nil {
				t.Fatal(err)
			}
			_ = saved.SetStatus(storage.TaskWaitingOnOthers)
			_ = saved.SetPercentComplete(37.5)
			if err = st.SaveTask(ctx, user.ID, saved); err != nil {
				t.Fatal(err)
			}
			call(b, taskOp("Change", id, `<t:Subject>Правка</t:Subject><t:Complete>0</t:Complete>`))
			saved, _ = (&ewsHandler{store: st}).getTask(ctx, user, id)
			if saved.Status != storage.TaskWaitingOnOthers || saved.PercentComplete != 37.5 {
				t.Fatal("mobile edit erased progress", saved)
			}
			completed := call(b, taskOp("Change", id, `<t:Complete>1</t:Complete>`))
			if completed.find("Responses").child("Change").value("Status") != "1" {
				t.Fatal(completed.render())
			}
			delta := call(a, "").find("Commands")
			if delta == nil || delta.child("Change") == nil || delta.find("DateCompleted") == nil {
				t.Fatal("missing completion delta")
			}
			saved, _ = (&ewsHandler{store: st}).getTask(ctx, user, id)
			if saved.Status != storage.TaskCompleted || saved.PercentComplete != 100 || saved.CompletedAt == "" {
				t.Fatal(saved)
			}
			if out := taskXML(*saved, user.ID); strings.Contains(out, "_eas:") || strings.Contains(out, `[&quot;`) {
				t.Fatal("private metadata leaked into EWS", out)
			}
			call(a, taskOp("Change", id, `<t:Complete>0</t:Complete><t:DueDate/><t:UtcDueDate/><t:Categories/><Body xmlns="AirSyncBase:"><Type>1</Type><Data/></Body>`))
			saved, _ = (&ewsHandler{store: st}).getTask(ctx, user, id)
			if saved.Completed || saved.PercentComplete != 0 || saved.Body != "" || saved.Due != "" || saved.CompletedAt != "" {
				t.Fatal(saved)
			}
			call(b, "")
			call(a, taskOp("Delete", id, ""))
			removed := call(b, "")
			if removed.find("Commands") == nil || removed.find("Commands").child("Delete") == nil {
				t.Fatal(removed.render())
			}
			if tasks, e := st.ListTasks(ctx, user.ID); e != nil || len(tasks) != 0 {
				t.Fatal(tasks, e)
			}
		})
	}
}

type failTaskStateStore struct {
	storage.Driver
	collection string
	fail       bool
}

func (s *failTaskStateStore) PutFlowSyncState(ctx context.Context, u, d, c, v string) error {
	if s.fail && c == s.collection {
		s.fail = false
		return errors.New("injected lost sync-state write")
	}
	return s.Driver.PutFlowSyncState(ctx, u, d, c, v)
}

func TestActiveSyncTaskAddSurvivesLostResponseAndDeletion(t *testing.T) {
	ctx, st, u, h, d, _ := syncFixture(t)
	initial, err := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, "0", "")))
	if err != nil {
		t.Fatal(err)
	}
	key := taskResponse(t, initial).value("SyncKey")
	failure := &failTaskStateStore{Driver: st, collection: systemFolderID(u.ID, "tasks"), fail: true}
	h.store = failure
	request := taskSyncRequest(u.ID, key, taskOp("Add", "retry", `<t:Subject>Одна задача</t:Subject><t:Complete>0</t:Complete>`))
	if _, err = h.syncCollections(ctx, u, d, []byte(request)); err == nil {
		t.Fatal("failure injection missed")
	}
	tasks, err := st.ListTasks(ctx, u.ID)
	if err != nil || len(tasks) != 1 {
		t.Fatal(tasks, err)
	}
	id := tasks[0].ID
	// Reconstruct the handler after committing the task, before committing its
	// response. The durable ClientId receipt must outlive this failure window.
	h = &easHandler{store: st, ms: h.ms}
	out, err := h.syncCollections(ctx, u, d, []byte(request))
	if err != nil {
		t.Fatal(err)
	}
	c := taskResponse(t, out)
	if c.find("Responses").child("Add").value("ServerId") != id || c.child("Commands") != nil {
		t.Fatal(out)
	}
	if again, e := h.syncCollections(ctx, u, d, []byte(request)); e != nil || again != out {
		t.Fatal("response replay changed", e)
	}
	if err = st.DeleteTask(ctx, u.ID, id); err != nil {
		t.Fatal(err)
	}
	// A delayed Add retry with a newer SyncKey must not resurrect the task.
	out, err = h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, c.value("SyncKey"), taskOp("Add", "retry", `<t:Subject>Stale</t:Subject><t:Complete>0</t:Complete>`))))
	if err != nil {
		t.Fatal(err)
	}
	c = taskResponse(t, out)
	if c.find("Commands") == nil || c.find("Commands").child("Delete") == nil {
		t.Fatal(out)
	}
	if tasks, e := st.ListTasks(ctx, u.ID); e != nil || len(tasks) != 0 {
		t.Fatal("deleted task resurrected", tasks, e)
	}
	// Same ClientId on another device is a different creation.
	other, err := st.EnsureFlowSyncDevice(ctx, u.ID, "other", "test")
	if err != nil {
		t.Fatal(err)
	}
	newID, err := st.CreateFlowSyncTask(ctx, u.ID, other.ID, "retry", &storage.Task{Title: "Other device"})
	if err != nil || newID == id {
		t.Fatal(newID, err)
	}
}

func TestActiveSyncTaskWindowsFilterFetchEstimateAndPing(t *testing.T) {
	ctx, st, u, h, d, _ := syncFixture(t)
	for i := 0; i < 3; i++ {
		task := storage.Task{Title: fmt.Sprint(i), Body: strings.Repeat("текст", 100)}
		if err := st.SaveTask(ctx, u.ID, &task); err != nil {
			t.Fatal(err)
		}
	}
	key := "0"
	call := func(extra string) *protocolNode {
		t.Helper()
		out, err := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, key, extra)))
		if err != nil {
			t.Fatal(err)
		}
		c := taskResponse(t, out)
		key = c.value("SyncKey")
		return c
	}
	call("")
	collection := systemFolderID(u.ID, "tasks")
	estimate := `<GetItemEstimate xmlns="GetItemEstimate:"><Collections><Collection><CollectionId>` + collection + `</CollectionId><SyncKey>` + key + `</SyncKey></Collection></Collections></GetItemEstimate>`
	out, _, err := h.itemEstimate(ctx, u, d, []byte(estimate), false)
	if err != nil || extractTag(out, "Estimate") != "3" {
		t.Fatal(out, err)
	}
	options := `<WindowSize>1</WindowSize><Options><FilterType>8</FilterType><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>5</TruncationSize></BodyPreference></Options>`
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		c := call(options)
		commands := c.child("Commands")
		if commands == nil || len(commands.Children) != 1 {
			t.Fatal(c.render())
		}
		id := commands.Children[0].value("ServerId")
		if seen[id] {
			t.Fatal("duplicate window item")
		}
		seen[id] = true
		if (c.child("MoreAvailable") != nil) != (i < 2) {
			t.Fatal(c.render())
		}
		if c.find("Body").value("Truncated") != "1" {
			t.Fatal(c.render())
		}
	}
	if call("").child("Commands") != nil {
		t.Fatal("idle sync repeated items")
	}
	var id string
	for id = range seen {
		break
	}
	oldKey := key
	c := call(`<GetChanges>0</GetChanges><Options><BodyPreference xmlns="AirSyncBase:"><Type>1</Type></BodyPreference></Options><Commands><Fetch><ServerId>` + id + `</ServerId></Fetch></Commands>`)
	if key != oldKey || c.find("Body").value("Truncated") != "0" || c.find("Body").value("Data") != strings.Repeat("текст", 100) {
		t.Fatal(c.render())
	}
	// Completing on the web wakes Ping and hides only the completed task.
	task, err := (&ewsHandler{store: st}).getTask(ctx, u, id)
	if err != nil {
		t.Fatal(err)
	}
	_ = task.SetStatus(storage.TaskCompleted)
	if err = st.SaveTask(ctx, u.ID, task); err != nil {
		t.Fatal(err)
	}
	ping, err := h.ping(ctx, u, d, []byte(`<Ping xmlns="Ping:"><HeartbeatInterval>60</HeartbeatInterval><Folders><Folder><Id>`+collection+`</Id><Class>Tasks</Class></Folder></Folders></Ping>`))
	if err != nil || extractTag(ping, "Status") != "2" {
		t.Fatal(ping, err)
	}
	c = call(options)
	if c.find("SoftDelete") == nil || c.find("SoftDelete").value("ServerId") != id {
		t.Fatal(c.render())
	}
	if _, err = (&ewsHandler{store: st}).getTask(ctx, u, id); err != nil {
		t.Fatal("filter deleted server task", err)
	}
	_ = task.SetStatus(storage.TaskNotStarted)
	if err = st.SaveTask(ctx, u.ID, task); err != nil {
		t.Fatal(err)
	}
	c = call(options)
	if c.find("Commands") == nil || c.find("Commands").child("Add") == nil {
		t.Fatal("reopened task not added back", c.render())
	}
}

func TestActiveSyncTaskValidationAndIsolation(t *testing.T) {
	ctx, st, u, h, d, _ := syncFixture(t)
	other, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	foreign := storage.Task{Title: "Private"}
	if err = st.SaveTask(ctx, other.ID, &foreign); err != nil {
		t.Fatal(err)
	}
	initial, err := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, "0", "")))
	if err != nil {
		t.Fatal(err)
	}
	key := taskResponse(t, initial).value("SyncKey")
	for i, fields := range []string{
		`<t:Subject>Bad</t:Subject><t:Complete>2</t:Complete>`,
		`<t:Complete>0</t:Complete><t:Recurrence><t:Type>1</t:Type></t:Recurrence>`,
		`<t:Complete>0</t:Complete><t:UtcDueDate>tomorrow</t:UtcDueDate>`,
		`<t:Complete>0</t:Complete><t:DueDate>2026-10-20T12:00:00Z</t:DueDate>`,
		`<t:Complete>0</t:Complete><t:DateCompleted>2026-10-20T12:00:00Z</t:DateCompleted>`,
		`<t:Complete>0</t:Complete><t:ReminderSet>1</t:ReminderSet>`,
		`<t:Complete>0</t:Complete><t:Complete>1</t:Complete>`,
		`<t:Complete>0</t:Complete><Subject xmlns="Email:">Wrong namespace</Subject>`,
		`<t:Complete>0</t:Complete><Body xmlns="AirSyncBase:"><Type>1</Type><Truncated>1</Truncated><Data>Part</Data></Body>`,
	} {
		out, e := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, key, taskOp("Add", fmt.Sprint(i), fields))))
		if e != nil {
			t.Fatal(e)
		}
		c := taskResponse(t, out)
		key = c.value("SyncKey")
		if c.find("Responses").child("Add").value("Status") == "1" {
			t.Fatal("accepted invalid task", out)
		}
	}
	if tasks, e := st.ListTasks(ctx, u.ID); e != nil || len(tasks) != 0 {
		t.Fatal("partial writes", tasks, e)
	}
	for _, kind := range []string{"Change", "Delete", "Fetch"} {
		commands := taskOp(kind, foreign.ID, `<t:Subject>Overwrite</t:Subject>`)
		if kind == "Fetch" {
			commands = `<Commands><Fetch><ServerId>` + foreign.ID + `</ServerId></Fetch></Commands>`
		}
		out, e := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, key, commands)))
		if e != nil {
			t.Fatal(e)
		}
		c := taskResponse(t, out)
		key = c.value("SyncKey")
		if c.find("Responses").child(kind).value("Status") == "1" {
			t.Fatal("foreign access", out)
		}
	}
	out, err := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(other.ID, "0", "")))
	if err != nil || extractTag(out, "Status") != "8" {
		t.Fatal(out, err)
	}
	out, err = h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, "stale", taskOp("Add", "stale", `<t:Complete>0</t:Complete>`))))
	if err != nil || extractTag(out, "Status") != "3" {
		t.Fatal(out, err)
	}
	if tasks, e := st.ListTasks(ctx, other.ID); e != nil || len(tasks) != 1 || tasks[0].Title != "Private" {
		t.Fatal(tasks, e)
	}
	// Binary assignments checked independently of encode/decode round trips.
	raw := []byte{3, 1, 106, 0, 0, 9, 0x60, 3, 'x', 0, 1}
	decoded, err := decodeWire(raw)
	if err != nil || string(decoded) != `<Subject xmlns="Tasks:">x</Subject>` {
		t.Fatal(string(decoded), err)
	}
	encoded, err := encodeWire(string(decoded))
	if err != nil || !bytes.Equal(encoded, raw) {
		t.Fatalf("%x %v", encoded, err)
	}
}

func TestActiveSyncTaskBodyTailAndItemFetch(t *testing.T) {
	ctx, st, u, h, d, _ := syncFixture(t)
	task := storage.Task{Title: "Long task", Body: strings.Repeat("a", 100) + "old"}
	if err := st.SaveTask(ctx, u.ID, &task); err != nil {
		t.Fatal(err)
	}
	key := "0"
	call := func(extra string) *protocolNode {
		t.Helper()
		out, e := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, key, extra)))
		if e != nil {
			t.Fatal(e)
		}
		c := taskResponse(t, out)
		key = c.value("SyncKey")
		return c
	}
	options := `<Options><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>10</TruncationSize></BodyPreference></Options>`
	call(options)
	call("")
	task.Body = strings.Repeat("a", 100) + "new"
	if err := st.SaveTask(ctx, u.ID, &task); err != nil {
		t.Fatal(err)
	}
	c := call("")
	if c.child("Commands") == nil || c.child("Commands").child("Change") == nil {
		t.Fatal("equal-length tail edit lost", c.render())
	}
	if c.find("Body").value("Data") != strings.Repeat("a", 10) {
		t.Fatal(c.render())
	}
	fetch := `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><CollectionId xmlns="AirSync:">` + systemFolderID(u.ID, "tasks") + `</CollectionId><ServerId xmlns="AirSync:">` + task.ID + `</ServerId><Options><BodyPreference xmlns="AirSyncBase:"><Type>1</Type></BodyPreference></Options></Fetch></ItemOperations>`
	out, e := h.itemOperations(ctx, u, []byte(fetch))
	if e != nil {
		t.Fatal(e)
	}
	doc, e := parseProtocolXML([]byte(out))
	if e != nil || doc.find("Fetch").value("Status") != "1" || doc.find("Body").value("Data") != task.Body {
		t.Fatal(out, e)
	}
	if _, e = encodeWire(out); e != nil {
		t.Fatal(e)
	}
	foreign := strings.Replace(fetch, systemFolderID(u.ID, "tasks"), "foreign", 1)
	out, e = h.itemOperations(ctx, u, []byte(foreign))
	if e != nil {
		t.Fatal(e)
	}
	doc, _ = parseProtocolXML([]byte(out))
	if doc.find("Fetch").value("Status") != "6" {
		t.Fatal(out)
	}
	if call("").child("Commands") != nil {
		t.Fatal("Fetch restarted sync")
	}
}

func TestActiveSyncTaskAddRetryKeepsNewerEdit(t *testing.T) {
	ctx, st, u, h, d, _ := syncFixture(t)
	initial, e := h.syncCollections(ctx, u, d, []byte(taskSyncRequest(u.ID, "0", "")))
	if e != nil {
		t.Fatal(e)
	}
	key := taskResponse(t, initial).value("SyncKey")
	h.store = &failTaskStateStore{Driver: st, collection: systemFolderID(u.ID, "tasks"), fail: true}
	request := taskSyncRequest(u.ID, key, taskOp("Add", "old-add", `<t:Complete>0</t:Complete><t:Subject>Old</t:Subject>`))
	if _, e = h.syncCollections(ctx, u, d, []byte(request)); e == nil {
		t.Fatal("expected injected failure")
	}
	tasks, e := st.ListTasks(ctx, u.ID)
	if e != nil || len(tasks) != 1 {
		t.Fatal(tasks, e)
	}
	task := tasks[0]
	task.Title = "Edited elsewhere"
	if e = st.SaveTask(ctx, u.ID, &task); e != nil {
		t.Fatal(e)
	}
	h.store = st
	out, e := h.syncCollections(ctx, u, d, []byte(request))
	if e != nil {
		t.Fatal(e)
	}
	c := taskResponse(t, out)
	if c.find("Responses").child("Add").value("ServerId") != task.ID || c.find("Commands") == nil || c.find("Commands").child("Change") == nil || c.find("Commands").find("Subject").Text != "Edited elsewhere" {
		t.Fatal(out)
	}
}
