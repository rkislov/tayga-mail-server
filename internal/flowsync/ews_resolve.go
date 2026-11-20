// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func (h *ewsHandler) resolveEWSFolder(ctx context.Context, u *storage.User, body string) (string, error) {
	if id := extractAttr(body, "FolderId", "Id"); id != "" {
		return id, nil
	}
	distinguished := strings.ToLower(extractAttr(body, "DistinguishedFolderId", "Id"))
	if distinguished == "" {
		distinguished = "inbox"
	}
	principal := u
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	if folder := doc.find("DistinguishedFolderId"); folder != nil {
		if mailbox := folder.child("Mailbox"); mailbox != nil {
			principal, err = h.delegatedMailbox(ctx, u, mailbox.value("EmailAddress"))
			if err != nil {
				return "", err
			}
		}
	}
	if principal.ID != u.ID {
		if distinguished == "root" || distinguished == "msgfolderroot" {
			return "root:" + principal.ID, nil
		}
		if distinguished == "calendar" {
			cal, err := h.store.GetCalendarByName(ctx, principal.ID, "default")
			if err != nil {
				return "", err
			}
			if _, err = h.ewsCalendar(ctx, u.ID, cal.ID, false); err != nil {
				return "", err
			}
			return cal.ID, nil
		}
		name, ok := map[string]string{"inbox": "INBOX", "sentitems": "Sent", "deleteditems": "Trash", "drafts": "Drafts", "junkemail": "Junk", "outbox": "Outbox"}[distinguished]
		if !ok {
			return "", storage.ErrUnauthorized
		}
		mb, err := h.store.GetMailbox(ctx, principal.ID, name)
		if err != nil {
			return "", err
		}
		return mb.ID, nil
	}
	if _, ok := ewsSystemFolders[distinguished]; ok {
		return systemFolderID(u.ID, distinguished), nil
	}
	switch distinguished {
	case "root", "msgfolderroot":
		return "root:" + u.ID, nil
	case "recoverableitemsroot", "recoverableitemsdeletions":
		if _, e := h.recoveryMailbox(ctx, u); e != nil {
			return "", e
		}
		name := "Recoverable Items"
		if distinguished == "recoverableitemsdeletions" {
			name = ewsRecoveryFolder
		}
		mb, e := h.store.GetMailbox(ctx, u.ID, name)
		if e != nil {
			return "", e
		}
		return mb.ID, nil
	case "calendar":
		c, e := h.store.GetCalendarByName(ctx, u.ID, "default")
		if e != nil {
			return "", e
		}
		return c.ID, nil
	case "contacts":
		a, e := h.store.GetAddressBookByName(ctx, u.ID, "default")
		if e != nil {
			return "", e
		}
		return a.ID, nil
	case "notes":
		n, e := h.store.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
		if e != nil {
			return "", e
		}
		return n.ID, nil
	}
	name, ok := map[string]string{"inbox": "INBOX", "sentitems": "Sent", "deleteditems": "Trash", "drafts": "Drafts", "junkemail": "Junk", "outbox": "Outbox"}[distinguished]
	if !ok {
		return "", storage.ErrNotFound
	}
	m, e := h.store.GetMailbox(ctx, u.ID, name)
	if e != nil {
		return "", e
	}
	return m.ID, nil
}

func (h *ewsHandler) ensureStandardMailFolders(ctx context.Context, u *storage.User) error {
	for _, name := range []string{"INBOX", "Sent", "Trash", "Drafts", "Junk", "Outbox"} {
		if _, err := h.store.GetMailbox(ctx, u.ID, name); err == nil {
			continue
		} else if err != storage.ErrNotFound {
			return err
		}
		path := ""
		if h.ms != nil {
			var err error
			path, err = h.ms.EnsureFolder(u.Email, name)
			if err != nil {
				return err
			}
		}
		if _, err := h.store.EnsureMailbox(ctx, u.ID, name, path); err != nil {
			return err
		}
	}
	return nil
}
func ewsOperationError(op, code, message string) string {
	return fmt.Sprintf(`<m:%sResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"><m:ResponseMessages><m:%sResponseMessage ResponseClass="Error"><m:MessageText>%s</m:MessageText><m:ResponseCode>%s</m:ResponseCode></m:%sResponseMessage></m:ResponseMessages></m:%sResponse>`, op, op, xmlEscape(message), xmlEscape(code), op, op)
}
