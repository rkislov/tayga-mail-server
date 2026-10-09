// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// ActiveSync WBXML (MS-ASWBXML) minimal codec — original FlowSync encoding path.
// Code pages cover FolderHierarchy, AirSync, Email, Provision, Ping, GetItemEstimate.

const (
	wbxmlVersion  = 0x03
	wbxmlPublicID = 0x01
	wbxmlCharset  = 0x6A // UTF-8
	wbxmlSwitch   = 0x00
	wbxmlEnd      = 0x01
	wbxmlStrI     = 0x03
	wbxmlOpaque   = 0xC3
)

// Code pages (MS-ASWBXML).
const (
	cpAirSync         = 0
	cpContacts        = 1
	cpEmail           = 2
	cpCalendar        = 4
	cpFolderHierarchy = 5
	cpGetItemEstimate = 6
	cpPing            = 13
	cpProvision       = 14
)

type wbTag struct {
	page byte
	code byte
}

var (
	// AirSync (0)
	tagSync            = wbTag{cpAirSync, 0x05}
	tagAdd             = wbTag{cpAirSync, 0x07}
	tagChange          = wbTag{cpAirSync, 0x08}
	tagDelete          = wbTag{cpAirSync, 0x09}
	tagClientID        = wbTag{cpAirSync, 0x0C}
	tagSyncKey         = wbTag{cpAirSync, 0x0B}
	tagServerID        = wbTag{cpAirSync, 0x0D}
	tagStatus          = wbTag{cpAirSync, 0x0E}
	tagCollection      = wbTag{cpAirSync, 0x0F}
	tagClass           = wbTag{cpAirSync, 0x10}
	tagCollectionID    = wbTag{cpAirSync, 0x12}
	tagCommands        = wbTag{cpAirSync, 0x16}
	tagCollections     = wbTag{cpAirSync, 0x1C}
	tagApplicationData = wbTag{cpAirSync, 0x1D}

	// Contacts (1)
	tagContactBusinessPhone = wbTag{cpContacts, 0x11}
	tagContactCompany       = wbTag{cpContacts, 0x16}
	tagContactEmail1        = wbTag{cpContacts, 0x1A}
	tagContactEmail2        = wbTag{cpContacts, 0x1B}
	tagContactFileAs        = wbTag{cpContacts, 0x1D}
	tagContactFirst         = wbTag{cpContacts, 0x1E}
	tagContactHomePhone     = wbTag{cpContacts, 0x20}
	tagContactJobTitle      = wbTag{cpContacts, 0x26}
	tagContactLast          = wbTag{cpContacts, 0x30}
	tagContactMobile        = wbTag{cpContacts, 0x36}

	// Email (2)
	tagEmailFrom         = wbTag{cpEmail, 0x07}
	tagEmailSubject      = wbTag{cpEmail, 0x08}
	tagEmailDateReceived = wbTag{cpEmail, 0x0F}
	tagEmailRead         = wbTag{cpEmail, 0x13}

	// Calendar (4)
	tagCalAllDay    = wbTag{cpCalendar, 0x06}
	tagCalEndTime   = wbTag{cpCalendar, 0x16}
	tagCalLocation  = wbTag{cpCalendar, 0x1D}
	tagCalSubject   = wbTag{cpCalendar, 0x23}
	tagCalUID       = wbTag{cpCalendar, 0x24}
	tagCalStartTime = wbTag{cpCalendar, 0x25}

	// FolderHierarchy (5)
	tagFHDisplayName = wbTag{cpFolderHierarchy, 0x07}
	tagFHServerID    = wbTag{cpFolderHierarchy, 0x08}
	tagFHParentID    = wbTag{cpFolderHierarchy, 0x09}
	tagFHType        = wbTag{cpFolderHierarchy, 0x0A}
	tagFHStatus      = wbTag{cpFolderHierarchy, 0x0C}
	tagFHChanges     = wbTag{cpFolderHierarchy, 0x0E}
	tagFHAdd         = wbTag{cpFolderHierarchy, 0x0F}
	tagFHSyncKey     = wbTag{cpFolderHierarchy, 0x12}
	tagFHFolderSync  = wbTag{cpFolderHierarchy, 0x16}
	tagFHCount       = wbTag{cpFolderHierarchy, 0x17}

	// GetItemEstimate (6)
	tagGIEGetItemEstimate = wbTag{cpGetItemEstimate, 0x05}
	tagGIECollection      = wbTag{cpGetItemEstimate, 0x08}
	tagGIEStatus          = wbTag{cpGetItemEstimate, 0x0A}
	tagGIEEstimate        = wbTag{cpGetItemEstimate, 0x0C}
	tagGIEResponse        = wbTag{cpGetItemEstimate, 0x0D}
	tagGIECollectionID    = wbTag{cpGetItemEstimate, 0x12}

	// Ping (13)
	tagPing       = wbTag{cpPing, 0x05}
	tagPingStatus = wbTag{cpPing, 0x08}

	// Provision (14)
	tagProvProvision  = wbTag{cpProvision, 0x05}
	tagProvPolicies   = wbTag{cpProvision, 0x06}
	tagProvPolicy     = wbTag{cpProvision, 0x07}
	tagProvPolicyType = wbTag{cpProvision, 0x08}
	tagProvPolicyKey  = wbTag{cpProvision, 0x09}
	tagProvData       = wbTag{cpProvision, 0x0A}
	tagProvStatus     = wbTag{cpProvision, 0x0B}

	// Provision policy data fields (same page)
	tagProvDevicePasswordEnabled              = wbTag{cpProvision, 0x14}
	tagProvAlphanumericDevicePasswordRequired = wbTag{cpProvision, 0x16}
	tagProvDeviceEncryptionEnabled            = wbTag{cpProvision, 0x1A}
	tagProvRequireDeviceEncryption            = wbTag{cpProvision, 0x20}
	tagProvAllowSimpleDevicePassword          = wbTag{cpProvision, 0x22}
	tagProvMaxInactivityTimeDeviceLock        = wbTag{cpProvision, 0x23}
	tagProvMaxDevicePasswordFailedAttempts    = wbTag{cpProvision, 0x24}
	tagProvMinDevicePasswordLength            = wbTag{cpProvision, 0x25}
	tagProvAllowStorageCard                   = wbTag{cpProvision, 0x28}
	tagProvAllowCamera                        = wbTag{cpProvision, 0x29}
)

type wbEncoder struct {
	buf  bytes.Buffer
	page byte
}

func newWBEncoder() *wbEncoder {
	e := &wbEncoder{page: 0xFF}
	e.buf.WriteByte(wbxmlVersion)
	e.buf.WriteByte(wbxmlPublicID)
	e.buf.WriteByte(wbxmlCharset)
	e.buf.WriteByte(0x00) // empty string table
	return e
}

func (e *wbEncoder) switchPage(p byte) {
	if e.page == p {
		return
	}
	e.buf.WriteByte(wbxmlSwitch)
	e.buf.WriteByte(p)
	e.page = p
}

func (e *wbEncoder) start(t wbTag) {
	e.switchPage(t.page)
	e.buf.WriteByte((t.code & 0x3F) | 0x40) // has content
}

func (e *wbEncoder) end() { e.buf.WriteByte(wbxmlEnd) }

func (e *wbEncoder) str(s string) {
	e.buf.WriteByte(wbxmlStrI)
	e.buf.WriteString(s)
	e.buf.WriteByte(0x00)
}

func (e *wbEncoder) taggedStr(t wbTag, s string) {
	e.start(t)
	e.str(s)
	e.end()
}

func (e *wbEncoder) bytes() []byte { return e.buf.Bytes() }

func encodeFolderSyncWBXML(syncKey string, folders []folderChange) []byte {
	e := newWBEncoder()
	e.start(tagFHFolderSync)
	e.taggedStr(tagFHStatus, "1")
	e.taggedStr(tagFHSyncKey, syncKey)
	e.start(tagFHChanges)
	e.taggedStr(tagFHCount, fmt.Sprintf("%d", len(folders)))
	for _, f := range folders {
		e.start(tagFHAdd)
		e.taggedStr(tagFHServerID, f.ServerID)
		e.taggedStr(tagFHParentID, f.ParentID)
		e.taggedStr(tagFHDisplayName, f.DisplayName)
		e.taggedStr(tagFHType, fmt.Sprintf("%d", f.Type))
		e.end()
	}
	e.end() // Changes
	e.end() // FolderSync
	return e.bytes()
}

type folderChange struct {
	ServerID    string
	ParentID    string
	DisplayName string
	Type        int
}

type syncAdd struct {
	ServerID string
	Subject  string
	From     string
	Date     string
	Read     bool
}

func encodeSyncWBXML(syncKey, collectionID, class string, adds []syncAdd) []byte {
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	if class != "" {
		e.taggedStr(tagClass, class)
	}
	e.taggedStr(tagSyncKey, syncKey)
	e.taggedStr(tagCollectionID, collectionID)
	e.taggedStr(tagStatus, "1")
	e.start(tagCommands)
	for _, a := range adds {
		e.start(tagAdd)
		e.taggedStr(tagServerID, a.ServerID)
		e.start(tagApplicationData)
		e.taggedStr(tagEmailSubject, a.Subject)
		if a.From != "" {
			e.taggedStr(tagEmailFrom, a.From)
		}
		e.taggedStr(tagEmailDateReceived, a.Date)
		e.taggedStr(tagEmailRead, fmt.Sprintf("%d", bool01(a.Read)))
		e.end() // ApplicationData
		e.end() // Add
	}
	e.end() // Commands
	e.end() // Collection
	e.end() // Collections
	e.end() // Sync
	return e.bytes()
}

func encodeCalendarSyncWBXML(syncKey, collectionID string, adds []calendarAdd) []byte {
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	e.taggedStr(tagClass, "Calendar")
	e.taggedStr(tagSyncKey, syncKey)
	e.taggedStr(tagCollectionID, collectionID)
	e.taggedStr(tagStatus, "1")
	e.start(tagCommands)
	for _, a := range adds {
		e.start(tagAdd)
		e.taggedStr(tagServerID, a.ServerID)
		e.start(tagApplicationData)
		e.taggedStr(tagCalSubject, a.Subject)
		if a.Location != "" {
			e.taggedStr(tagCalLocation, a.Location)
		}
		if a.StartTime != "" {
			e.taggedStr(tagCalStartTime, a.StartTime)
		}
		if a.EndTime != "" {
			e.taggedStr(tagCalEndTime, a.EndTime)
		}
		e.taggedStr(tagCalUID, a.UID)
		e.taggedStr(tagCalAllDay, fmt.Sprintf("%d", bool01(a.AllDay)))
		e.end()
		e.end()
	}
	e.end()
	e.end()
	e.end()
	e.end()
	return e.bytes()
}

func encodeContactsSyncWBXML(syncKey, collectionID string, adds []contactAdd) []byte {
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	e.taggedStr(tagClass, "Contacts")
	e.taggedStr(tagSyncKey, syncKey)
	e.taggedStr(tagCollectionID, collectionID)
	e.taggedStr(tagStatus, "1")
	e.start(tagCommands)
	for _, a := range adds {
		e.start(tagAdd)
		e.taggedStr(tagServerID, a.ServerID)
		e.start(tagApplicationData)
		e.taggedStr(tagContactFileAs, a.FileAs)
		if a.FirstName != "" {
			e.taggedStr(tagContactFirst, a.FirstName)
		}
		if a.LastName != "" {
			e.taggedStr(tagContactLast, a.LastName)
		}
		if a.Email1 != "" {
			e.taggedStr(tagContactEmail1, a.Email1)
		}
		if a.Mobile != "" {
			e.taggedStr(tagContactMobile, a.Mobile)
		}
		e.end()
		e.end()
	}
	e.end()
	e.end()
	e.end()
	e.end()
	return e.bytes()
}

func encodeProvisionWBXML(policyKey string, p devicePolicy) []byte {
	e := newWBEncoder()
	e.start(tagProvProvision)
	e.taggedStr(tagProvStatus, "1")
	e.start(tagProvPolicies)
	e.start(tagProvPolicy)
	e.taggedStr(tagProvPolicyType, "MS-EAS-Provisioning-WBXML")
	e.taggedStr(tagProvStatus, "1")
	e.taggedStr(tagProvPolicyKey, policyKey)
	e.start(tagProvData)
	e.taggedStr(tagProvDevicePasswordEnabled, fmt.Sprintf("%d", bool01(p.DevicePasswordEnabled)))
	e.taggedStr(tagProvMinDevicePasswordLength, fmt.Sprintf("%d", p.MinDevicePasswordLength))
	e.taggedStr(tagProvMaxInactivityTimeDeviceLock, fmt.Sprintf("%d", p.MaxInactivityTimeDeviceLock))
	e.taggedStr(tagProvMaxDevicePasswordFailedAttempts, fmt.Sprintf("%d", p.MaxDevicePasswordFailedAttempts))
	e.taggedStr(tagProvAllowSimpleDevicePassword, fmt.Sprintf("%d", bool01(p.AllowSimpleDevicePassword)))
	e.taggedStr(tagProvAlphanumericDevicePasswordRequired, fmt.Sprintf("%d", bool01(p.AlphanumericDevicePasswordRequired)))
	e.taggedStr(tagProvRequireDeviceEncryption, fmt.Sprintf("%d", bool01(p.RequireDeviceEncryption)))
	e.taggedStr(tagProvDeviceEncryptionEnabled, fmt.Sprintf("%d", bool01(p.RequireDeviceEncryption)))
	e.taggedStr(tagProvAllowStorageCard, fmt.Sprintf("%d", bool01(p.AllowStorageCard)))
	e.taggedStr(tagProvAllowCamera, fmt.Sprintf("%d", bool01(p.AllowCamera)))
	e.end() // Data
	e.end()
	e.end()
	e.end()
	return e.bytes()
}

func encodePingWBXML() []byte {
	e := newWBEncoder()
	e.start(tagPing)
	e.taggedStr(tagPingStatus, "1")
	e.end()
	return e.bytes()
}

func encodeItemEstimateWBXML(collectionID string, estimate int) []byte {
	e := newWBEncoder()
	e.start(tagGIEGetItemEstimate)
	e.start(tagGIEResponse)
	e.taggedStr(tagGIEStatus, "1")
	e.start(tagGIECollection)
	e.taggedStr(tagGIECollectionID, collectionID)
	e.taggedStr(tagGIEEstimate, fmt.Sprintf("%d", estimate))
	e.end()
	e.end()
	e.end()
	return e.bytes()
}

func requestWantsWBXML(contentType, accept string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "vnd.ms-sync.wbxml") || strings.Contains(ct, "application/vnd.ms-sync.wbxml") {
		return true
	}
	if strings.Contains(strings.ToLower(accept), "vnd.ms-sync.wbxml") {
		return true
	}
	return len(body) > 0 && body[0] == wbxmlVersion
}

// extractWBXMLTagString walks ActiveSync WBXML and returns the first inline string
// for a tag matching local name heuristics used by Sync (CollectionId).
func extractWBXMLTagString(body []byte, wantLocal string) string {
	if len(body) < 4 || body[0] != wbxmlVersion {
		return ""
	}
	r := bytes.NewReader(body)
	_, _ = r.ReadByte() // version
	if err := skipMultiByteInt(r); err != nil {
		return ""
	}
	if err := skipMultiByteInt(r); err != nil { // charset
		return ""
	}
	stLen, err := readMultiByteInt(r)
	if err != nil {
		return ""
	}
	if stLen > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(stLen)); err != nil {
			return ""
		}
	}

	page := byte(0)
	want := strings.ToLower(wantLocal)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return ""
		}
		switch b {
		case wbxmlSwitch:
			p, err := r.ReadByte()
			if err != nil {
				return ""
			}
			page = p
		case wbxmlEnd:
			continue
		case wbxmlStrI:
			if _, err := readCString(r); err != nil {
				return ""
			}
		case wbxmlOpaque:
			n, err := readMultiByteInt(r)
			if err != nil {
				return ""
			}
			if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
				return ""
			}
		default:
			tagID := b & 0x3F
			hasContent := b&0x40 != 0
			hasAttrs := b&0x80 != 0
			if hasAttrs {
				// skip attribute tokens until END — rare in AS
				for {
					ab, err := r.ReadByte()
					if err != nil {
						return ""
					}
					if ab == wbxmlEnd {
						break
					}
					if ab == wbxmlStrI {
						_, _ = readCString(r)
					}
				}
			}
			name := asTagName(page, tagID)
			if hasContent && strings.EqualFold(name, want) {
				// next token often STR_I
				nb, err := r.ReadByte()
				if err != nil {
					return ""
				}
				if nb == wbxmlStrI {
					s, err := readCString(r)
					if err != nil {
						return ""
					}
					return s
				}
				_ = r.UnreadByte()
			}
			_ = page
		}
	}
}

func asTagName(page, tagID byte) string {
	if name := wbFieldName(page, tagID); name != "" {
		return name
	}
	switch page {
	case cpFolderHierarchy:
		switch tagID {
		case 0x12:
			return "SyncKey"
		case 0x08:
			return "ServerId"
		}
	}
	return ""
}

func readCString(r *bytes.Reader) (string, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if c == 0 {
			return b.String(), nil
		}
		b.WriteByte(c)
	}
}

func readMultiByteInt(r *bytes.Reader) (uint64, error) {
	var v uint64
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		v = (v << 7) | uint64(b&0x7F)
		if b&0x80 == 0 {
			return v, nil
		}
	}
}

func skipMultiByteInt(r *bytes.Reader) error {
	_, err := readMultiByteInt(r)
	return err
}
