// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-message"
	msgmail "github.com/emersion/go-message/mail"

	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

func ewsCalendarAttachmentTag(kind string, properties ...string) string {
	if kind == "Message" || (len(properties) > 0 && properties[0] != "") {
		return "ItemAttachment"
	}
	return "FileAttachment"
}

// Store MIME bytes as a normal downloadable .eml, with an explicit EWS kind.
// The kind must never be inferred from a file's user-supplied media type.
func parseEWSMessageAttachment(n *protocolNode) (ewsCalendarUpload, string, string) {
	fail := func(code, message string) (ewsCalendarUpload, string, string) {
		return ewsCalendarUpload{}, code, message
	}
	seen := map[string]bool{}
	for _, field := range n.Children {
		name := field.Name.Local
		if seen[name] || (name != "Name" && name != "ContentType" && name != "IsInline" && name != "Message") || (name != "Message" && len(field.Children) != 0) {
			return fail("ErrorInvalidRequest", "Unsupported or duplicate item attachment property")
		}
		seen[name] = true
	}
	name := strings.TrimSpace(n.value("Name"))
	if name == "" {
		return fail("ErrorRequiredPropertyMissing", "Attachment name required")
	}
	if len(name) > 255 || (!strings.HasSuffix(strings.ToLower(name), ".eml") && len(name) > 251) || name == "." || name == ".." || strings.ContainsAny(name, "/\\\r\n\x00") {
		return fail("ErrorInvalidRequest", "Invalid attachment name")
	}
	if v := strings.TrimSpace(n.value("IsInline")); v != "" && v != "false" && v != "0" {
		return fail("ErrorInvalidRequest", "Inline item attachments are unsupported")
	}
	if v := strings.TrimSpace(n.value("ContentType")); v != "" && v != "message/rfc822" {
		return fail("ErrorInvalidRequest", "Attached messages require message/rfc822")
	}
	message := n.child("Message")
	if message == nil {
		return fail("ErrorInvalidAttachmentId", "Only Message item attachments are supported")
	}
	data, err := ewsMessageAttachmentMIME(message)
	if err != nil {
		return fail("ErrorInvalidRequest", "Invalid or unsupported attached message properties")
	}
	if len(data) > maxEWSCalendarAttachmentBytes {
		return fail("ErrorAttachmentSizeLimitExceeded", "Attached message exceeds 32 MiB")
	}
	if len(data) == 0 {
		return fail("ErrorRequiredPropertyMissing", "MIME content required")
	}
	if _, err = parseMessageContent(data); err != nil {
		return fail("ErrorInvalidRequest", "Invalid attached MIME message")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".eml") {
		name += ".eml"
	}
	return ewsCalendarUpload{file: &storage.CalendarAttachment{Filename: name, ContentType: "message/rfc822", EWSItemType: "Message", Size: int64(len(data))}, data: data}, "", ""
}

func ewsMessageAttachmentMIME(n *protocolNode) ([]byte, error) {
	invalid := fmt.Errorf("invalid attached message")
	if mime := n.child("MimeContent"); mime != nil {
		if len(n.Children) != 1 || len(mime.Children) != 0 {
			return nil, invalid
		}
		return base64.StdEncoding.Strict().DecodeString(strings.Join(strings.Fields(mime.Text), ""))
	}
	var header msgmail.Header
	header.Set("MIME-Version", "1.0")
	header.Set("Content-Transfer-Encoding", "base64")
	media, body := "text/plain", ""
	seen := map[string]bool{}
	for _, field := range n.Children {
		name := field.Name.Local
		if seen[name] {
			return nil, invalid
		}
		seen[name] = true
		switch name {
		case "Subject":
			if len(field.Children) != 0 || strings.ContainsAny(field.Text, "\r\n\x00") {
				return nil, invalid
			}
			header.SetSubject(field.Text)
		case "Body":
			if len(field.Children) != 0 {
				return nil, invalid
			}
			switch nodeAttr(field, "BodyType") {
			case "Text":
			case "HTML":
				media = "text/html"
			default:
				return nil, invalid
			}
			body = field.Text
		case "DateTimeSent":
			if len(field.Children) != 0 {
				return nil, invalid
			}
			date, err := time.Parse(time.RFC3339, field.Text)
			if err != nil {
				return nil, invalid
			}
			header.SetDate(date)
		case "From", "ToRecipients", "CcRecipients":
			if name == "From" && len(field.Children) != 1 {
				return nil, invalid
			}
			var addresses []*mail.Address
			for _, mailbox := range field.Children {
				if mailbox.Name.Local != "Mailbox" {
					return nil, invalid
				}
				properties := map[string]bool{}
				for _, value := range mailbox.Children {
					key := value.Name.Local
					if properties[key] || len(value.Children) != 0 || (key != "Name" && key != "EmailAddress" && key != "RoutingType") || strings.ContainsAny(value.Text, "\r\n\x00") {
						return nil, invalid
					}
					properties[key] = true
				}
				if route := mailbox.value("RoutingType"); route != "" && route != "SMTP" {
					return nil, invalid
				}
				a, err := mail.ParseAddress(mailbox.value("EmailAddress"))
				if err != nil {
					return nil, invalid
				}
				a.Name = mailbox.value("Name")
				addresses = append(addresses, a)
			}
			key := map[string]string{"From": "From", "ToRecipients": "To", "CcRecipients": "Cc"}[name]
			header.SetAddressList(key, addresses)
		default:
			return nil, invalid
		}
	}
	if len(n.Children) == 0 {
		return nil, invalid
	}
	header.SetContentType(media, map[string]string{"charset": "utf-8"})
	var out bytes.Buffer
	w, err := message.CreateWriter(&out, header.Header)
	if err != nil {
		return nil, err
	}
	if _, err = w.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func ewsAttachedMessageXML(a messageAttachment, reference, request string) (string, error) {
	c, err := parseMessageContent(a.Data)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString(`<t:Message>`)
	if extractTag(request, "IncludeMimeContent") == "true" || extractTag(request, "IncludeMimeContent") == "1" {
		fmt.Fprintf(&out, `<t:MimeContent CharacterSet="UTF-8">%s</t:MimeContent>`, base64.StdEncoding.EncodeToString(a.Data))
	}
	body, kind := c.Text, "Text"
	if c.HTML != "" {
		if extractTag(request, "BodyType") == "Text" {
			body = mailsearch.ExtractHTMLText(c.HTML)
		} else {
			body, kind = c.HTML, "HTML"
		}
	}
	fmt.Fprintf(&out, `<t:ItemClass>IPM.Note</t:ItemClass><t:Subject>%s</t:Subject><t:Body BodyType="%s">%s</t:Body>`, xmlEscape(c.Subject), kind, xmlEscape(body))
	if len(c.Attachments) > 0 {
		out.WriteString(`<t:Attachments>`)
		for i, file := range c.Attachments {
			fmt.Fprintf(&out, `<t:FileAttachment><t:AttachmentId Id="%s:%d"/><t:Name>%s</t:Name><t:ContentType>%s</t:ContentType><t:Size>%d</t:Size><t:IsInline>%t</t:IsInline></t:FileAttachment>`, xmlEscape(reference), i, xmlEscape(file.Name), xmlEscape(file.ContentType), len(file.Data), file.Inline)
		}
		out.WriteString(`</t:Attachments>`)
	}
	if !c.SentDate.IsZero() {
		fmt.Fprintf(&out, `<t:DateTimeSent>%s</t:DateTimeSent>`, ewsMailSentDate(c.SentDate, c.SentDate))
	}
	fmt.Fprintf(&out, `<t:Size>%d</t:Size><t:HasAttachments>%t</t:HasAttachments>`, len(a.Data), len(c.Attachments) > 0)
	out.WriteString(ewsAddress("ToRecipients", c.To))
	out.WriteString(ewsAddress("CcRecipients", c.Cc))
	out.WriteString(ewsAddress("From", c.From))
	out.WriteString(ewsMailThreadFields(c.MessageID, c.References, c.InReplyTo))
	out.WriteString(`</t:Message>`)
	return out.String(), nil
}
