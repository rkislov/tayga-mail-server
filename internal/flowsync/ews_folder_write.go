// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

func folderWriteMessage(op, code, message, payload string) string {
	class := "Success"
	if code != "NoError" {
		class = "Error"
	}
	return fmt.Sprintf(`<m:%sResponseMessage ResponseClass="%s"><m:MessageText>%s</m:MessageText><m:ResponseCode>%s</m:ResponseCode>%s</m:%sResponseMessage>`, op, class, xmlEscape(message), code, payload, op)
}

func folderWritePayload(mb *storage.Mailbox) string {
	return fmt.Sprintf(`<m:Folders><t:Folder><t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>%s</t:DisplayName></t:Folder></m:Folders>`, xmlEscape(mb.ID), xmlEscape(mb.Name[strings.LastIndex(mb.Name, "/")+1:]))
}

func (h *ewsHandler) mailFolderForWrite(ctx context.Context, u *storage.User, ref *protocolNode) (*storage.Mailbox, error) {
	if ref == nil {
		return nil, storage.ErrNotFound
	}
	id, err := h.resolveEWSFolder(ctx, u, ref.render())
	if err != nil {
		return nil, err
	}
	return (&easHandler{store: h.store}).mailboxByID(ctx, u.ID, id)
}

// EWS folder mutations are restricted to the authenticated user's mail folders.
func (h *ewsHandler) writeEWSFolders(ctx context.Context, u *storage.User, op, body string) (string, error) {
	h.syncMu.Lock()
	defer h.syncMu.Unlock()
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	root := doc.find(op)
	if root == nil {
		return ewsOperationError(op, "ErrorInvalidRequest", "Missing operation"), nil
	}
	if email := root.findValue("EmailAddress"); email != "" && !strings.EqualFold(email, u.Email) {
		return ewsOperationError(op, "ErrorAccessDenied", "Mailbox delegation is not supported"), nil
	}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		return "", err
	}
	var results strings.Builder
	switch op {
	case "CreateFolder":
		parent := root.child("ParentFolderId")
		folders := root.child("Folders")
		if parent == nil || folders == nil || len(parent.Children) != 1 || len(folders.Children) == 0 {
			return ewsOperationError(op, "ErrorInvalidRequest", "Missing parent or folders"), nil
		}
		parentID, e := h.resolveEWSFolder(ctx, u, parent.render())
		prefix := ""
		if e == nil && parentID != "root:"+u.ID {
			mb, lookupErr := h.mailFolderForWrite(ctx, u, parent)
			e = lookupErr
			if e == nil {
				prefix = mb.Name + "/"
			}
		}
		for _, folder := range folders.Children {
			if e != nil {
				results.WriteString(folderWriteMessage(op, "ErrorFolderNotFound", "Parent folder unavailable", ""))
				continue
			}
			name := folder.value("DisplayName")
			if folder.Name.Local != "Folder" || !validFolderDisplay(name) || (folder.value("FolderClass") != "" && folder.value("FolderClass") != "IPF.Note") {
				results.WriteString(folderWriteMessage(op, "ErrorInvalidPropertySet", "Only mail folders with valid names are supported", ""))
				continue
			}
			full := prefix + name
			if _, err := h.store.GetMailbox(ctx, u.ID, full); err == nil {
				results.WriteString(folderWriteMessage(op, "ErrorFolderExists", "Folder already exists", ""))
				continue
			} else if err != storage.ErrNotFound {
				return "", err
			}
			path := ""
			if h.ms != nil {
				path, err = h.ms.EnsureFolder(u.Email, full)
				if err != nil {
					return "", err
				}
			}
			mb, err := h.store.CreateMailbox(ctx, u.ID, full, path)
			if err != nil {
				return "", err
			}
			results.WriteString(folderWriteMessage(op, "NoError", "", folderWritePayload(mb)))
		}
	case "UpdateFolder":
		changes := root.child("FolderChanges")
		if changes == nil || len(changes.Children) == 0 {
			return ewsOperationError(op, "ErrorInvalidRequest", "Missing folder changes"), nil
		}
		for _, change := range changes.Children {
			mb, e := h.mailFolderForWrite(ctx, u, change.child("FolderId"))
			if e != nil {
				results.WriteString(folderWriteMessage(op, "ErrorFolderNotFound", "Folder unavailable", ""))
				continue
			}
			updates := change.child("Updates")
			name := ""
			valid := updates != nil && len(updates.Children) > 0
			if valid {
				for _, update := range updates.Children {
					// Do not acknowledge properties that are not persisted by the mail store.
					if update.Name.Local != "SetFolderField" || extractAttr(update.render(), "FieldURI", "FieldURI") != "folder:DisplayName" || update.child("Folder") == nil {
						valid = false
						break
					}
					name = update.child("Folder").value("DisplayName")
					if !validFolderDisplay(name) {
						valid = false
						break
					}
				}
			}
			if !valid {
				results.WriteString(folderWriteMessage(op, "ErrorInvalidPropertySet", "Unsupported folder property or invalid display name", ""))
				continue
			}
			full := mb.Name[:strings.LastIndex(mb.Name, "/")+1] + name
			code, e := h.renameEWSMailTree(ctx, u, mb, full)
			if e != nil {
				return "", e
			}
			if code != "NoError" {
				results.WriteString(folderWriteMessage(op, code, "Cannot rename folder", ""))
				continue
			}
			mb.Name = full
			results.WriteString(folderWriteMessage(op, "NoError", "", folderWritePayload(mb)))
		}
	case "MoveFolder":
		ids := root.child("FolderIds")
		parent := root.child("ToFolderId")
		if ids == nil || len(ids.Children) == 0 || parent == nil || len(parent.Children) != 1 {
			return ewsOperationError(op, "ErrorInvalidRequest", "Missing folders or destination"), nil
		}
		parentID, e := h.resolveEWSFolder(ctx, u, parent.render())
		prefix := ""
		if e == nil && parentID != "root:"+u.ID {
			mb, lookupErr := h.mailFolderForWrite(ctx, u, parent)
			e = lookupErr
			if e == nil {
				prefix = mb.Name + "/"
			}
		}
		if e != nil {
			return ewsOperationError(op, "ErrorFolderNotFound", "Destination unavailable"), nil
		}
		for _, ref := range ids.Children {
			mb, e := h.mailFolderForWrite(ctx, u, ref)
			if e != nil {
				results.WriteString(folderWriteMessage(op, "ErrorFolderNotFound", "Folder unavailable", ""))
				continue
			}
			full := prefix + mb.Name[strings.LastIndex(mb.Name, "/")+1:]
			code, e := h.renameEWSMailTree(ctx, u, mb, full)
			if e != nil {
				return "", e
			}
			if code != "NoError" {
				results.WriteString(folderWriteMessage(op, code, "Cannot move folder", ""))
				continue
			}
			mb.Name = full
			results.WriteString(folderWriteMessage(op, "NoError", "", folderWritePayload(mb)))
		}
	case "DeleteFolder", "EmptyFolder":
		ids := root.child("FolderIds")
		mode := extractAttr(root.render(), op, "DeleteType")
		if ids == nil || len(ids.Children) == 0 || (mode != "HardDelete" && mode != "MoveToDeletedItems" && mode != "SoftDelete") {
			return ewsOperationError(op, "ErrorInvalidRequest", "Missing folders or invalid DeleteType"), nil
		}
		for _, ref := range ids.Children {
			mb, e := h.mailFolderForWrite(ctx, u, ref)
			if e != nil {
				results.WriteString(folderWriteMessage(op, "ErrorFolderNotFound", "Folder unavailable", ""))
				continue
			}
			if op == "DeleteFolder" && protectedEWSMailbox(mb.Name) {
				results.WriteString(folderWriteMessage(op, "ErrorDeleteDistinguishedFolder", "Cannot delete a system folder", ""))
				continue
			}
			if op == "DeleteFolder" && (mode == "MoveToDeletedItems" || mode == "SoftDelete") {
				destination := "Trash/" + mb.Name[strings.LastIndex(mb.Name, "/")+1:]
				if mode == "SoftDelete" {
					recovery, e := h.recoveryMailbox(ctx, u)
					if e != nil {
						return "", e
					}
					destination = recovery.Name + "/" + mb.Name[strings.LastIndex(mb.Name, "/")+1:] + "-" + mb.ID
				}
				code, e := h.renameEWSMailTree(ctx, u, mb, destination)
				if e != nil {
					return "", e
				}
				results.WriteString(folderWriteMessage(op, code, "", ""))
				continue
			}
			mailboxes, e := h.store.ListMailboxes(ctx, u.ID)
			if e != nil {
				return "", e
			}
			deleteChildren := op == "DeleteFolder" || extractAttr(root.render(), op, "DeleteSubFolders") == "true" || extractAttr(root.render(), op, "DeleteSubFolders") == "1"
			for i := len(mailboxes) - 1; i >= 0; i-- {
				target := mailboxes[i]
				if target.ID != mb.ID && !(deleteChildren && strings.HasPrefix(target.Name, mb.Name+"/")) {
					continue
				}
				if e = h.emptyEWSMailbox(ctx, u, target, mode); e != nil {
					return "", e
				}
				if op == "DeleteFolder" || target.ID != mb.ID {
					if e = h.store.DeleteMailbox(ctx, u.ID, target.Name); e != nil {
						return "", e
					}
					// Other protocol moves may still reference files under this Maildir.
					// Delete indexed contents, but do not recursively remove shared paths.
				}
			}
			results.WriteString(folderWriteMessage(op, "NoError", "", ""))
		}
	}
	return fmt.Sprintf(`<m:%sResponse%s><m:ResponseMessages>%s</m:ResponseMessages></m:%sResponse>`, op, ewsNamespaces, results.String(), op), nil
}

func protectedEWSMailbox(name string) bool {
	return storage.SystemMailboxRole(name) != "" || strings.EqualFold(name, "Outbox") || name == "Recoverable Items" || name == ewsRecoveryFolder
}

func (h *ewsHandler) renameEWSMailTree(ctx context.Context, u *storage.User, mb *storage.Mailbox, name string) (string, error) {
	if mb.Name == name {
		return "NoError", nil
	}
	if protectedEWSMailbox(mb.Name) || protectedEWSMailbox(name) {
		return "ErrorAccessDenied", nil
	}
	if strings.HasPrefix(name, mb.Name+"/") {
		return "ErrorInvalidFolderId", nil
	}
	mailboxes, err := h.store.ListMailboxes(ctx, u.ID)
	if err != nil {
		return "", err
	}
	var tree []*storage.Mailbox
	for _, candidate := range mailboxes {
		if candidate.Name == mb.Name || strings.HasPrefix(candidate.Name, mb.Name+"/") {
			tree = append(tree, candidate)
		}
	}
	for _, candidate := range tree {
		dest := name + strings.TrimPrefix(candidate.Name, mb.Name)
		if _, err := h.store.GetMailbox(ctx, u.ID, dest); err == nil {
			return "ErrorFolderExists", nil
		} else if err != storage.ErrNotFound {
			return "", err
		}
	}
	if h.ms != nil {
		// Ensure the destination's parent exists before moving a nested Maildir.
		if cut := strings.LastIndex(name, "/"); cut >= 0 {
			if _, err = h.ms.EnsureFolder(u.Email, name[:cut]); err != nil {
				return "", err
			}
		}
		if err = h.ms.RenameFolder(u.Email, mb.Name, name); err != nil {
			return "", err
		}
	}
	for _, candidate := range tree {
		dest := name + strings.TrimPrefix(candidate.Name, mb.Name)
		path := candidate.Path
		if h.ms != nil {
			path = filepath.Join(h.ms.UserRoot(u.Email), "."+dest)
		}
		if err = h.store.RenameMailbox(ctx, u.ID, candidate.Name, dest, path); err != nil {
			return "", err
		}
	}
	return "NoError", nil
}

func (h *ewsHandler) emptyEWSMailbox(ctx context.Context, u *storage.User, mb *storage.Mailbox, mode string) error {
	messages, err := h.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return err
	}
	destination := ""
	if mode == "MoveToDeletedItems" && mb.Name != "Trash" {
		trash, err := h.store.GetMailbox(ctx, u.ID, "Trash")
		if err != nil {
			return err
		}
		destination = trash.ID
	} else if mode == "SoftDelete" {
		recovery, err := h.recoveryMailbox(ctx, u)
		if err != nil {
			return err
		}
		destination = recovery.ID
	}
	ids := make([]string, 0, len(messages))
	for _, msg := range messages {
		ids = append(ids, msg.ID)
	}
	disposed, err := h.store.DisposeMessages(ctx, u.ID, ids, destination)
	if err != nil {
		return err
	}
	if destination == "" && h.ms != nil {
		for _, msg := range disposed {
			if err = h.ms.Delete(msg.FilePath); err != nil {
				return err
			}
		}
	}
	return nil
}
