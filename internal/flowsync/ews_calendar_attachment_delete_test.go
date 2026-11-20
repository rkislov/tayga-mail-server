// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func ewsAttachmentDelete(refs ...string) string {
	var b strings.Builder
	b.WriteString(`<m:DeleteAttachment><m:AttachmentIds>`)
	for _, ref := range refs {
		b.WriteString(`<t:AttachmentId Id="` + xmlEscape(ref) + `"/>`)
	}
	b.WriteString(`</m:AttachmentIds></m:DeleteAttachment>`)
	return b.String()
}

func TestEWSDeleteCalendarAttachmentRevokesAccessAndUpdatesCopies(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms, publicURL: "https://mail.example.com"}
	if err := f.st.(*storage.Store).SetCalendarAttachmentLinks(h.publicURL, []byte(strings.Repeat("s", 32))); err != nil {
		t.Fatal(err)
	}
	response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("delete.txt", "text/plain", []byte("one"))+ewsFileUpload("keep.txt", "text/plain", []byte("two"))))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	removed, kept := files[0], files[1]
	before, err := h.notificationSnapshot(f.ctx, f.guest, &ewsSubscription{all: true})
	if err != nil {
		t.Fatal(err)
	}
	links, err := f.st.CalendarAttachmentLinks(f.ctx, f.received.ID, f.guest.ID)
	if err != nil || len(links) != 2 {
		t.Fatal(links, err)
	}
	var token string
	for _, link := range links {
		if link.Attachment.ID == removed.ID {
			token = calendarDownloadToken(link.URL)
		}
	}
	if token == "" {
		t.Fatal("no signed download link")
	}
	if _, err = f.st.CalendarDownloadAttachment(f.ctx, token); err != nil {
		t.Fatal(err)
	}
	oldKey := nodeAttr(response.find("AttachmentId"), "RootItemChangeKey")
	ref := "calendar:" + f.original.ID + ":" + removed.ID
	response = calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentDelete(ref))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	saved, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if err != nil {
		t.Fatal(err)
	}
	remaining, err := f.st.ListCalendarEventAttachments(f.ctx, saved.ID)
	if err != nil || len(remaining) != 1 || remaining[0].ID != kept.ID {
		t.Fatal(remaining, err)
	}
	root := response.find("RootItemId")
	if nodeAttr(root, "RootItemId") != saved.ID || nodeAttr(root, "RootItemChangeKey") == oldKey || nodeAttr(root, "RootItemChangeKey") != ewsCalendarChangeKey(saved, remaining) {
		t.Fatal(response.render())
	}
	for _, id := range []string{f.original.ID, f.received.ID} {
		object, err := f.st.GetCalendarObjectByID(f.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		event, err := calutil.ParseICS(object.Data)
		if err != nil || len(event.Attendees) != 1 || strings.Contains(object.Data, removed.ID) || !strings.Contains(object.Data, kept.ID) {
			t.Fatal(object.Data, err)
		}
	}
	if _, err = f.st.CalendarDownloadAttachment(f.ctx, token); err == nil {
		t.Fatal("deleted signed download remains valid")
	}
	for _, user := range []*storage.User{f.owner, f.guest} {
		eventID := f.original.ID
		if user.ID == f.guest.ID {
			eventID = f.received.ID
		}
		if _, err := f.h.calendarAttachment(f.ctx, user, "calendar:"+eventID+":"+removed.ID); err == nil {
			t.Fatal("deleted EAS file accessible")
		}
		doc := calendarSOAP(t, h, f.ctx, user, `<m:GetAttachment><m:AttachmentIds><t:AttachmentId Id="calendar:`+eventID+`:`+removed.ID+`"/></m:AttachmentIds></m:GetAttachment>`)
		if doc.find("ResponseCode").Text == "NoError" {
			t.Fatal("deleted EWS file accessible")
		}
	}
	after, err := h.notificationSnapshot(f.ctx, f.guest, &ewsSubscription{all: true})
	if err != nil || before[f.received.ID].hash == after[f.received.ID].hash {
		t.Fatal("recipient change missing", err)
	}
	if calendarBlobCount(t, f.h.ms.Root) != 1 {
		t.Fatal("blob not removed")
	}
	response = calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentDelete(ref))
	if response.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
		t.Fatal(response.render())
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("delete sent invitation", len(jobs))
	}
}

func TestEWSDeleteCalendarAttachmentBatchAndAuthorization(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("first.txt", "text/plain", []byte("one"))+ewsFileUpload("second.txt", "text/plain", []byte("two"))))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	files, _ := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
	sourceRef := "calendar:" + f.original.ID + ":" + files[0].ID
	copyRef := "calendar:" + f.received.ID + ":" + files[0].ID
	for _, ref := range []string{sourceRef, copyRef} {
		denied := calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentDelete(ref))
		if denied.find("ResponseCode").Text != "ErrorAccessDenied" {
			t.Fatal(denied.render())
		}
	}
	// Request-only identifiers must not include the CreateAttachment response attributes.
	invalid := `<m:DeleteAttachment><m:AttachmentIds><t:AttachmentId Id="` + sourceRef + `" RootItemId="` + f.original.ID + `"/></m:AttachmentIds></m:DeleteAttachment>`
	if doc := calendarSOAP(t, h, f.ctx, f.owner, invalid); doc.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
		t.Fatal(doc.render())
	}
	response = calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentDelete(sourceRef, sourceRef, "not-a-reference", "calendar:"+f.original.ID+":"+files[1].ID))
	results := response.find("ResponseMessages").Children
	want := []string{"NoError", "ErrorInvalidAttachmentId", "ErrorInvalidAttachmentId", "NoError"}
	if len(results) != len(want) {
		t.Fatal(response.render())
	}
	for i, result := range results {
		if result.value("ResponseCode") != want[i] {
			t.Fatal(response.render())
		}
	}
	saved, _ := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	remaining, _ := f.st.ListCalendarEventAttachments(f.ctx, saved.ID)
	if len(remaining) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
		t.Fatal("batch retained files")
	}
	if nodeAttr(results[3].find("RootItemId"), "RootItemChangeKey") != ewsCalendarChangeKey(saved, remaining) {
		t.Fatal(response.render())
	}
	item := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemIds><t:ItemId Id="`+saved.ID+`"/></m:ItemIds></m:GetItem>`)
	if item.find("HasAttachments").Text != "false" {
		t.Fatal(item.render())
	}
}

func TestEWSDeleteCalendarAttachmentRollback(t *testing.T) {
	for _, kind := range []string{"metadata", "copy"} {
		t.Run(kind, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("keep.txt", "text/plain", []byte("keep"))))
			if response.find("ResponseCode").Text != "NoError" {
				t.Fatal(response.render())
			}
			files, _ := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
			original, _ := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
			copy, _ := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
			sql := `CREATE TRIGGER fail_ews_delete_attachment BEFORE DELETE ON calendar_attachments BEGIN SELECT RAISE(ABORT,'injected'); END`
			if kind == "copy" {
				sql = `CREATE TRIGGER fail_ews_delete_attachment BEFORE UPDATE ON calendar_objects WHEN OLD.id='` + f.received.ID + `' BEGIN SELECT RAISE(ABORT,'injected'); END`
			}
			if _, err := f.st.(*storage.Store).DB().Exec(sql); err != nil {
				t.Fatal(err)
			}
			response = calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentDelete("calendar:"+f.original.ID+":"+files[0].ID))
			if response.find("ResponseCode").Text != "ErrorInternalServerError" {
				t.Fatal(response.render())
			}
			for _, before := range []*storage.CalendarObject{original, copy} {
				after, err := f.st.GetCalendarObjectByID(f.ctx, before.ID)
				if err != nil || after.Data != before.Data || after.ETag != before.ETag {
					t.Fatal("partial deletion", err)
				}
			}
			if _, err := f.h.calendarAttachment(f.ctx, f.guest, "calendar:"+f.received.ID+":"+files[0].ID); err != nil {
				t.Fatal("rollback revoked file", err)
			}
			if calendarBlobCount(t, f.h.ms.Root) != 1 {
				t.Fatal("rollback deleted blob")
			}
		})
	}
}

func TestEWSDeleteCalendarAttachmentPersonalAndMissingBlob(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	response := calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentCreate(f.received.ID, ewsCalendarChangeKey(f.received, nil), ewsFileUpload("personal.txt", "text/plain", []byte("private"))))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	ref := nodeAttr(response.find("AttachmentId"), "Id")
	// Logical deletion remains available even if disk cleanup is unavailable.
	h.ms = nil
	response = calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentDelete(ref))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	original, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if err != nil || original.Data != f.original.Data {
		t.Fatal("private deletion changed source", err)
	}
	files, err := f.st.ListCalendarAttachments(f.ctx, f.received.ID)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
	if _, err := f.h.calendarAttachment(f.ctx, f.guest, ref); err == nil {
		t.Fatal("orphaned blob still downloadable")
	}
}

func TestEWSDeleteCalendarAttachmentRejectsStaleTransaction(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("keep.txt", "text/plain", []byte("keep"))))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	files, _ := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
	before, _ := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	data, err := calutil.WithoutFileAttachment(before.Data, files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	stale := *before
	stale.ExpectedETag, stale.Data = before.ETag, data
	updated := calendarSOAP(t, h, f.ctx, f.owner, standardMeetingUpdate(before.ID, ewsCalendarChangeKey(before, files), "SendToNone", standardMeetingField("SetItemField", "Subject", "Concurrent edit")))
	if updated.find("ResponseCode").Text != "NoError" {
		t.Fatal(updated.render())
	}
	if _, err = f.st.DeleteCalendarClientAttachments(f.ctx, f.owner.ID, &stale, files); err != storage.ErrCalendarConflict {
		t.Fatal("stale delete accepted", err)
	}
	after, _ := f.st.GetCalendarObjectByID(f.ctx, before.ID)
	if !strings.Contains(after.Data, "Concurrent edit") {
		t.Fatal(after.Data)
	}
	if _, err = f.h.calendarAttachment(f.ctx, f.guest, "calendar:"+f.received.ID+":"+files[0].ID); err != nil {
		t.Fatal("stale transaction revoked file", err)
	}
}
