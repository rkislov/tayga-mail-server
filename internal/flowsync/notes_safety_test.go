package flowsync

import (
	"errors"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/noteutil"
	"github.com/tayga/tms/internal/storage"
)

func TestNotesSyncLifecycleConflictsAndReplay(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	folder, err := st.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
	if err != nil {
		t.Fatal(err)
	}
	call := func(key, commands string) string {
		t.Helper()
		request := `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + folder.ID + `</CollectionId>` + commands + `</Collection></Collections></Sync>`
		wire, err := encodeWire(request)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeWire(wire)
		if err != nil {
			t.Fatal(err)
		}
		result, err := h.syncCollections(ctx, u, dev, decoded)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	key := extractTag(call("0", ""), "SyncKey")
	commands := `<Commands><Add><ClientId>phone-note</ClientId><ApplicationData><Subject xmlns="Notes:">Планы</Subject><Body xmlns="AirSyncBase:"><Type>1</Type><Data>&lt;literal&gt;</Data></Body></ApplicationData></Add></Commands>`
	added := call(key, commands)
	replay := call(key, commands)
	if replay != added {
		t.Fatal("retry changed response")
	}
	items, err := st.ListNoteItemsInFolder(ctx, folder.ID, false)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	n := items[0]
	if n.BodyText != "<literal>" || strings.Contains(n.BodyHTML, "<literal>") {
		t.Fatal("plain body interpreted as HTML", n)
	}
	tree, _ := parseProtocolXML([]byte(added))
	if commands := tree.find("Commands"); commands != nil && commands.child("Add") != nil {
		t.Fatal("duplicate Add", added)
	}
	key = extractTag(added, "SyncKey")
	stale := *n
	n.Title = "server title"
	if err := st.UpdateNoteItem(ctx, n); err != nil {
		t.Fatal(err)
	}
	stale.Title = "stale overwrite"
	if err := st.UpdateNoteItem(ctx, &stale); !errors.Is(err, storage.ErrNoteConflict) {
		t.Fatal("missing atomic conflict", err)
	}
	change := `<Commands><Change><ServerId>` + n.ID + `</ServerId><ApplicationData><Subject xmlns="Notes:">phone title</Subject></ApplicationData></Change></Commands>`
	conflict := call(key, change)
	tree, _ = parseProtocolXML([]byte(conflict))
	if tree.find("Responses").find("Status").Text != "7" || tree.find("Commands").child("Change") == nil {
		t.Fatal(conflict)
	}
	current, _ := st.GetNoteItemByID(ctx, n.ID)
	if current.Title != "server title" {
		t.Fatal(current.Title)
	}
	key = extractTag(conflict, "SyncKey")
	clear := `<Commands><Change><ServerId>` + n.ID + `</ServerId><ApplicationData><Subject xmlns="Notes:"/><Body xmlns="AirSyncBase:"><Type>1</Type><Data/></Body></ApplicationData></Change></Commands>`
	cleared := call(key, clear)
	tree, _ = parseProtocolXML([]byte(cleared))
	if tree.find("Responses").find("Status").Text != "1" {
		t.Fatal(cleared)
	}
	current, _ = st.GetNoteItemByID(ctx, n.ID)
	if current.Title != "" || current.BodyText != "" {
		t.Fatal("cannot clear", current)
	}
	versions, err := st.ListNoteVersions(ctx, n.ID)
	if err != nil || len(versions) != 2 || versions[0].Note.BodyText != "<literal>" {
		t.Fatal(versions, err)
	}
	key = extractTag(cleared, "SyncKey")
	deleted := call(key, `<Commands><Delete><ServerId>`+n.ID+`</ServerId></Delete></Commands>`)
	tree, _ = parseProtocolXML([]byte(deleted))
	if tree.find("Responses").find("Status").Text != "1" {
		t.Fatal(deleted)
	}
	if _, err := st.GetNoteItemByID(ctx, n.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestNoteStructuredBodyProtectionAndFolderScope(t *testing.T) {
	ctx, st, u, h, _, _ := syncFixture(t)
	folder, _ := st.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
	other, _ := st.EnsureNoteFolder(ctx, u.ID, "other", "Other")
	doc := noteutil.DocumentFromHTML(`<ul data-type="checklist"><li data-type="taskItem" data-checked="true">Done</li></ul><div data-type="drawing" data-attachment-id="drawing1" data-preview-id="preview1"></div>`)
	raw := noteutil.MarshalDocumentJSON(doc)
	html, text := noteutil.DeriveBodies(raw)
	n, err := st.CreateNoteItem(ctx, &storage.NoteItem{FolderID: folder.ID, UserID: u.ID, Title: "Rich", DocumentJSON: raw, BodyHTML: html, BodyText: text})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"change", "delete"} {
		_, err := h.wbNotes(ctx, u, other.ID, clientOp{Kind: kind, ServerID: n.ID, Fields: map[string]string{"Body": "stolen"}})
		if !errors.Is(err, storage.ErrUnauthorized) {
			t.Fatal(kind, err)
		}
	}
	_, err = h.wbNotes(ctx, u, folder.ID, clientOp{Kind: "change", ServerID: n.ID, Fields: map[string]string{"Body": "flattened"}})
	if !errors.Is(err, errInvalidNote) {
		t.Fatal("lossy update accepted", err)
	}
	got, _ := st.GetNoteItemByID(ctx, n.ID)
	if got.DocumentJSON != raw {
		t.Fatal("rich data lost")
	}
	_, err = h.wbNotes(ctx, u, folder.ID, clientOp{Kind: "change", ServerID: n.ID, Fields: map[string]string{"Body": html}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.wbNotes(ctx, u, folder.ID, clientOp{Kind: "change", ServerID: n.ID, Fields: map[string]string{"Body": ""}})
	if err != nil {
		t.Fatal(err)
	}
	history, _ := st.ListNoteVersions(ctx, n.ID)
	if len(history) != 2 || history[0].Note.DocumentJSON != raw {
		t.Fatal(history)
	}
}

func TestEWSNoteChangeKeysBodyTypesAndAtomicUpdates(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	create := calendarSOAP(t, h, ctx, u, `<m:CreateItem><m:Items><t:Message><t:ItemClass>IPM.StickyNote</t:ItemClass><t:Subject>Draft</t:Subject><t:Body BodyType="Text">Original</t:Body></t:Message></m:Items></m:CreateItem>`)
	id := extractAttr(create.render(), "ItemId", "Id")
	key := extractAttr(create.render(), "ItemId", "ChangeKey")
	n, err := st.GetNoteItemByID(ctx, id)
	if err != nil || key != n.ETag {
		t.Fatal(create.render(), err)
	}
	update := func(ck, fields string) *protocolNode {
		t.Helper()
		return calendarSOAP(t, h, ctx, u, `<m:UpdateItem ConflictResolution="NeverOverwrite"><m:ItemChanges><t:ItemChange><t:ItemId Id="`+id+`" ChangeKey="`+xmlEscape(ck)+`"/><t:Updates>`+fields+`</t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`)
	}
	fields := `<t:SetItemField><t:FieldURI FieldURI="item:Body"/><t:Message><t:Body BodyType="Text">&lt;b&gt;literal&lt;/b&gt;</t:Body></t:Message></t:SetItemField>`
	for _, tc := range []struct{ key, code string }{{"", "ErrorChangeKeyRequired"}, {"stale", "ErrorIrresolvableConflict"}} {
		r := update(tc.key, fields)
		if r.find("ResponseCode").Text != tc.code {
			t.Fatal(r.render())
		}
	}
	changed := update(key, fields)
	if changed.find("ResponseCode").Text != "NoError" {
		t.Fatal(changed.render())
	}
	key = extractAttr(changed.render(), "ItemId", "ChangeKey")
	n, _ = st.GetNoteItemByID(ctx, id)
	if n.ETag != key || n.BodyText != "<b>literal</b>" {
		t.Fatal(n)
	}
	bad := update(key, `<t:SetItemField><t:FieldURI FieldURI="item:Subject"/><t:Message><t:Subject>partial</t:Subject></t:Message></t:SetItemField><t:SetItemField><t:FieldURI FieldURI="item:Unknown"/></t:SetItemField>`)
	if bad.find("ResponseCode").Text != "ErrorInvalidPropertySet" {
		t.Fatal(bad.render())
	}
	n, _ = st.GetNoteItemByID(ctx, id)
	if n.Title != "Draft" {
		t.Fatal("partial write")
	}
	cleared := update(key, `<t:SetItemField><t:FieldURI FieldURI="item:Body"/><t:Message><t:Body BodyType="HTML"/></t:Message></t:SetItemField><t:DeleteItemField><t:FieldURI FieldURI="item:Subject"/></t:DeleteItemField>`)
	if cleared.find("ResponseCode").Text != "NoError" {
		t.Fatal(cleared.render())
	}
	n, _ = st.GetNoteItemByID(ctx, id)
	if n.BodyText != "" || n.Title != "" {
		t.Fatal("cannot clear")
	}
}

func TestEWSNotesPreserveRichBodiesAndAppend(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	folder, _ := st.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
	doc := noteutil.DocumentFromHTML(`<ul data-type="checklist"><li data-type="taskItem" data-checked="true">Done</li></ul><div data-type="drawing" data-attachment-id="draw" data-preview-id="preview"></div>`)
	raw := noteutil.MarshalDocumentJSON(doc)
	html, text := noteutil.DeriveBodies(raw)
	n, err := st.CreateNoteItem(ctx, &storage.NoteItem{FolderID: folder.ID, UserID: u.ID, DocumentJSON: raw, BodyHTML: html, BodyText: text})
	if err != nil {
		t.Fatal(err)
	}
	h := &ewsHandler{store: st, ms: eas.ms}
	update := func(fields string) *protocolNode {
		t.Helper()
		return calendarSOAP(t, h, ctx, u, `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="`+n.ID+`" ChangeKey="`+xmlEscape(n.ETag)+`"/><t:Updates>`+fields+`</t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`)
	}
	r := update(`<t:SetItemField><t:FieldURI FieldURI="item:Body"/><t:Message><t:Body BodyType="Text">Flattened</t:Body></t:Message></t:SetItemField>`)
	if r.find("ResponseCode").Text != "ErrorInvalidPropertySet" {
		t.Fatal(r.render())
	}
	got, _ := st.GetNoteItemByID(ctx, n.ID)
	if got.ETag != n.ETag || got.DocumentJSON != raw {
		t.Fatal("lossy write committed")
	}
	r = update(`<t:SetItemField><t:FieldURI FieldURI="item:Subject"/><t:Message><t:Subject>New title</t:Subject></t:Message></t:SetItemField>`)
	if r.find("ResponseCode").Text != "NoError" {
		t.Fatal(r.render())
	}
	n, _ = st.GetNoteItemByID(ctx, n.ID)
	if n.DocumentJSON != raw {
		t.Fatal("title update discarded structure")
	}
	r = update(`<t:AppendToItemField><t:FieldURI FieldURI="item:Body"/><t:Message><t:Body BodyType="Text">More text</t:Body></t:Message></t:AppendToItemField>`)
	if r.find("ResponseCode").Text != "NoError" {
		t.Fatal(r.render())
	}
	n, _ = st.GetNoteItemByID(ctx, n.ID)
	if !strings.Contains(n.BodyHTML, `data-checked="true"`) || !strings.Contains(n.BodyHTML, `data-attachment-id="draw"`) || !strings.Contains(n.BodyText, "More text") {
		t.Fatal(n)
	}
	body, _, err := eas.syncNotes(ctx, u, folder.ID, "next", false, nil)
	if err != nil || !strings.Contains(body, "More text") {
		t.Fatal(body, err)
	}
	get := calendarSOAP(t, h, ctx, u, `<m:GetItem><m:ItemIds><t:ItemId Id="`+n.ID+`"/></m:ItemIds></m:GetItem>`)
	item := get.find("Message")
	if item == nil || item.value("Body") != n.BodyHTML || extractAttr(item.render(), "ItemId", "ChangeKey") != n.ETag {
		t.Fatal(get.render())
	}
}

func TestNotesSendOnlyAcknowledgesOwnWrites(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	folder, _ := st.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
	key := "0"
	call := func(commands string) string {
		t.Helper()
		raw, err := h.syncCollections(ctx, u, dev, []byte(`<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>`+key+`</SyncKey><CollectionId>`+folder.ID+`</CollectionId><GetChanges>0</GetChanges>`+commands+`</Collection></Collections></Sync>`))
		if err != nil {
			t.Fatal(err)
		}
		key = extractTag(raw, "SyncKey")
		return raw
	}
	call("")
	added := call(`<Commands><Add><ClientId>new</ClientId><ApplicationData><Subject xmlns="Notes:">First</Subject></ApplicationData></Add></Commands>`)
	doc, _ := parseProtocolXML([]byte(added))
	id := doc.find("Responses").find("ServerId").Text
	for _, title := range []string{"Second", "Third"} {
		changed := call(`<Commands><Change><ServerId>` + id + `</ServerId><ApplicationData><Subject xmlns="Notes:">` + title + `</Subject></ApplicationData></Change></Commands>`)
		doc, _ = parseProtocolXML([]byte(changed))
		if doc.find("Responses").find("Status").Text != "1" {
			t.Fatal("own write treated as a conflict", changed)
		}
	}
	n, _ := st.GetNoteItemByID(ctx, id)
	if n.Title != "Third" {
		t.Fatal(n.Title)
	}
}
