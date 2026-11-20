// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

// Resolve each mailbox-qualified reference independently; a batch can contain
// folders from different principals. Folder IDs never bypass access checks.
func (h *ewsHandler) getFolder(ctx context.Context, u *storage.User, body string) (string, error) {
	listing, err := h.findFolder(ctx, u)
	if err != nil {
		return "", err
	}
	doc, err := parseProtocolXML([]byte(listing))
	if err != nil {
		return "", err
	}
	folders := map[string]*protocolNode{}
	for _, f := range doc.find("Folders").Children {
		folders[nodeAttr(f.child("FolderId"), "Id")] = f
	}
	request, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	group := request.find("FolderIds")
	if group == nil {
		group = request.find("GetFolder")
	}
	if group == nil || len(group.Children) == 0 || len(group.Children) > 512 {
		return ewsOperationError("GetFolder", "ErrorInvalidRequest", "FolderIds required"), nil
	}
	var out strings.Builder
	for _, ref := range group.Children {
		if ref.Name.Local != "FolderId" && ref.Name.Local != "DistinguishedFolderId" {
			out.WriteString(folderWriteMessage("GetFolder", "ErrorInvalidFolderId", "Invalid folder reference", ""))
			continue
		}
		id, err := h.resolveEWSFolder(ctx, u, ref.render())
		f := folders[id]
		if err == nil && id == "root:"+u.ID {
			f = &protocolNode{}
			raw := fmt.Sprintf(`<t:Folder><t:FolderId Id="%s"/><t:DisplayName>Mailbox</t:DisplayName><t:ChildFolderCount>%d</t:ChildFolderCount></t:Folder>`, xmlEscape(id), len(folders))
			parsed, _ := parseProtocolXML([]byte(raw))
			f = parsed.Children[0]
		}
		if err == nil && systemFolderName(u, id) != "" && f == nil {
			raw := systemFolderXML(u, systemFolderName(u, id))
			parsed, e := parseProtocolXML([]byte(raw))
			if e == nil {
				f = parsed.Children[0]
			}
		}
		if err != nil || f == nil {
			out.WriteString(folderWriteMessage("GetFolder", "ErrorFolderNotFound", "Folder unavailable", ""))
			continue
		}
		if taygaRequested(body, "TaygaSyncVersion") { // keep extension metadata local to this response
			raw := ewsTypeNodeXML(f)
			at := strings.LastIndex(raw, "</")
			if at >= 0 {
				raw = raw[:at] + taygaProperty("TaygaSyncVersion", "1") + raw[at:]
			}
			out.WriteString(`<m:GetFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Folders>` + raw + `</m:Folders></m:GetFolderResponseMessage>`)
		} else {
			out.WriteString(`<m:GetFolderResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Folders>` + ewsTypeNodeXML(f) + `</m:Folders></m:GetFolderResponseMessage>`)
		}
	}
	return `<m:GetFolderResponse` + ewsNamespaces + `><m:ResponseMessages>` + out.String() + `</m:ResponseMessages></m:GetFolderResponse>`, nil
}

// Keep stable EWS prefixes on serialized type elements, including legacy clients
// that inspect qualified XML names rather than namespace-expanded names.
func ewsTypeNodeXML(n *protocolNode) string {
	var out strings.Builder
	fmt.Fprintf(&out, "<t:%s", n.Name.Local)
	for _, a := range n.Attr {
		fmt.Fprintf(&out, ` %s="%s"`, a.Name.Local, xmlEscape(a.Value))
	}
	out.WriteString(">")
	out.WriteString(xmlEscape(n.Text))
	for _, child := range n.Children {
		out.WriteString(ewsTypeNodeXML(child))
	}
	fmt.Fprintf(&out, "</t:%s>", n.Name.Local)
	return out.String()
}
