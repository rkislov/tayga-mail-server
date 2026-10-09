// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

func accountWipeAcknowledged(body []byte, wbxml bool) bool {
	if !wbxml {
		decoder := xml.NewDecoder(bytes.NewReader(body))
		inside := false
		for {
			token, err := decoder.Token()
			if err != nil {
				return false
			}
			switch node := token.(type) {
			case xml.StartElement:
				if node.Name.Local == "AccountOnlyRemoteWipe" && node.Name.Space == "Provision:" {
					inside = true
				}
				if inside && node.Name.Local == "Status" {
					var status string
					if decoder.DecodeElement(&status, &node) == nil {
						return strings.TrimSpace(status) == "1"
					}
				}
			case xml.EndElement:
				if node.Name.Local == "AccountOnlyRemoteWipe" {
					inside = false
				}
			}
		}
	}
	if len(body) < 4 || body[0] != wbxmlVersion {
		return false
	}
	r := bytes.NewReader(body)
	r.ReadByte()
	if skipMultiByteInt(r) != nil || skipMultiByteInt(r) != nil {
		return false
	}
	length, err := readMultiByteInt(r)
	if err != nil || length > uint64(r.Len()) {
		return false
	}
	io.CopyN(io.Discard, r, int64(length))
	page := byte(0)
	stack := []wbTag{}
	for {
		token, err := r.ReadByte()
		if err != nil {
			return false
		}
		switch token {
		case wbxmlSwitch:
			page, err = r.ReadByte()
			if err != nil {
				return false
			}
		case wbxmlEnd:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case wbxmlStrI:
			var text strings.Builder
			for {
				b, err := r.ReadByte()
				if err != nil {
					return false
				}
				if b == 0 {
					break
				}
				text.WriteByte(b)
			}
			if len(stack) >= 2 && stack[len(stack)-1] == (wbTag{cpProvision, 0x0b}) && stack[len(stack)-2] == (wbTag{cpProvision, 0x3b}) && text.String() == "1" {
				return true
			}
		default:
			if token&0x80 != 0 || token&0x3f < 5 {
				return false
			}
			if token&0x40 != 0 {
				stack = append(stack, wbTag{page, token & 0x3f})
			}
		}
	}
}
func encodeAccountWipeWBXML(ack bool) []byte {
	e := newWBEncoder()
	e.start(tagProvProvision)
	e.taggedStr(tagProvStatus, "1")
	if !ack {
		e.start(wbTag{cpProvision, 0x3b})
		e.end()
	}
	e.end()
	return e.bytes()
}
