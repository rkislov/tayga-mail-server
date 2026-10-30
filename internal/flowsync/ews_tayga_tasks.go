// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/tayga/tms/internal/storage"
	"github.com/teambition/rrule-go"
)

const taskTodoProperty = "_tayga:VTodo"

// decodeTaskTodo validates a complete series without expanding an unbounded rule.
// Detached instances belong to the same UID and keep their original recurrence ID.
func decodeTaskTodo(raw string) (*ical.Calendar, *ical.Component, error) {
	c, err := ical.NewDecoder(strings.NewReader(raw)).Decode()
	if err != nil {
		return nil, nil, err
	}
	var master *ical.Component
	var todos []*ical.Component
	for _, child := range c.Children {
		if child.Name == "VTIMEZONE" {
			continue
		}
		if child.Name != "VTODO" || len(todos) >= 1001 {
			return nil, nil, errors.New("expected one task series, at most 1000 exceptions")
		}
		todos = append(todos, child)
		if child.Props.Get("RECURRENCE-ID") == nil {
			if master != nil {
				return nil, nil, errors.New("multiple task masters")
			}
			master = child
		}
	}
	if master == nil {
		return nil, nil, errors.New("missing task master")
	}
	uid, _ := master.Props.Text("UID")
	if uid == "" {
		return nil, nil, errors.New("missing task UID")
	}
	recurring := taskTodoRecurring(c)
	start := master.Props.Get("DTSTART")
	if recurring && start == nil {
		return nil, nil, errors.New("recurring task requires DTSTART")
	}
	seen := map[string]bool{}
	for _, todo := range todos {
		id, _ := todo.Props.Text("UID")
		if id != uid {
			return nil, nil, errors.New("mixed task UIDs")
		}
		for _, name := range []string{"ATTENDEE", "ORGANIZER"} {
			if todo.Props.Get(name) != nil {
				return nil, nil, fmt.Errorf("task property %s is not supported; local task retained", name)
			}
		}
		for _, name := range []string{"DTSTART", "DUE", "COMPLETED", "RECURRENCE-ID"} {
			if _, err := todoTime(todo, name); err != nil {
				return nil, nil, err
			}
		}
		if p := todo.Props.Get("DURATION"); p != nil {
			if todo.Props.Get("DUE") != nil || todo.Props.Get("DTSTART") == nil {
				return nil, nil, errors.New("task DURATION requires DTSTART and excludes DUE")
			}
			d, err := p.Duration()
			if err != nil || d <= 0 {
				return nil, nil, errors.New("invalid task duration")
			}
		}
		if rid := todo.Props.Get("RECURRENCE-ID"); rid != nil {
			if rid.Params.Get("RANGE") != "" {
				return nil, nil, errors.New("task RANGE exceptions are not supported")
			}
			key, err := taskDateKey(*rid, start)
			if err != nil {
				return nil, nil, err
			}
			if seen[key] {
				return nil, nil, errors.New("duplicate task recurrence ID")
			}
			seen[key] = true
			for _, name := range []string{"RRULE", "RDATE", "EXDATE"} {
				if todo.Props.Get(name) != nil {
					return nil, nil, errors.New("exception cannot define a recurrence rule")
				}
			}
		}
		if p := todo.Props.Get("RRULE"); p != nil {
			if len(todo.Props.Values("RRULE")) != 1 {
				return nil, nil, errors.New("multiple task recurrence rules")
			}
			opt, err := todo.Props.RecurrenceRule()
			if err != nil {
				return nil, nil, err
			}
			if opt.Freq < rrule.YEARLY || opt.Freq > rrule.DAILY || opt.Interval < 0 || opt.Count < 0 || opt.Count > 10000 || len(opt.Byhour)+len(opt.Byminute)+len(opt.Bysecond) > 0 {
				return nil, nil, errors.New("unsupported dense task recurrence")
			}
			if opt.Count > 0 && !opt.Until.IsZero() {
				return nil, nil, errors.New("task recurrence cannot combine COUNT and UNTIL")
			}
			opt.Dtstart, err = start.DateTime(time.UTC)
			if err != nil {
				return nil, nil, err
			}
			if _, err = rrule.NewRRule(*opt); err != nil {
				return nil, nil, err
			}
		}
		for _, name := range []string{"RDATE", "EXDATE"} {
			for _, p := range todo.Props.Values(name) {
				for _, value := range strings.Split(p.Value, ",") {
					copy := p
					copy.Value = value
					if _, err := taskDateKey(copy, start); err != nil {
						return nil, nil, err
					}
				}
			}
		}
		status, _ := todo.Props.Text("STATUS")
		if status != "" && status != "NEEDS-ACTION" && status != "IN-PROCESS" && status != "COMPLETED" && !(todo != master && status == "CANCELLED") {
			return nil, nil, errors.New("invalid task status")
		}
		if p := todo.Props.Get("PERCENT-COMPLETE"); p != nil {
			n, err := p.Int()
			if err != nil || n < 0 || n > 100 {
				return nil, nil, errors.New("invalid task percentage")
			}
		}
	}
	return c, master, nil
}
func taskDateKey(p ical.Prop, start *ical.Prop) (string, error) {
	if start == nil {
		return "", errors.New("task recurrence requires DTSTART")
	}
	date := p.Params.Get("VALUE") == "DATE" || len(p.Value) == 8
	startDate := start.Params.Get("VALUE") == "DATE" || len(start.Value) == 8
	if date != startDate {
		return "", errors.New("task recurrence value type mismatch")
	}
	t, err := p.DateTime(time.UTC)
	if err != nil {
		return "", err
	}
	if date {
		return t.Format("2006-01-02"), nil
	}
	return t.UTC().Format(time.RFC3339), nil
}
func taskTodoRecurring(c *ical.Calendar) bool {
	for _, todo := range c.Children {
		if todo.Name == "VTODO" {
			for _, name := range []string{"RRULE", "RDATE", "EXDATE", "RECURRENCE-ID"} {
				if todo.Props.Get(name) != nil {
					return true
				}
			}
		}
	}
	return false
}
func encodeTaskTodo(c *ical.Calendar) (string, error) {
	if c.Props.Get("PRODID") == nil {
		c.Props.SetText("PRODID", "-//Tayga//Tasks//EN")
	}
	if c.Props.Get("VERSION") == nil {
		c.Props.SetText("VERSION", "2.0")
	}
	for _, v := range c.Children {
		if v.Name == "VTODO" && v.Props.Get("DTSTAMP") == nil {
			v.Props.SetDateTime("DTSTAMP", time.Unix(0, 0).UTC())
		}
	}
	var b strings.Builder
	err := ical.NewEncoder(&b).Encode(c)
	return b.String(), err
}
func setTodoNumber(todo *ical.Component, name, value string) {
	p := ical.NewProp(name)
	p.Value = value
	todo.Props.Set(p)
}

func todoState(status string) string {
	switch status {
	case storage.TaskCompleted:
		return "COMPLETED"
	case storage.TaskInProgress, storage.TaskWaitingOnOthers:
		return "IN-PROCESS"
	default:
		return "NEEDS-ACTION"
	}
}
func todoTime(todo *ical.Component, name string) (string, error) {
	p := todo.Props.Get(name)
	if p == nil {
		return "", nil
	}
	v, err := p.DateTime(time.UTC)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", name, err)
	}
	return v.UTC().Format(time.RFC3339), nil
}
func setTodoTime(todo *ical.Component, name, value string) {
	if value == "" {
		delete(todo.Props, name)
		return
	}
	v, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return
	}
	old, err := todoTime(todo, name)
	if err == nil && old != "" {
		t, _ := time.Parse(time.RFC3339, old)
		if t.Equal(v) {
			return
		} // Preserve DATE, floating dates and original TZID.
	}
	todo.Props.SetDateTime(name, v.UTC())
}
func taskTodo(t *storage.Task) (string, error) {
	props := map[string]string{}
	if t.EWSProperties != "" {
		if err := json.Unmarshal([]byte(t.EWSProperties), &props); err != nil {
			return "", err
		}
	}
	var c *ical.Calendar
	var todo *ical.Component
	if raw := props[taskTodoProperty]; raw != "" {
		var err error
		c, todo, err = decodeTaskTodo(raw)
		if err != nil {
			return "", err
		}
	} else {
		c = ical.NewCalendar()
		c.Props.SetText("PRODID", "-//Tayga//Tasks//EN")
		c.Props.SetText("VERSION", "2.0")
		todo = ical.NewComponent("VTODO")
		todo.Props.SetText("UID", t.ID)
		c.Children = append(c.Children, todo)
	}
	// Series are edited as complete resources. Other write paths are guarded in storage.
	if taskTodoRecurring(c) {
		return encodeTaskTodo(c)
	}
	todo.Props.SetText("SUMMARY", t.Title)
	todo.Props.SetText("DESCRIPTION", t.Body)
	todo.Props.SetText("STATUS", todoState(t.Status))
	setTodoNumber(todo, "PERCENT-COMPLETE", strconv.FormatFloat(t.PercentComplete, 'f', 0, 64))
	todo.Props.SetText("X-TAYGA-TASK-STATUS", t.Status)
	todo.Props.SetText("X-TAYGA-TASK-PERCENT", strconv.FormatFloat(t.PercentComplete, 'f', -1, 64))
	setTodoTime(todo, "DUE", t.Due)
	completed := ""
	if t.Completed {
		completed = t.CompletedAt
		if v := props["task:CompleteDate"]; v != "" {
			completed = extractTag(v, "CompleteDate")
		}
	}
	setTodoTime(todo, "COMPLETED", completed)
	setTodoTime(todo, "DTSTART", extractTag(props["task:StartDate"], "StartDate"))
	// Canonical EWS metadata wins when Outlook or another client edits it.
	importance := extractTag(props["item:Importance"], "Importance")
	priority := "0"
	switch importance {
	case "High":
		priority = "1"
	case "Normal":
		priority = "5"
	case "Low":
		priority = "9"
	}
	if p := todo.Props.Get("PRIORITY"); p != nil {
		n, _ := strconv.Atoi(p.Value)
		if (importance == "High" && n >= 1 && n <= 4) || (importance == "Normal" && n == 5) || (importance == "Low" && n >= 6 && n <= 9) {
			priority = p.Value
		}
	}
	setTodoNumber(todo, "PRIORITY", priority)
	class := map[string]string{"Normal": "PUBLIC", "Personal": "PRIVATE", "Private": "PRIVATE", "Confidential": "CONFIDENTIAL"}[extractTag(props["item:Sensitivity"], "Sensitivity")]
	if class == "" {
		delete(todo.Props, "CLASS")
	} else {
		todo.Props.SetText("CLASS", class)
	}
	if raw := props["item:Categories"]; raw != "" {
		doc, err := parseProtocolXML([]byte(raw))
		if err != nil {
			return "", err
		}
		var values []string
		categories := doc.find("Categories")
		if categories == nil {
			return "", errors.New("invalid task categories")
		}
		for _, p := range categories.Children {
			values = append(values, p.Text)
		}
		p := ical.NewProp("CATEGORIES")
		p.SetTextList(values)
		todo.Props.Set(p)
	} else {
		delete(todo.Props, "CATEGORIES")
	}
	return encodeTaskTodo(c)
}
func taskWithTodoXML(t *storage.Task, user, body string) (string, error) {
	raw := taskXML(*t, user)
	if !taygaRequested(body, "TaygaVTodo") {
		return raw, nil
	}
	data, err := taskTodo(t)
	if err != nil {
		return "", err
	}
	return strings.Replace(raw, "</t:Task>", taygaProperty("TaygaVTodo", data)+"</t:Task>", 1), nil
}
func applyTodoTask(t *storage.Task, raw string) error {
	c, todo, err := decodeTaskTodo(raw)
	if err != nil {
		return err
	}
	title, err := todo.Props.Text("SUMMARY")
	if err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		title = "Task"
	}
	body, err := todo.Props.Text("DESCRIPTION")
	if err != nil {
		return err
	}
	due, err := todoTime(todo, "DUE")
	if err != nil {
		return err
	}
	if due == "" && todo.Props.Get("DURATION") != nil {
		start, e := todo.Props.Get("DTSTART").DateTime(time.UTC)
		if e != nil {
			return e
		}
		d, e := todo.Props.Get("DURATION").Duration()
		if e != nil {
			return e
		}
		due = start.Add(d).UTC().Format(time.RFC3339)
	}
	status, _ := todo.Props.Text("STATUS")
	state := storage.TaskNotStarted
	switch status {
	case "", "NEEDS-ACTION":
	case "IN-PROCESS":
		state = storage.TaskInProgress
	case "COMPLETED":
		state = storage.TaskCompleted
	default:
		return fmt.Errorf("unsupported task status %s", status)
	}
	preserved, _ := todo.Props.Text("X-TAYGA-TASK-STATUS")
	if (preserved == storage.TaskWaitingOnOthers || preserved == storage.TaskDeferred) && todoState(preserved) == status {
		state = preserved
	}
	percent := float64(0)
	if p := todo.Props.Get("PERCENT-COMPLETE"); p != nil {
		percent, err = strconv.ParseFloat(p.Value, 64)
		if err != nil {
			return err
		}
	}
	if p := todo.Props.Get("X-TAYGA-TASK-PERCENT"); p != nil {
		original, e := strconv.ParseFloat(p.Value, 64)
		if e == nil && strconv.FormatFloat(original, 'f', 0, 64) == strconv.FormatFloat(percent, 'f', -1, 64) {
			percent = original
		}
	}
	if err = t.SetPercentComplete(percent); err != nil {
		return err
	}
	if err = t.SetStatus(state); err != nil {
		return err
	}
	t.Title, t.Body, t.Due = title, body, due
	props := map[string]string{}
	if t.EWSProperties != "" {
		if err = json.Unmarshal([]byte(t.EWSProperties), &props); err != nil {
			return err
		}
	}
	for name, field := range map[string]string{"DTSTART": "task:StartDate", "COMPLETED": "task:CompleteDate"} {
		value, e := todoTime(todo, name)
		if e != nil {
			return e
		}
		delete(props, field)
		if value != "" && (name != "COMPLETED" || t.Completed) {
			tag := strings.Split(field, ":")[1]
			props[field] = "<t:" + tag + ">" + value + "</t:" + tag + ">"
		}
	}
	delete(props, "item:Importance")
	if p := todo.Props.Get("PRIORITY"); p != nil {
		n, e := strconv.Atoi(p.Value)
		if e != nil || n < 0 || n > 9 {
			return errors.New("invalid task priority")
		}
		value := "Normal"
		if n > 0 && n < 5 {
			value = "High"
		}
		if n > 5 {
			value = "Low"
		}
		if n > 0 {
			props["item:Importance"] = "<t:Importance>" + value + "</t:Importance>"
		}
	}
	delete(props, "item:Sensitivity")
	if p := todo.Props.Get("CLASS"); p != nil {
		value := map[string]string{"PUBLIC": "Normal", "PRIVATE": "Private", "CONFIDENTIAL": "Confidential"}[p.Value]
		if value == "" {
			return errors.New("invalid task class")
		}
		props["item:Sensitivity"] = "<t:Sensitivity>" + value + "</t:Sensitivity>"
	}
	delete(props, "item:Categories")
	if p := todo.Props.Get("CATEGORIES"); p != nil {
		values, e := p.TextList()
		if e != nil {
			return e
		}
		var b strings.Builder
		b.WriteString("<t:Categories>")
		for _, v := range values {
			b.WriteString("<t:String>" + xmlEscape(v) + "</t:String>")
		}
		b.WriteString("</t:Categories>")
		props["item:Categories"] = b.String()
	}
	for _, k := range []string{"X-TAYGA-TASK-STATUS", "X-TAYGA-TASK-PERCENT"} {
		delete(todo.Props, k)
	}
	delete(props, "_tayga:Recurring")
	if taskTodoRecurring(c) {
		props["_tayga:Recurring"] = "1"
	}
	t.Recurring = taskTodoRecurring(c)
	t.FullTaskResource = true
	normalized, err := encodeTaskTodo(c)
	if err != nil {
		return err
	}
	props[taskTodoProperty] = normalized
	encoded, err := json.Marshal(props)
	t.EWSProperties = string(encoded)
	return err
}

func (h *ewsHandler) taygaTaskWrite(ctx context.Context, u *storage.User, body string, create bool) (string, bool, error) {
	props := taygaProperties(body)
	raw, hasTodo := props["TaygaVTodo"]
	id, key := extractAttr(body, "ItemId", "Id"), extractAttr(body, "ItemId", "ChangeKey")
	remove := props["TaygaDelete"] == "true"
	if !hasTodo && !(remove && taskID(id)) {
		return "", false, nil
	}
	op := "UpdateItem"
	if create {
		op = "CreateItem"
	}
	fail := func(code, msg string) (string, bool, error) { return ewsOperationError(op, code, msg), true, nil }
	if len(raw) > 2<<20 {
		return fail("ErrorDataSizeLimitExceeded", "Task exceeds 2 MiB")
	}
	t := &storage.Task{}
	if create {
		if systemFolderName(u, extractAttr(body, "FolderId", "Id")) != "tasks" {
			return fail("ErrorAccessDenied", "Task folder unavailable")
		}
		if _, err := uuid.Parse(props["TaygaCreateId"]); err != nil {
			return fail("ErrorInvalidIdMalformed", "Invalid create ID")
		}
	} else {
		var err error
		t, err = h.getTask(ctx, u, id)
		if err != nil {
			return fail("ErrorItemNotFound", "Task unavailable")
		}
		if key == "" || key != strconv.FormatInt(t.Revision, 10) {
			return fail("ErrorIrresolvableConflict", "Task changed; reload")
		}
		if remove {
			err = h.store.DeleteTaskVersion(ctx, u.ID, id, t.Revision)
			if err == storage.ErrUnauthorized {
				return fail("ErrorAccessDenied", "Only the author can delete this task")
			}
			if err == storage.ErrTaskConflict || err == storage.ErrNotFound {
				return fail("ErrorIrresolvableConflict", "Task changed; reload")
			}
			return ewsNotificationSuccess(op, ""), true, err
		}
		old, err := taskTodo(t)
		if err != nil {
			return "", true, err
		}
		_, a, e1 := decodeTaskTodo(old)
		_, b, e2 := decodeTaskTodo(raw)
		if e1 != nil || e2 != nil {
			return fail("ErrorInvalidPropertySet", "Invalid task resource")
		}
		ua, _ := a.Props.Text("UID")
		ub, _ := b.Props.Text("UID")
		if ua != ub {
			return fail("ErrorInvalidPropertySet", "Task UID cannot change")
		}
	}
	previous := *t
	if err := applyTodoTask(t, raw); err != nil {
		return fail("ErrorInvalidPropertySet", err.Error())
	}
	if !create && !previous.CanManage {
		// Assigned users may change only progress. Compare complete normalized
		// resources with progress/volatile properties removed before preserving the
		// author's metadata for the storage-level permission check.
		old, e1 := taskTodo(&previous)
		fresh, e2 := taskTodo(t)
		strip := func(s string) string {
			c, v, e := decodeTaskTodo(s)
			if e != nil {
				return "!"
			}
			for _, k := range []string{"STATUS", "PERCENT-COMPLETE", "COMPLETED", "X-TAYGA-TASK-STATUS", "X-TAYGA-TASK-PERCENT", "DTSTAMP", "LAST-MODIFIED", "CREATED", "X-MOZ-GENERATION"} {
				delete(v.Props, k)
			}
			out, _ := encodeTaskTodo(c)
			return out
		}
		if e1 != nil || e2 != nil || strip(old) != strip(fresh) {
			return fail("ErrorAccessDenied", "Only task progress may be changed by the assignee")
		}
		t.EWSProperties = previous.EWSProperties
	}
	var err error
	if create {
		id, err = h.store.CreateEWSTask(ctx, u.ID, props["TaygaCreateId"], t)
		if err == nil {
			t, err = h.getTask(ctx, u, id)
		}
	} else {
		err = h.store.SaveTask(ctx, u.ID, t)
	}
	if err == storage.ErrTaskSeries {
		return fail("ErrorInvalidPropertySet", err.Error())
	}
	if err == storage.ErrUnauthorized {
		return fail("ErrorAccessDenied", "Task modification is not allowed")
	}
	if err == storage.ErrNotFound {
		return fail("ErrorItemNotFound", "Task was deleted or changed; local data retained")
	}
	if err != nil {
		return "", true, err
	}
	item, err := taskWithTodoXML(t, u.ID, body)
	return ewsNotificationSuccess(op, "<m:Items>"+item+"</m:Items>"), true, err
}
