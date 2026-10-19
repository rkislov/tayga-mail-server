package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestTaskProgressAPI(t *testing.T) {
	f := newFileFixture(t)
	f.call(-1, "GET", "/api/v1/tasks", "", 401)
	w := f.call(0, "POST", "/api/v1/tasks", `{"title":"Plan","status":"WaitingOnOthers","percent_complete":42.5}`, 200)
	var task storage.Task
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Status != storage.TaskWaitingOnOthers || task.PercentComplete != 42.5 || task.Completed {
		t.Fatal(task)
	}
	path := "/api/v1/tasks/" + task.ID
	read := func() storage.Task {
		t.Helper()
		var list struct {
			Tasks []storage.Task `json:"tasks"`
		}
		if err := json.Unmarshal(f.call(0, "GET", "/api/v1/tasks", "", 200).Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Tasks) != 1 {
			t.Fatal(list)
		}
		return list.Tasks[0]
	}
	for _, body := range []string{
		`{"title":"Broken","status":"unknown"}`,
		`{"title":"Broken","status":"InProgress","percent_complete":101}`,
		`{"title":"Broken","status":"InProgress","percent_complete":-1}`,
		`{"title":"Broken","status":"InProgress","percent_complete":"bad"}`,
	} {
		f.call(0, "PUT", path, body, 400)
	}
	if got := read(); got.Revision != task.Revision || got.Title != "Plan" {
		t.Fatal(got)
	}
	legacy := fmt.Sprintf(`{"title":"Renamed","completed":false,"revision":%d}`, task.Revision)
	f.call(0, "PUT", path, legacy, 200)
	task = read()
	if task.Status != storage.TaskWaitingOnOthers || task.PercentComplete != 42.5 {
		t.Fatal(task)
	}
	f.call(0, "PUT", path, legacy, 409)
	body := fmt.Sprintf(`{"title":"Completed","status":"Completed","revision":%d}`, task.Revision)
	f.call(1, "PUT", path, body, 409)
	if strings.Contains(f.call(1, "GET", "/api/v1/tasks", "", 200).Body.String(), task.ID) {
		t.Fatal("foreign task visible")
	}
	f.call(0, "PUT", path, body, 200)
	task = read()
	if task.Status != storage.TaskCompleted || task.PercentComplete != 100 || !task.Completed {
		t.Fatal(task)
	}
	// Old cached code includes the new fields in GET/spread/PUT, but only
	// changes the completed checkbox. It must still reopen the task.
	task.Completed = false
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", path, string(encoded), 200)
	task = read()
	if task.Status != storage.TaskNotStarted || task.PercentComplete != 0 || task.Completed {
		t.Fatal(task)
	}
}
