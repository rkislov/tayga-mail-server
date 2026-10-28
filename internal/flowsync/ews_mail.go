// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func ewsAddress(field, value string) string {
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "<t:%s>", field)
	for _, a := range addresses {
		fmt.Fprintf(&out, `<t:Mailbox><t:Name>%s</t:Name><t:EmailAddress>%s</t:EmailAddress><t:RoutingType>SMTP</t:RoutingType></t:Mailbox>`, xmlEscape(a.Name), xmlEscape(a.Address))
	}
	fmt.Fprintf(&out, "</t:%s>", field)
	return out.String()
}

// DateTimeSent is the message's RFC 5322 Date, not its import/receipt time.
// Native EWS clients use this property to populate the message-list Date column.
func ewsMailSentDate(sent, received time.Time) string {
	if sent.IsZero() {
		sent = received
	}
	return sent.UTC().Format("2006-01-02T15:04:05Z")
}

func (h *ewsHandler) getMailItem(ctx context.Context, u *storage.User, msg *storage.Message, request string) (string, error) {
	if err := h.ensureMailboxOwned(ctx, u.ID, msg.MailboxID); err != nil {
		return ewsFault("ErrorAccessDenied", "forbidden"), nil
	}
	if h.ms == nil {
		return "", fmt.Errorf("mail storage unavailable")
	}
	shape, err := parseEWSMailShape(request)
	if err != nil {
		return "", err
	}
	metadata, err := h.store.EWSMailMetadata(ctx, msg.ID)
	if err != nil {
		return "", err
	}
	var raw []byte
	if shape.mime || shape.wants("item:Body") || shape.wants("item:Attachments") {
		raw, err = h.ms.Read(msg.FilePath)
		if err != nil {
			return "", err
		}
	}
	// Native Thunderbird requests IdOnly + IncludeMimeContent when opening mail.
	// Pass the original bytes through, even when its MIME is not previewable.
	if shape.idOnly && len(shape.fields) == 0 && len(shape.extended) == 0 {
		mime := ""
		if shape.mime {
			mime = `<t:MimeContent CharacterSet="UTF-8">` + base64.StdEncoding.EncodeToString(raw) + `</t:MimeContent>`
		}
		return ewsNotificationSuccess("GetItem", `<m:Items><t:Message>`+mime+`<t:ItemId Id="`+xmlEscape(msg.ID)+`" ChangeKey="`+digest(msg.ID+msg.Flags+metadata)+`"/></t:Message></m:Items>`), nil
	}
	var content messageContent
	if shape.wants("item:Body") || shape.wants("item:Attachments") {
		content, err = parseMessageContent(raw)
	} else {
		reader, openErr := h.ms.Open(msg.FilePath)
		if openErr != nil {
			return "", openErr
		}
		content, err = parseMessageMetadata(reader)
		reader.Close()
	}
	if err != nil {
		return "", err
	}
	body, kind := content.Text, "Text"
	if content.HTML != "" && extractTag(request, "BodyType") != "Text" {
		body, kind = content.HTML, "HTML"
	}
	if kind == "Text" && body == "" && content.HTML != "" {
		body = mailsearch.ExtractHTMLText(content.HTML)
	}
	sensitivity, early, extended, flag := metadataFields(metadata, msg.Flags)
	var out strings.Builder
	out.WriteString(`<m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages><m:GetItemResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Items><t:Message>`)
	if shape.mime {
		fmt.Fprintf(&out, `<t:MimeContent CharacterSet="UTF-8">%s</t:MimeContent>`, base64.StdEncoding.EncodeToString(raw))
	}
	fmt.Fprintf(&out, `<t:ItemId Id="%s" ChangeKey="%s"/><t:ItemClass>IPM.Note</t:ItemClass><t:Subject>%s</t:Subject>%s`, xmlEscape(msg.ID), digest(msg.ID+msg.Flags+metadata), xmlEscape(content.Subject), sensitivity)
	if shape.wants("item:Body") {
		fmt.Fprintf(&out, `<t:Body BodyType="%s">%s</t:Body>`, kind, xmlEscape(body))
	}
	if shape.wants("item:Attachments") && len(content.Attachments) > 0 {
		out.WriteString(`<t:Attachments>`)
		for i, a := range content.Attachments {
			fmt.Fprintf(&out, `<t:FileAttachment><t:AttachmentId Id="%s:%d"/><t:Name>%s</t:Name><t:ContentType>%s</t:ContentType><t:Size>%d</t:Size><t:IsInline>%t</t:IsInline></t:FileAttachment>`, xmlEscape(msg.ID), i, xmlEscape(a.Name), xmlEscape(a.ContentType), len(a.Data), a.ContentID != "")
		}
		out.WriteString(`</t:Attachments>`)
	}
	fmt.Fprintf(&out, `<t:DateTimeSent>%s</t:DateTimeSent><t:DateTimeReceived>%s</t:DateTimeReceived><t:Size>%d</t:Size>%s<t:HasAttachments>%t</t:HasAttachments>%s%s`, ewsMailSentDate(content.SentDate, msg.InternalDate), msg.InternalDate.UTC().Format("2006-01-02T15:04:05Z"), msg.Size, early, len(content.Attachments) > 0, extended, flag)
	out.WriteString(ewsAddress("ToRecipients", content.To))
	out.WriteString(ewsAddress("CcRecipients", content.Cc))
	out.WriteString(ewsAddress("From", content.From))
	fmt.Fprintf(&out, `<t:IsRead>%t</t:IsRead>`, storage.HasFlag(msg.Flags, `\Seen`))
	out.WriteString(`</t:Message></m:Items></m:GetItemResponseMessage></m:ResponseMessages></m:GetItemResponse>`)
	return shape.project(out.String())
}
func (h *ewsHandler) attachment(ctx context.Context, u *storage.User, reference string) (messageAttachment, error) {
	if strings.HasPrefix(reference, "calendar:") {
		return readCalendarAttachment(ctx, h.store, h.ms, u, reference)
	}
	parts := strings.Split(reference, ":")
	if len(parts) != 2 {
		return messageAttachment{}, storage.ErrNotFound
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 {
		return messageAttachment{}, storage.ErrNotFound
	}
	msg, err := h.store.GetMessageByID(ctx, parts[0])
	if err != nil {
		return messageAttachment{}, err
	}
	if err = h.ensureMailboxOwned(ctx, u.ID, msg.MailboxID); err != nil {
		return messageAttachment{}, err
	}
	if h.ms == nil {
		return messageAttachment{}, fmt.Errorf("mail storage unavailable")
	}
	raw, err := h.ms.Read(msg.FilePath)
	if err != nil {
		return messageAttachment{}, err
	}
	content, err := parseMessageContent(raw)
	if err != nil {
		return messageAttachment{}, err
	}
	if index >= len(content.Attachments) {
		return messageAttachment{}, storage.ErrNotFound
	}
	return content.Attachments[index], nil
}
func (h *ewsHandler) getAttachment(ctx context.Context, u *storage.User, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	ids := doc.find("AttachmentIds")
	var out strings.Builder
	out.WriteString(`<m:GetAttachmentResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><m:ResponseMessages>`)
	if ids != nil {
		for _, id := range ids.Children {
			reference := extractAttr(id.render(), "AttachmentId", "Id")
			a, err := h.attachment(ctx, u, reference)
			if err != nil {
				out.WriteString(`<m:GetAttachmentResponseMessage ResponseClass="Error"><m:ResponseCode>ErrorInvalidAttachmentId</m:ResponseCode></m:GetAttachmentResponseMessage>`)
				continue
			}
			fmt.Fprintf(&out, `<m:GetAttachmentResponseMessage ResponseClass="Success"><m:ResponseCode>NoError</m:ResponseCode><m:Attachments><t:FileAttachment><t:AttachmentId Id="%s"/><t:Name>%s</t:Name><t:ContentType>%s</t:ContentType><t:Content>%s</t:Content></t:FileAttachment></m:Attachments></m:GetAttachmentResponseMessage>`, xmlEscape(reference), xmlEscape(a.Name), xmlEscape(a.ContentType), base64.StdEncoding.EncodeToString(a.Data))
		}
	}
	out.WriteString(`</m:ResponseMessages></m:GetAttachmentResponse>`)
	return out.String(), nil
}
