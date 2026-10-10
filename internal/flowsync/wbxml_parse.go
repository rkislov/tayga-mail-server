// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bytes"
	"io"
)

// parseWBXMLClientOps walks an ActiveSync Sync request and extracts Add/Change/Delete ops.
func parseWBXMLClientOps(body []byte) []clientOp {
	if len(body) < 4 || body[0] != wbxmlVersion {
		return nil
	}
	r := bytes.NewReader(body)
	_, _ = r.ReadByte() // version
	if err := skipMultiByteInt(r); err != nil {
		return nil
	}
	if err := skipMultiByteInt(r); err != nil {
		return nil
	}
	stLen, err := readMultiByteInt(r)
	if err != nil {
		return nil
	}
	if stLen > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(stLen)); err != nil {
			return nil
		}
	}

	page := byte(0)
	var stack []wbFrame
	var ops []clientOp
	var cur *clientOp

	flush := func() {
		if cur != nil {
			ops = append(ops, *cur)
			cur = nil
		}
	}

	for {
		b, err := r.ReadByte()
		if err != nil {
			flush()
			return ops
		}
		switch b {
		case wbxmlSwitch:
			p, err := r.ReadByte()
			if err != nil {
				flush()
				return ops
			}
			page = p
		case wbxmlEnd:
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.kind != "" && cur != nil && cur.Kind == top.kind {
				flush()
			}
		case wbxmlStrI:
			s, err := readCString(r)
			if err != nil {
				flush()
				return ops
			}
			if len(stack) == 0 || cur == nil {
				continue
			}
			top := stack[len(stack)-1]
			switch top.name {
			case "ClientId":
				cur.ClientID = s
			case "ServerId":
				cur.ServerID = s
			default:
				if top.name != "" && top.name != "Add" && top.name != "Change" && top.name != "Delete" &&
					top.name != "ApplicationData" && top.name != "Commands" {
					if cur.Fields == nil {
						cur.Fields = map[string]string{}
					}
					cur.Fields[top.name] = s
				}
			}
		case wbxmlOpaque:
			n, err := readMultiByteInt(r)
			if err != nil {
				flush()
				return ops
			}
			_, _ = io.CopyN(io.Discard, r, int64(n))
		default:
			tagID := b & 0x3F
			hasContent := b&0x40 != 0
			hasAttrs := b&0x80 != 0
			if hasAttrs {
				for {
					ab, err := r.ReadByte()
					if err != nil {
						flush()
						return ops
					}
					if ab == wbxmlEnd {
						break
					}
					if ab == wbxmlStrI {
						_, _ = readCString(r)
					}
				}
			}
			name := wbFieldName(page, tagID)
			kind := ""
			switch name {
			case "Add", "Change", "Delete":
				kind = stringsToLower(name)
				flush()
				cur = &clientOp{Kind: kind, Fields: map[string]string{}}
			}
			if hasContent {
				stack = append(stack, wbFrame{name: name, kind: kind})
			}
		}
	}
}

type wbFrame struct {
	name string
	kind string // set for Add/Change/Delete frames
}

func stringsToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func wbFieldName(page, tagID byte) string {
	switch page {
	case cpAirSync:
		switch tagID {
		case 0x07:
			return "Add"
		case 0x08:
			return "Change"
		case 0x09:
			return "Delete"
		case 0x0C:
			return "ClientId"
		case 0x0D:
			return "ServerId"
		case 0x12:
			return "CollectionId"
		case 0x10:
			return "Class"
		case 0x0B:
			return "SyncKey"
		case 0x16:
			return "Commands"
		case 0x1D:
			return "ApplicationData"
		}
	case cpEmail:
		return map[byte]string{0x16: "To", 0x18: "From", 0x14: "Subject", 0x17: "Cc", 0x19: "ReplyTo", 0x0F: "DateReceived", 0x12: "Importance", 0x15: "Read", 0x13: "MessageClass"}[tagID]
	case cpCalendar:
		return map[byte]string{0x06: "AllDayEvent", 0x0D: "BusyStatus", 0x11: "DTStamp", 0x12: "EndTime", 0x17: "Location", 0x18: "MeetingStatus", 0x19: "OrganizerEmail", 0x1A: "OrganizerName", 0x26: "Subject", 0x28: "UID", 0x27: "StartTime", 0x25: "Sensitivity", 0x05: "Timezone"}[tagID]
	case cpContacts:
		return map[byte]string{0x05: "Anniversary", 0x06: "AssistantName", 0x08: "Birthday", 0x0D: "BusinessAddressCity", 0x11: "BusinessAddressStreet", 0x12: "BusinessFaxNumber", 0x13: "BusinessPhoneNumber", 0x19: "CompanyName", 0x1A: "Department", 0x1B: "Email1Address", 0x1C: "Email2Address", 0x1D: "Email3Address", 0x1E: "FileAs", 0x1F: "FirstName", 0x27: "HomePhoneNumber", 0x28: "JobTitle", 0x29: "LastName", 0x2A: "MiddleName", 0x2B: "MobilePhoneNumber", 0x2C: "OfficeLocation", 0x37: "WebPage"}[tagID]
	case 12:
		if tagID == 0x0D {
			return "NickName"
		}
	case cpGetItemEstimate:
		if tagID == 0x0A {
			return "CollectionId"
		}
	case 17:
		return map[byte]string{0x0A: "Body", 0x06: "BodyType", 0x0B: "Data"}[tagID]
	}
	return ""
}
