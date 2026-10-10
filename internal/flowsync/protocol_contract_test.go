// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// Literal wire values from MS-ASWBXML, independent of encoder constants.
func TestWireFieldContract(t *testing.T) {
	for _, tc := range []struct {
		tag         wbTag
		page, token byte
		name        string
	}{
		{tagEmailFrom, 2, 0x18, "From"}, {tagEmailSubject, 2, 0x14, "Subject"}, {tagEmailRead, 2, 0x15, "Read"},
		{tagContactFileAs, 1, 0x1e, "FileAs"}, {tagContactFirst, 1, 0x1f, "FirstName"}, {tagContactLast, 1, 0x29, "LastName"},
		{tagContactEmail1, 1, 0x1b, "Email1Address"}, {tagContactMobile, 1, 0x2b, "MobilePhoneNumber"},
		{tagCalEndTime, 4, 0x12, "EndTime"}, {tagCalLocation, 4, 0x17, "Location"}, {tagCalSubject, 4, 0x26, "Subject"}, {tagCalStartTime, 4, 0x27, "StartTime"}, {tagCalUID, 4, 0x28, "UID"},
	} {
		if tc.tag.page != tc.page || tc.tag.code != tc.token {
			t.Errorf("%s: incorrect wire tag %+v", tc.name, tc.tag)
		}
		if got := wbFieldName(tc.page, tc.token); got != tc.name {
			t.Errorf("decode %s: %s", tc.name, got)
		}
	}
	if tagPingStatus != (wbTag{13, 7}) || tagGIEStatus != (wbTag{6, 14}) || tagGIECollectionID != (wbTag{6, 10}) {
		t.Fatal("invalid control response tags")
	}
}

func TestXMLFieldParsing(t *testing.T) {
	if got := extractAttr(`<m:GetItem xmlns:m="m" xmlns:t="t"><m:ItemIds><t:ItemId Id='id&amp;1'/></m:ItemIds></m:GetItem>`, "ItemId", "Id"); got != "id&1" {
		t.Fatal(got)
	}
	if got := extractTag(`<t:Body xmlns:t="t" BodyType="Text">A &amp; B</t:Body>`, "Body"); got != "A & B" {
		t.Fatal(got)
	}
	if got := extractAttr(`<ItemIds Id="wrong"/>`, "ItemId", "Id"); got != "" {
		t.Fatal(got)
	}
}

func requireXML(t *testing.T, s string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := d.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("invalid response XML: %v", err)
		}
	}
}
