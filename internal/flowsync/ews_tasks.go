// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

func taskID(id string) bool {
	return strings.HasPrefix(id, "task:") || strings.HasPrefix(id, "mailtask:")
}
func taskXML(t storage.Task, user string) string {
	status, percent := "NotStarted", "0"
	if t.Completed {
		status, percent = "Completed", "100"
	}
	due := ""
	if t.Due != "" {
		due = "<t:DueDate>" + xmlEscape(t.Due) + "</t:DueDate>"
	}
	raw := fmt.Sprintf(`<t:Task><t:ItemId Id="%s" ChangeKey="%d"/><t:ParentFolderId Id="%s"/><t:ItemClass>IPM.Task</t:ItemClass><t:Subject>%s</t:Subject><t:Body BodyType="Text">%s</t:Body><t:Status>%s</t:Status><t:PercentComplete>%s</t:PercentComplete><t:IsComplete>%t</t:IsComplete>%s</t:Task>`, xmlEscape(t.ID), t.Revision, xmlEscape(systemFolderID(user, "tasks")), xmlEscape(t.Title), xmlEscape(t.Body), status, percent, t.Completed, due)
	props := map[string]string{}
	_ = json.Unmarshal([]byte(t.EWSProperties), &props)
	keys := []string{}
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var extra strings.Builder
	for _, k := range keys {
		extra.WriteString(props[k])
	}
	return strings.Replace(raw, "</t:Task>", extra.String()+"</t:Task>", 1)
}
func (h *ewsHandler) findTasks(ctx context.Context, u *storage.User) (string, error) {
	tasks, err := h.store.ListTasks(ctx, u.ID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(taskXML(t, u.ID))
	}
	return ewsNotificationSuccess("FindItem", fmt.Sprintf(`<m:RootFolder TotalItemsInView="%d" IncludesLastItemInRange="true"><t:Items>%s</t:Items></m:RootFolder>`, len(tasks), b.String())), nil
}
func (h *ewsHandler) getTask(ctx context.Context, u *storage.User, id string) (*storage.Task, error) {
	tasks, err := h.store.ListTasks(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, storage.ErrNotFound
}
func applyTaskField(t *storage.Task, field, value string) bool {
	switch field {
	case "item:Subject":
		t.Title = value
	case "item:Body":
		t.Body = value
	case "task:Status":
		if value != "NotStarted" && value != "Completed" {
			return false
		}
		t.Completed = value == "Completed"
	case "task:PercentComplete":
		if value != "0" && value != "100" {
			return false
		}
		t.Completed = value == "100"
	case "task:DueDate":
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return false
			}
		}
		t.Due = value
	default:
		return false
	}
	return true
}
func (h *ewsHandler) writeTask(ctx context.Context, u *storage.User, body string, create bool) (string, error) {
	t := &storage.Task{}
	if !create {
		var err error
		t, err = h.getTask(ctx, u, extractAttr(body, "ItemId", "Id"))
		if err != nil {
			return ewsFault("ErrorItemNotFound", "task unavailable"), nil
		}
		key := extractAttr(body, "ItemId", "ChangeKey")
		if key != "" && key != strconv.FormatInt(t.Revision, 10) {
			return ewsFault("ErrorChangeKeyRequiredForWriteOperations", "task changed; reload"), nil
		}
	}
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	if create {
		if folder := extractAttr(body, "FolderId", "Id"); folder != "" && systemFolderName(u, folder) != "tasks" {
			return ewsFault("ErrorFolderNotFound", "task folder unavailable"), nil
		}
		n := doc.find("Task")
		if n == nil {
			return ewsFault("ErrorInvalidRequest", "missing Task"), nil
		}
		for _, c := range n.Children {
			field := "task:" + c.Name.Local
			if c.Name.Local == "Subject" || c.Name.Local == "Body" {
				field = "item:" + c.Name.Local
			}
			if !applyTaskField(t, field, c.Text) {
				slog.Warn("flowsync unsupported task field", "field", field)
				return ewsFault("ErrorInvalidPropertySet", "unsupported task field: "+field), nil
			}
		}
	} else {
		updates := doc.find("Updates")
		if updates == nil {
			return ewsFault("ErrorInvalidRequest", "missing updates"), nil
		}
		props := map[string]string{}
		if t.EWSProperties != "" {
			if err := json.Unmarshal([]byte(t.EWSProperties), &props); err != nil {
				return "", err
			}
		}
		for _, up := range updates.Children {
			deleting := up.Name.Local == "DeleteItemField"
			if !deleting && up.Name.Local != "SetItemField" {
				return ewsFault("ErrorInvalidPropertySet", "unsupported task operation"), nil
			}
			field := nodeAttr(up.child("FieldURI"), "FieldURI")
			extended := up.child("ExtendedFieldURI")
			if extended != nil {
				key := extendedKey(extended)
				if nodeAttr(extended, "PropertyType") == "" || len(key) > 1024 {
					return ewsFault("ErrorInvalidPropertySet", "invalid task extended property"), nil
				}
				if deleting {
					delete(props, key)
					continue
				}
				n := up.find("ExtendedProperty")
				if n == nil || extendedKey(n.child("ExtendedFieldURI")) != key || len(n.render()) > 16384 {
					return ewsFault("ErrorInvalidPropertySet", "invalid task extended property"), nil
				}
				props[key] = metadataXML(n)
				continue
			}
			parts := strings.Split(field, ":")
			if len(parts) != 2 {
				return ewsFault("ErrorInvalidPropertySet", "missing task FieldURI"), nil
			}
			// Outlook clears optional fields even when this server never stored them.
			if deleting {
				switch field {
				case "item:ReminderDueBy", "item:ReminderIsSet", "item:ReminderMinutesBeforeStart", "task:BillingInformation", "task:Companies", "task:Contacts", "task:Mileage", "task:ActualWork", "task:TotalWork":
					delete(props, field)
					continue
				}
			}
			value := ""
			var n *protocolNode
			if !deleting {
				container := up.child("Task")
				if container == nil && parts[0] == "item" {
					container = up.child("Item")
				}
				if container == nil || len(container.Children) != 1 || container.Children[0].Name.Local != parts[1] {
					return ewsFault("ErrorInvalidPropertySet", "task value does not match "+field), nil
				}
				n = container.Children[0]
				value = n.Text
			}
			switch field {
			case "item:Importance", "item:Sensitivity", "item:Categories", "task:StartDate", "task:CompleteDate":
				if deleting {
					delete(props, field)
					continue
				}
				if field == "item:Importance" && value != "Low" && value != "Normal" && value != "High" {
					return ewsFault("ErrorInvalidPropertySet", "invalid importance"), nil
				}
				if field == "item:Sensitivity" && value != "Normal" && value != "Personal" && value != "Private" && value != "Confidential" {
					return ewsFault("ErrorInvalidPropertySet", "invalid sensitivity"), nil
				}
				if strings.HasPrefix(field, "task:") {
					if _, err := time.Parse(time.RFC3339, value); err != nil {
						return ewsFault("ErrorInvalidPropertySet", "invalid task date"), nil
					}
				}
				if len(n.render()) > 16384 {
					return ewsFault("ErrorInvalidPropertySet", "task property too large"), nil
				}
				props[field] = metadataXML(n)
			default:
				if !applyTaskField(t, field, value) {
					slog.Warn("flowsync unsupported task field", "field", field)
					return ewsFault("ErrorInvalidPropertySet", "unsupported task field: "+field), nil
				}
			}
		}
		encoded, err := json.Marshal(props)
		if err != nil {
			return "", err
		}
		t.EWSProperties = string(encoded)

	}
	if strings.TrimSpace(t.Title) == "" {
		t.Title = "Task"
	}
	if err = h.store.SaveTask(ctx, u.ID, t); err != nil {
		return "", err
	}
	op := "UpdateItem"
	if create {
		op = "CreateItem"
	}
	return ewsNotificationSuccess(op, "<m:Items>"+taskXML(*t, u.ID)+"</m:Items>"), nil
}

func (h *ewsHandler) taskFolderXML(ctx context.Context, u *storage.User, name string) (string, error) {
	tasks, err := h.store.ListTasks(ctx, u.ID)
	if err != nil {
		return "", err
	}
	raw := systemFolderXML(u, name)
	raw = strings.Replace(raw, "<t:TotalCount>0</t:TotalCount>", fmt.Sprintf("<t:TotalCount>%d</t:TotalCount>", len(tasks)), 1)
	if name == "tasks" {
		raw = strings.ReplaceAll(raw, "<t:CreateContents>false</t:CreateContents>", "<t:CreateContents>true</t:CreateContents>")
		raw = strings.ReplaceAll(raw, "<t:Modify>None</t:Modify>", "<t:Modify>Owned</t:Modify>")
	}
	return raw, nil
}
