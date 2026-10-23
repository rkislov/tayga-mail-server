// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strconv"
	"strings"
)

func (h *easHandler) sendMail(ctx context.Context, u *storage.User, body []byte) (string, error) {
	return h.composeMail(ctx, u, body, "SendMail")
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
				var a messageAttachment
				var err error
				if strings.HasPrefix(ref, "calendar:") {
					a, err = h.calendarAttachment(ctx, u, ref)
				} else {
					a, err = helper.attachment(ctx, u, ref)
				}
				if err != nil {
					out.WriteString(`<Fetch><Status>6</Status></Fetch>`)
					continue
				}
				data, rangeXML, status := attachmentByteRange(a.Data, fetch.child("Options"))
				if status != 1 {
					fmt.Fprintf(&out, `<Fetch><Status>%d</Status></Fetch>`, status)
					continue
				}
				fmt.Fprintf(&out, `<Fetch><Status>1</Status><FileReference xmlns="AirSyncBase:">%s</FileReference><Properties><ContentType xmlns="AirSyncBase:">%s</ContentType>%s<Data>%s</Data></Properties></Fetch>`, xmlEscape(ref), xmlEscape(a.ContentType), rangeXML, base64.StdEncoding.EncodeToString(data))
				continue
			}
			id := fetch.value("ServerId")
			if taskID(id) {
				task, err := helper.getTask(ctx, u, id)
				collection := systemFolderID(u.ID, "tasks")
				if err != nil || (fetch.value("CollectionId") != "" && fetch.value("CollectionId") != collection) {
					out.WriteString(`<Fetch><Status>6</Status></Fetch>`)
					continue
				}
				prefs, err := preferencesFrom(fetch, syncPreferences{Bodies: []bodyPreference{{Type: "1"}}})
				if err != nil {
					out.WriteString(`<Fetch><Status>2</Status></Fetch>`)
					continue
				}
				properties, err := parseProtocolXML([]byte(taskApplicationData(*task, prefs)))
				if err != nil {
					return "", err
				}
				properties.Children[0].Name.Local = "Properties"
				properties.Children[0].Name.Space = "ItemOperations:"
				fmt.Fprintf(&out, `<Fetch><Status>1</Status><CollectionId xmlns="AirSync:">%s</CollectionId><ServerId xmlns="AirSync:">%s</ServerId>%s</Fetch>`, xmlEscape(collection), xmlEscape(id), properties.render())
				continue
			}
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
			properties := renderMailBody(syncAdd{ServerID: id, Body: text, BodyType: kind, NativeBodyType: content.nativeBodyType(), EstimatedSize: size, Truncated: truncated, Attachments: content.Attachments})
			fmt.Fprintf(&out, `<Fetch><Status>1</Status><CollectionId xmlns="AirSync:">%s</CollectionId><ServerId xmlns="AirSync:">%s</ServerId><Properties>%s</Properties></Fetch>`, xmlEscape(msg.MailboxID), xmlEscape(id), properties)
		}
	}
	out.WriteString(`</Response></ItemOperations>`)
	return out.String(), nil
}

// ItemOperations ranges are inclusive byte offsets, before base64 encoding.
// A client may resume a file download; returning the entire file for a Range
// request would corrupt the chunks it assembles.
func attachmentByteRange(data []byte, options *protocolNode) ([]byte, string, int) {
	if options == nil || len(options.Children) == 0 {
		return data, "", 1
	}
	if len(options.Children) != 1 || options.Children[0].Name.Local != "Range" {
		return nil, "", 2
	}
	parts := strings.Split(options.Children[0].Text, "-")
	if len(parts) != 2 {
		return nil, "", 8
	}
	start, e1 := strconv.ParseUint(parts[0], 10, 32)
	end, e2 := strconv.ParseUint(parts[1], 10, 32)
	if e1 != nil || e2 != nil || start > end || start >= uint64(len(data)) {
		return nil, "", 8
	}
	if end >= uint64(len(data)) {
		end = uint64(len(data)) - 1
	}
	return data[start : end+1], fmt.Sprintf(`<Range>%d-%d</Range><Total>%d</Total>`, start, end, len(data)), 1
}
