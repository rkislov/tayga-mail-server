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
	"time"
)

type messageContent struct {
	Subject, From, To, Cc, ReplyTo, Text, HTML string
	SentDate                                   time.Time
	Attachments                                []messageAttachment
	MIME                                       []byte
	SMIME                                      bool
}
type messageAttachment struct {
	Name, ContentType, ContentID string
	ContentLocation              string
	Inline                       bool
	Data                         []byte
}

// NativeBodyType describes the stored body, not a requested plain-text preview.
// Mail clients use it to decide whether opening the item should fetch HTML.
func (c messageContent) nativeBodyType() string {
	if c.HTML != "" {
		return "2"
	}
	return "1"
}

func parseMessageContent(raw []byte) (messageContent, error) {
	return readMessageContent(bytes.NewReader(raw), raw, false)
}

// Metadata discovery must not allocate/decode every attachment or reject a
// readable message merely because a MIME part exceeds the body-preview limit.
func parseMessageMetadata(source io.Reader) (messageContent, error) {
	return readMessageContent(source, nil, true)
}

func readMessageContent(source io.Reader, raw []byte, metadataOnly bool) (messageContent, error) {
	var out messageContent
	r, err := msgmail.CreateReader(source)
	if err != nil && !message.IsUnknownCharset(err) {
		return out, err
	}
	defer r.Close()
	out.MIME = raw
	out.SentDate, _ = r.Header.Date()
	media, params, _ := r.Header.ContentType()
	out.SMIME = media == "application/pkcs7-mime" || media == "application/x-pkcs7-mime" || (media == "multipart/signed" && strings.Contains(params["protocol"], "pkcs7-signature"))
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
		var data []byte
		if !metadataOnly {
			data, err = io.ReadAll(io.LimitReader(p.Body, (32<<20)+1))
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
				_, disposition, _ := h.ContentDisposition()
				name := disposition["filename"]
				if name == "" {
					name = params["name"]
				}
				out.Attachments = append(out.Attachments, messageAttachment{Name: name, ContentType: media, ContentID: strings.Trim(h.Get("Content-ID"), " <>"), ContentLocation: h.Get("Content-Location"), Inline: true, Data: data})
			}
		case *msgmail.AttachmentHeader:
			name, _ := h.Filename()
			media, _, _ := h.ContentType()
			out.Attachments = append(out.Attachments, messageAttachment{Name: name, ContentType: media, ContentID: strings.Trim(h.Get("Content-ID"), " <>"), ContentLocation: h.Get("Content-Location"), Data: data})
		}
	}
	// Some senders label CID parts as attachments even though HTML embeds them.
	for i := range out.Attachments {
		a := &out.Attachments[i]
		if (a.ContentID != "" && strings.Contains(out.HTML, "cid:"+a.ContentID)) || (a.ContentLocation != "" && strings.Contains(out.HTML, a.ContentLocation)) {
			a.Inline = true
		}
	}
	return out, nil
}
