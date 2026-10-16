// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/tayga/tms/internal/storage"
)

func (h *easHandler) repairSyncFetchReplay(ctx context.Context, kind collectionKind, collectionID string, request *protocolNode, state collectionState) (string, error) {
	commands := request.child("Commands")
	if commands == nil || commands.child("Fetch") == nil {
		return state.Response, nil
	}
	doc, err := parseProtocolXML([]byte(state.Response))
	if err != nil {
		return "", err
	}
	if doc.find("Fetch") != nil {
		return state.Response, nil
	}
	preferences, err := preferencesFrom(request, state.Preferences)
	if err != nil {
		return "", err
	}
	fetches, err := h.fetchSyncMail(context.WithValue(ctx, preferencesKey{}, preferences), kind, collectionID, request)
	if err != nil {
		return "", err
	}
	added, err := parseProtocolXML([]byte(`<Collection xmlns="AirSync:">` + renderWritebackResponses(fetches) + `</Collection>`))
	if err != nil {
		return "", err
	}
	collection := doc.find("Collection")
	if collection == nil {
		return "", fmt.Errorf("invalid cached collection response")
	}
	responses := collection.child("Responses")
	if responses == nil {
		responses = &protocolNode{Name: xml.Name{Space: "AirSync:", Local: "Responses"}}
		// Responses precede MoreAvailable and Commands in the collection schema.
		at := len(collection.Children)
		for i, node := range collection.Children {
			if node.Name.Local == "MoreAvailable" || node.Name.Local == "Commands" {
				at = i
				break
			}
		}
		collection.Children = append(collection.Children, nil)
		copy(collection.Children[at+1:], collection.Children[at:])
		collection.Children[at] = responses
	}
	responses.Children = append(responses.Children, added.find("Responses").Children...)
	return doc.render(), nil
}

// Sync/Fetch is distinct from ItemOperations/Fetch. Apple Mail uses it to
// retrieve a truncated message even when GetChanges is zero (MS-ASCMD 2.2.3.67.2).
// The collection has already been authorized and its sync key validated.
func (h *easHandler) fetchSyncMail(ctx context.Context, kind collectionKind, collectionID string, request *protocolNode) ([]writebackResult, error) {
	commands := request.child("Commands")
	if commands == nil {
		return nil, nil
	}
	var results []writebackResult
	for _, fetch := range commands.Children {
		if fetch.Name.Local != "Fetch" {
			continue
		}
		id := fetch.value("ServerId")
		result := writebackResult{Kind: "fetch", ServerID: id, Status: 1}
		if kind != kindMail || id == "" {
			result.Status = 4
			results = append(results, result)
			continue
		}
		msg, err := h.store.GetMessageByID(ctx, id)
		if errors.Is(err, storage.ErrNotFound) || (err == nil && msg.MailboxID != collectionID) {
			result.Status = 8
			results = append(results, result)
			continue
		}
		if err != nil {
			return nil, err
		}
		if h.ms == nil {
			return nil, fmt.Errorf("mail storage unavailable")
		}
		raw, err := h.ms.Read(msg.FilePath)
		if err != nil {
			return nil, err
		}
		content, err := parseMessageContent(raw)
		if err != nil {
			return nil, err
		}
		body, bodyType, size, truncated := preferredBody(content, contextPreferences(ctx))
		full := renderMailSyncXML("", collectionID, []syncAdd{{
			ServerID: id, Subject: content.Subject, From: content.From, To: content.To,
			Date: msg.InternalDate.UTC().Format("2006-01-02T15:04:05.000Z"),
			Read: storage.HasFlag(msg.Flags, `\Seen`), Body: body, BodyType: bodyType,
			EstimatedSize: size, Truncated: truncated, Attachments: content.Attachments,
		}}, nil)
		doc, err := parseProtocolXML([]byte(full))
		if err != nil {
			return nil, err
		}
		result.ApplicationData = doc.find("ApplicationData").render()
		results = append(results, result)
	}
	return results, nil
}
