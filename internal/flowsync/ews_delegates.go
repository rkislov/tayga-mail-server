package flowsync

import (
	"context"
	"errors"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func delegateResult(code, message, user string) string {
	class := "Success"
	if code != "NoError" {
		class = "Error"
	}
	return fmt.Sprintf(`<m:DelegateUserResponseMessageType ResponseClass="%s"><m:MessageText>%s</m:MessageText><m:ResponseCode>%s</m:ResponseCode>%s</m:DelegateUserResponseMessageType>`, class, xmlEscape(message), code, user)
}
func ewsDelegateXML(d *storage.MailboxDelegate, u *storage.User, permissions bool) string {
	inbox, calendar := d.EWSInboxPermission, d.EWSCalendarPermission
	if inbox == "" || inbox == "None" {
		inbox = "None"
	}
	if calendar == "" || calendar == "None" {
		calendar = "None"
	}
	if d.CanRead && inbox == "None" {
		inbox = "Reviewer"
	}
	if d.CanManageMail {
		inbox = "Editor"
	}
	if (d.CanReadCalendar || d.CanRespondInvites) && calendar == "None" {
		calendar = "Reviewer"
	}
	if d.CanManageCalendar {
		calendar = "Editor"
	}
	out := `<m:DelegateUser><t:UserId><t:PrimarySmtpAddress>` + xmlEscape(u.Email) + `</t:PrimarySmtpAddress><t:DisplayName>` + xmlEscape(u.DisplayName) + `</t:DisplayName></t:UserId>`
	if permissions {
		out += `<t:DelegatePermissions><t:CalendarFolderPermissionLevel>` + calendar + `</t:CalendarFolderPermissionLevel><t:TasksFolderPermissionLevel>None</t:TasksFolderPermissionLevel><t:InboxFolderPermissionLevel>` + inbox + `</t:InboxFolderPermissionLevel><t:ContactsFolderPermissionLevel>None</t:ContactsFolderPermissionLevel><t:NotesFolderPermissionLevel>None</t:NotesFolderPermissionLevel><t:JournalFolderPermissionLevel>None</t:JournalFolderPermissionLevel></t:DelegatePermissions>`
	}
	return out + fmt.Sprintf(`<t:ReceiveCopiesOfMeetingMessages>%t</t:ReceiveCopiesOfMeetingMessages><t:ViewPrivateItems>false</t:ViewPrivateItems></m:DelegateUser>`, d.EWSReceiveMeetings)
}
func ewsBool(node *protocolNode) (bool, error) {
	switch strings.TrimSpace(node.Text) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean")
}
func applyDelegatePermissions(d *storage.MailboxDelegate, node *protocolNode) error {
	seenFields := map[string]bool{}
	for _, field := range node.Children {
		if seenFields[field.Name.Local] {
			return fmt.Errorf("duplicate delegate field")
		}
		seenFields[field.Name.Local] = true
		switch field.Name.Local {
		case "UserId":
		case "DelegatePermissions":
			seen := map[string]bool{}
			for _, permission := range field.Children {
				key, value := permission.Name.Local, strings.TrimSpace(permission.Text)
				if seen[key] {
					return fmt.Errorf("duplicate permission")
				}
				seen[key] = true
				switch key {
				case "InboxFolderPermissionLevel", "CalendarFolderPermissionLevel":
					if value != "None" && value != "Reviewer" && value != "Editor" {
						return fmt.Errorf("supported levels: None, Reviewer, Editor")
					}
					if key == "InboxFolderPermissionLevel" && (d.CanManageMail && value != "Editor" || d.CanRead && value == "None") {
						return fmt.Errorf("broader web mailbox rights must be changed in web delegation settings")
					}
					if key == "CalendarFolderPermissionLevel" && (d.CanManageCalendar && value != "Editor" || (d.CanReadCalendar || d.CanRespondInvites) && value == "None") {
						return fmt.Errorf("broader web calendar rights must be changed in web delegation settings")
					}
					if key == "InboxFolderPermissionLevel" {
						d.EWSInboxPermission = value
					} else {
						d.EWSCalendarPermission = value
					}
				case "TasksFolderPermissionLevel", "ContactsFolderPermissionLevel", "NotesFolderPermissionLevel", "JournalFolderPermissionLevel":
					if value != "None" {
						return fmt.Errorf("folder type does not support delegation")
					}
				default:
					return fmt.Errorf("unsupported permission")
				}
			}
		case "ReceiveCopiesOfMeetingMessages":
			value, err := ewsBool(field)
			if err != nil {
				return err
			}
			d.EWSReceiveMeetings = value
		case "ViewPrivateItems":
			value, err := ewsBool(field)
			if err != nil {
				return err
			}
			if value {
				return fmt.Errorf("private-item delegation is not supported")
			}
		default:
			return fmt.Errorf("unsupported delegate field")
		}
	}
	if d.EWSReceiveMeetings && d.EWSCalendarPermission != "Editor" && !d.CanManageCalendar {
		return fmt.Errorf("meeting delivery requires Calendar Editor")
	}
	return nil
}
func (h *ewsHandler) delegatesEWS(ctx context.Context, actor *storage.User, op, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	request := doc.find(op)
	if request == nil {
		return ewsOperationError(op, "ErrorInvalidRequest", "Missing operation"), nil
	}
	seen := map[string]bool{}
	for _, child := range request.Children {
		name := child.Name.Local
		if seen[name] {
			return ewsOperationError(op, "ErrorInvalidRequest", "Duplicate request field"), nil
		}
		seen[name] = true
		allowed := name == "Mailbox" || name == "UserIds" && (op == "GetDelegate" || op == "RemoveDelegate") || name == "DelegateUsers" && (op == "AddDelegate" || op == "UpdateDelegate") || name == "DeliverMeetingRequests" && (op == "AddDelegate" || op == "UpdateDelegate")
		if !allowed {
			return ewsOperationError(op, "ErrorInvalidRequest", "Unsupported request field"), nil
		}
	}
	if value := nodeAttr(request, "IncludePermissions"); value != "" && value != "true" && value != "false" && value != "1" && value != "0" {
		return ewsOperationError(op, "ErrorInvalidRequest", "Invalid IncludePermissions"), nil
	}
	mailbox := request.child("Mailbox")
	if mailbox == nil || !strings.EqualFold(strings.TrimSpace(mailbox.value("EmailAddress")), actor.Email) {
		return ewsOperationError(op, "ErrorAccessDenied", "Only the mailbox owner can manage delegates"), nil
	}
	if request.child("DeliverMeetingRequests") != nil && request.value("DeliverMeetingRequests") != "DelegatesAndMe" {
		return ewsOperationError(op, "ErrorInvalidRequest", "Supported delivery mode: DelegatesAndMe"), nil
	}
	grants, err := h.store.ListMailboxDelegates(ctx, actor.ID)
	if err != nil {
		return "", err
	}
	byEmail := map[string]*storage.MailboxDelegate{}
	users := map[string]*storage.User{}
	for _, grant := range grants {
		user, e := h.store.GetUserByID(ctx, grant.DelegateID)
		if e != nil {
			return "", e
		}
		byEmail[strings.ToLower(user.Email)] = grant
		users[user.ID] = user
	}
	var out strings.Builder
	var nodes []*protocolNode
	if op == "GetDelegate" || op == "RemoveDelegate" {
		if ids := request.child("UserIds"); ids != nil {
			nodes = ids.Children
		}
	} else if group := request.child("DelegateUsers"); group != nil {
		nodes = group.Children
	}
	if len(nodes) > 100 {
		return ewsOperationError(op, "ErrorInvalidRequest", "Too many delegates"), nil
	}
	if op == "GetDelegate" && request.child("UserIds") == nil {
		for _, d := range grants {
			out.WriteString(delegateResult("NoError", "", ewsDelegateXML(d, users[d.DelegateID], nodeAttr(request, "IncludePermissions") == "true" || nodeAttr(request, "IncludePermissions") == "1")))
		}
	} else {
		if len(nodes) == 0 && (op != "UpdateDelegate" || request.child("DeliverMeetingRequests") == nil) {
			return ewsOperationError(op, "ErrorInvalidRequest", "Delegate users required"), nil
		}
		for _, node := range nodes {
			id := node
			if op == "AddDelegate" || op == "UpdateDelegate" {
				id = node.child("UserId")
			}
			if id == nil || id.child("PrimarySmtpAddress") == nil || len(id.Children) != 1 {
				out.WriteString(delegateResult("ErrorInvalidUserInfo", "PrimarySmtpAddress is required", ""))
				continue
			}
			email := strings.ToLower(strings.TrimSpace(id.value("PrimarySmtpAddress")))
			previous := byEmail[email]
			if op == "GetDelegate" {
				if previous == nil {
					out.WriteString(delegateResult("ErrorNotDelegate", "User is not a delegate", ""))
				} else {
					out.WriteString(delegateResult("NoError", "", ewsDelegateXML(previous, users[previous.DelegateID], nodeAttr(request, "IncludePermissions") == "true" || nodeAttr(request, "IncludePermissions") == "1")))
				}
				continue
			}
			if op == "RemoveDelegate" {
				if previous == nil {
					out.WriteString(delegateResult("ErrorNotDelegate", "User is not a delegate", ""))
					continue
				}
				e := h.store.DeleteMailboxDelegate(ctx, actor.ID, previous.DelegateID)
				if e != nil {
					return "", e
				}
				delete(byEmail, email)
				out.WriteString(delegateResult("NoError", "", ""))
				continue
			}
			if op == "AddDelegate" && previous != nil {
				out.WriteString(delegateResult("ErrorDelegateAlreadyExists", "Delegate already exists", ""))
				continue
			}
			if op == "UpdateDelegate" && previous == nil {
				out.WriteString(delegateResult("ErrorNotDelegate", "User is not a delegate", ""))
				continue
			}
			delegate, e := h.store.GetUserByEmail(ctx, email)
			if e != nil || !delegate.Enabled || delegate.ID == actor.ID || delegate.TenantID != actor.TenantID || delegate.AuthSource == "resource" {
				out.WriteString(delegateResult("ErrorInvalidUserInfo", "Active user in the same tenant required", ""))
				continue
			}
			d := &storage.MailboxDelegate{OwnerID: actor.ID, DelegateID: delegate.ID, CanSendOnBehalf: true}
			if previous != nil {
				copy := *previous
				d = &copy
			}
			if e = applyDelegatePermissions(d, node); e != nil {
				out.WriteString(delegateResult("ErrorInvalidDelegatePermission", e.Error(), ""))
				continue
			}
			if e = h.store.EnsureDAVDefaults(ctx, actor.ID); e != nil {
				return "", e
			}
			if e = h.ensureStandardMailFolders(ctx, actor); e != nil {
				return "", e
			}
			mode := "create"
			if op == "UpdateDelegate" {
				mode = "update"
			}
			if e = h.store.UpsertMailboxDelegate(ctx, d, mode); e != nil {
				code := "ErrorInternalServerError"
				if errors.Is(e, storage.ErrDelegateExists) {
					code = "ErrorDelegateAlreadyExists"
				}
				if errors.Is(e, storage.ErrNotFound) {
					code = "ErrorNotDelegate"
				}
				out.WriteString(delegateResult(code, "Cannot save delegate", ""))
				continue
			}
			byEmail[email] = d
			users[delegate.ID] = delegate
			out.WriteString(delegateResult("NoError", "", ewsDelegateXML(d, delegate, false)))
		}
	}
	delivery := ""
	if op == "GetDelegate" {
		delivery = `<m:DeliverMeetingRequests>DelegatesAndMe</m:DeliverMeetingRequests>`
	}
	return fmt.Sprintf(`<m:%sResponse%s ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:ResponseMessages>%s</m:ResponseMessages>%s</m:%sResponse>`, op, ewsNamespaces, out.String(), delivery, op), nil
}
