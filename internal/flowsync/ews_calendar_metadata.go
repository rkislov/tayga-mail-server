// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/emersion/go-ical"
)

const calendarMetadataProperty = "X-TAYGA-EWS-CALENDAR-METADATA"

func calendarMetadataField(name string) bool {
	switch name {
	case "Sensitivity", "Importance", "ExtendedProperty", "LegacyFreeBusyStatus", "IsResponseRequested", "AllowNewTimeProposal", "ReminderIsSet", "ReminderMinutesBeforeStart", "Categories":
		return true
	}
	return false
}
func calendarMetadataKey(n *protocolNode) (string, error) {
	if n.Name.Local == "ExtendedProperty" {
		key, err := validateAttachedExtended(n)
		return "ExtendedProperty:" + key, err
	}
	if len(n.Attr) > 0 || (len(n.Children) > 0 && n.Name.Local != "Categories") {
		return "", fmt.Errorf("invalid calendar metadata")
	}
	v := n.Text
	valid := func(values string) bool { return strings.Contains(" "+values+" ", " "+v+" ") && v != "" }
	switch n.Name.Local {
	case "Sensitivity":
		if !valid("Normal Personal Private Confidential") {
			return "", fmt.Errorf("invalid sensitivity")
		}
	case "Importance":
		if !valid("Low Normal High") {
			return "", fmt.Errorf("invalid importance")
		}
	case "LegacyFreeBusyStatus":
		if !valid("Free Tentative Busy OOF WorkingElsewhere NoData") {
			return "", fmt.Errorf("invalid busy status")
		}
	case "IsResponseRequested", "AllowNewTimeProposal", "ReminderIsSet":
		if !valid("true false 0 1") {
			return "", fmt.Errorf("invalid calendar boolean")
		}
	case "ReminderMinutesBeforeStart":
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 525600 {
			return "", fmt.Errorf("invalid reminder interval")
		}
	case "Categories":
		if len(n.Children) > 100 || strings.TrimSpace(v) != "" {
			return "", fmt.Errorf("invalid categories")
		}
		for _, c := range n.Children {
			if c.Name.Local != "String" || len(c.Children) > 0 || len(c.Attr) > 0 || len(c.Text) > 255 || strings.ContainsAny(c.Text, "\r\n\x00") {
				return "", fmt.Errorf("invalid category")
			}
		}
	default:
		return "", fmt.Errorf("unsupported calendar metadata")
	}
	return n.Name.Local, nil
}
func calendarMetadata(data string) (map[string]string, error) {
	props := map[string]string{}
	raw := davTextProperty(data, calendarMetadataProperty)
	if raw == "" {
		return props, nil
	}
	b, e := base64.StdEncoding.DecodeString(raw)
	if e != nil || len(b) > 65536 {
		return nil, fmt.Errorf("invalid stored calendar metadata")
	}
	if e = json.Unmarshal(b, &props); e != nil {
		return nil, e
	}
	return props, nil
}
func applyCalendarMetadata(data string, edits []ewsCalendarEdit) (string, error) {
	props, err := calendarMetadata(data)
	if err != nil {
		return "", err
	}
	touched := false
	for _, e := range edits {
		if !calendarMetadataField(e.name) {
			continue
		}
		touched = true
		key := e.name
		if e.node != nil {
			if e.kind == "DeleteItemField" && e.name == "ExtendedProperty" {
				identity, _, e2 := attachedExtendedIdentity(e.node.child("ExtendedFieldURI"))
				if e2 != nil {
					return "", e2
				}
				key = "ExtendedProperty:" + identity
			} else {
				key, err = calendarMetadataKey(e.node)
			}
			if err != nil {
				return "", err
			}
		}
		if e.kind == "AppendToItemField" {
			return "", fmt.Errorf("calendar metadata cannot be appended")
		}
		if e.kind == "DeleteItemField" {
			delete(props, key)
		} else {
			props[key] = e.node.render()
		}
	}
	if !touched {
		return data, nil
	}
	if len(props) > 128 {
		return "", fmt.Errorf("too many calendar properties")
	}
	raw, err := json.Marshal(props)
	if err != nil || len(raw) > 65536 {
		return "", fmt.Errorf("calendar properties exceed limit")
	}
	cal, event, _, err := calendarComponent(data)
	if err != nil {
		return "", err
	}
	event.Props.SetText(calendarMetadataProperty, base64.StdEncoding.EncodeToString(raw))
	value := func(key string) string {
		n, e := parseProtocolXML([]byte(props[key]))
		if e != nil {
			return ""
		}
		if key == "ExtendedProperty:dist:Common:id:34051" || key == "ExtendedProperty:dist:Common:id:34049" {
			return n.find("Value").Text
		}
		return n.Children[0].Text
	}
	// Map interoperable fields to iCalendar as well as retaining exact EWS values.
	if v := value("Sensitivity"); v != "" {
		class := "PUBLIC"
		if v == "Private" || v == "Personal" {
			class = "PRIVATE"
		}
		if v == "Confidential" {
			class = "CONFIDENTIAL"
		}
		event.Props.SetText("CLASS", class)
	} else {
		for _, e := range edits {
			if e.name == "Sensitivity" {
				delete(event.Props, "CLASS")
			}
		}
	}
	if v := value("Importance"); v != "" {
		priority := "5"
		if v == "High" {
			priority = "1"
		}
		if v == "Low" {
			priority = "9"
		}
		event.Props.Set(&ical.Prop{Name: "PRIORITY", Value: priority})
	} else {
		for _, e := range edits {
			if e.name == "Importance" {
				delete(event.Props, "PRIORITY")
			}
		}
	}
	if v := value("LegacyFreeBusyStatus"); v != "" {
		transp := "OPAQUE"
		if v == "Free" {
			transp = "TRANSPARENT"
		}
		event.Props.SetText("TRANSP", transp)
		event.Props.SetText("X-MICROSOFT-CDO-BUSYSTATUS", strings.ToUpper(v))
	} else {
		for _, e := range edits {
			if e.name == "LegacyFreeBusyStatus" {
				delete(event.Props, "TRANSP")
				delete(event.Props, "X-MICROSOFT-CDO-BUSYSTATUS")
			}
		}
	}
	if xml := props["Categories"]; xml != "" {
		n, e := parseProtocolXML([]byte(xml))
		if e != nil {
			return "", e
		}
		delete(event.Props, "CATEGORIES")
		for _, c := range n.Children[0].Children {
			prop := &ical.Prop{Name: "CATEGORIES"}
			prop.SetText(c.Text)
			event.Props.Add(prop)
		}
	}
	var b bytes.Buffer
	if err = ical.NewEncoder(&b).Encode(cal); err != nil {
		return "", err
	}
	return b.String(), nil
}
func calendarMetadataXML(data string) (string, error) {
	props, err := calendarMetadata(data)
	if err != nil {
		return "", err
	}
	// Interoperable iCalendar values win if another calendar client edited them.
	if props["Sensitivity"] != "" {
		class := strings.ToUpper(davTextProperty(data, "CLASS"))
		value := "Normal"
		switch class {
		case "PRIVATE":
			value = "Private"
		case "CONFIDENTIAL":
			value = "Confidential"
		}
		old, e := parseProtocolXML([]byte(props["Sensitivity"]))
		if e != nil {
			return "", e
		}
		if class == "PRIVATE" && old.Children[0].Text == "Personal" {
			value = "Personal"
		}
		old.Children[0].Text = value
		props["Sensitivity"] = old.Children[0].render()
	}
	if props["Importance"] != "" {
		v, e := strconv.Atoi(davTextProperty(data, "PRIORITY"))
		value := "Normal"
		if e == nil && v > 0 && v < 5 {
			value = "High"
		}
		if e == nil && v > 5 {
			value = "Low"
		}
		n, e := parseProtocolXML([]byte(props["Importance"]))
		if e != nil {
			return "", e
		}
		n.Children[0].Text = value
		props["Importance"] = n.Children[0].render()
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if k == "LegacyFreeBusyStatus" {
			continue
		}
		n, e := parseProtocolXML([]byte(props[k]))
		if e != nil {
			return "", e
		}
		key, e := calendarMetadataKey(n.Children[0])
		if e != nil || key != k {
			return "", fmt.Errorf("invalid stored calendar field")
		}
		b.WriteString(n.Children[0].render())
	}
	// Outlook writes these reminders as named Common properties but requests
	// the corresponding standard ItemType fields when reopening the event.
	for _, r := range []struct{ field, id, kind string }{{"ReminderIsSet", "34051", "Boolean"}, {"ReminderMinutesBeforeStart", "34049", "Integer"}} {
		if props[r.field] != "" {
			continue
		}
		if raw := props["ExtendedProperty:dist:Common:id:"+r.id]; raw != "" {
			n, e := parseProtocolXML([]byte(raw))
			if e != nil {
				return "", e
			}
			x := n.find("ExtendedFieldURI")
			if nodeAttr(x, "PropertyType") == r.kind {
				fmt.Fprintf(&b, "<t:%s>%s</t:%s>", r.field, xmlEscape(n.find("Value").Text), r.field)
			}
		}
	}
	return b.String(), nil
}
