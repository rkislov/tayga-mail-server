package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestTaskAssignmentAPI(t *testing.T) {
	f := newFileFixture(t)
	var task storage.Task
	decode := func(raw []byte) {
		t.Helper()
		if err := json.Unmarshal(raw, &task); err != nil {
			t.Fatal(err)
		}
	}
	decode(f.call(0, "POST", "/api/v1/tasks", fmt.Sprintf(`{"title":"Assigned","assignee_id":%q,"owner_id":%q,"can_update":true}`, f.users[1], f.users[2]), 200).Body.Bytes())
	if task.OwnerID != f.users[0] || task.AssignmentState != "pending" {
		t.Fatal(task)
	}
	path := "/api/v1/tasks/" + task.ID
	f.call(2, "GET", path, "", 404)
	f.call(2, "GET", path+"/history", "", 404)
	candidates := f.call(0, "GET", "/api/v1/tasks/assignees?q=reader&limit=1", "", 200)
	if !strings.Contains(candidates.Body.String(), f.users[1]) || strings.Contains(candidates.Body.String(), f.users[2]) {
		t.Fatal(candidates.Body.String())
	}
	f.call(1, "POST", path+"/assignment", fmt.Sprintf(`{"action":"accept","revision":%d}`, task.Revision), 200)
	decode(f.call(1, "GET", path, "", 200).Body.Bytes())
	if !task.CanUpdate || task.CanManage {
		t.Fatal(task)
	}
	update := task
	update.Title = "No permission"
	body, _ := json.Marshal(update)
	f.call(1, "PUT", path, string(body), 403)
	task.SetPercentComplete(60)
	body, _ = json.Marshal(task)
	decode(f.call(1, "PUT", path, string(body), 200).Body.Bytes())
	if task.PercentComplete != 60 {
		t.Fatal(task)
	}
	f.call(1, "DELETE", path, "", 404)
	// A forged generic PUT cannot transfer assignment or ownership.
	task.AssigneeID = f.users[2]
	task.OwnerID = f.users[2]
	task.AssignmentState = "accepted"
	body, _ = json.Marshal(task)
	decode(f.call(1, "PUT", path, string(body), 200).Body.Bytes())
	if task.AssigneeID != f.users[1] || task.OwnerID != f.users[0] {
		t.Fatal(task)
	}
	notices := f.call(0, "GET", "/api/v1/tasks/notifications", "", 200)
	if !strings.Contains(notices.Body.String(), `"unread":2`) {
		t.Fatal(notices.Body.String())
	}
	f.call(0, "POST", path+"/assignment", fmt.Sprintf(`{"action":"revoke","revision":%d}`, task.Revision), 200)
	f.call(1, "GET", path, "", 404)
	f.call(1, "GET", path+"/history", "", 404)
	f.call(0, "POST", path+"/assignment", `{"action":"invented"}`, 409)
	f.call(0, "GET", path+"/unknown", "", 405)
}
