// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"fmt"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	msgmail "github.com/emersion/go-message/mail"
	"github.com/tayga/tms/internal/mailsearch"
	"io"
	"strings"
)

type messageContent struct {
	Subject, From, To, Cc, ReplyTo, Text, HTML string
	Attachments                                []messageAttachment
}
type messageAttachment struct {
	Name, ContentType, ContentID string
	Data                         []byte
}

func parseMessageContent(raw []byte) (messageContent, error) {
	var out messageContent
	r, err := msgmail.CreateReader(bytes.NewReader(raw))
	if err != nil && !message.IsUnknownCharset(err) {
		return out, err
	}
	defer r.Close()
	out.Subject = mailsearch.DecodeHeader(r.Header.Get("Subject"))
	out.From = mailsearch.DecodeHeader(r.Header.Get("From"))
	out.To = mailsearch.DecodeHeader(r.Header.Get("To"))
	out.Cc = mailsearch.DecodeHeader(r.Header.Get("Cc"))
	out.ReplyTo = mailsearch.DecodeHeader(r.Header.Get("Reply-To"))
	total := 0
	for count := 0; count <= 256; count++ {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil && !message.IsUnknownCharset(err) {
			return out, err
		}
		if count == 256 {
			return out, fmt.Errorf("too many MIME parts")
		}
		data, err := io.ReadAll(io.LimitReader(p.Body, (32<<20)+1))
		if err != nil {
			return out, err
		}
		if len(data) > 32<<20 {
			return out, fmt.Errorf("MIME part too large")
		}
		total += len(data)
		if total > 64<<20 {
			return out, fmt.Errorf("decoded message too large")
		}
		switch h := p.Header.(type) {
		case *msgmail.InlineHeader:
			media, _, _ := h.ContentType()
			if media == "text/plain" && out.Text == "" {
				out.Text = string(data)
			}
			if media == "text/html" && out.HTML == "" {
				out.HTML = string(data)
			}
			if media != "text/plain" && media != "text/html" {
				_, params, _ := h.ContentType()
				out.Attachments = append(out.Attachments, messageAttachment{Name: params["name"], ContentType: media, ContentID: strings.Trim(h.Get("Content-ID"), " <>"), Data: data})
			}
		case *msgmail.AttachmentHeader:
			name, _ := h.Filename()
			media, _, _ := h.ContentType()
			out.Attachments = append(out.Attachments, messageAttachment{Name: name, ContentType: media, ContentID: strings.Trim(h.Get("Content-ID"), " <>"), Data: data})
		}
	}
	return out, nil
}
