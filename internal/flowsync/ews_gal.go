// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

// A virtual, read-only address book makes the directory browsable in clients
// which only show mailbox folders in People. No copies enter personal contacts.
func galFolderID(u *storage.User) string   { return "ews-gal:" + u.ID + ":" + u.DomainID }
func galItemPrefix(u *storage.User) string { return "ews-gal-item:" + u.ID + ":" }

const galReadRights = `<t:EffectiveRights><t:CreateAssociated>false</t:CreateAssociated><t:CreateContents>false</t:CreateContents><t:CreateHierarchy>false</t:CreateHierarchy><t:Delete>false</t:Delete><t:Modify>false</t:Modify><t:Read>true</t:Read></t:EffectiveRights>`

func (h *ewsHandler) galEnabled(ctx context.Context, u *storage.User) (bool, error) {
	v, _, err := h.store.GetSetting(ctx, "domain.gal."+u.DomainID)
	return v == "true", err
}

func (h *ewsHandler) galContacts(ctx context.Context, u *storage.User) ([]*storage.AddressObject, error) {
	enabled, err := h.galEnabled(ctx, u)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, storage.ErrUnauthorized
	}
	users, err := h.store.ListUsersByDomain(ctx, u.DomainID)
	if err != nil {
		return nil, err
	}
	rooms, err := h.store.ListCalendarResourcesByDomain(ctx, u.DomainID, true)
	if err != nil {
		return nil, err
	}
	resources := map[string]string{}
	for _, room := range rooms {
		resources[room.Email] = room.DisplayName
	}
	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(users[i].DisplayName+users[i].Email) < strings.ToLower(users[j].DisplayName+users[j].Email)
	})
	var out []*storage.AddressObject
	for _, peer := range users {
		if !peer.Enabled || peer.TenantID != u.TenantID {
			continue
		}
		name := peer.DisplayName
		if peer.AuthSource == "resource" {
			var ok bool
			name, ok = resources[peer.Email]
			if !ok {
				continue
			}
		}
		if name == "" {
			name = peer.Email
		}
		data := buildVCARD(peer.ID, map[string]string{"FileAs": name, "Email1Address": peer.Email})
		out = append(out, &storage.AddressObject{ID: galItemPrefix(u) + peer.ID, AddressBookID: galFolderID(u), UID: peer.ID, Data: data, ETag: digest(data)})
	}
	return out, nil
}

func galContactXML(o *storage.AddressObject) string {
	// EffectiveRights belongs to ItemType, before the ContactType fields.
	return strings.Replace(ewsContactXML(o), "<t:FileAs>", galReadRights+"<t:FileAs>", 1)
}

func (h *ewsHandler) galFolderXML(ctx context.Context, u *storage.User) (string, error) {
	enabled, err := h.galEnabled(ctx, u)
	if err != nil || !enabled {
		return "", err
	}
	items, err := h.galContacts(ctx, u)
	if err != nil {
		return "", err
	}
	var hashes strings.Builder
	for _, item := range items {
		hashes.WriteString(item.ID + item.ETag)
	}
	return fmt.Sprintf(`<t:ContactsFolder><t:FolderId Id="%s" ChangeKey="%s"/><t:ParentFolderId Id="%s"/><t:DisplayName>Общий адресный список (GAL)</t:DisplayName><t:FolderClass>IPF.Contact</t:FolderClass><t:TotalCount>%d</t:TotalCount><t:ChildFolderCount>0</t:ChildFolderCount>%s</t:ContactsFolder>`, xmlEscape(galFolderID(u)), digest(hashes.String()), xmlEscape("root:"+u.ID), len(items), galReadRights), nil
}

func (h *ewsHandler) findGALItems(ctx context.Context, u *storage.User, body string) (string, error) {
	items, err := h.galContacts(ctx, u)
	if err == storage.ErrUnauthorized {
		return ewsFault("ErrorAccessDenied", "Directory unavailable"), nil
	}
	if err != nil {
		return "", err
	}
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	if restriction := doc.find("Restriction"); restriction != nil {
		if len(restriction.Children) != 1 {
			return ewsFault("ErrorInvalidRestriction", "Invalid directory filter"), nil
		}
		if _, valid := galMatches(restriction.Children[0], &storage.AddressObject{}); !valid {
			return ewsFault("ErrorInvalidRestriction", "Unsupported directory filter"), nil
		}
		filtered := items[:0]
		for _, item := range items {
			match, valid := galMatches(restriction.Children[0], item)
			if !valid {
				return ewsFault("ErrorInvalidRestriction", "Unsupported directory filter"), nil
			}
			if match {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	total, start, end := len(items), 0, len(items)
	nextOffset, last := len(items), true
	if page := doc.find("IndexedPageItemView"); page != nil {
		offset, e := strconv.Atoi(nodeAttr(page, "Offset"))
		limit, le := strconv.Atoi(nodeAttr(page, "MaxEntriesReturned"))
		base := nodeAttr(page, "BasePoint")
		if e != nil || le != nil || offset < 0 || limit < 1 || (base != "Beginning" && base != "End") {
			return ewsFault("ErrorInvalidRequest", "Invalid directory page"), nil
		}
		limit = min(limit, 512)
		if base == "End" {
			end = max(0, total-offset)
			start = max(0, end-limit)
			nextOffset, last = offset+(end-start), start == 0
		} else {
			start = min(offset, total)
			end = min(start+limit, total)
			nextOffset, last = end, end == total
		}
	}
	var b strings.Builder
	for _, item := range items[start:end] {
		b.WriteString(galContactXML(item))
	}
	return fmt.Sprintf(`<m:FindItemResponse%s><m:ResponseMessages><m:FindItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:RootFolder TotalItemsInView="%d" IndexedPagingOffset="%d" IncludesLastItemInRange="%t"><t:Items>%s</t:Items></m:RootFolder></m:FindItemResponseMessage></m:ResponseMessages></m:FindItemResponse>`, ewsNamespaces, total, nextOffset, last, b.String()), nil
}

func galMatches(n *protocolNode, item *storage.AddressObject) (bool, bool) {
	if n.Name.Local == "And" || n.Name.Local == "Or" {
		result := n.Name.Local == "And"
		for _, child := range n.Children {
			match, valid := galMatches(child, item)
			if !valid {
				return false, false
			}
			if n.Name.Local == "And" {
				result = result && match
			} else {
				result = result || match
			}
		}
		return result, len(n.Children) > 0
	}
	if n.Name.Local != "Contains" && n.Name.Local != "IsEqualTo" {
		return false, false
	}
	field, constant := n.find("FieldURI"), n.find("Constant")
	if field == nil || constant == nil {
		return false, false
	}
	c := parseVCard(item.Data)
	value := ""
	switch nodeAttr(field, "FieldURI") {
	case "contacts:DisplayName", "contacts:FileAs", "item:Subject":
		value = c.FN
	case "contacts:EmailAddress", "contacts:EmailAddresses":
		value = c.Email
	default:
		return false, false
	}
	query := nodeAttr(constant, "Value")
	comparison := nodeAttr(n, "ContainmentComparison")
	if comparison == "" || comparison == "IgnoreCase" {
		value, query = strings.ToLower(value), strings.ToLower(query)
	} else if comparison != "Exact" {
		return false, false
	}
	if n.Name.Local == "IsEqualTo" {
		return value == query, true
	}
	switch nodeAttr(n, "ContainmentMode") {
	case "", "Substring":
		return strings.Contains(value, query), true
	case "Prefixed":
		return strings.HasPrefix(value, query), true
	case "FullString":
		return value == query, true
	default:
		return false, false
	}
}

func (h *ewsHandler) getGALItem(ctx context.Context, u *storage.User, id string) (string, error) {
	if !strings.HasPrefix(id, galItemPrefix(u)) {
		return ewsFault("ErrorItemNotFound", "Directory entry unavailable"), nil
	}
	items, err := h.galContacts(ctx, u)
	if err == storage.ErrUnauthorized {
		return ewsFault("ErrorItemNotFound", "Directory entry unavailable"), nil
	}
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID == id {
			return ewsNotificationSuccess("GetItem", "<m:Items>"+galContactXML(item)+"</m:Items>"), nil
		}
	}
	return ewsFault("ErrorItemNotFound", "Directory entry unavailable"), nil
}

func (h *ewsHandler) readableEWSCollection(ctx context.Context, u *storage.User, id string) error {
	if id == galFolderID(u) {
		enabled, err := h.galEnabled(ctx, u)
		if err != nil {
			return err
		}
		if enabled {
			return nil
		}
		return storage.ErrUnauthorized
	}
	_, err := (&easHandler{store: h.store}).resolveCollection(ctx, u.ID, id)
	return err
}
