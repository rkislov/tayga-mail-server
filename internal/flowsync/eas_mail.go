// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func (h *easHandler) sendMail(ctx context.Context, u *storage.User, body []byte) (string, error) {
	if extractTag(string(body), "ClientId") == "" {
		return `<SendMail xmlns="ComposeMail:"><Status>101</Status></SendMail>`, nil
	}
	raw, err := base64.StdEncoding.DecodeString(extractTag(string(body), "Mime"))
	if err != nil {
		return `<SendMail xmlns="ComposeMail:"><Status>101</Status></SendMail>`, nil
	}
	doc, err := parseProtocolXML(body)
	if err != nil {
		return "", err
	}
	if err = h.sender.send(ctx, u, extractTag(string(body), "ClientId"), raw, doc.find("SaveInSentItems") != nil); err != nil {
		return `<SendMail xmlns="ComposeMail:"><Status>110</Status></SendMail>`, nil
	}
	// MS-ASCMD: successful SendMail is HTTP 200 with an empty body.
	return "", nil
}
func (h *easHandler) itemOperations(ctx context.Context, u *storage.User, body []byte) (string, error) {
	doc, err := parseProtocolXML(body)
	if err != nil {
		return "", err
	}
	root := doc.find("ItemOperations")
	var out strings.Builder
	out.WriteString(`<ItemOperations xmlns="ItemOperations:"><Status>1</Status><Response>`)
	helper := &ewsHandler{store: h.store, ms: h.ms}
	if root != nil {
		for _, fetch := range root.Children {
			if fetch.Name.Local != "Fetch" {
				return `<ItemOperations xmlns="ItemOperations:"><Status>2</Status></ItemOperations>`, nil
			}
			if store := fetch.value("Store"); store != "" && store != "Mailbox" {
				out.WriteString(`<Fetch><Status>4</Status></Fetch>`)
				continue
			}
			ref := fetch.value("FileReference")
			if ref != "" {
				a, err := helper.attachment(ctx, u, ref)
				if err != nil {
					out.WriteString(`<Fetch><Status>6</Status></Fetch>`)
					continue
				}
				fmt.Fprintf(&out, `<Fetch><Status>1</Status><FileReference xmlns="AirSyncBase:">%s</FileReference><Properties><ContentType xmlns="AirSyncBase:">%s</ContentType><Data>%s</Data></Properties></Fetch>`, xmlEscape(ref), xmlEscape(a.ContentType), base64.StdEncoding.EncodeToString(a.Data))
				continue
			}
			id := fetch.value("ServerId")
			msg, err := h.store.GetMessageByID(ctx, id)
			if err != nil {
				out.WriteString(`<Fetch><Status>6</Status></Fetch>`)
				continue
			}
			if _, err = h.mailboxByID(ctx, u.ID, msg.MailboxID); err != nil || (fetch.value("CollectionId") != "" && fetch.value("CollectionId") != msg.MailboxID) {
				out.WriteString(`<Fetch><Status>6</Status></Fetch>`)
				continue
			}
			if h.ms == nil {
				return "", fmt.Errorf("mail storage unavailable")
			}
			raw, err := h.ms.Read(msg.FilePath)
			if err != nil {
				return "", err
			}
			content, err := parseMessageContent(raw)
			if err != nil {
				return "", err
			}
			prefs, err := preferencesFrom(fetch, syncPreferences{Bodies: []bodyPreference{{Type: "2", Size: 1 << 20, HasSize: true}}})
			if err != nil {
				out.WriteString(`<Fetch><Status>2</Status></Fetch>`)
				continue
			}
			text, kind, size, truncated := preferredBody(content, prefs)
			flag := 0
			if truncated {
				flag = 1
			}
			fmt.Fprintf(&out, `<Fetch><Status>1</Status><CollectionId xmlns="AirSync:">%s</CollectionId><ServerId xmlns="AirSync:">%s</ServerId><Properties><Body xmlns="AirSyncBase:"><Type>%s</Type><EstimatedDataSize>%d</EstimatedDataSize><Truncated>%d</Truncated><Data>%s</Data></Body></Properties></Fetch>`, xmlEscape(msg.MailboxID), xmlEscape(id), kind, size, flag, xmlEscape(text))
		}
	}
	out.WriteString(`</Response></ItemOperations>`)
	return out.String(), nil
}
