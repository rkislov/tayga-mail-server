// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

type taskDeviceKey struct{}

var errInvalidEASTask = errors.New("invalid or unsupported task fields")

func acknowledgeTaskAdds(writes []writebackResult, state *collectionState) {
	if state.Items == nil {
		state.Items = map[string]string{}
	}
	for _, w := range writes {
		if w.Kind == "add" && w.Status == 1 {
			// Creations have revision 1. A later edit (including one made between
			// committing the Add and building its response) must still be a Change.
			state.Items[w.ServerID] = taskSnapshotHash(storage.Task{Revision: 1}, state.itemPreferences())
		}
	}
}

func taskMetadata(props map[string]string, key string) *protocolNode {
	doc, err := parseProtocolXML([]byte(props[key]))
	if err != nil {
		return nil
	}
	return doc.Children[0]
}
func taskMetadataText(props map[string]string, key string) string {
	if n := taskMetadata(props, key); n != nil {
		return n.Text
	}
	return ""
}
func setTaskMetadata(props map[string]string, key, name, value string) {
	if value == "" {
		delete(props, key)
		return
	}
	props[key] = metadataXML(&protocolNode{Name: xml.Name{Local: name}, Text: value})
}
func easTaskDate(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", errInvalidEASTask
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z"), nil
}

// Local wall-clock dates are paired with their UTC instant. The anchor prevents
// an edit through EWS/web from reusing a stale local date from another client.
func taskLocalDate(props map[string]string, name, utc string) string {
	var pair [2]string
	if json.Unmarshal([]byte(props["_eas:"+name]), &pair) == nil && pair[1] == utc {
		return pair[0]
	}
	return utc
}

func taskApplicationData(t storage.Task, prefs syncPreferences) string {
	props := map[string]string{}
	_ = json.Unmarshal([]byte(t.EWSProperties), &props)
	var b strings.Builder
	b.WriteString(`<ApplicationData xmlns="AirSync:">`)
	field := func(name, value string) { fmt.Fprintf(&b, `<%s xmlns="Tasks:">%s</%s>`, name, xmlEscape(value), name) }
	field("Subject", t.Title)
	field("Complete", strconv.Itoa(bool01(t.Completed)))
	if t.Completed {
		value := taskMetadataText(props, "task:CompleteDate")
		if value == "" {
			value = t.CompletedAt
		}
		date, _ := easTaskDate(value)
		field("DateCompleted", date)
	}
	for _, date := range []struct{ name, value string }{{"StartDate", taskMetadataText(props, "task:StartDate")}, {"DueDate", t.Due}} {
		utc, err := easTaskDate(date.value)
		if err == nil && utc != "" {
			field(date.name, taskLocalDate(props, date.name, utc))
			field("Utc"+date.name, utc)
		}
	}
	importance := map[string]string{"Low": "0", "Normal": "1", "High": "2"}[taskMetadataText(props, "item:Importance")]
	if importance == "" {
		importance = "1"
	}
	field("Importance", importance)
	sensitivity := map[string]string{"Normal": "0", "Personal": "1", "Private": "2", "Confidential": "3"}[taskMetadataText(props, "item:Sensitivity")]
	if sensitivity == "" {
		sensitivity = "0"
	}
	field("Sensitivity", sensitivity)
	if categories := taskMetadata(props, "item:Categories"); categories != nil {
		b.WriteString(`<Categories xmlns="Tasks:">`)
		for _, category := range categories.Children {
			field("Category", category.Text)
		}
		b.WriteString(`</Categories>`)
	}
	reminder := "0"
	if taskMetadataText(props, "item:ReminderIsSet") == "true" {
		reminder = "1"
	}
	field("ReminderSet", reminder)
	if date, _ := easTaskDate(taskMetadataText(props, "item:ReminderDueBy")); date != "" {
		field("ReminderTime", date)
	}
	for _, name := range []string{"OrdinalDate", "SubOrdinalDate"} {
		if v := props["_eas:"+name]; v != "" {
			field(name, v)
		}
	}
	content := messageContent{Text: t.Body}
	var html [2]string
	if json.Unmarshal([]byte(props["_eas:body"]), &html) == nil && html[0] == t.Body {
		content.HTML = html[1]
	}
	body, kind, size, truncated := preferredBody(content, prefs)
	fmt.Fprintf(&b, `<Body xmlns="AirSyncBase:"><Type>%s</Type><EstimatedDataSize>%d</EstimatedDataSize><Truncated>%d</Truncated><Data>%s</Data></Body>`, kind, size, bool01(truncated), xmlEscape(body))
	b.WriteString(`</ApplicationData>`)
	return b.String()
}

func (h *easHandler) syncTasks(ctx context.Context, u *storage.User, collectionID, next string, binary bool, wb []writebackResult) (string, []byte, error) {
	full, _, err := h.taskSnapshot(ctx, u, collectionID, next, wb)
	if err != nil {
		return "", nil, err
	}
	if binary {
		raw, err := encodeWire(full)
		return "", raw, err
	}
	return full, nil, nil
}

func taskSnapshotHash(t storage.Task, prefs syncPreferences) string {
	// Include the stored revision, not merely a truncated body preview: an
	// equal-length edit beyond TruncationSize must still reach other devices.
	return "task-v1:" + digest(fmt.Sprintf("%d:%s", t.Revision, prefs.hash()))
}

func (h *easHandler) taskSnapshot(ctx context.Context, u *storage.User, collectionID, next string, wb []writebackResult) (string, map[string]string, error) {
	if collectionID != systemFolderID(u.ID, "tasks") {
		return "", nil, storage.ErrNotFound
	}
	tasks, err := h.store.ListTasks(ctx, u.ID)
	if err != nil {
		return "", nil, err
	}
	prefs := contextPreferences(ctx)
	hashes := map[string]string{}
	var b strings.Builder
	fmt.Fprintf(&b, `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>%s</SyncKey><CollectionId>%s</CollectionId><Status>1</Status>`, xmlEscape(next), xmlEscape(collectionID))
	b.WriteString(renderWritebackResponses(wb))
	b.WriteString(`<Commands>`)
	for _, task := range tasks {
		if prefs.Filter == 8 && task.Completed {
			continue
		}
		hashes[task.ID] = taskSnapshotHash(task, prefs)
		fmt.Fprintf(&b, `<Add><ServerId>%s</ServerId>%s</Add>`, xmlEscape(task.ID), taskApplicationData(task, prefs))
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String(), hashes, nil
}

func (h *easHandler) wbTask(ctx context.Context, u *storage.User, collectionID string, op clientOp) (string, error) {
	if collectionID != systemFolderID(u.ID, "tasks") {
		return op.ServerID, storage.ErrNotFound
	}
	if op.Kind == "delete" {
		return op.ServerID, h.store.DeleteTask(ctx, u.ID, op.ServerID)
	}
	t := &storage.Task{}
	if op.Kind == "change" {
		var err error
		t, err = (&ewsHandler{store: h.store}).getTask(ctx, u, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
	} else if op.Kind != "add" || op.ClientID == "" || op.ServerID != "" {
		return op.ServerID, errInvalidEASTask
	}
	if err := applyEASTask(t, op.ApplicationData); err != nil {
		return op.ServerID, err
	}
	if op.Kind == "add" {
		device, _ := ctx.Value(taskDeviceKey{}).(string)
		return h.store.CreateFlowSyncTask(ctx, u.ID, device, op.ClientID, t)
	}
	return t.ID, h.store.SaveTask(ctx, u.ID, t)
}

// Validate the entire operation before saving. No partial success or silently
// dropped recurrence/unknown fields; empty elements intentionally clear values.
func applyEASTask(t *storage.Task, data *protocolNode) error {
	bad := func() error { return errInvalidEASTask }
	if data == nil || len(data.Children) == 0 {
		return bad()
	}
	props := map[string]string{}
	if t.EWSProperties != "" {
		if err := json.Unmarshal([]byte(t.EWSProperties), &props); err != nil {
			return err
		}
	}
	fields := map[string]*protocolNode{}
	for _, n := range data.Children {
		if fields[n.Name.Local] != nil {
			return bad()
		}
		fields[n.Name.Local] = n
		if n.Name.Local == "Body" && n.Name.Space == "AirSyncBase:" {
			seen := map[string]bool{}
			for _, c := range n.Children {
				if c.Name.Space != "AirSyncBase:" || seen[c.Name.Local] || len(c.Children) != 0 {
					return bad()
				}
				seen[c.Name.Local] = true
				switch c.Name.Local {
				case "Type", "Data", "EstimatedDataSize", "Truncated":
				default:
					return bad()
				}
			}
			if n.value("Truncated") != "" && n.value("Truncated") != "0" {
				return bad()
			}
			if n.child("Data") == nil {
				return bad()
			}
			switch n.value("Type") {
			case "1":
				t.Body = n.value("Data")
				delete(props, "_eas:body")
			case "2":
				t.Body = mailsearch.ExtractHTMLText(n.value("Data"))
				raw, _ := json.Marshal([2]string{t.Body, n.value("Data")})
				props["_eas:body"] = string(raw)
			default:
				return bad()
			}
			continue
		}
		if n.Name.Space != "Tasks:" {
			return bad()
		}
		if n.Name.Local != "Categories" && len(n.Children) != 0 {
			return bad()
		}
		switch n.Name.Local {
		case "Subject":
			t.Title = n.Text
		case "Body":
			t.Body = n.Text
			delete(props, "_eas:body")
		case "Complete", "ReminderSet":
			if n.Text != "0" && n.Text != "1" {
				return bad()
			}
		case "StartDate", "UtcStartDate", "DueDate", "UtcDueDate", "DateCompleted", "ReminderTime", "OrdinalDate":
			if _, err := easTaskDate(n.Text); err != nil {
				return err
			}
		case "Importance", "Sensitivity":
			values := []string{"Low", "Normal", "High"}
			if n.Name.Local == "Sensitivity" {
				values = []string{"Normal", "Personal", "Private", "Confidential"}
			}
			i, err := strconv.Atoi(n.Text)
			if err != nil || i < 0 || i >= len(values) {
				return bad()
			}
			setTaskMetadata(props, "item:"+n.Name.Local, n.Name.Local, values[i])
		case "Categories":
			categories := &protocolNode{Name: xml.Name{Local: "Categories"}}
			for _, c := range n.Children {
				if c.Name.Space != "Tasks:" || c.Name.Local != "Category" || len(c.Children) != 0 {
					return bad()
				}
				categories.Children = append(categories.Children, &protocolNode{Name: xml.Name{Local: "String"}, Text: c.Text})
			}
			props["item:Categories"] = metadataXML(categories)
		case "SubOrdinalDate":
			if len(n.Text) > 1024 {
				return bad()
			}
		default:
			return bad()
		}
	}
	if n := fields["Complete"]; n != nil {
		if n.Text == "1" {
			_ = t.SetStatus(storage.TaskCompleted)
		} else if t.Completed {
			_ = t.SetStatus(storage.TaskNotStarted)
		}
	}
	if t.ID == "" && fields["Complete"] == nil {
		return bad()
	}
	if n := fields["DateCompleted"]; n != nil {
		if n.Text != "" && !t.Completed {
			return bad()
		}
		v, _ := easTaskDate(n.Text)
		setTaskMetadata(props, "task:CompleteDate", "CompleteDate", v)
	}
	if !t.Completed {
		delete(props, "task:CompleteDate")
	}
	for _, name := range []string{"StartDate", "DueDate"} {
		local, utc := fields[name], fields["Utc"+name]
		if local == nil && utc == nil {
			continue
		}
		if local == nil || utc == nil || (local.Text == "") != (utc.Text == "") {
			return bad()
		}
		l, _ := easTaskDate(local.Text)
		v, _ := easTaskDate(utc.Text)
		pair, _ := json.Marshal([2]string{l, v})
		props["_eas:"+name] = string(pair)
		if name == "DueDate" {
			t.Due = v
		} else {
			setTaskMetadata(props, "task:StartDate", "StartDate", v)
		}
	}
	if n := fields["ReminderSet"]; n != nil {
		setTaskMetadata(props, "item:ReminderIsSet", "ReminderIsSet", strconv.FormatBool(n.Text == "1"))
	}
	if n := fields["ReminderTime"]; n != nil {
		v, _ := easTaskDate(n.Text)
		setTaskMetadata(props, "item:ReminderDueBy", "ReminderDueBy", v)
	}
	if taskMetadataText(props, "item:ReminderIsSet") == "true" && taskMetadataText(props, "item:ReminderDueBy") == "" {
		return bad()
	}
	for _, name := range []string{"OrdinalDate", "SubOrdinalDate"} {
		if n := fields[name]; n != nil {
			props["_eas:"+name] = n.Text
		}
	}
	raw, err := json.Marshal(props)
	t.EWSProperties = string(raw)
	return err
}
