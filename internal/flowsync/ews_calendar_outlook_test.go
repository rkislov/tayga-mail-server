// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"os"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
)

func TestOutlookMacCalendarCreatePropertiesAndAttachments(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	calendar, err := st.GetCalendarByName(ctx, owner.ID, "default")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/outlook_mac_calendar_create.xml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML([]byte(strings.ReplaceAll(string(raw), "TEST-CALENDAR-ID", calendar.ID)))
	if err != nil {
		t.Fatal(err)
	}
	created := calendarSOAP(t, h, ctx, owner, doc.find("CreateItem").render())
	if created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	id := nodeAttr(created.find("ItemId"), "Id")
	saved, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.UID != "outlook-calendar-test-20261124" || davTextProperty(saved.Data, "CLASS") != "PRIVATE" || davTextProperty(saved.Data, "TRANSP") != "TRANSPARENT" {
		t.Fatal(saved.Data)
	}
	got := calendarSOAP(t, h, ctx, owner, `<m:GetItem><m:ItemIds><t:ItemId Id="`+id+`"/></m:ItemIds></m:GetItem>`)
	item := got.find("CalendarItem")
	if item == nil || item.value("Sensitivity") != "Private" || item.value("Importance") != "Normal" || item.value("LegacyFreeBusyStatus") != "Free" {
		t.Fatal(got.render())
	}
	if item.value("ReminderIsSet") != "false" || item.value("ReminderMinutesBeforeStart") != "15" {
		t.Fatal("Outlook reminder settings changed", got.render())
	}
	if item.value("AppointmentState") != "0" || item.value("IsOrganizer") != "true" || item.child("EffectiveRights").value("Modify") != "true" {
		t.Fatal("owner item reopened read-only", got.render())
	}
	count := 0
	for _, n := range item.Children {
		if n.Name.Local == "ExtendedProperty" {
			count++
		}
	}
	if count != 2 {
		t.Fatal("lost distinct named extended properties", got.render())
	}
	// A standard edit must retain opaque properties and the stable client UID.
	updated := calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(saved, nil), "SendToNone", standardMeetingField("SetItemField", "Location", "After reopen")))
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	saved, err = st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	props, err := calendarMetadata(saved.Data)
	if err != nil || len(props) != 7 {
		t.Fatal(props, err)
	}
	ev, err := calutil.ParseICS(saved.Data)
	if err != nil || ev.Location != "After reopen" {
		t.Fatal(ev, err)
	}
	// Both typed and ordinary attachments work on the parent created by Outlook.
	msg := attachedItemUpload(`<t:Message><t:Subject>Synthetic message</t:Subject><t:Body BodyType="Text">TMS fixture only</t:Body></t:Message>`)
	attached := calendarSOAP(t, h, ctx, owner, ewsAttachmentCreate(id, ewsCalendarChangeKey(saved, nil), msg))
	if attached.find("ResponseCode").Text != "NoError" {
		t.Fatal(attached.render())
	}
	ref := nodeAttr(attached.find("AttachmentId"), "Id")
	read := calendarSOAP(t, h, ctx, owner, ewsAttachmentFetch(ref, ""))
	if read.find("Message") == nil || read.find("Message").value("Subject") != "Synthetic message" {
		t.Fatal(read.render())
	}
	if jobs := drainCalendarJobs(t, ctx, st); len(jobs) != 0 {
		t.Fatal("personal test sent calendar mail", len(jobs))
	}
}

func TestOutlookCalendarDuplicateExtendedSelectorsAndScalarAreAtomic(t *testing.T) {
	for _, fields := range []string{
		`<t:ExtendedProperty><t:ExtendedFieldURI DistinguishedPropertySetId="Common" PropertyId="34051" PropertyType="Boolean"/><t:Value>false</t:Value></t:ExtendedProperty><t:ExtendedProperty><t:ExtendedFieldURI DistinguishedPropertySetId="Common" PropertyId="34051" PropertyType="Boolean"/><t:Value>true</t:Value></t:ExtendedProperty>`,
		`<t:Location>A</t:Location><t:Location>B</t:Location>`,
	} {
		ctx, st, owner, eas, _, _ := syncFixture(t)
		h := &ewsHandler{store: st, ms: eas.ms}
		cal, _ := st.GetCalendarByName(ctx, owner.ID, "default")
		before, _ := st.ListCalendarObjects(ctx, cal.ID)
		got := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(cal.ID, "SendToNone", fields))
		if got.find("ResponseCode").Text != "ErrorInvalidPropertySet" {
			t.Fatal(got.render())
		}
		after, _ := st.ListCalendarObjects(ctx, cal.ID)
		if len(before) != len(after) {
			t.Fatal("partial calendar create")
		}
	}
}

func TestOutlookCalendarExtendedUpdateDeleteAndTypeMismatch(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, owner.ID, "default")
	selector := `DistinguishedPropertySetId="Common" PropertyId="34049" PropertyType="Integer"`
	prop := `<t:ExtendedProperty><t:ExtendedFieldURI ` + selector + `/><t:Value>15</t:Value></t:ExtendedProperty>`
	created := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(cal.ID, "SendToNone", prop))
	if created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	id := nodeAttr(created.find("ItemId"), "Id")
	item, _ := st.GetCalendarObjectByID(ctx, id)
	update := `<t:SetItemField><t:ExtendedFieldURI ` + selector + `/><t:CalendarItem>` + strings.ReplaceAll(prop, `>15<`, `>30<`) + `</t:CalendarItem></t:SetItemField>`
	got := calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(item, nil), "SendToNone", update))
	if got.find("ResponseCode").Text != "NoError" {
		t.Fatal(got.render())
	}
	item, _ = st.GetCalendarObjectByID(ctx, id)
	props, _ := calendarMetadata(item.Data)
	if !strings.Contains(props["ExtendedProperty:dist:Common:id:34049"], ">30<") {
		t.Fatal(props)
	}
	bad := strings.Replace(update, `PropertyType="Integer"`, `PropertyType="String"`, 1)
	got = calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(item, nil), "SendToNone", bad))
	if got.find("ResponseCode").Text != "ErrorInvalidPropertySet" {
		t.Fatal(got.render())
	}
	unchanged, _ := st.GetCalendarObjectByID(ctx, id)
	if unchanged.ETag != item.ETag {
		t.Fatal("invalid type update changed item")
	}
	del := `<t:DeleteItemField><t:ExtendedFieldURI ` + selector + `/></t:DeleteItemField>`
	got = calendarSOAP(t, h, ctx, owner, standardMeetingUpdate(id, ewsCalendarChangeKey(item, nil), "SendToNone", del))
	if got.find("ResponseCode").Text != "NoError" {
		t.Fatal(got.render())
	}
	item, _ = st.GetCalendarObjectByID(ctx, id)
	props, _ = calendarMetadata(item.Data)
	if len(props) != 0 {
		t.Fatal("deleted extended property survived", props)
	}
}

func TestOutlookCalendarEffectiveRightsFollowACL(t *testing.T) {
	ctx, st, owner, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, owner.ID, "default")
	viewer := standardMeetingPeer(t, ctx, st, owner, "calendar-viewer")
	created := calendarSOAP(t, h, ctx, owner, standardMeetingCreate(cal.ID, "SendToNone", ""))
	id := nodeAttr(created.find("ItemId"), "Id")
	for _, rights := range []string{"read", "write"} {
		if err := st.SetCalendarACL(ctx, cal.ID, viewer.ID, rights); err != nil {
			t.Fatal(err)
		}
		got := calendarSOAP(t, h, ctx, viewer, `<m:GetItem><m:ItemIds><t:ItemId Id="`+id+`"/></m:ItemIds></m:GetItem>`)
		item := got.find("CalendarItem")
		if item == nil {
			t.Fatal(got.render())
		}
		want := "false"
		if rights == "write" {
			want = "true"
		}
		if item.child("EffectiveRights").value("Modify") != want || item.child("EffectiveRights").value("Delete") != want || item.value("IsOrganizer") != "false" {
			t.Fatal("advertised rights differ from ACL", rights, got.render())
		}
	}
}
