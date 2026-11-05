// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

func ewsFileUpload(name, contentType string, data []byte) string {
	return `<t:FileAttachment><t:Name>` + xmlEscape(name) + `</t:Name><t:ContentType>` + xmlEscape(contentType) + `</t:ContentType><t:Content>` + base64.StdEncoding.EncodeToString(data) + `</t:Content></t:FileAttachment>`
}
func ewsAttachmentCreate(id, key, files string) string {
	return `<m:CreateAttachment><m:ParentItemId Id="` + xmlEscape(id) + `" ChangeKey="` + xmlEscape(key) + `"/><m:Attachments>` + files + `</m:Attachments></m:CreateAttachment>`
}
func calendarBlobCount(t *testing.T, root string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(filepath.Join(root, "calendar-attachments"), func(path string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}
func TestEWSCreateCalendarAttachmentsRoundTripAndSync(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms, publicURL: "https://mail.example.com"}
	before, err := h.notificationSnapshot(f.ctx, f.guest, &ewsSubscription{all: true})
	if err != nil {
		t.Fatal(err)
	}
	oldKey := ewsCalendarChangeKey(f.original, nil)
	data := []byte{0, 1, 2, 255, 0, 13, 10}
	body := ewsAttachmentCreate(f.original.ID, oldKey, ewsFileUpload("План & бюджет.pdf", "application/pdf", data)+ewsFileUpload("empty.txt", "text/plain", nil))
	response := calendarSOAP(t, h, f.ctx, f.owner, body)
	if response.find("ResponseCode").Text != "NoError" || len(response.find("ResponseMessages").Children) != 2 {
		t.Fatal(response.render())
	}
	saved, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if err != nil {
		t.Fatal(err)
	}
	files, err := f.st.ListCalendarEventAttachments(f.ctx, saved.ID)
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	key := ewsCalendarChangeKey(saved, files)
	for _, result := range response.find("ResponseMessages").Children {
		attachment := result.find("AttachmentId")
		if nodeAttr(attachment, "RootItemId") != saved.ID || nodeAttr(attachment, "RootItemChangeKey") != key || key == oldKey {
			t.Fatal(response.render())
		}
	}
	event, err := calutil.ParseICS(saved.Data)
	if err != nil || len(event.Attachments) != 2 || len(event.Attendees) != 1 || event.UID != f.original.UID {
		t.Fatal(saved.Data, err)
	}
	found := false
	for _, file := range files {
		reference := "calendar:" + saved.ID + ":" + file.ID
		downloaded := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetAttachment><m:AttachmentIds><t:AttachmentId Id="`+reference+`"/></m:AttachmentIds></m:GetAttachment>`)
		if downloaded.find("ResponseCode").Text != "NoError" {
			t.Fatal(downloaded.render())
		}
		got, err := base64.StdEncoding.DecodeString(downloaded.find("Content").Text)
		if err != nil {
			t.Fatal(err)
		}
		if file.Filename == "План & бюджет.pdf" {
			found = true
			if !bytes.Equal(got, data) {
				t.Fatal(got)
			}
		} else if len(got) != 0 {
			t.Fatal(got)
		}
		// Invited users fetch through their own event reference, including EAS.
		shared, err := f.h.calendarAttachment(f.ctx, f.guest, "calendar:"+f.received.ID+":"+file.ID)
		if err != nil || !bytes.Equal(shared.Data, got) {
			t.Fatal(shared, err)
		}
	}
	if !found || calendarBlobCount(t, f.h.ms.Root) != 2 {
		t.Fatal("files missing")
	}
	copy, err := f.st.GetCalendarObjectByID(f.ctx, f.received.ID)
	if err != nil || !strings.Contains(copy.Data, "План & бюджет.pdf") {
		t.Fatal(copy, err)
	}
	after, err := h.notificationSnapshot(f.ctx, f.guest, &ewsSubscription{all: true})
	if err != nil || before[f.received.ID].hash == after[f.received.ID].hash {
		t.Fatal("recipient not notified", err)
	}
	item := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemIds><t:ItemId Id="`+saved.ID+`"/></m:ItemIds></m:GetItem>`)
	if nodeAttr(item.find("ItemId"), "ChangeKey") != key || item.find("HasAttachments").Text != "true" {
		t.Fatal(item.render())
	}
	// Lost-response retries with the old key cannot attach duplicate files.
	retry := calendarSOAP(t, h, f.ctx, f.owner, body)
	if retry.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
		t.Fatal(retry.render())
	}
	if calendarBlobCount(t, f.h.ms.Root) != 2 {
		t.Fatal("retry left extra files")
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("CreateAttachment sent invitations", len(jobs))
	}
}

func TestEWSCreateCalendarAttachmentValidationAndAccess(t *testing.T) {
	for _, kind := range []string{"missing-name", "bad-name", "base64", "item", "inline", "metadata", "duplicate", "missing-content", "stale", "foreign", "missing-parent", "too-many"} {
		t.Run(kind, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			file := ewsFileUpload("plan.txt", "text/plain", []byte("test"))
			id, key, user := f.original.ID, ewsCalendarChangeKey(f.original, nil), f.owner
			expected := "ErrorInvalidRequest"
			switch kind {
			case "missing-name":
				file = strings.Replace(file, "<t:Name>plan.txt</t:Name>", "", 1)
				expected = "ErrorRequiredPropertyMissing"
			case "bad-name":
				file = strings.Replace(file, "plan.txt", "../plan.txt", 1)
			case "base64":
				file = strings.Replace(file, "dGVzdA==", "!not base64!", 1)
			case "item":
				file = `<t:ItemAttachment><t:Name>message</t:Name></t:ItemAttachment>`
				expected = "ErrorInvalidAttachmentId"
			case "inline":
				file = strings.Replace(file, "</t:FileAttachment>", "<t:IsInline>true</t:IsInline></t:FileAttachment>", 1)
			case "metadata":
				file = strings.Replace(file, "text/plain", "bad type", 1)
			case "duplicate":
				file = strings.Replace(file, "</t:FileAttachment>", "<t:Name>other.txt</t:Name></t:FileAttachment>", 1)
			case "missing-content":
				file = strings.Replace(file, "<t:Content>dGVzdA==</t:Content>", "", 1)
				expected = "ErrorRequiredPropertyMissing"
			case "stale":
				key = "old-key"
				expected = "ErrorIrresolvableConflict"
			case "foreign":
				user = f.guest
				expected = "ErrorAccessDenied"
			case "missing-parent":
				id = "missing"
				expected = "ErrorItemNotFound"
			case "too-many":
				file = strings.Repeat(file, 101)
			}
			// A valid first file must not be committed when a later file is invalid.
			response := calendarSOAP(t, h, f.ctx, user, ewsAttachmentCreate(id, key, ewsFileUpload("valid.txt", "text/plain", []byte("first"))+file))
			if response.find("ResponseCode").Text != expected {
				t.Fatal(expected, response.render())
			}
			files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
			if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal("invalid request left files", files, err)
			}
			after, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
			if err != nil || after.Data != f.original.Data || after.ETag != f.original.ETag {
				t.Fatal("invalid request changed event", err)
			}
		})
	}
}

func TestEWSCreateCalendarAttachmentsRollback(t *testing.T) {
	for _, kind := range []string{"metadata", "copy"} {
		t.Run(kind, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			sql := `CREATE TRIGGER fail_ews_attachment BEFORE INSERT ON calendar_attachments WHEN NEW.filename='second.txt' BEGIN SELECT RAISE(ABORT,'injected'); END`
			if kind == "copy" {
				sql = `CREATE TRIGGER fail_ews_attachment BEFORE UPDATE ON calendar_objects WHEN OLD.id='` + f.received.ID + `' BEGIN SELECT RAISE(ABORT,'injected'); END`
			}
			if _, err := f.st.(*storage.Store).DB().Exec(sql); err != nil {
				t.Fatal(err)
			}
			response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("first.txt", "text/plain", []byte("one"))+ewsFileUpload("second.txt", "text/plain", []byte("two"))))
			if response.find("ResponseCode").Text != "ErrorInternalServerError" {
				t.Fatal(response.render())
			}
			for _, before := range []*storage.CalendarObject{f.original, f.received} {
				after, err := f.st.GetCalendarObjectByID(f.ctx, before.ID)
				if err != nil || after.Data != before.Data || after.ETag != before.ETag {
					t.Fatal("partial event commit", err)
				}
			}
			files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
			if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal("rollback left files", files, err)
			}
		})
	}
}

func TestEWSCreateCalendarAttachmentsConcurrentVersion(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	body := ewsAttachmentCreate(f.original.ID, ewsCalendarChangeKey(f.original, nil), ewsFileUpload("once.txt", "text/plain", []byte("one")))
	var wg sync.WaitGroup
	results := make(chan string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := h.createCalendarAttachment(f.ctx, f.owner, body)
			if err != nil {
				results <- "error: " + err.Error()
			} else {
				results <- extractTag(response, "ResponseCode")
			}
		}()
	}
	wg.Wait()
	close(results)
	codes := map[string]int{}
	for code := range results {
		codes[code]++
	}
	if codes["NoError"] != 1 || codes["ErrorIrresolvableConflict"] != 1 {
		t.Fatal(codes)
	}
	files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
	if err != nil || len(files) != 1 || calendarBlobCount(t, f.h.ms.Root) != 1 {
		t.Fatal(files, err)
	}
}

func TestEWSCreateCalendarAttachmentPersonalCopyAndPreservation(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	// The recipient's private attachment must never change the organizer's event.
	response := calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentCreate(f.received.ID, ewsCalendarChangeKey(f.received, nil), ewsFileUpload("personal.txt", "text/plain", []byte("private"))))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	source, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if err != nil || source.Data != f.original.Data {
		t.Fatal("recipient file changed organizer", err)
	}
	files, err := f.st.ListCalendarEventAttachments(f.ctx, source.ID)
	if err != nil || len(files) != 0 {
		t.Fatal("private file leaked", files, err)
	}
	// An imported recurring event retains its rule, zone and alarm after upload.
	calendar, _ := f.st.GetCalendarByName(f.ctx, f.owner.ID, "default")
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:ews-attachment-recurring\r\nDTSTART;TZID=Europe/Moscow:20261110T120000\r\nDTEND;TZID=Europe/Moscow:20261110T130000\r\nRRULE:FREQ=DAILY;COUNT=3\r\nEXDATE;TZID=Europe/Moscow:20261111T120000\r\nSUMMARY:Recurring\r\nBEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nDESCRIPTION:Reminder\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	parsed, err := calutil.ParseICS(ics)
	if err != nil {
		t.Fatal(err)
	}
	object, err := f.st.UpsertCalendarObject(f.ctx, &storage.CalendarObject{CalendarID: calendar.ID, UID: parsed.UID, HrefName: "ews-attachment-recurring.ics", Component: "VEVENT", Data: ics, DTStart: parsed.Start, DTEnd: parsed.End})
	if err != nil {
		t.Fatal(err)
	}
	response = calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(object.ID, ewsCalendarChangeKey(object, nil), ewsFileUpload("series.txt", "text/plain", []byte("series"))))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	saved, err := f.st.GetCalendarObjectByID(f.ctx, object.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []string{"DTSTART;TZID=Europe/Moscow:20261110T120000", "RRULE:FREQ=DAILY;COUNT=3", "EXDATE;TZID=Europe/Moscow:20261111T120000", "BEGIN:VALARM", "TRIGGER:-PT15M"} {
		if !strings.Contains(saved.Data, property) {
			t.Fatal("lost property", property, saved.Data)
		}
	}
}

func TestEWSCreateCalendarAttachmentSizeLimits(t *testing.T) {
	ctx, st, user, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	calendar, _ := st.GetCalendarByName(ctx, user.ID, "default")
	created := calendarSOAP(t, h, ctx, user, standardMeetingCreate(calendar.ID, "SendToNone", ""))
	id := nodeAttr(created.find("ItemId"), "Id")
	key := nodeAttr(created.find("ItemId"), "ChangeKey")
	// This SOAP request exceeds the previous 8 MiB endpoint limit.
	data := bytes.Repeat([]byte("x"), 7<<20)
	response := calendarSOAP(t, h, ctx, user, ewsAttachmentCreate(id, key, ewsFileUpload("large.bin", "application/octet-stream", data)))
	if response.find("ResponseCode").Text != "NoError" {
		t.Fatal(response.render())
	}
	files, err := st.ListCalendarAttachments(ctx, id)
	if err != nil || len(files) != 1 || files[0].Size != int64(len(data)) {
		t.Fatal(files, err)
	}
	saved, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// Enforce the combined decoded limit as well as the transport limit.
	body := ewsAttachmentCreate(id, ewsCalendarChangeKey(saved, files), ewsFileUpload("too-large.bin", "application/octet-stream", bytes.Repeat([]byte("x"), maxEWSCalendarAttachmentBytes+1)))
	result, err := h.createCalendarAttachment(ctx, user, body)
	if err != nil || extractTag(result, "ResponseCode") != "ErrorAttachmentSizeLimitExceeded" {
		t.Fatal(result, err)
	}
	files, err = st.ListCalendarAttachments(ctx, id)
	if err != nil || len(files) != 1 || calendarBlobCount(t, eas.ms.Root) != 1 {
		t.Fatal("oversize upload leaked file", files, err)
	}
}
