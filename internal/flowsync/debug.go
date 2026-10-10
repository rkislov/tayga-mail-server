// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.
package flowsync

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const traceLimit = 64 << 10

// Capture a bounded prefix without consuming or replacing the application's stream.
type traceBuffer struct {
	data      []byte
	truncated bool
}

func (b *traceBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := traceLimit - len(b.data)
	if len(p) > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}

type traceBody struct {
	io.Reader
	io.Closer
}

func numericTrace(s string) string {
	if len(s) <= 10 {
		if _, e := strconv.ParseUint(s, 10, 32); e == nil {
			return s
		}
	}
	return "[redacted]"
}
func protocolStructure(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	if data[0] == wbxmlVersion {
		return wbStructure(data)
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	var out, stack []string
	for len(out) < 256 {
		token, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			out = append(out, "[invalid or truncated XML]")
			break
		}
		switch v := token.(type) {
		case xml.StartElement:
			stack = append(stack, v.Name.Local)
			out = append(out, "<"+boundedLog(v.Name.Local)+">")
		case xml.EndElement:
			out = append(out, "</"+boundedLog(v.Name.Local)+">")
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			s := strings.TrimSpace(string(v))
			if s == "" {
				continue
			}
			value := "[redacted]"
			if len(stack) > 0 {
				switch stack[len(stack)-1] {
				case "Status", "Type", "Count", "TotalCount", "ChildFolderCount":
					value = numericTrace(s)
				case "ResponseCode":
					if s == "NoError" || strings.HasPrefix(s, "Error") {
						value = boundedLog(s)
					}
				}
			}
			out = append(out, value)
		}
	}
	return out
}
func wbStructure(data []byte) []string {
	r := bytes.NewReader(data)
	r.ReadByte()
	if skipMultiByteInt(r) != nil || skipMultiByteInt(r) != nil {
		return []string{"[invalid WBXML header]"}
	}
	size, e := readMultiByteInt(r)
	if e != nil || int64(size) > int64(r.Len()) {
		return []string{"[invalid WBXML string table]"}
	}
	r.Seek(int64(size), io.SeekCurrent)
	var out []string
	var stack []wbTag
	page := byte(0)
	for len(out) < 256 && r.Len() > 0 {
		token, e := r.ReadByte()
		if e != nil {
			break
		}
		switch token {
		case wbxmlSwitch:
			p, e := r.ReadByte()
			if e != nil {
				return append(out, "[truncated page]")
			}
			page = p
		case wbxmlEnd:
			out = append(out, "</>")
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case wbxmlStrI:
			s, e := readCString(r)
			if e != nil {
				return append(out, "[truncated string]")
			}
			value := "[redacted]"
			if len(stack) > 0 {
				tag := stack[len(stack)-1]
				if tag == tagFHStatus || tag == tagFHType || tag == tagFHCount || tag == tagProvStatus || tag == tagStatus {
					value = numericTrace(s)
				}
			}
			out = append(out, value)
		case wbxmlOpaque:
			n, e := readMultiByteInt(r)
			if e != nil || int64(n) > int64(r.Len()) {
				return append(out, "[truncated opaque]")
			}
			r.Seek(int64(n), io.SeekCurrent)
			out = append(out, "[opaque redacted]")
		default:
			if token&0x80 != 0 || token&0x3f < 5 {
				return append(out, "[unsupported WBXML token]")
			}
			tag := wbTag{page, token & 0x3f}
			out = append(out, fmt.Sprintf("<page:%d token:0x%02x>", tag.page, tag.code))
			if token&0x40 != 0 {
				stack = append(stack, tag)
			}
		}
	}
	return out
}
