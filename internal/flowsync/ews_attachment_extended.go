// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Extended properties remain opaque item metadata, never interpreted as folder,
// scheduling, security, or storage settings.
func attachedExtendedIdentity(n *protocolNode) (string, string, error) {
	bad := fmt.Errorf("invalid ExtendedFieldURI")
	if n == nil || n.Name.Local != "ExtendedFieldURI" || len(n.Children) != 0 || strings.TrimSpace(n.Text) != "" {
		return "", "", bad
	}
	attrs := map[string]string{}
	for _, a := range n.Attr {
		key := a.Name.Local
		if a.Name.Space != "" || attrs[key] != "" || a.Value == "" || !strings.Contains(" PropertyTag PropertySetId DistinguishedPropertySetId PropertyName PropertyId PropertyType ", " "+key+" ") {
			return "", "", bad
		}
		attrs[key] = a.Value
	}
	kind := attrs["PropertyType"]
	base := strings.TrimSuffix(kind, "Array")
	if !strings.Contains(" ApplicationTime Binary Boolean CLSID Currency Double Float Integer Long Short String SystemTime ", " "+base+" ") || base == "" || kind == "BooleanArray" {
		return "", "", bad
	}
	number := func(value string, bits int) (uint64, error) {
		radix := 10
		if strings.HasPrefix(strings.ToLower(value), "0x") {
			radix = 16
			value = value[2:]
		}
		return strconv.ParseUint(value, radix, bits)
	}
	if tag := attrs["PropertyTag"]; tag != "" {
		v, err := number(tag, 16)
		if err != nil || len(attrs) != 2 {
			return "", "", bad
		}
		return "tag:" + strconv.FormatUint(v, 10), kind, nil
	}
	set, dist := attrs["PropertySetId"], attrs["DistinguishedPropertySetId"]
	if (set == "") == (dist == "") {
		return "", "", bad
	}
	if set != "" {
		if !attachedGUID(set) {
			return "", "", bad
		}
		set = strings.ToLower(set)
	} else {
		if !strings.Contains(" Address Appointment CalendarAssistant Common InternetHeaders Meeting PublicStrings Sharing Task UnifiedMessaging ", " "+dist+" ") {
			return "", "", bad
		}
		set = "dist:" + dist
	}
	name, id := attrs["PropertyName"], attrs["PropertyId"]
	if (name == "") == (id == "") || len(attrs) != 3 {
		return "", "", bad
	}
	if id != "" {
		v, err := number(id, 16)
		if err != nil {
			return "", "", bad
		}
		name = "id:" + strconv.FormatUint(v, 10)
	} else {
		if len(name) > 255 || strings.ContainsAny(name, "\x00\r\n") {
			return "", "", bad
		}
		name = "name:" + name
	}
	return set + ":" + name, kind, nil
}
func attachedGUID(v string) bool {
	if len(v) != 36 || v[8] != '-' || v[13] != '-' || v[18] != '-' || v[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(v, "-", ""))
	return err == nil
}
func validateAttachedExtended(n *protocolNode) (string, error) {
	bad := fmt.Errorf("invalid ExtendedProperty")
	if len(n.Attr) != 0 || len(n.Children) != 2 || strings.TrimSpace(n.Text) != "" || len(n.render()) > 16384 {
		return "", bad
	}
	field := n.Children[0]
	identity, kind, err := attachedExtendedIdentity(field)
	if err != nil {
		return "", err
	}
	values := n.Children[1]
	var list []*protocolNode
	if strings.HasSuffix(kind, "Array") {
		if values.Name.Local != "Values" || len(values.Attr) != 0 || strings.TrimSpace(values.Text) != "" || (len(values.Children) == 0 || len(values.Children) > 1024) {
			return "", bad
		}
		list = values.Children
		kind = strings.TrimSuffix(kind, "Array")
	} else {
		list = []*protocolNode{values}
	}
	for _, value := range list {
		if value.Name.Local != "Value" || len(value.Attr) != 0 || len(value.Children) != 0 {
			return "", bad
		}
		v := value.Text
		switch kind {
		case "Binary":
			_, err = base64.StdEncoding.Strict().DecodeString(strings.Join(strings.Fields(v), ""))
		case "Boolean":
			if v != "true" && v != "false" && v != "0" && v != "1" {
				return "", bad
			}
		case "CLSID":
			if !attachedGUID(v) {
				return "", bad
			}
		case "Integer", "Short", "Currency":
			bits := 64
			if kind == "Integer" {
				bits = 32
			}
			if kind == "Short" {
				bits = 16
			}
			_, err = strconv.ParseInt(v, 10, bits)
		case "Long":
			if strings.HasPrefix(v, "-") {
				_, err = strconv.ParseInt(v, 10, 64)
			} else {
				_, err = strconv.ParseUint(v, 10, 64)
			}
		case "ApplicationTime", "Double", "Float":
			bits := 64
			if kind == "Float" {
				bits = 32
			}
			var f float64
			f, err = strconv.ParseFloat(v, bits)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return "", bad
			}
		case "SystemTime":
			_, err = time.Parse(time.RFC3339, v)
		}
		if err != nil {
			return "", bad
		}
	}
	*n = *metadataNode(n)
	return identity, nil
}
func attachedExtendedFields(n *protocolNode) ([]*protocolNode, error) {
	var fields []*protocolNode
	seen := map[string]bool{}
	for _, f := range n.Children {
		if f.Name.Local != "ExtendedProperty" {
			continue
		}
		id, err := validateAttachedExtended(f)
		if err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate extended property")
		}
		seen[id] = true
		fields = append(fields, f)
		if len(fields) > 128 {
			return nil, fmt.Errorf("too many extended properties")
		}
	}
	return fields, nil
}
func attachedMessageProperties(n *protocolNode) (*protocolNode, string, error) {
	fields, err := attachedExtendedFields(n)
	if err != nil {
		return nil, "", err
	}
	copy := &protocolNode{Name: n.Name, Attr: n.Attr, Text: n.Text}
	for _, f := range n.Children {
		if f.Name.Local != "ExtendedProperty" {
			copy.Children = append(copy.Children, f)
		}
	}
	if len(fields) == 0 {
		return copy, "", nil
	}
	props := (&protocolNode{Name: xml.Name{Space: ewsTypesNS, Local: "Message"}, Children: fields}).render()
	return copy, props, nil
}
