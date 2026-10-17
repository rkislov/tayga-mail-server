// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tayga/tms/internal/storage"
)

const ewsNamespaces = ` xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"`

// Some EWS operations use ResponseMessageType directly, without ResponseMessages.
func ewsDirectResponse(op, code, message, payload string) string {
	class := "Success"
	if code != "NoError" {
		class = "Error"
	}
	if message != "" {
		message = "<m:MessageText>" + xmlEscape(message) + "</m:MessageText>"
	}
	return fmt.Sprintf(`<m:%sResponse%s ResponseClass="%s">%s<m:ResponseCode>%s</m:ResponseCode>%s</m:%sResponse>`, op, ewsNamespaces, class, message, code, payload, op)
}

type directoryEntry struct{ name, email, kind, itemID, contact string }

func (e directoryEntry) mailbox() string {
	s := `<t:Name>` + xmlEscape(e.name) + `</t:Name><t:EmailAddress>` + xmlEscape(e.email) + `</t:EmailAddress><t:RoutingType>SMTP</t:RoutingType><t:MailboxType>` + e.kind + `</t:MailboxType>`
	if e.itemID != "" {
		s += `<t:ItemId Id="` + xmlEscape(e.itemID) + `"/>`
	}
	return s
}
func (h *ewsHandler) directoryEWS(ctx context.Context, u *storage.User, op, body string) (string, error) {
	root, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	req := root.find(op)
	if req == nil {
		return ewsFault("ErrorInvalidRequest", "Missing operation"), nil
	}
	enabled, _, err := h.store.GetSetting(ctx, "domain.gal."+u.DomainID)
	if err != nil {
		return "", err
	}
	var entries []directoryEntry
	var rooms []*storage.CalendarResource
	domain, err := h.store.GetDomainByID(ctx, u.DomainID)
	if err != nil {
		return "", err
	}
	// A stable synthetic list identifies this domain's rooms. It is not a mailbox.
	roomList := "tayga-rooms-" + domain.ID + "@" + domain.Name
	if enabled == "true" {
		users, e := h.store.ListUsersByDomain(ctx, u.DomainID)
		if e != nil {
			return "", e
		}
		rooms, err = h.store.ListCalendarResourcesByDomain(ctx, u.DomainID, true)
		if err != nil {
			return "", err
		}
		enabledUsers := map[string]bool{}
		for _, user := range users {
			if user.Enabled {
				enabledUsers[user.Email] = true
			}
			if user.Enabled && user.AuthSource != "resource" {
				entries = append(entries, directoryEntry{name: user.DisplayName, email: user.Email, kind: "Mailbox"})
			}
		}
		visibleRooms := rooms[:0]
		for _, room := range rooms {
			if !enabledUsers[room.Email] {
				continue
			}
			visibleRooms = append(visibleRooms, room)
			entries = append(entries, directoryEntry{name: room.DisplayName, email: room.Email, kind: "Mailbox"})
		}
		rooms = visibleRooms
	}
	var b strings.Builder
	switch op {
	case "GetRoomLists":
		for _, room := range rooms {
			if room.Kind == "room" {
				b.WriteString(`<t:Address>` + (directoryEntry{name: domain.Name, email: roomList, kind: "PublicDL"}).mailbox() + `</t:Address>`)
				break
			}
		}
		return ewsDirectResponse(op, "NoError", "", "<m:RoomLists>"+b.String()+"</m:RoomLists>"), nil
	case "GetRooms", "ExpandDL":
		requested := strings.TrimSpace(req.findValue("EmailAddress"))
		if enabled != "true" || !strings.EqualFold(requested, roomList) {
			if op == "GetRooms" {
				return ewsDirectResponse(op, "ErrorNameResolutionNoResults", "Room list not found", ""), nil
			}
			return ewsOperationError(op, "ErrorNameResolutionNoResults", "Room list not found"), nil
		}
		for _, room := range rooms {
			if room.Kind != "room" {
				continue
			}
			entry := (directoryEntry{name: room.DisplayName, email: room.Email, kind: "Mailbox"}).mailbox()
			if op == "GetRooms" {
				b.WriteString("<t:Room><t:Id>" + entry + "</t:Id></t:Room>")
			} else {
				b.WriteString("<t:Mailbox>" + entry + "</t:Mailbox>")
			}
		}
		if op == "GetRooms" {
			return ewsDirectResponse(op, "NoError", "", "<m:Rooms>"+b.String()+"</m:Rooms>"), nil
		}
		return `<m:ExpandDLResponse` + ewsNamespaces + `><m:ResponseMessages><m:ExpandDLResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:DLExpansion IncludesLastItemInRange="true">` + b.String() + `</m:DLExpansion></m:ExpandDLResponseMessage></m:ResponseMessages></m:ExpandDLResponse>`, nil
	}
	query := strings.ToLower(strings.TrimSpace(req.value("UnresolvedEntry")))
	query = strings.TrimPrefix(query, "smtp:")
	if query == "" || len(query) > 512 {
		return ewsOperationError(op, "ErrorInvalidRequest", "A non-empty search term is required"), nil
	}
	scope := extractAttr(body, "ResolveNames", "SearchScope")
	if scope == "" {
		scope = "ActiveDirectoryContacts"
	}
	switch scope {
	case "ActiveDirectory", "ActiveDirectoryContacts", "ContactsActiveDirectory", "Contacts":
	default:
		return ewsOperationError(op, "ErrorInvalidRequest", "Invalid search scope"), nil
	}
	if scope == "Contacts" {
		entries = nil
	}
	if scope != "ActiveDirectory" {
		books := []string{}
		if folders := req.child("ParentFolderIds"); folders != nil {
			for _, folder := range folders.Children {
				id, e := h.resolveEWSFolder(ctx, u, folder.render())
				if e != nil {
					return ewsOperationError(op, "ErrorAccessDenied", "Contact folder unavailable"), nil
				}
				book, e := h.store.GetAddressBookByID(ctx, u.ID, id)
				if e != nil || book.UserID != u.ID {
					return ewsOperationError(op, "ErrorAccessDenied", "Contact folder unavailable"), nil
				}
				books = append(books, id)
			}
		} else {
			book, e := h.store.GetAddressBookByName(ctx, u.ID, "default")
			if e != nil && e != storage.ErrNotFound {
				return "", e
			}
			if book != nil {
				books = append(books, book.ID)
			}
		}
		var contacts []directoryEntry
		for _, id := range books {
			cards, e := h.store.ListAddressObjects(ctx, id)
			if e != nil {
				return "", e
			}
			for _, card := range cards {
				c := parseVCard(card.Data)
				if c.Email != "" {
					contacts = append(contacts, directoryEntry{name: c.FN, email: c.Email, kind: "Contact", itemID: card.ID})
				}
			}
		}
		if scope == "ContactsActiveDirectory" {
			entries = append(contacts, entries...)
		} else {
			entries = append(entries, contacts...)
		}
	}
	seen := map[string]bool{}
	var matches []directoryEntry
	for _, entry := range entries {
		if !strings.Contains(strings.ToLower(entry.name+" "+entry.email), query) || seen[strings.ToLower(entry.email)] {
			continue
		}
		seen[strings.ToLower(entry.email)] = true
		if entry.name == "" {
			entry.name = entry.email
		}
		matches = append(matches, entry)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return strings.ToLower(matches[i].name+matches[i].email) < strings.ToLower(matches[j].name+matches[j].email)
	})
	if len(matches) == 0 {
		return ewsOperationError(op, "ErrorNameResolutionNoResults", "No results were found"), nil
	}
	last := len(matches) <= 100
	if !last {
		matches = matches[:100]
	}
	full := extractAttr(body, "ResolveNames", "ReturnFullContactData") == "true"
	for _, entry := range matches {
		b.WriteString("<t:Resolution><t:Mailbox>" + entry.mailbox() + "</t:Mailbox>")
		if full || entry.itemID != "" {
			source := "ActiveDirectory"
			if entry.itemID != "" {
				source = "Store"
			}
			b.WriteString("<t:Contact><t:DisplayName>" + xmlEscape(entry.name) + "</t:DisplayName><t:EmailAddresses><t:Entry Key=\"EmailAddress1\">SMTP:" + xmlEscape(entry.email) + "</t:Entry></t:EmailAddresses><t:ContactSource>" + source + "</t:ContactSource></t:Contact>")
		}
		b.WriteString("</t:Resolution>")
	}
	class, code := "Success", "NoError"
	if len(matches) > 1 {
		class, code = "Warning", "ErrorNameResolutionMultipleResults"
	}
	return fmt.Sprintf(`<m:ResolveNamesResponse%s><m:ResponseMessages><m:ResolveNamesResponseMessage ResponseClass="%s"><m:ResponseCode>%s</m:ResponseCode><m:ResolutionSet TotalItemsInView="%d" IncludesLastItemInRange="%t">%s</m:ResolutionSet></m:ResolveNamesResponseMessage></m:ResponseMessages></m:ResolveNamesResponse>`, ewsNamespaces, class, code, len(matches), last, b.String()), nil
}
func (n *protocolNode) findValue(name string) string {
	if child := n.find(name); child != nil {
		return child.Text
	}
	return ""
}
