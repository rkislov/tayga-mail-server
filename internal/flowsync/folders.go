package flowsync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type moveRes struct{ Src, Dst string }

func (h *easHandler) folderCreate(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	display := extractTag(string(body), "DisplayName")
	parent := extractTag(string(body), "ParentId")
	typStr := extractTag(string(body), "Type")
	if display == "" {
		return folderOpStatusXML("FolderCreate", 5, ""), nil, nil
	}
	if parent != "" && parent != "0" {
		return folderOpStatusXML("FolderCreate", 5, ""), nil, nil
	}
	typ := 12
	if typStr != "" {
		fmt.Sscanf(typStr, "%d", &typ)
	}

	switch typ {
	case folderTypeCalendar:
		cal, err := h.store.EnsureCalendar(ctx, u.ID, sanitizeFolderName(display), display)
		if err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderCreate", 1, cal.ID), nil, nil
	case folderTypeContacts:
		ab, err := h.store.EnsureAddressBook(ctx, u.ID, sanitizeFolderName(display), display)
		if err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderCreate", 1, ab.ID), nil, nil
	default:
		path := display
		if h.ms != nil {
			_, _ = h.ms.EnsureFolder(u.Email, display)
			path = h.ms.UserRoot(u.Email)
			if !strings.EqualFold(display, "INBOX") {
				path = filepath.Join(path, "."+display)
			}
		}
		mb, err := h.store.CreateMailbox(ctx, u.ID, display, path)
		if err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderCreate", 1, mb.ID), nil, nil
	}
}

func (h *easHandler) folderDelete(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	serverID := extractTag(string(body), "ServerId")
	if serverID == "" {
		return folderOpStatusXML("FolderDelete", 5, ""), nil, nil
	}
	if mb, err := h.mailboxByID(ctx, u.ID, serverID); err == nil {
		if err := h.store.DeleteMailbox(ctx, u.ID, mb.Name); err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderDelete", 1, ""), nil, nil
	}
	if cal, err := h.store.GetCalendarByID(ctx, u.ID, serverID); err == nil {
		objs, _ := h.store.ListCalendarObjects(ctx, cal.ID)
		for _, o := range objs {
			_ = h.store.DeleteCalendarObject(ctx, cal.ID, o.HrefName)
		}
		if err := h.store.DeleteCalendar(ctx, u.ID, cal.Name); err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderDelete", 1, ""), nil, nil
	}
	if ab, err := h.store.GetAddressBookByID(ctx, u.ID, serverID); err == nil {
		objs, _ := h.store.ListAddressObjects(ctx, ab.ID)
		for _, o := range objs {
			_ = h.store.DeleteAddressObject(ctx, ab.ID, o.HrefName)
		}
		if err := h.store.DeleteAddressBook(ctx, u.ID, ab.Name); err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderDelete", 1, ""), nil, nil
	}
	return folderOpStatusXML("FolderDelete", 6, ""), nil, nil
}

func (h *easHandler) folderUpdate(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	serverID := extractTag(string(body), "ServerId")
	display := extractTag(string(body), "DisplayName")
	if serverID == "" || display == "" {
		return folderOpStatusXML("FolderUpdate", 5, ""), nil, nil
	}
	mb, err := h.mailboxByID(ctx, u.ID, serverID)
	if err != nil {
		return folderOpStatusXML("FolderUpdate", 6, ""), nil, nil
	}
	path := mb.Path
	if h.ms != nil {
		path = filepath.Join(h.ms.UserRoot(u.Email), "."+display)
	}
	if err := h.store.RenameMailbox(ctx, u.ID, mb.Name, display, path); err != nil {
		return "", nil, err
	}
	return folderOpStatusXML("FolderUpdate", 1, ""), nil, nil
}

func (h *easHandler) moveItems(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	src := string(body)
	dstID := extractTag(src, "DstFldId")
	if dstID == "" {
		return moveItemsXML(nil), nil, nil
	}
	dstMB, err := h.mailboxByID(ctx, u.ID, dstID)
	if err != nil {
		return moveItemsXML(nil), nil, nil
	}

	var srcIDs []string
	rest := src
	for {
		id := extractTag(rest, "SrcMsgId")
		if id == "" {
			break
		}
		srcIDs = append(srcIDs, id)
		idx := strings.Index(rest, id)
		if idx < 0 {
			break
		}
		rest = rest[idx+len(id):]
	}

	var results []moveRes
	for _, sid := range srcIDs {
		msg, err := h.store.GetMessageByID(ctx, sid)
		if err != nil {
			continue
		}
		if _, err := h.mailboxByID(ctx, u.ID, msg.MailboxID); err != nil {
			continue
		}
		moved, err := h.store.MoveMessage(ctx, sid, dstMB.ID)
		if err != nil {
			continue
		}
		results = append(results, moveRes{Src: sid, Dst: moved.ID})
	}
	return moveItemsXML(results), nil, nil
}

func folderOpStatusXML(op string, status int, serverID string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	fmt.Fprintf(&b, `<%s xmlns="FolderHierarchy:"><Status>%d</Status>`, op, status)
	if serverID != "" {
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(serverID))
	}
	fmt.Fprintf(&b, `</%s>`, op)
	return b.String()
}

func moveItemsXML(results []moveRes) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<MoveItems xmlns="Move:"><Response>`)
	for _, r := range results {
		b.WriteString(`<Response>`)
		fmt.Fprintf(&b, `<Status>3</Status><SrcMsgId>%s</SrcMsgId><DstMsgId>%s</DstMsgId>`,
			xmlEscape(r.Src), xmlEscape(r.Dst))
		b.WriteString(`</Response>`)
	}
	if len(results) == 0 {
		b.WriteString(`<Status>1</Status>`)
	}
	b.WriteString(`</Response></MoveItems>`)
	return b.String()
}

func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	if name == "" {
		return storage.NewID()
	}
	return strings.ToLower(name)
}
