package flowsync

import "testing"

func TestParseWBXMLClientChangeRead(t *testing.T) {
	sid := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	e.taggedStr(tagSyncKey, "1")
	e.taggedStr(tagCollectionID, "mb-1")
	e.start(tagCommands)
	e.start(tagChange)
	e.taggedStr(tagServerID, sid)
	e.start(tagApplicationData)
	e.taggedStr(tagEmailRead, "1")
	e.end() // ApplicationData
	e.end() // Change
	e.end() // Commands
	e.end() // Collection
	e.end() // Collections
	e.end() // Sync

	ops := parseWBXMLClientOps(e.bytes())
	if len(ops) != 1 {
		t.Fatalf("ops=%+v", ops)
	}
	if ops[0].Kind != "change" || ops[0].ServerID != sid || ops[0].Fields["Read"] != "1" {
		t.Fatalf("got %+v", ops[0])
	}
}

func TestParseWBXMLClientAddContact(t *testing.T) {
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	e.taggedStr(tagClass, "Contacts")
	e.taggedStr(tagCollectionID, "ab-1")
	e.start(tagCommands)
	e.start(tagAdd)
	e.taggedStr(tagClientID, "client-1")
	e.start(tagApplicationData)
	e.taggedStr(tagContactFileAs, "Ada")
	e.taggedStr(tagContactFirst, "Ada")
	e.taggedStr(tagContactLast, "Lovelace")
	e.taggedStr(tagContactEmail1, "ada@ex.com")
	e.end()
	e.end()
	e.end()
	e.end()
	e.end()
	e.end()

	ops := parseWBXMLClientOps(e.bytes())
	if len(ops) != 1 || ops[0].Kind != "add" {
		t.Fatalf("ops=%+v", ops)
	}
	if ops[0].ClientID != "client-1" {
		t.Fatalf("client id %q", ops[0].ClientID)
	}
	if ops[0].Fields["FileAs"] != "Ada" || ops[0].Fields["Email1Address"] != "ada@ex.com" {
		t.Fatalf("fields %+v", ops[0].Fields)
	}
}

func TestParseWBXMLClientDelete(t *testing.T) {
	sid := "11111111-2222-3333-4444-555555555555"
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	e.start(tagCommands)
	e.start(tagDelete)
	e.taggedStr(tagServerID, sid)
	e.end()
	e.end()
	e.end()
	e.end()
	e.end()

	ops := parseWBXMLClientOps(e.bytes())
	if len(ops) != 1 || ops[0].Kind != "delete" || ops[0].ServerID != sid {
		t.Fatalf("ops=%+v", ops)
	}
}

func TestParseWBXMLClientAddContactExtraFields(t *testing.T) {
	e := newWBEncoder()
	e.start(tagSync)
	e.start(tagCollections)
	e.start(tagCollection)
	e.start(tagCommands)
	e.start(tagAdd)
	e.taggedStr(tagClientID, "c2")
	e.start(tagApplicationData)
	e.taggedStr(tagContactCompany, "Analytical")
	e.taggedStr(tagContactJobTitle, "Mathematician")
	e.taggedStr(tagContactBusinessPhone, "+1-555-0100")
	e.taggedStr(tagContactHomePhone, "+1-555-0101")
	e.taggedStr(tagContactEmail2, "ada@alt.ex")
	e.end()
	e.end()
	e.end()
	e.end()
	e.end()
	e.end()

	ops := parseWBXMLClientOps(e.bytes())
	if len(ops) != 1 {
		t.Fatalf("ops=%+v", ops)
	}
	f := ops[0].Fields
	if f["CompanyName"] != "Analytical" || f["JobTitle"] != "Mathematician" ||
		f["BusinessPhoneNumber"] != "+1-555-0100" || f["Email2Address"] != "ada@alt.ex" {
		t.Fatalf("fields %+v", f)
	}
}

func TestParseWBXMLIgnoresServerIdOutsideCommands(t *testing.T) {
	// FolderSync-style ServerId strings must not become delete ops.
	e := newWBEncoder()
	e.start(tagSync)
	e.taggedStr(tagServerID, "not-a-delete")
	e.end()
	if ops := parseWBXMLClientOps(e.bytes()); len(ops) != 0 {
		t.Fatalf("unexpected ops %+v", ops)
	}
}
