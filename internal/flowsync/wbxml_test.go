// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"bytes"
	"testing"
)

func TestEncodeFolderSyncWBXMLContainsUUID(t *testing.T) {
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	bin := encodeFolderSyncWBXML("1", []folderChange{{
		ServerID: id, ParentID: "0", DisplayName: "INBOX", Type: 2,
	}})
	if len(bin) < 10 || bin[0] != wbxmlVersion {
		t.Fatalf("bad wbxml header: %x", bin[:min(8, len(bin))])
	}
	if !bytes.Contains(bin, []byte(id)) {
		t.Fatalf("expected mailbox UUID in WBXML payload")
	}
	if !bytes.Contains(bin, []byte("INBOX")) {
		t.Fatal("expected folder name")
	}
}

func TestExtractCollectionIDFromWBXML(t *testing.T) {
	coll := "11111111-2222-3333-4444-555555555555"
	bin := encodeSyncWBXML("2", coll, "Email", nil)
	got := extractWBXMLTagString(bin, "CollectionId")
	if got != coll {
		t.Fatalf("got %q want %q", got, coll)
	}
}

func TestRequestWantsWBXML(t *testing.T) {
	if !requestWantsWBXML("application/vnd.ms-sync.wbxml", "", nil) {
		t.Fatal("content-type")
	}
	if !requestWantsWBXML("", "application/vnd.ms-sync.wbxml", nil) {
		t.Fatal("accept")
	}
	if !requestWantsWBXML("", "", []byte{wbxmlVersion, 0x01}) {
		t.Fatal("body magic")
	}
	if requestWantsWBXML("application/xml", "application/xml", []byte(`<Sync/>`)) {
		t.Fatal("should prefer xml")
	}
}

// Literal protocol tokens make this independent of the encoder constants.
func TestProvisionPolicyWireTokens(t *testing.T) {
	data := encodeProvisionWBXML("123", defaultDevicePolicy())
	for _, wire := range [][]byte{{0x4a, 0x4d, 0x4e, 0x03, '1', 0, 0x01}, {0x54, 0x03, '4', 0, 0x01}} {
		if !bytes.Contains(data, wire) {
			t.Fatalf("missing policy wire structure %x in %x", wire, data)
		}
	}
}

func TestFolderHierarchyUsesProtocolPageSeven(t *testing.T) {
	payload := encodeFolderSyncWBXML("1", nil)
	if !bytes.HasPrefix(payload, []byte{3, 1, 106, 0, 0, 7, 0x56}) {
		t.Fatalf("wrong FolderHierarchy page: %x", payload)
	}
	// A real client sends page 7, independently of server constants.
	request := []byte{3, 1, 106, 0, 0, 7, 0x56, 0x52, 3, '0', 0, 1, 1}
	if got := extractWBXMLTagString(request, "SyncKey"); got != "0" {
		t.Fatalf("cannot read client hierarchy key: %q", got)
	}
}
