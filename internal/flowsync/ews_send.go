// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
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
	// Keep conversation headers when a client submits structured EWS mail,
	// as well as when it submits the original MIME bytes.
	threadHeaders := map[string]string{}
	if headers := node.child("InternetMessageHeaders"); headers != nil {
		for _, header := range headers.Children {
			name := strings.ToLower(nodeAttr(header, "HeaderName"))
			switch name {
			case "references", "in-reply-to", "thread-index", "thread-topic":
				threadHeaders[name] = header.Text
			}
		}
	}
	if value := node.value("References"); value != "" {
		threadHeaders["references"] = value
	}
	if value := node.value("ConversationIndex"); value != "" {
		threadHeaders["thread-index"] = value
	}
	if value := node.value("ConversationTopic"); value != "" {
		threadHeaders["thread-topic"] = value
	}
	for _, field := range []struct{ key, name string }{{"references", "References"}, {"in-reply-to", "In-Reply-To"}} {
		if ids := conversationIDs(threadHeaders[field.key]); len(ids) > 0 {
			fmt.Fprintf(&b, "%s: %s\r\n", field.name, strings.Join(ids, " "))
		}
	}
	if value := threadHeaders["thread-index"]; value != "" {
		index := validConversationIndex(value)
		if index == nil {
			return nil, fmt.Errorf("invalid Thread-Index")
		}
		fmt.Fprintf(&b, "Thread-Index: %s\r\n", base64.StdEncoding.EncodeToString(index))
	}
	if value := threadHeaders["thread-topic"]; value != "" {
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("invalid Thread-Topic")
		}
		fmt.Fprintf(&b, "Thread-Topic: %s\r\n", mime.QEncoding.Encode("utf-8", value))
	}
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
	if items == nil || len(items.Children) != 1 {
		return ewsOperationError("CreateItem", "ErrorInvalidRequest", "one mail item is required"), nil
	}
	item := items.Children[0]
	var raw []byte
	if item.Name.Local == "ReplyToItem" || item.Name.Local == "ReplyAllToItem" {
		ref := item.child("ReferenceItemId")
		var parent *storage.Message
		if ref != nil {
			parent, err = h.store.GetMessageByID(ctx, nodeAttr(ref, "Id"))
		}
		if parent == nil || err != nil || h.ensureMailboxOwned(ctx, u.ID, parent.MailboxID) != nil {
			return ewsOperationError("CreateItem", "ErrorItemNotFound", "reply source unavailable"), nil
		}
		raw, err = h.replyFromEWS(ctx, u, item, parent)
	} else if item.Name.Local == "Message" {
		raw, err = messageFromEWS(u, item)
	} else {
		return ewsOperationError("CreateItem", "ErrorInvalidRequest", "unsupported mail item"), nil
	}
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidMimeContent", "invalid message"), nil
	}
	destination := "Drafts"
	destinationOwner := u
	// Validate the actual owned destination, including Thunderbird's explicit Sent copy.
	if folder := operation.child("SavedItemFolderId"); folder != nil {
		id, e := h.resolveEWSFolder(ctx, u, folder.render())
		if e != nil {
			return ewsOperationError("CreateItem", "ErrorAccessDenied", "Folder unavailable"), nil
		}
		mb, e := h.store.GetMailboxByID(ctx, id)
		if e != nil || (disposition == "SendAndSaveCopy" && mb.Name != "Sent") {
			return ewsOperationError("CreateItem", "ErrorInvalidFolderId", "unsupported destination"), nil
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
	}
	if destinationOwner.ID != u.ID && item.child("From") == nil && item.child("MimeContent") == nil {
		raw, err = messageFromEWS(destinationOwner, item)
		if err != nil {
			return "", err
		}
	}
	if item.child("Sender") == nil && item.child("MimeContent") == nil {
		if header, e := mail.ReadMessage(bytes.NewReader(raw)); e == nil {
			if from, e := header.Header.AddressList("From"); e == nil && len(from) == 1 {
				if principal, e := h.store.ResolveRecipient(ctx, from[0].Address); e == nil && principal.ID != u.ID {
					if grant, e := h.store.GetMailboxDelegate(ctx, principal.ID, u.ID); e == nil && !grant.CanSendAs && grant.CanSendOnBehalf {
						raw = []byte("Sender: " + (&mail.Address{Address: u.Email}).String() + "\r\n" + string(raw))
					}
				}
			}
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

// Native EWS clients can refer to the source instead of supplying a complete
// MIME message. Resolve that reference only after authorizing its mailbox.
func (h *ewsHandler) replyFromEWS(ctx context.Context, u *storage.User, item *protocolNode, parent *storage.Message) ([]byte, error) {
	if h.ms == nil {
		return nil, fmt.Errorf("mail storage unavailable")
	}
	original, err := h.ms.Read(parent.FilePath)
	if err != nil {
		return nil, err
	}
	header, err := mail.ReadMessage(bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	node := &protocolNode{Name: xml.Name{Local: "Message"}}
	for _, child := range item.Children {
		if child.Name.Local == "ReferenceItemId" {
			continue
		}
		copy := *child
		if copy.Name.Local == "NewBodyContent" {
			copy.Name.Local = "Body"
		}
		node.Children = append(node.Children, &copy)
	}
	if node.child("Subject") == nil {
		subject := readMsgHeaders(h.ms, parent.FilePath).Subject
		if !conversationReplyPrefix.MatchString(subject) {
			subject = "Re: " + subject
		}
		node.Children = append(node.Children, &protocolNode{Name: xml.Name{Local: "Subject"}, Text: subject})
	}
	seen := map[string]bool{strings.ToLower(u.Email): true}
	addRecipients := func(kind, value string) error {
		addresses, err := mail.ParseAddressList(value)
		if err != nil {
			return err
		}
		list := &protocolNode{Name: xml.Name{Local: kind}}
		for _, address := range addresses {
			key := strings.ToLower(address.Address)
			if seen[key] {
				continue
			}
			seen[key] = true
			list.Children = append(list.Children, &protocolNode{Name: xml.Name{Local: "Mailbox"}, Children: []*protocolNode{{Name: xml.Name{Local: "EmailAddress"}, Text: address.Address}}})
		}
		if len(list.Children) > 0 {
			node.Children = append(node.Children, list)
		}
		return nil
	}
	if node.child("ToRecipients") == nil {
		replyTo := header.Header.Get("Reply-To")
		if replyTo == "" {
			replyTo = header.Header.Get("From")
		}
		if item.Name.Local == "ReplyAllToItem" && header.Header.Get("To") != "" {
			replyTo += ", " + header.Header.Get("To")
		}
		if err := addRecipients("ToRecipients", replyTo); err != nil {
			return nil, err
		}
	}
	if item.Name.Local == "ReplyAllToItem" && node.child("CcRecipients") == nil && header.Header.Get("Cc") != "" {
		if err := addRecipients("CcRecipients", header.Header.Get("Cc")); err != nil {
			return nil, err
		}
	}
	raw, err := messageFromEWS(u, node)
	if err != nil {
		return nil, err
	}
	raw, err = ensureReplyHeaders(raw, readMsgHeaders(h.ms, parent.FilePath))
	if err != nil {
		return nil, err
	}
	return mergeSmartMail(raw, original, false)
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
