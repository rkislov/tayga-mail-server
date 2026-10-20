// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/mail"
	"sort"
	"strings"

	"github.com/emersion/go-message"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
	xhtml "golang.org/x/net/html"
)

// ComposeMail commands share submission, sender validation and ClientId replay
// protection. Success is HTTP 200 with no WBXML document (MS-ASCMD).
func (h *easHandler) composeMail(ctx context.Context, u *storage.User, body []byte, command string) (string, error) {
	status := func(code int, result string) (string, error) {
		if trace, ok := ctx.Value(traceKey{}).(*requestTrace); ok {
			trace.Result = result
		}
		return fmt.Sprintf(`<%s xmlns="ComposeMail:"><Status>%d</Status></%s>`, command, code, command), nil
	}
	doc, err := parseProtocolXML(body)
	if err != nil {
		return status(101, "compose_invalid_document")
	}
	root := doc.child(command)
	if root == nil || root.value("ClientId") == "" {
		return status(101, "compose_missing_client_id")
	}
	raw, err := base64.StdEncoding.DecodeString(root.value("Mime"))
	if err != nil || len(raw) == 0 {
		return status(101, "compose_invalid_mime")
	}
	if command != "SendMail" {
		source := root.child("Source")
		if source == nil || source.value("FolderId") == "" || source.value("ItemId") == "" || source.child("LongId") != nil {
			return status(150, "compose_source_not_found")
		}
		// A recurrence instance is a calendar operation, not an email reply.
		if source.child("InstanceId") != nil {
			return status(147, "compose_unexpected_item_class")
		}
		msg, err := h.store.GetMessageByID(ctx, source.value("ItemId"))
		if err != nil || msg.MailboxID != source.value("FolderId") {
			return status(150, "compose_source_not_found")
		}
		if _, err = h.mailboxByID(ctx, u.ID, msg.MailboxID); err != nil {
			return status(150, "compose_source_not_found")
		}
		// iPhone sends ReplaceMime: its MIME already contains the entire reply.
		// Appending the original here would duplicate body text and attachments.
		if root.child("ReplaceMime") == nil {
			if h.ms == nil {
				return status(110, "compose_source_unavailable")
			}
			original, err := h.ms.Read(msg.FilePath)
			if err != nil {
				return status(110, "compose_source_unavailable")
			}
			raw, err = mergeSmartMail(raw, original, command == "SmartForward")
			if err != nil {
				return status(110, "compose_merge_failed")
			}
		}
	}
	if err := h.sender.send(ctx, u, root.value("ClientId"), raw, root.child("SaveInSentItems") != nil); err != nil {
		return status(110, "compose_submission_failed")
	}
	if trace, ok := ctx.Value(traceKey{}).(*requestTrace); ok {
		trace.Result = "compose_accepted"
	}
	return "", nil
}

// Without ReplaceMime the device sends only its new content. Include the
// original body, and original attachments on forward (inline images on reply).
// Boundaries and headers must be deterministic for ClientId replay protection.
func mergeSmartMail(draft, original []byte, forward bool) ([]byte, error) {
	current, err := parseMessageContent(draft)
	if err != nil {
		return nil, err
	}
	previous, err := parseMessageContent(original)
	if err != nil {
		return nil, err
	}
	if current.SMIME || previous.SMIME {
		return nil, fmt.Errorf("S/MIME requires ReplaceMime")
	}
	envelope, err := mail.ReadMessage(bytes.NewReader(draft))
	if err != nil {
		return nil, err
	}
	source, err := mail.ReadMessage(bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	if !forward && envelope.Header.Get("In-Reply-To") == "" && source.Header.Get("Message-Id") != "" {
		envelope.Header["In-Reply-To"] = []string{source.Header.Get("Message-Id")}
		if envelope.Header.Get("References") == "" {
			envelope.Header["References"] = []string{strings.TrimSpace(source.Header.Get("References") + " " + source.Header.Get("Message-Id"))}
		}
	}
	var header message.Header
	keys := make([]string, 0, len(envelope.Header))
	for key := range envelope.Header {
		if strings.HasPrefix(strings.ToLower(key), "content-") || strings.EqualFold(key, "MIME-Version") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range envelope.Header[key] {
			header.Add(key, value)
		}
	}
	boundary := "flowsync-" + digest(string(draft) + "\x00" + string(original))[:40]
	header.SetContentType("multipart/mixed", map[string]string{"boundary": boundary})
	var out bytes.Buffer
	writer, err := message.CreateWriter(&out, header)
	if err != nil {
		return nil, err
	}
	var alternative message.Header
	alternative.SetContentType("multipart/alternative", map[string]string{"boundary": "alt-" + boundary})
	bodyWriter, err := writer.CreatePart(alternative)
	if err != nil {
		return nil, err
	}
	plain := func(c messageContent) string {
		if c.Text != "" {
			return c.Text
		}
		return mailsearch.ExtractHTMLText(c.HTML)
	}
	label := "\r\n\r\n---------- Original message ----------\r\nFrom: " + previous.From +
		"\r\nTo: " + previous.To + "\r\nDate: " + source.Header.Get("Date") + "\r\nSubject: " + previous.Subject + "\r\n\r\n"
	writePart := func(parent *message.Writer, header message.Header, data []byte) error {
		part, err := parent.CreatePart(header)
		if err != nil {
			return err
		}
		if _, err = part.Write(data); err != nil {
			return err
		}
		return part.Close()
	}
	var textHeader message.Header
	textHeader.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	if err = writePart(bodyWriter, textHeader, []byte(plain(current)+label+plain(previous))); err != nil {
		return nil, err
	}
	if current.HTML != "" || previous.HTML != "" {
		textHeader.SetContentType("text/html", map[string]string{"charset": "utf-8"})
		markup := "<!doctype html><html><body>" + smartMailHTML(current) +
			"<pre>" + html.EscapeString(label) + "</pre><blockquote>" + smartMailHTML(previous) + "</blockquote></body></html>"
		if err = writePart(bodyWriter, textHeader, []byte(markup)); err != nil {
			return nil, err
		}
	}
	if err = bodyWriter.Close(); err != nil {
		return nil, err
	}
	attachments := append([]messageAttachment(nil), current.Attachments...)
	for _, a := range previous.Attachments {
		if forward || a.Inline {
			attachments = append(attachments, a)
		}
	}
	for _, a := range attachments {
		var ah message.Header
		contentType := a.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		ah.SetContentType(contentType, nil)
		disposition := "attachment"
		if a.Inline {
			disposition = "inline"
		}
		ah.SetContentDisposition(disposition, map[string]string{"filename": a.Name})
		ah.Set("Content-Transfer-Encoding", "base64")
		if a.ContentID != "" {
			ah.Set("Content-ID", "<"+a.ContentID+">")
		}
		if a.ContentLocation != "" {
			ah.Set("Content-Location", a.ContentLocation)
		}
		if err = writePart(writer, ah, a.Data); err != nil {
			return nil, err
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Extract body markup rather than nesting complete HTML documents. Preserve
// embedded styles and body presentation attributes when quoting a rich message.
func smartMailHTML(c messageContent) string {
	if c.HTML == "" {
		return "<pre>" + html.EscapeString(c.Text) + "</pre>"
	}
	doc, err := xhtml.Parse(strings.NewReader(c.HTML))
	if err != nil {
		return "<pre>" + html.EscapeString(c.Text) + "</pre>"
	}
	var out strings.Builder
	var visit func(*xhtml.Node)
	visit = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && (n.Data == "style" || n.Data == "body") {
			if n.Data == "body" {
				n.Data = "div"
			}
			_ = xhtml.Render(&out, n)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return out.String()
}
