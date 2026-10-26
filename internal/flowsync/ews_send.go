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
	fmt.Fprintf(&b, "From: %s\r\n", (&mail.Address{Address: u.Email}).String())
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
	raw, err := messageFromEWS(u, items.Children[0])
	if err != nil {
		return ewsOperationError("CreateItem", "ErrorInvalidMimeContent", "invalid message"), nil
	}
	destination := "Drafts"
	// Validate the actual owned destination, including Thunderbird's explicit Sent copy.
	if folder := operation.child("SavedItemFolderId"); folder != nil {
		id, err := h.resolveEWSFolder(ctx, u, folder.render())
		if err != nil || h.ensureMailboxOwned(ctx, u.ID, id) != nil {
			return ewsOperationError("CreateItem", "ErrorAccessDenied", "folder unavailable"), nil
		}
		mb, e := h.store.GetMailboxByID(ctx, id)
		if e != nil || (disposition == "SendAndSaveCopy" && mb.Name != "Sent") {
			return ewsOperationError("CreateItem", "ErrorInvalidFolderId", "unsupported destination"), nil
		}
		destination = mb.Name
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
	if err = h.ensureStandardMailFolders(ctx, u); err != nil {
		return "", err
	}
	mb, err := h.store.GetMailbox(ctx, u.ID, destination)
	if err != nil {
		return "", err
	}
	rel, size, err := h.ms.Deliver(u.Email, destination, raw)
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
	if err != nil || h.ensureMailboxOwned(ctx, u.ID, msg.MailboxID) != nil {
		return ewsOperationError("SendItem", "ErrorItemNotFound", "item unavailable"), nil
	}
	if !strings.Contains(msg.Flags, `\Draft`) {
		return ewsOperationError("SendItem", "ErrorInvalidOperation", "only draft messages can be sent"), nil
	}
	raw, err := h.ms.Read(msg.FilePath)
	if err != nil {
		return "", err
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
