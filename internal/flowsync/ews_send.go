// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

func ewsSuccess(op string) string {
	return fmt.Sprintf(`<m:%sResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"><m:ResponseMessages><m:%sResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode></m:%sResponseMessage></m:ResponseMessages></m:%sResponse>`, op, op, op, op)
}

func messageFromEWS(u *storage.User, node *protocolNode) ([]byte, error) {
	if content := node.child("MimeContent"); content != nil {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(content.Text))
	}
	var b bytes.Buffer
	from := u.Email
	if value := node.child("From"); value != nil {
		address := value.find("EmailAddress")
		if address == nil {
			return nil, fmt.Errorf("invalid From")
		}
		from = address.Text
	}
	address, err := mail.ParseAddress(from)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&b, "From: %s\r\n", address.String())
	if sender := node.child("Sender"); sender != nil {
		value := sender.find("EmailAddress")
		if value == nil {
			return nil, fmt.Errorf("invalid Sender")
		}
		address, err := mail.ParseAddress(value.Text)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "Sender: %s\r\n", address.String())
	}
	for _, field := range []struct{ xml, header string }{{"ToRecipients", "To"}, {"CcRecipients", "Cc"}, {"BccRecipients", "Bcc"}} {
		var addresses []string
		if list := node.child(field.xml); list != nil {
			for _, entry := range list.Children {
				addr, err := mail.ParseAddress(entry.value("EmailAddress"))
				if err != nil {
					return nil, err
				}
				addresses = append(addresses, addr.String())
			}
		}
		if len(addresses) > 0 {
			fmt.Fprintf(&b, "%s: %s\r\n", field.header, strings.Join(addresses, ", "))
		}
	}
	subject := strings.ReplaceAll(strings.ReplaceAll(node.value("Subject"), "\r", " "), "\n", " ")
	fmt.Fprintf(&b, "Subject: %s\r\nMIME-Version: 1.0\r\n", mime.QEncoding.Encode("utf-8", subject))
	body := node.child("Body")
	kind := "text/plain"
	text := ""
	if body != nil {
		text = body.Text
		for _, a := range body.Attr {
			if a.Name.Local == "BodyType" && a.Value == "HTML" {
				kind = "text/html"
			}
		}
	}
	fmt.Fprintf(&b, "Content-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", kind)
	q := quotedprintable.NewWriter(&b)
	if _, err := q.Write([]byte(text)); err != nil {
		return nil, err
	}
	if err := q.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (h *ewsHandler) createMailItem(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	operation := doc.find("CreateItem")
	if operation == nil {
		return ewsOperationError("CreateItem", "ErrorInvalidRequest", "missing CreateItem"), nil
	}
	disposition := "SaveOnly"
	for _, a := range operation.Attr {
		if a.Name.Local == "MessageDisposition" {
			disposition = a.Value
		}
	}
	if disposition != "SaveOnly" && disposition != "SendOnly" && disposition != "SendAndSaveCopy" {
		return ewsOperationError("CreateItem", "ErrorInvalidRequest", "invalid disposition"), nil
	}
	items := operation.child("Items")
	if items == nil || len(items.Children) != 1 || items.Children[0].Name.Local != "Message" {
		return ewsOperationError("CreateItem", "ErrorInvalidRequest", "one Message is required"), nil
	}
	destination := "Drafts"
	destinationOwner := u
	if folder := operation.child("SavedItemFolderId"); folder != nil {
		id, e := h.resolveEWSFolder(ctx, u, folder.render())
		if e != nil {
			return ewsOperationError("CreateItem", "ErrorAccessDenied", "Folder unavailable"), nil
		}
		mb, e := h.store.GetMailboxByID(ctx, id)
		if e != nil {
			return ewsOperationError("CreateItem", "ErrorAccessDenied", "Folder unavailable"), nil
		}
		destination = mb.Name
		destinationOwner, e = h.store.GetUserByID(ctx, mb.UserID)
		if e != nil {
			return "", e
		}
		if disposition == "SaveOnly" {
			if e = h.mailboxRight(ctx, u.ID, mb.ID, "i"); e != nil {
				return ewsOperationError("CreateItem", "ErrorAccessDenied", "Folder write denied"), nil
			}
		}
		if disposition == "SendAndSaveCopy" && destination != "Sent" {
			return ewsOperationError("CreateItem", "ErrorInvalidFolderId", "Sent destination required"), nil
		}
	}
	defaultFrom := u
	if destinationOwner.ID != u.ID && items.Children[0].child("From") == nil && items.Children[0].child("MimeContent") == nil {
		defaultFrom = destinationOwner
	}
	raw, err := messageFromEWS(defaultFrom, items.Children[0])
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidMimeContent", "Invalid message"), nil
	}
	if items.Children[0].child("MimeContent") == nil && items.Children[0].child("Sender") == nil {
		message, e := mail.ReadMessage(bytes.NewReader(raw))
		if e != nil {
			return "", e
		}
		addresses, e := message.Header.AddressList("From")
		if e != nil || len(addresses) != 1 {
			return ewsOperationError("CreateItem", "ErrorInvalidMimeContent", "Invalid sender"), nil
		}
		from, e := h.store.ResolveRecipient(ctx, addresses[0].Address)
		if e == nil && from.ID != u.ID {
			d, e := h.store.GetMailboxDelegate(ctx, from.ID, u.ID)
			if e == nil && !d.CanSendAs && d.CanSendOnBehalf {
				raw = append([]byte("Sender: "+(&mail.Address{Address: u.Email, Name: u.DisplayName}).String()+"\r\n"), raw...)
			}
		}
	}
	if disposition != "SaveOnly" && destinationOwner.ID != u.ID {
		message, e := mail.ReadMessage(bytes.NewReader(raw))
		if e != nil {
			return "", e
		}
		from, e := message.Header.AddressList("From")
		if e != nil || len(from) != 1 {
			return ewsOperationError("CreateItem", "ErrorInvalidMimeContent", "Invalid sender"), nil
		}
		principal, e := h.store.ResolveRecipient(ctx, from[0].Address)
		if e != nil || principal.ID != destinationOwner.ID {
			return ewsOperationError("CreateItem", "ErrorAccessDenied", "Sent owner must match From"), nil
		}
	}
	if disposition != "SaveOnly" {
		if err = h.sender.send(ctx, u, "", raw, disposition == "SendAndSaveCopy"); err != nil {
			return ewsOperationError("CreateItem", "ErrorMessageSubmissionBlocked", "message was not accepted"), nil
		}
		return ewsNotificationSuccess("CreateItem", "<m:Items/>"), nil
	}
	if h.ms == nil {
		return "", fmt.Errorf("mailstore unavailable")
	}
	if err = h.ensureStandardMailFolders(ctx, destinationOwner); err != nil {
		return "", err
	}
	mb, err := h.store.GetMailbox(ctx, destinationOwner.ID, destination)
	if err != nil {
		return "", err
	}
	rel, size, err := h.ms.Deliver(destinationOwner.Email, destination, raw)
	if err != nil {
		return "", err
	}
	flags := `\Seen`
	if destination == "Drafts" || strings.EqualFold(items.Children[0].value("IsDraft"), "true") {
		flags = `\Draft \Seen`
	}
	record := &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size, Flags: flags, InternalDate: time.Now().UTC()}
	mailsearch.ApplyHeaders(record, raw)
	inserted, err := h.store.InsertMessage(ctx, record)
	if err != nil {
		return "", err
	}
	_ = mailsearch.Index(ctx, h.store, inserted.ID, raw)
	return ewsCreateResponse("Message", inserted.ID), nil
}

func (h *ewsHandler) sendItem(ctx context.Context, u *storage.User, body string) (string, error) {
	id := extractAttr(body, "ItemId", "Id")
	msg, err := h.store.GetMessageByID(ctx, id)
	if err != nil || h.mailboxRight(ctx, u.ID, msg.MailboxID, "t") != nil {
		return ewsOperationError("SendItem", "ErrorItemNotFound", "item unavailable"), nil
	}
	if !strings.Contains(msg.Flags, `\Draft`) {
		return ewsOperationError("SendItem", "ErrorInvalidOperation", "only draft messages can be sent"), nil
	}
	raw, err := h.ms.Read(msg.FilePath)
	if err != nil {
		return "", err
	}
	if doc, e := parseProtocolXML([]byte(body)); e == nil {
		if folder := doc.find("SavedItemFolderId"); folder != nil {
			destination, e := h.resolveEWSFolder(ctx, u, folder.render())
			if e != nil {
				return ewsOperationError("SendItem", "ErrorAccessDenied", "Sent unavailable"), nil
			}
			mb, e := h.store.GetMailboxByID(ctx, destination)
			if e != nil || mb.Name != "Sent" {
				return ewsOperationError("SendItem", "ErrorInvalidFolderId", "Sent required"), nil
			}
			message, e := mail.ReadMessage(bytes.NewReader(raw))
			if e != nil {
				return "", e
			}
			from, e := message.Header.AddressList("From")
			if e != nil || len(from) != 1 {
				return ewsOperationError("SendItem", "ErrorInvalidMimeContent", "Invalid sender"), nil
			}
			owner, e := h.store.ResolveRecipient(ctx, from[0].Address)
			if e != nil || owner.ID != mb.UserID {
				return ewsOperationError("SendItem", "ErrorAccessDenied", "Sent owner must match From"), nil
			}
		}
	}
	save := extractAttr(body, "SendItem", "SaveItemToFolder") == "true"
	if err = h.sender.send(ctx, u, "ews-draft:"+id, raw, save); err != nil {
		return ewsOperationError("SendItem", "ErrorMessageSubmissionBlocked", "message was not accepted"), nil
	}
	if err = h.store.DeleteMessage(ctx, id); err != nil {
		return "", err
	}
	return ewsSuccess("SendItem"), nil
}
