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
	if id != "root:"+u.ID {
		if err = h.ensureMailboxOwned(ctx, u.ID, id); err != nil {
			return ewsOperationError("FindFolder", "ErrorAccessDenied", "folder unavailable"), nil
		}
	}
	return h.findFolder(ctx, u)
}
