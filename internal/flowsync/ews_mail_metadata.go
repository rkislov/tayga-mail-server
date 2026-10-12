// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"github.com/tayga/tms/internal/storage"
	"sort"
	"strconv"
	"strings"
)

func nodeAttr(n *protocolNode, key string) string {
	if n != nil {
		for _, a := range n.Attr {
			if a.Name.Local == key {
				return a.Value
			}
		}
	}
	return ""
}
func extendedKey(n *protocolNode) string {
	keys := []string{}
	if n != nil {
		for _, a := range n.Attr {
			keys = append(keys, a.Name.Local+"="+a.Value)
		}
	}
	sort.Strings(keys)
	return "extended:" + strings.Join(keys, ";")
}
func metadataNode(n *protocolNode) *protocolNode {
	copy := &protocolNode{Name: xml.Name{Space: "http://schemas.microsoft.com/exchange/services/2006/types", Local: n.Name.Local}, Attr: n.Attr, Text: n.Text}
	for _, c := range n.Children {
		copy.Children = append(copy.Children, metadataNode(c))
	}
	return copy
}
func metadataXML(n *protocolNode) string { return metadataNode(n).render() }

func (h *ewsHandler) updateMailMetadata(ctx context.Context, u *storage.User, msg *storage.Message, body string) (string, error) {
	doc, err := parseProtocolXML([]byte(body))
	if err != nil {
		return ewsFault("ErrorInvalidRequest", "invalid XML"), nil
	}
	updates := doc.find("Updates")
	if updates == nil || len(updates.Children) == 0 {
		return ewsFault("ErrorInvalidRequest", "missing Updates"), nil
	}
	raw, err := h.store.EWSMailMetadata(ctx, msg.ID)
	if err != nil {
		return "", err
	}
	props := map[string]string{}
	if err = json.Unmarshal([]byte(raw), &props); err != nil {
		return "", err
	}
	flags := storage.ParseFlags(msg.Flags)
	invalid := func(field string) (string, error) {
		return ewsFault("ErrorInvalidPropertySet", "Unsupported or invalid mail property: "+field), nil
	}
	for _, change := range updates.Children {
		deleting := change.Name.Local == "DeleteItemField"
		if !deleting && change.Name.Local != "SetItemField" && change.Name.Local != "AppendToItemField" {
			return invalid(change.Name.Local)
		}
		field := nodeAttr(change.child("FieldURI"), "FieldURI")
		extended := change.child("ExtendedFieldURI")
		key := field
		if extended != nil {
			key = extendedKey(extended)
			if len(key) > 1024 {
				return invalid("ExtendedFieldURI")
			}
			kind := nodeAttr(extended, "PropertyType")
			if kind == "" {
				return invalid("ExtendedFieldURI")
			}
			if deleting {
				delete(props, key)
				continue
			}
			property := change.find("ExtendedProperty")
			if property == nil || extendedKey(property.child("ExtendedFieldURI")) != key {
				return invalid("ExtendedProperty")
			}
			if len(property.render()) > 16384 {
				return invalid("ExtendedProperty too large")
			}
			if change.Name.Local == "AppendToItemField" {
				return invalid("ExtendedProperty append")
			}
			props[key] = metadataXML(property)
			continue
		}
		names := map[string]string{"message:IsRead": "IsRead", "item:Flag": "Flag", "item:Categories": "Categories", "item:Importance": "Importance", "item:Sensitivity": "Sensitivity"}
		name, ok := names[field]
		if !ok {
			return invalid(field)
		}
		if change.Name.Local == "AppendToItemField" && field != "item:Categories" {
			return invalid(field + " append")
		}
		if deleting {
			if field == "message:IsRead" {
				return invalid(field)
			}
			delete(props, key)
			if field == "item:Flag" {
				flags = removeFlag(flags, `\Flagged`)
			}
			continue
		}
		value := change.find(name)
		if value == nil {
			return invalid(field)
		}
		switch field {
		case "message:IsRead":
			v, e := strconv.ParseBool(value.Text)
			if e != nil {
				return invalid(field)
			}
			if v {
				flags = addFlag(flags, `\Seen`)
			} else {
				flags = removeFlag(flags, `\Seen`)
			}
			continue
		case "item:Flag":
			status := value.value("FlagStatus")
			if status != "Flagged" && status != "Complete" && status != "NotFlagged" {
				return invalid(field)
			}
			if status == "Flagged" {
				flags = addFlag(flags, `\Flagged`)
			} else {
				flags = removeFlag(flags, `\Flagged`)
			}
		case "item:Importance":
			if value.Text != "Low" && value.Text != "Normal" && value.Text != "High" {
				return invalid(field)
			}
		case "item:Sensitivity":
			if value.Text != "Normal" && value.Text != "Personal" && value.Text != "Private" && value.Text != "Confidential" {
				return invalid(field)
			}
		case "item:Categories":
			if change.Name.Local == "AppendToItemField" && props[key] != "" {
				old, e := parseProtocolXML([]byte(props[key]))
				if e != nil {
					return "", e
				}
				value.Children = append(old.Children[0].Children, value.Children...)
			}
			seen := map[string]bool{}
			children := []*protocolNode{}
			for _, c := range value.Children {
				if c.Name.Local != "String" || len(c.Text) > 256 {
					return invalid(field)
				}
				if !seen[c.Text] {
					children = append(children, c)
					seen[c.Text] = true
				}
			}
			value.Children = children
		}
		props[key] = metadataXML(value)
	}
	data, err := json.Marshal(props)
	if err != nil {
		return "", err
	}
	if len(data) > 65536 || len(props) > 128 {
		return invalid("metadata limit")
	}
	normalized := storage.NormalizeFlags(flags)
	if err = h.store.UpdateEWSMailMetadata(ctx, msg.ID, normalized, string(data)); err != nil {
		return "", err
	}
	response := ewsUpdateResponse("Message", msg.ID)
	return strings.Replace(response, `ChangeKey="`+xmlEscape(msg.ID)+`"`, `ChangeKey="`+digest(msg.ID+normalized+string(data))+`"`, 1), nil
}
func metadataFields(raw, flags string) (sensitivity, early, extended, flag string) {
	props := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &props)
	sensitivity = props["item:Sensitivity"]
	early = props["item:Categories"] + props["item:Importance"]
	keys := []string{}
	for key := range props {
		if strings.HasPrefix(key, "extended:") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		extended += props[key]
	}
	flag = props["item:Flag"]
	if storage.HasFlag(flags, `\Flagged`) {
		if !strings.Contains(flag, ">Flagged<") {
			flag = "<t:Flag><t:FlagStatus>Flagged</t:FlagStatus></t:Flag>"
		}
	} else if strings.Contains(flag, ">Flagged<") {
		flag = ""
	}
	if flag == "" {
		flag = "<t:Flag><t:FlagStatus>NotFlagged</t:FlagStatus></t:Flag>"
	}
	return
}
