// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Protocol names are shared by the XML and binary transports. Numeric assignments
// are defined by MS-ASWBXML; unknown request tags remain opaque, never reinterpreted.
var wirePages = map[byte]string{0: "AirSync:", 1: "Contacts:", 2: "Email:", 4: "Calendar:", 5: "Move:", 6: "GetItemEstimate:", 7: "FolderHierarchy:", 12: "Contacts2:", 13: "Ping:", 14: "Provision:", 17: "AirSyncBase:", 18: "Settings:", 20: "ItemOperations:", 21: "ComposeMail:", 23: "Notes:"}
var wireTags = map[byte]map[byte]string{
	0:  {5: "Sync", 6: "Responses", 7: "Add", 8: "Change", 9: "Delete", 10: "Fetch", 11: "SyncKey", 12: "ClientId", 13: "ServerId", 14: "Status", 15: "Collection", 16: "Class", 18: "CollectionId", 19: "GetChanges", 20: "MoreAvailable", 21: "WindowSize", 22: "Commands", 23: "Options", 24: "FilterType", 27: "Conflict", 28: "Collections", 29: "ApplicationData", 30: "DeletesAsMoves", 32: "Supported", 33: "SoftDelete", 34: "MIMESupport", 35: "MIMETruncation", 36: "Wait", 37: "Limit", 38: "Partial", 41: "HeartbeatInterval"},
	5:  {5: "MoveItems", 6: "Move", 7: "SrcMsgId", 8: "SrcFldId", 9: "DstFldId", 10: "Response", 11: "Status", 12: "DstMsgId"},
	6:  {5: "GetItemEstimate", 6: "Version", 7: "Collections", 8: "Collection", 9: "Class", 10: "CollectionId", 11: "DateTime", 12: "Estimate", 13: "Response", 14: "Status"},
	7:  {7: "DisplayName", 8: "ServerId", 9: "ParentId", 10: "Type", 12: "Status", 14: "Changes", 15: "Add", 16: "Delete", 17: "Update", 18: "SyncKey", 19: "FolderCreate", 20: "FolderDelete", 21: "FolderUpdate", 22: "FolderSync", 23: "Count"},
	13: {5: "Ping", 7: "Status", 8: "HeartbeatInterval", 9: "Folders", 10: "Folder", 11: "Id", 12: "Class", 13: "MaxFolders"},
	14: {5: "Provision", 6: "Policies", 7: "Policy", 8: "PolicyType", 9: "PolicyKey", 10: "Data", 11: "Status", 12: "RemoteWipe", 13: "EASProvisionDoc", 14: "DevicePasswordEnabled", 15: "AlphanumericDevicePasswordRequired", 20: "MinDevicePasswordLength", 21: "MaxInactivityTimeDeviceLock", 22: "MaxDevicePasswordFailedAttempts", 24: "AllowSimpleDevicePassword", 27: "AllowStorageCard", 28: "AllowCamera", 29: "RequireDeviceEncryption", 59: "AccountOnlyRemoteWipe"},
	17: {5: "BodyPreference", 6: "Type", 7: "TruncationSize", 8: "AllOrNone", 10: "Body", 11: "Data", 12: "EstimatedDataSize", 13: "Truncated", 14: "Attachments", 15: "Attachment", 16: "DisplayName", 17: "FileReference", 18: "Method", 19: "ContentId", 20: "ContentLocation", 21: "IsInline", 22: "NativeBodyType", 23: "ContentType", 24: "Preview", 32: "Location", 33: "Annotation", 34: "Street", 35: "City", 36: "State", 37: "Country", 38: "PostalCode", 39: "Latitude", 40: "Longitude", 41: "Accuracy", 42: "Altitude", 43: "AltitudeAccuracy", 44: "LocationUri"},
	18: {5: "Settings", 6: "Status", 7: "Get", 8: "Set", 22: "DeviceInformation", 23: "Model", 24: "IMEI", 25: "FriendlyName", 26: "OS", 27: "OSLanguage", 28: "PhoneNumber", 29: "UserInformation", 30: "EmailAddresses", 31: "SMTPAddress", 32: "UserAgent", 33: "EnableOutboundSMS", 34: "MobileOperator", 35: "PrimarySmtpAddress", 36: "Accounts", 37: "Account", 38: "AccountId", 39: "AccountName", 40: "UserDisplayName", 41: "SendDisabled"},
	20: {5: "ItemOperations", 6: "Fetch", 7: "Store", 8: "Options", 9: "Range", 10: "Total", 11: "Properties", 12: "Data", 13: "Status", 14: "Response", 15: "Version", 16: "Schema", 17: "Part", 18: "EmptyFolderContents", 19: "DeleteSubFolders", 20: "UserName", 21: "Password"},
	21: {5: "SendMail", 6: "SmartForward", 7: "SmartReply", 8: "SaveInSentItems", 9: "ReplaceMime", 11: "Source", 12: "FolderId", 13: "ItemId", 14: "LongId", 15: "InstanceId", 16: "Mime", 17: "ClientId", 18: "Status", 19: "AccountId"},
	23: {5: "Subject", 6: "MessageClass", 7: "LastModifiedDate", 8: "Categories", 9: "Category"},
}

func wireName(page, token byte) string {
	if name := wireTags[page][token]; name != "" {
		return name
	}
	return wbFieldName(page, token)
}

// decodeWire validates the entire document before a command can mutate storage.
func decodeWire(body []byte) ([]byte, error) {
	if len(body) < 4 || body[0] != 3 {
		return nil, fmt.Errorf("invalid WBXML header")
	}
	r := bytes.NewReader(body[1:])
	public, err := readMultiByteInt(r)
	if err != nil || public != 1 {
		return nil, fmt.Errorf("invalid WBXML public ID")
	}
	charset, err := readMultiByteInt(r)
	if err != nil || charset != 106 {
		return nil, fmt.Errorf("unsupported WBXML charset")
	}
	table, err := readMultiByteInt(r)
	if err != nil || table != 0 {
		return nil, fmt.Errorf("unsupported WBXML string table")
	}
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	var stack []xml.StartElement
	page := byte(0)
	roots := 0
	for r.Len() > 0 {
		token, _ := r.ReadByte()
		switch token {
		case 0:
			page, err = r.ReadByte()
			if err != nil {
				return nil, io.ErrUnexpectedEOF
			}
		case 1:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unbalanced WBXML")
			}
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			err = enc.EncodeToken(start.End())
		case 3, 0xc3:
			if len(stack) == 0 {
				return nil, fmt.Errorf("WBXML text outside root")
			}
			var value []byte
			if token == 3 {
				var s string
				s, err = readCString(r)
				value = []byte(s)
			} else {
				var n uint64
				n, err = readMultiByteInt(r)
				if err == nil {
					if n > uint64(r.Len()) {
						return nil, io.ErrUnexpectedEOF
					}
					value = make([]byte, int(n))
					_, err = io.ReadFull(r, value)
				}
			}
			if err == nil {
				if stack[len(stack)-1].Name.Space == "ComposeMail:" && stack[len(stack)-1].Name.Local == "Mime" {
					value = []byte(base64.StdEncoding.EncodeToString(value))
				}
				err = enc.EncodeToken(xml.CharData(value))
			}
		default:
			if token&0x80 != 0 || token&0x3f < 5 {
				return nil, fmt.Errorf("unsupported WBXML token")
			}
			if len(stack) >= 64 {
				return nil, fmt.Errorf("WBXML nesting limit")
			}
			name := wireName(page, token&0x3f)
			if name == "" {
				name = fmt.Sprintf("Unknown_%d_%d", page, token&0x3f)
			}
			ns := wirePages[page]
			if ns == "" {
				ns = fmt.Sprintf("Unknown%d:", page)
			}
			start := xml.StartElement{Name: xml.Name{Space: ns, Local: name}}
			if len(stack) == 0 {
				roots++
				if roots > 1 {
					return nil, fmt.Errorf("multiple WBXML roots")
				}
			}
			err = enc.EncodeToken(start)
			if token&0x40 != 0 {
				stack = append(stack, start)
			} else if err == nil {
				err = enc.EncodeToken(start.End())
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if len(stack) != 0 || roots != 1 {
		return nil, fmt.Errorf("incomplete WBXML document")
	}
	if err = enc.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeWire(body string) ([]byte, error) {
	dec := xml.NewDecoder(strings.NewReader(body))
	enc := newWBEncoder()
	var names []xml.Name
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return enc.bytes(), nil
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			names = append(names, t.Name)
			found := false
			for page, ns := range wirePages {
				if ns != t.Name.Space {
					continue
				}
				for code := byte(5); code < 64; code++ {
					if wireName(page, code) == t.Name.Local {
						enc.start(wbTag{page, code})
						found = true
						break
					}
				}
				break
			}
			if !found {
				return nil, fmt.Errorf("unsupported response element %s:%s", t.Name.Space, t.Name.Local)
			}
		case xml.EndElement:
			names = names[:len(names)-1]
			enc.end()
		case xml.CharData:
			if len(t) > 0 {
				if len(names) > 0 && ((names[len(names)-1].Space == "ItemOperations:" && names[len(names)-1].Local == "Data") || (names[len(names)-1].Space == "ComposeMail:" && names[len(names)-1].Local == "Mime")) {
					raw, e := base64.StdEncoding.DecodeString(string(t))
					if e != nil {
						return nil, e
					}
					enc.buf.WriteByte(wbxmlOpaque)
					writeMBInt(&enc.buf, uint64(len(raw)))
					enc.buf.Write(raw)
				} else {
					enc.str(string(t))
				}
			}
		}
	}
}

func validateCommandXML(body []byte, command string) error {
	d := xml.NewDecoder(bytes.NewReader(body))
	depth, roots := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if !strings.EqualFold(t.Name.Local, command) {
					return fmt.Errorf("wrong command")
				}
			}
			depth++
			if depth > 64 {
				return fmt.Errorf("nesting limit")
			}
		case xml.EndElement:
			depth--
		}
	}
	if roots != 1 || depth != 0 {
		return fmt.Errorf("invalid root")
	}
	return nil
}

func writeMBInt(w *bytes.Buffer, value uint64) {
	var b [10]byte
	i := len(b) - 1
	b[i] = byte(value & 127)
	for value>>7 != 0 {
		value >>= 7
		i--
		b[i] = byte(value&127) | 128
	}
	w.Write(b[i:])
}
