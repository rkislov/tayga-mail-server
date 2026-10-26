package flowsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func TestInvitedCalendarFilesSyncDownloadAndRevocation(t *testing.T) {
	f := newMeetingFixture(t)
	ctx := context.WithValue(f.ctx, easVersionKey{}, "16.1")
	ews := &ewsHandler{store: f.st, ms: f.h.ms}
	before, err := ews.notificationSnapshot(ctx, f.guest, &ewsSubscription{all: true})
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("%PDF-1.7\nshared boarding pass\n")
	root := filepath.Join(f.h.ms.Root, "calendar-attachments")
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "shared.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := f.st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: f.original.ID, UserID: f.owner.ID, Filename: "Boarding.pdf", ContentType: "application/pdf", Size: int64(len(data)), StoragePath: "shared.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	ref := "calendar:" + f.received.ID + ":" + file.ID
	// A file added after the invitation wakes Outlook and appears in EAS metadata.
	after, err := ews.notificationSnapshot(ctx, f.guest, &ewsSubscription{all: true})
	if err != nil {
		t.Fatal(err)
	}
	if after[f.received.ID].hash == before[f.received.ID].hash {
		t.Fatal("attachment-only change did not notify guest")
	}
	xml, _, err := f.h.syncCalendar(ctx, f.guest, f.received.CalendarID, "key", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := encodeWire(xml)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWire(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decoded), ref) {
		t.Fatal("guest sync missing file", string(decoded))
	}
	fetch := `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><FileReference xmlns="AirSyncBase:">` + ref + `</FileReference></Fetch></ItemOperations>`
	result, err := f.h.itemOperations(ctx, f.guest, []byte(fetch))
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := parseProtocolXML([]byte(result))
	got, err := base64.StdEncoding.DecodeString(doc.find("Data").Text)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("guest download failed", result, err)
	}
	// EWS GetItem and GetAttachment refer to the recipient's own event ID.
	doc = calendarSOAP(t, ews, ctx, f.guest, `<m:GetItem><m:ItemShape><t:BaseShape>AllProperties</t:BaseShape></m:ItemShape><m:ItemIds><t:ItemId Id="`+f.received.ID+`"/></m:ItemIds></m:GetItem>`)
	if doc.find("AttachmentId") == nil || nodeAttr(doc.find("AttachmentId"), "Id") != ref {
		t.Fatal(doc.render())
	}
	doc = calendarSOAP(t, ews, ctx, f.guest, `<m:GetAttachment><m:AttachmentIds><t:AttachmentId Id="`+ref+`"/></m:AttachmentIds></m:GetAttachment>`)
	got, err = base64.StdEncoding.DecodeString(doc.find("Content").Text)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal(doc.render(), err)
	}
	// Knowing the organizer's object ID is not a substitute for owning the copy.
	if _, err = f.h.calendarAttachment(ctx, f.guest, "calendar:"+f.original.ID+":"+file.ID); err == nil {
		t.Fatal("organizer reference bypassed ownership")
	}
	other, err := f.st.EnsureCalendar(ctx, f.guest.ID, "other", "Other")
	if err != nil {
		t.Fatal(err)
	}
	fake, err := f.st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: other.ID, UID: f.original.UID, HrefName: "forged.ics", Data: f.original.Data, Component: "VEVENT"})
	if err != nil {
		t.Fatal(err)
	}
	if files, err := f.st.ListCalendarEventAttachments(ctx, fake.ID); err != nil || len(files) != 0 {
		t.Fatal("UID alone granted file access", files, err)
	}
	if _, err = f.h.calendarAttachment(ctx, f.guest, "calendar:"+fake.ID+":"+file.ID); err == nil {
		t.Fatal("forged copy granted download")
	}
	// Move both real copies: the invitation and files remain bound to identities.
	ownCal, _ := f.st.EnsureCalendar(ctx, f.owner.ID, "work", "Work")
	if _, err = f.st.MoveCalendarObject(ctx, f.owner.ID, f.original.ID, f.original.CalendarID, ownCal.ID); err != nil {
		t.Fatal(err)
	}
	guestCal, _ := f.st.EnsureCalendar(ctx, f.guest.ID, "work", "Work")
	if _, err = f.st.MoveCalendarObject(ctx, f.guest.ID, f.received.ID, f.received.CalendarID, guestCal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.h.calendarAttachment(ctx, f.guest, ref); err != nil {
		t.Fatal("move lost guest access", err)
	}
	// Removing the guest revokes previously issued references immediately.
	if _, err = f.h.wbCalendar(ctx, f.owner, ownCal.ID, calendarOp(t, "change", f.original.ID, `<Attendees/>`)); err != nil {
		t.Fatal(err)
	}
	if files, err := f.st.ListCalendarEventAttachments(ctx, f.received.ID); err != nil || len(files) != 0 {
		t.Fatal("removed guest sees files", files, err)
	}
	if _, err = f.h.calendarAttachment(ctx, f.guest, ref); err == nil {
		t.Fatal("removed guest still downloads")
	}
	if _, err = f.h.calendarAttachment(ctx, f.owner, "calendar:"+f.original.ID+":"+file.ID); err != nil {
		t.Fatal("revocation lost owner's file", err)
	}
}

func TestInvitedCalendarSignedDownloadWire(t *testing.T) {
	f := newMeetingFixture(t)
	st := f.st.(*storage.Store)
	if err := st.SetCalendarAttachmentLinks("https://mail.example.com", bytes.Repeat([]byte{13}, 32)); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(f.ctx, easVersionKey{}, "16.1")
	data := []byte("%PDF-1.7\nshared through an invitation\n")
	root := filepath.Join(f.h.ms.Root, "calendar-attachments")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "signed.pdf"), data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := st.CreateCalendarAttachment(ctx, &storage.CalendarAttachment{CalendarObjectID: f.original.ID, UserID: f.owner.ID, Filename: "Билет.pdf", ContentType: "application/pdf", Size: int64(len(data)), StoragePath: "signed.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	_, wire, err := f.h.syncCalendar(ctx, f.guest, f.received.CalendarID, "key", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeWire(wire)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML(decoded)
	if err != nil {
		t.Fatal(err)
	}
	ref := doc.find("FileReference").Text
	if !strings.HasPrefix(ref, "https://mail.example.com/calendar-download/") || !strings.Contains(doc.find("Body").value("Data"), ref) {
		t.Fatal("missing native and browser download paths")
	}
	// Repeat serialization is byte-stable; polling must not cause resync loops.
	_, again, err := f.h.syncCalendar(ctx, f.guest, f.received.CalendarID, "key", true, nil)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatal("unstable calendar sync", err)
	}
	for _, reference := range []string{ref, "calendar:" + f.received.ID + ":" + file.ID} {
		request := `<ItemOperations xmlns="ItemOperations:"><Fetch><Store>Mailbox</Store><FileReference xmlns="AirSyncBase:">` + xmlEscape(reference) + `</FileReference></Fetch></ItemOperations>`
		wire, err := encodeWire(request)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeWire(wire)
		if err != nil {
			t.Fatal(err)
		}
		result, err := f.h.itemOperations(ctx, f.guest, decoded)
		if err != nil {
			t.Fatal(err)
		}
		wire, err = encodeWire(result)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err = decodeWire(wire)
		if err != nil {
			t.Fatal(err)
		}
		doc, _ := parseProtocolXML(decoded)
		got, err := base64.StdEncoding.DecodeString(doc.find("Data").Text)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal("wire download failed", err)
		}
	}
	ews := &ewsHandler{store: f.st, ms: f.h.ms}
	xml, err := ews.calendarItemXML(ctx, f.guest, f.received, true)
	if err != nil || !strings.Contains(xml, ref) {
		t.Fatal("EWS body misses link", err)
	}
	if _, err = f.h.wbCalendar(ctx, f.owner, f.original.CalendarID, calendarOp(t, "change", f.original.ID, `<Attendees/>`)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.h.calendarAttachment(ctx, f.guest, ref); err == nil {
		t.Fatal("removed guest still downloads with signed native reference")
	}
}
