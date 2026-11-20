// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

// These read-only, empty containers allow clients to discover optional Exchange
// folder categories. They do not imply support for tasks, journaling or public
// folder CRUD. IDs are scoped to the authenticated account, not shared globally.
var ewsSystemFolders = map[string]struct{ Name, Class, Kind string }{
	"tasks":               {"Tasks", "IPF.Task", "TasksFolder"},
	"journal":             {"Journal", "IPF.Journal", "Folder"},
	"searchfolders":       {"Search Folders", "IPF.Note", "Folder"},
	"publicfoldersroot":   {"Public Folders", "IPF.Note", "Folder"},
	"voicemail":           {"Voice Mail", "IPF.Note", "Folder"},
	"recipientcache":      {"Recipient Cache", "IPF.Contact", "ContactsFolder"},
	"conversationhistory": {"Conversation History", "IPF.Note", "Folder"},
	"mycontacts":          {"My Contacts", "IPF.Contact", "ContactsFolder"},
	"todosearch":          {"To-Do Search", "IPF.Task", "TasksFolder"},
}

func systemFolderID(user, name string) string { return "ews-system:" + user + ":" + name }
func systemFolderName(u *storage.User, id string) string {
	prefix := "ews-system:" + u.ID + ":"
	if !strings.HasPrefix(id, prefix) {
		return ""
	}
	name := strings.TrimPrefix(id, prefix)
	if _, ok := ewsSystemFolders[name]; ok {
		return name
	}
	return ""
}
func systemFolderXML(u *storage.User, name string) string {
	f, ok := ewsSystemFolders[name]
	if !ok {
		return ""
	}
	return fmt.Sprintf(`<t:%s><t:FolderId Id="%s" ChangeKey="1"/><t:ParentFolderId Id="%s"/><t:DisplayName>%s</t:DisplayName><t:FolderClass>%s</t:FolderClass><t:TotalCount>0</t:TotalCount><t:ChildFolderCount>0</t:ChildFolderCount><t:EffectiveRights><t:CreateAssociated>false</t:CreateAssociated><t:CreateContents>false</t:CreateContents><t:CreateHierarchy>false</t:CreateHierarchy><t:Delete>false</t:Delete><t:Modify>false</t:Modify><t:Read>true</t:Read></t:EffectiveRights></t:%s>`, f.Kind, xmlEscape(systemFolderID(u.ID, name)), xmlEscape("root:"+u.ID), f.Name, f.Class, f.Kind)
}
func emptyEWSItems() string {
	return `<m:FindItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder TotalItemsInView="0" IncludesLastItemInRange="true"><t:Items/></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`
}
func (h *ewsHandler) findRequestedFolder(ctx context.Context, u *storage.User, body string) (string, error) {
	id, err := h.resolveEWSFolder(ctx, u, body)
	if err != nil {
		return ewsOperationError("FindFolder", "ErrorFolderNotFound", "folder unavailable"), nil
	}
	if id == galFolderID(u) {
		if err := h.readableEWSCollection(ctx, u, id); err != nil {
			return ewsOperationError("FindFolder", "ErrorAccessDenied", "Directory unavailable"), nil
		}
	}
	if systemFolderName(u, id) != "" || id == galFolderID(u) {
		return `<m:FindFolderResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:FindFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder TotalItemsInView="0" IncludesLastItemInRange="true"><t:Folders/></m:RootFolder></m:FindFolderResponseMessage></m:ResponseMessages></m:FindFolderResponse>`, nil
	}
	if id == "root:"+u.ID {
		return h.findFolder(ctx, u)
	}
	listing, e := h.findFolder(ctx, u)
	if e != nil {
		return "", e
	}
	doc, e := parseProtocolXML([]byte(listing))
	if e != nil {
		return "", e
	}
	visible := doc.find("Folders")
	parents := map[string]string{}
	found := false
	for _, f := range visible.Children {
		key := nodeAttr(f.child("FolderId"), "Id")
		parents[key] = nodeAttr(f.child("ParentFolderId"), "Id")
		if key == id {
			found = true
		}
	}
	if !found {
		return ewsOperationError("FindFolder", "ErrorAccessDenied", "Folder unavailable"), nil
	}
	var out strings.Builder
	for _, f := range visible.Children {
		key := nodeAttr(f.child("FolderId"), "Id")
		parent := parents[key]
		deep := extractAttr(body, "FindFolder", "Traversal") == "Deep"
		for steps := 0; parent != "" && steps < len(parents); steps++ {
			if parent == id {
				out.WriteString(f.render())
				break
			}
			if !deep {
				break
			}
			parent = parents[parent]
		}
	}
	return ewsNotificationSuccess("FindFolder", `<m:RootFolder IncludesLastItemInRange="true"><t:Folders>`+out.String()+`</t:Folders></m:RootFolder>`), nil
}
