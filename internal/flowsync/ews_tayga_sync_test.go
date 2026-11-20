package flowsync

import (
	"context"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestTaygaEWSRawContactRoundtripReplayConflictAndDelete(t *testing.T) {
	ctx, st, u, eas, _, book := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	raw := "BEGIN:VCARD\nVERSION:4.0\nUID:raw-person\nFN:Иван & test\nEMAIL:first@example.com\nEMAIL:second@example.com\nNOTE:Keep me\nX-CUSTOM:opaque\nEND:VCARD\n"
	id := "292b86ba-5d36-4ac1-a552-c67c6b00b5e0"
	create := `<m:CreateItem><m:SavedItemFolderId><t:FolderId Id="` + book + `"/></m:SavedItemFolderId><m:Items><t:Contact>` + taygaProperty("TaygaVCard", raw) + taygaProperty("TaygaCreateId", id) + `</t:Contact></m:Items></m:CreateItem>`
	response := calendarSOAP(t, h, ctx, u, create)
	if response.find("ResponseCode").Text != "NoError" || extractAttr(response.render(), "ItemId", "Id") != id {
		t.Fatal(response.render())
	}
	saved, err := st.GetAddressObjectByID(ctx, id)
	if err != nil || saved.Data != raw {
		t.Fatal(saved, err)
	}
	response = calendarSOAP(t, h, ctx, u, create)
	cards, _ := st.ListAddressObjects(ctx, book)
	if response.find("ResponseCode").Text != "NoError" || len(cards) != 1 {
		t.Fatal(response.render(), len(cards))
	}
	oldKey := extractAttr(response.render(), "ItemId", "ChangeKey")
	update := func(key, data string) string {
		return `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="` + id + `" ChangeKey="` + xmlEscape(key) + `"/><t:Updates><t:SetItemField><t:Contact>` + taygaProperty("TaygaVCard", data) + `</t:Contact></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
	}
	changed := strings.Replace(raw, "Keep me", "Changed", 1)
	response = calendarSOAP(t, h, ctx, u, update(oldKey, changed))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	newKey := extractAttr(response.render(), "ItemId", "ChangeKey")
	response = calendarSOAP(t, h, ctx, u, update(oldKey, raw))
	if response.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal(response.render())
	}
	other := *u
	other.ID = "foreign"
	response = calendarSOAP(t, h, ctx, &other, update(newKey, raw))
	if response.find("ResponseCode").Text != "ErrorAccessDenied" {
		t.Fatal(response.render())
	}
	response = calendarSOAP(t, h, ctx, u, `<m:GetItem><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape><t:AdditionalProperties>`+taygaProperty("TaygaVCard", "")+`</t:AdditionalProperties></m:ItemShape><m:ItemIds><t:ItemId Id="`+id+`"/></m:ItemIds></m:GetItem>`)
	if !strings.Contains(response.render(), "X-CUSTOM:opaque") {
		t.Fatal(response.render())
	}
	deletion := `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="` + id + `" ChangeKey="` + newKey + `"/><t:Updates><t:SetItemField><t:Contact>` + taygaProperty("TaygaDelete", "true") + `</t:Contact></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
	response = calendarSOAP(t, h, ctx, u, deletion)
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	if _, err = st.GetAddressObjectByID(ctx, id); err != storage.ErrNotFound {
		t.Fatal(err)
	}
}

func TestTaygaEWSRawCalendarPreservesRecurrenceAndRejectsStaleWrite(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	calendar, _ := st.GetCalendarByName(ctx, u.ID, "default")
	raw := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Tayga//Test//EN\nBEGIN:VEVENT\nUID:series-raw\nDTSTART:20261026T090000Z\nDTEND:20261026T100000Z\nSUMMARY:Series\nRRULE:FREQ=DAILY;COUNT=4\nBEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT15M\nDESCRIPTION:Reminder\nEND:VALARM\nEND:VEVENT\nBEGIN:VEVENT\nUID:series-raw\nRECURRENCE-ID:20261027T090000Z\nDTSTART:20261027T110000Z\nDTEND:20261027T120000Z\nSUMMARY:Exception\nEND:VEVENT\nEND:VCALENDAR\n"
	id := "58f0396e-56e7-44f2-9e41-cd1bff12806b"
	create := `<m:CreateItem><m:SavedItemFolderId><t:FolderId Id="` + calendar.ID + `"/></m:SavedItemFolderId><m:Items><t:CalendarItem>` + taygaProperty("TaygaICalendar", raw) + taygaProperty("TaygaCreateId", id) + `</t:CalendarItem></m:Items></m:CreateItem>`
	response := calendarSOAP(t, h, ctx, u, create)
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	response = calendarSOAP(t, h, ctx, u, create)
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	oldKey := extractAttr(response.render(), "ItemId", "ChangeKey")
	saved, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"RRULE:FREQ=DAILY;COUNT=4", "RECURRENCE-ID:20261027T090000Z", "BEGIN:VALARM"} {
		if !strings.Contains(saved.Data, expected) {
			t.Fatal(saved.Data)
		}
	}
	saved.Data = strings.Replace(saved.Data, "SUMMARY:Series", "SUMMARY:Web change", 1)
	if _, err = st.UpsertCalendarObject(ctx, saved); err != nil {
		t.Fatal(err)
	}
	update := `<m:UpdateItem><m:ItemChanges><t:ItemChange><t:ItemId Id="` + id + `" ChangeKey="` + oldKey + `"/><t:Updates><t:SetItemField><t:CalendarItem>` + taygaProperty("TaygaICalendar", raw) + `</t:CalendarItem></t:SetItemField></t:Updates></t:ItemChange></m:ItemChanges></m:UpdateItem>`
	response = calendarSOAP(t, h, ctx, u, update)
	if response.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal(response.render())
	}
}

func TestTaygaEWSCapabilityIsExplicitAndUIDValidation(t *testing.T) {
	ctx, st, u, eas, _, book := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	request := `<m:GetFolder><m:FolderShape><t:BaseShape>IdOnly</t:BaseShape></m:FolderShape><m:FolderIds><t:DistinguishedFolderId Id="msgfolderroot"/></m:FolderIds></m:GetFolder>`
	ordinary := calendarSOAP(t, h, ctx, u, request)
	if strings.Contains(ordinary.render(), "TaygaSyncVersion") {
		t.Fatal("unsolicited capability", ordinary.render())
	}
	request = strings.Replace(request, "</m:FolderShape>", `<t:AdditionalProperties>`+taygaProperty("TaygaSyncVersion", "")+`</t:AdditionalProperties></m:FolderShape>`, 1)
	capable := calendarSOAP(t, h, ctx, u, request)
	if !strings.Contains(capable.render(), "TaygaSyncVersion") || capable.find("Value").Text != "1" {
		t.Fatal(capable.render())
	}
	// Duplicate UID with a different creation ID must retain the original contact.
	raw := "BEGIN:VCARD\nVERSION:4.0\nUID:unique-person\nFN:Original\nEND:VCARD\n"
	create := func(id string) string {
		return `<m:CreateItem><m:SavedItemFolderId><t:FolderId Id="` + book + `"/></m:SavedItemFolderId><m:Items><t:Contact>` + taygaProperty("TaygaVCard", raw) + taygaProperty("TaygaCreateId", id) + `</t:Contact></m:Items></m:CreateItem>`
	}
	first := calendarSOAP(t, h, ctx, u, create("34e80f4f-b179-4083-aa62-b79297bc78f0"))
	if first.find("ResponseCode").Text != "NoError" {
		t.Fatal(first.render())
	}
	duplicate := calendarSOAP(t, h, ctx, u, create("a2d5aaf3-f6fe-466e-9690-dd9c447cbba1"))
	if duplicate.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal(duplicate.render())
	}
}

func TestEWSThunderbirdSendResponseAndExplicitSentCopy(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	submitted := 0
	h.sender = &mailSubmission{store: st, ms: eas.ms, submit: func(_ context.Context, _ *storage.User, _ []string, _ []byte) error { submitted++; return nil }}
	payload := `<m:Items><t:Message><t:Subject>Native send</t:Subject><t:Body BodyType="Text">Body</t:Body><t:ToRecipients><t:Mailbox><t:EmailAddress>bob@example.com</t:EmailAddress></t:Mailbox></t:ToRecipients></t:Message></m:Items>`
	sent := calendarSOAP(t, h, ctx, u, `<m:CreateItem MessageDisposition="SendOnly">`+payload+`</m:CreateItem>`)
	if sent.find("ResponseCode").Text != "NoError" || sent.find("Items") == nil || submitted != 1 {
		t.Fatal(sent.render(), submitted)
	}
	copied := calendarSOAP(t, h, ctx, u, `<m:CreateItem MessageDisposition="SaveOnly"><m:SavedItemFolderId><t:DistinguishedFolderId Id="sentitems"/></m:SavedItemFolderId>`+payload+`</m:CreateItem>`)
	if copied.find("ResponseCode").Text != "NoError" || submitted != 1 {
		t.Fatal(copied.render(), submitted)
	}
	mb, _ := st.GetMailbox(ctx, u.ID, "Sent")
	messages, err := st.ListMessages(ctx, mb.ID)
	if err != nil || len(messages) != 1 || storage.HasFlag(messages[0].Flags, `\Draft`) {
		t.Fatal(messages, err)
	}
}
