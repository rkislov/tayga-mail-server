// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.
package flowsync

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"io"
	"strings"
)

// Resolve only folders enumerated for the authenticated user, never a global ID.
func (h *ewsHandler) getFolder(ctx context.Context, u *storage.User, body string) (string, error) {
	listing, err := h.findFolder(ctx, u)
	if err != nil {
		return "", err
	}
	type folder struct {
		Kind  xml.Name
		Inner string `xml:",innerxml"`
		ID    struct {
			Value string `xml:"Id,attr"`
		} `xml:"FolderId"`
		Name string `xml:"DisplayName"`
	}
	folders := map[string]folder{}
	decoder := xml.NewDecoder(strings.NewReader(listing))
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
		if start, ok := token.(xml.StartElement); ok && (start.Name.Local == "Folder" || start.Name.Local == "CalendarFolder" || start.Name.Local == "ContactsFolder") {
			var f folder
			f.Kind = start.Name
			if e = decoder.DecodeElement(&f, &start); e != nil {
				return "", e
			}
			folders[f.ID.Value] = f
		}
	}
	aliases := map[string]string{"inbox": "INBOX", "sentitems": "Sent", "deleteditems": "Trash", "drafts": "Drafts", "junkemail": "Junk"}
	var out strings.Builder
	out.WriteString(`<m:GetFolderResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages>`)
	decoder = xml.NewDecoder(strings.NewReader(body))
	count := 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
		start, ok := token.(xml.StartElement)
		if !ok || (start.Name.Local != "FolderId" && start.Name.Local != "DistinguishedFolderId") {
			continue
		}
		id := ""
		for _, a := range start.Attr {
			if a.Name.Local == "Id" {
				id = a.Value
			}
		}
		count++
		var f folder
		found := false
		if start.Name.Local == "FolderId" {
			f, found = folders[id]
		} else {
			if id == "msgfolderroot" || id == "root" {
				f.Kind.Local = "Folder"
				f.Inner = fmt.Sprintf(`<t:FolderId Id="%s" ChangeKey="0"/><t:DisplayName>Mailbox</t:DisplayName><t:TotalCount>0</t:TotalCount><t:ChildFolderCount>%d</t:ChildFolderCount>`, xmlEscape("root:"+u.ID), len(folders))
				found = true
			} else if name, ok := aliases[id]; ok {
				for _, candidate := range folders {
					if strings.EqualFold(candidate.Name, name) {
						f = candidate
						found = true
						break
					}
				}
			} else {
				defaultID := ""
				switch id {
				case "calendar":
					c, e := h.store.GetCalendarByName(ctx, u.ID, "default")
					if e == nil {
						defaultID = c.ID
					}
				case "contacts":
					a, e := h.store.GetAddressBookByName(ctx, u.ID, "default")
					if e == nil {
						defaultID = a.ID
					}
				}
				f, found = folders[defaultID]
			}
		}
		if found {
			fmt.Fprintf(&out, `<m:GetFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Folders><t:%s>%s</t:%s></m:Folders></m:GetFolderResponseMessage>`, f.Kind.Local, f.Inner, f.Kind.Local)
		} else {
			out.WriteString(`<m:GetFolderResponseMessage ResponseClass="Error"><m:ResponseCode>ErrorFolderNotFound</m:ResponseCode><m:Folders/></m:GetFolderResponseMessage>`)
		}
	}
	if count == 0 {
		out.WriteString(`<m:GetFolderResponseMessage ResponseClass="Error"><m:ResponseCode>ErrorInvalidRequest</m:ResponseCode></m:GetFolderResponseMessage>`)
	}
	out.WriteString(`</m:ResponseMessages></m:GetFolderResponse>`)
	return out.String(), nil
}
