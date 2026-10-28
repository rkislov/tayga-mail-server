package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func calendarSOAP(t *testing.T, h *ewsHandler, ctx context.Context, u *storage.User, body string) *protocolNode {
	t.Helper()
	wire := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types"><s:Body>` + body + `</s:Body></s:Envelope>`
	r := httptest.NewRequest("POST", "/EWS/Exchange.asmx", strings.NewReader(wire)).WithContext(withUser(ctx, u))
	r.Header.Set("Content-Type", "text/xml; charset=utf-8")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	doc, err := parseProtocolXML(w.Body.Bytes())
	if err != nil || w.Code != 200 {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	return doc
}

func TestEWSCalendarReceivesIPhoneResponsesAndParticipantEdits(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	key := ""
	sync := func() *protocolNode {
		t.Helper()
		p := calendarSOAP(t, h, f.ctx, f.owner, `<m:SyncFolderItems><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape></m:ItemShape><m:SyncFolderId><t:FolderId Id="`+f.original.CalendarID+`"/></m:SyncFolderId><m:SyncState>`+key+`</m:SyncState></m:SyncFolderItems>`)
		if p.find("ResponseCode").Text != "NoError" {
			t.Fatal(p.render())
		}
		key = p.find("SyncState").Text
		return p
	}
	initial := sync()
	if initial.find("Create") == nil || initial.find("ResponseType").Text != "NoResponseReceived" || initial.find("MyResponseType").Text != "Organizer" {
		t.Fatal(initial.render())
	}
	if initial.find("Start").Text != "2026-10-22T17:00:00Z" {
		t.Fatal("invalid EWS date", initial.render())
	}
	oldKey := extractAttr(initial.render(), "ItemId", "ChangeKey")
	// Real EAS MeetingResponse command updates the organizer; EWS must expose
	// it as one Update, with the same item identity and a changed change key.
	for _, tc := range []struct{ eas, ews string }{{"1", "Accept"}, {"2", "Tentative"}, {"3", "Decline"}} {
		result := f.respond(t, f.guest, f.received.CalendarID, f.received.ID, tc.eas, "16.1", "")
		if result.value("Status") != "1" {
			t.Fatal(result.render())
		}
		page := sync()
		item := page.find("CalendarItem")
		if page.find("Update") == nil || page.find("Create") != nil || item == nil || item.find("ResponseType").Text != tc.ews || extractAttr(item.render(), "ItemId", "Id") != f.original.ID {
			t.Fatal(page.render())
		}
		ck := extractAttr(item.render(), "ItemId", "ChangeKey")
		if ck == "" || ck == oldKey {
			t.Fatal("unchanged calendar change key")
		}
		oldKey = ck
		got := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape></m:ItemShape><m:ItemIds><t:ItemId Id="`+f.original.ID+`"/></m:ItemIds></m:GetItem>`)
		if got.find("ResponseType").Text != tc.ews || extractAttr(got.render(), "ItemId", "ChangeKey") != ck || got.find("Organizer").find("EmailAddress").Text != f.owner.Email {
			t.Fatal(got.render())
		}
	}
	// A web/iPhone edit changes the attendee roster on the original event.
	for _, fields := range []string{
		`<Attendees><Attendee><Email>bob@example.com</Email><Name>Bob</Name></Attendee><Attendee><Email>carol@example.com</Email><Name>Carol</Name></Attendee></Attendees>`,
		`<Attendees/>`,
	} {
		if _, err := f.h.wbCalendar(f.ctx, f.owner, f.original.CalendarID, calendarOp(t, "change", f.original.ID, fields)); err != nil {
			t.Fatal(err)
		}
		p := sync()
		if p.find("Update") == nil || p.find("Create") != nil {
			t.Fatal(p.render())
		}
		if fields == `<Attendees/>` {
			if p.find("Attendee") != nil || p.find("IsMeeting").Text != "false" || p.find("RequiredAttendees") == nil {
				t.Fatal("removed attendees retained", p.render())
			}
		} else if len(p.find("RequiredAttendees").Children) != 2 {
			t.Fatal("new attendee missing", p.render())
		}
	}
	// The persisted cursor works after reconnecting to a new handler instance.
	h = &ewsHandler{store: f.st, ms: f.h.ms}
	if p := sync(); len(p.find("Changes").Children) != 0 {
		t.Fatal("unchanged event repeated", p.render())
	}
}

func TestEWSCalendarAttachmentsIncrementalDownloadAndRevocation(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, user.ID, "default")
	id, err := eas.wbCalendar(ctx, user, cal.ID, calendarOp(t, "add", "", `<Subject>Flight</Subject><StartTime>20261023T161000Z</StartTime><EndTime>20261023T171000Z</EndTime>`))
	if err != nil {
		t.Fatal(err)
	}
	key := ""
	sync := func() *protocolNode {
		t.Helper()
		p := calendarSOAP(t, h, ctx, user, `<m:SyncFolderItems><m:SyncFolderId><t:FolderId Id="`+cal.ID+`"/></m:SyncFolderId><m:SyncState>`+key+`</m:SyncState></m:SyncFolderItems>`)
		if p.find("ResponseCode").Text != "NoError" {
			t.Fatal(p.render())
		}
		key = p.find("SyncState").Text
		return p
	}
	initial := sync()
	oldKey := extractAttr(initial.render(), "ItemId", "ChangeKey")
	sub := &ewsSubscription{user: user.ID, folders: map[string]bool{cal.ID: true}, events: map[string]bool{"ModifiedEvent": true}}
	sub.objects, err = h.notificationSnapshot(ctx, user, sub)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 127, 128, 255}, 330000)...)
	root := filepath.Join(eas.ms.Root, "calendar-attachments")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ticket.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: id, UserID: user.ID, Filename: "Билет & посадочный.pdf", ContentType: "application/pdf", Size: int64(len(data)), StoragePath: "ticket.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	ref := "calendar:" + id + ":" + f.ID
	checkNotification := func() {
		t.Helper()
		snapshot, err := h.notificationSnapshot(ctx, user, sub)
		if err != nil {
			t.Fatal(err)
		}
		events := notificationChanges(sub, snapshot)
		if !strings.Contains(events, `<t:ModifiedEvent>`) || !strings.Contains(events, `Id="`+id+`"`) {
			t.Fatal("file-only modification missing from streaming notifications", events)
		}
	}
	checkNotification()
	page := sync()
	if page.find("Update") == nil || page.find("Create") != nil || page.find("HasAttachments").Text != "true" || page.find("FileAttachment").value("Name") != f.Filename || extractAttr(page.render(), "AttachmentId", "Id") != ref || extractAttr(page.render(), "ItemId", "ChangeKey") == oldKey {
		t.Fatal(page.render())
	}
	request := `<m:GetAttachment><m:AttachmentIds><t:AttachmentId Id="` + ref + `"/></m:AttachmentIds></m:GetAttachment>`
	got := calendarSOAP(t, h, ctx, user, request)
	decoded, err := base64.StdEncoding.DecodeString(got.find("Content").Text)
	if err != nil || !bytes.Equal(decoded, data) || got.find("ContentType").Text != "application/pdf" {
		t.Fatal("calendar file corrupted", err)
	}
	other := *user
	other.ID = "foreign-user"
	denied := calendarSOAP(t, h, ctx, &other, request)
	if denied.find("Content") != nil || denied.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
		t.Fatal("unauthorized file disclosed", denied.render())
	}
	denied = calendarSOAP(t, h, ctx, &other, `<m:GetItem><m:ItemIds><t:ItemId Id="`+id+`"/></m:ItemIds></m:GetItem>`)
	if denied.find("CalendarItem") != nil {
		t.Fatal("foreign calendar metadata disclosed")
	}
	if p := sync(); len(p.find("Changes").Children) != 0 {
		t.Fatal("file change repeatedly synchronized")
	}
	if err = st.DeleteCalendarAttachment(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	checkNotification()
	page = sync()
	if page.find("Update") == nil || page.find("HasAttachments").Text != "false" || page.find("FileAttachment") != nil || page.find("Attachments") == nil {
		t.Fatal("removed file retained", page.render())
	}
	denied = calendarSOAP(t, h, ctx, user, request)
	if denied.find("Content") != nil || denied.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
		t.Fatal("deleted file downloadable", denied.render())
	}
}

func TestEWSCalendarAttendeeRolesAndSelfResponse(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	cal, _ := st.GetCalendarByName(ctx, user.ID, "default")
	o, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: cal.ID, HrefName: "roles.ics", UID: "roles", Component: "VEVENT", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:roles\r\nDTSTART:20261023T120000Z\r\nDTEND:20261023T130000Z\r\nORGANIZER;CN=Owner:mailto:owner@example.com\r\nATTENDEE;PARTSTAT=ACCEPTED:mailto:" + user.Email + "\r\nATTENDEE;ROLE=OPT-PARTICIPANT;PARTSTAT=TENTATIVE;CN=Optional & guest:mailto:optional@example.com\r\nATTENDEE;CUTYPE=ROOM;PARTSTAT=DECLINED:mailto:room@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	p := calendarSOAP(t, h, ctx, user, `<m:GetItem><m:ItemIds><t:ItemId Id="`+o.ID+`"/></m:ItemIds></m:GetItem>`)
	if p.find("MyResponseType").Text != "Accept" || p.find("RequiredAttendees").find("ResponseType").Text != "Accept" || p.find("OptionalAttendees").find("ResponseType").Text != "Tentative" || p.find("Resources").find("ResponseType").Text != "Decline" || p.find("OptionalAttendees").find("Name").Text != "Optional & guest" {
		t.Fatal(p.render())
	}
	if p.find("CalendarItem").Name.Space != "http://schemas.microsoft.com/exchange/services/2006/types" {
		t.Fatal("wrong EWS namespace")
	}
}
