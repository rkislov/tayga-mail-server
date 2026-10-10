// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

type moveRes struct {
	Src, Dst string
	Status   int
}

func (h *easHandler) folderCreate(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	display := extractTag(string(body), "DisplayName")
	parent := extractTag(string(body), "ParentId")
	typStr := extractTag(string(body), "Type")
	if !validFolderDisplay(display) {
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
	case folderTypeCalendar, 13:
		cal, err := h.store.EnsureCalendar(ctx, u.ID, sanitizeFolderName(display), display)
		if err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderCreate", 1, cal.ID), nil, nil
	case folderTypeContacts, 14:
		ab, err := h.store.EnsureAddressBook(ctx, u.ID, sanitizeFolderName(display), display)
		if err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderCreate", 1, ab.ID), nil, nil
	case folderTypeNotes, 17:
		nf, err := h.store.CreateNoteFolder(ctx, &storage.NoteFolder{
			UserID: u.ID, Name: sanitizeFolderName(display), DisplayName: display,
		})
		if err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderCreate", 1, nf.ID), nil, nil
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
		if folderType(mb.Name) != 12 {
			return folderOpStatusXML("FolderDelete", 3, ""), nil, nil
		}
		if err := h.store.DeleteMailbox(ctx, u.ID, mb.Name); err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderDelete", 1, ""), nil, nil
	}
	if cal, err := h.store.GetCalendarByID(ctx, u.ID, serverID); err == nil {
		if cal.Name == "default" {
			return folderOpStatusXML("FolderDelete", 3, ""), nil, nil
		}
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
		if ab.Name == "default" {
			return folderOpStatusXML("FolderDelete", 3, ""), nil, nil
		}
		objs, _ := h.store.ListAddressObjects(ctx, ab.ID)
		for _, o := range objs {
			_ = h.store.DeleteAddressObject(ctx, ab.ID, o.HrefName)
		}
		if err := h.store.DeleteAddressBook(ctx, u.ID, ab.Name); err != nil {
			return "", nil, err
		}
		return folderOpStatusXML("FolderDelete", 1, ""), nil, nil
	}
	if nf, err := h.store.GetNoteFolderByID(ctx, u.ID, serverID); err == nil {
		if err := h.store.DeleteNoteFolder(ctx, u.ID, nf.ID); err != nil {
			return folderOpStatusXML("FolderDelete", 5, ""), nil, nil
		}
		return folderOpStatusXML("FolderDelete", 1, ""), nil, nil
	}
	return folderOpStatusXML("FolderDelete", 6, ""), nil, nil
}

func (h *easHandler) folderUpdate(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	serverID := extractTag(string(body), "ServerId")
	display := extractTag(string(body), "DisplayName")
	if serverID == "" || !validFolderDisplay(display) {
		return folderOpStatusXML("FolderUpdate", 5, ""), nil, nil
	}
	mb, err := h.mailboxByID(ctx, u.ID, serverID)
	if err != nil {
		return folderOpStatusXML("FolderUpdate", 6, ""), nil, nil
	}
	if folderType(mb.Name) != 12 {
		return folderOpStatusXML("FolderUpdate", 3, ""), nil, nil
	}
	path := mb.Path
	if err := h.store.RenameMailbox(ctx, u.ID, mb.Name, display, path); err != nil {
		return "", nil, err
	}
	return folderOpStatusXML("FolderUpdate", 1, ""), nil, nil
}

func (h *easHandler) moveItems(ctx context.Context, u *storage.User, body []byte, wbxml bool) (string, []byte, error) {
	_ = wbxml
	doc, err := parseProtocolXML(body)
	if err != nil {
		return "", nil, err
	}
	root := doc.find("MoveItems")
	if root == nil || len(root.Children) > 512 {
		return "", nil, fmt.Errorf("invalid moves")
	}
	var results []moveRes
	for _, move := range root.Children {
		if move.Name.Local != "Move" {
			continue
		}
		sid, src, dst := move.value("SrcMsgId"), move.value("SrcFldId"), move.value("DstFldId")
		r := moveRes{Src: sid, Status: 1}
		msg, e := h.store.GetMessageByID(ctx, sid)
		if e == nil && msg.MailboxID == src {
			if _, e = h.mailboxByID(ctx, u.ID, src); e == nil {
				r.Status = 2
				if _, e = h.mailboxByID(ctx, u.ID, dst); e == nil {
					r.Status = 4
					if src != dst {
						r.Status = 5
						moved, e := h.store.MoveMessage(ctx, sid, dst)
						if e == nil {
							r.Status = 3
							r.Dst = moved.ID
						}
					}
				}
			}
		}
		results = append(results, r)
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
	b.WriteString(`<MoveItems xmlns="Move:">`)
	for _, r := range results {
		b.WriteString(`<Response>`)
		status := r.Status
		if status == 0 {
			status = 3
		}
		fmt.Fprintf(&b, `<SrcMsgId>%s</SrcMsgId><Status>%d</Status>`, xmlEscape(r.Src), status)
		if status == 3 {
			fmt.Fprintf(&b, `<DstMsgId>%s</DstMsgId>`, xmlEscape(r.Dst))
		}
		b.WriteString(`</Response>`)
	}
	if len(results) == 0 {
		b.WriteString(`<Response><Status>1</Status></Response>`)
	}
	b.WriteString(`</MoveItems>`)
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

func validFolderDisplay(name string) bool {
	return strings.TrimSpace(name) != "" && len(name) <= 255 && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00\r\n")
}
