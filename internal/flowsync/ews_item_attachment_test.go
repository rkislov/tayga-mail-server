// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/storage"
)

func ewsMessageUpload(name string, data []byte) string {
	return `<t:ItemAttachment><t:Name>` + xmlEscape(name) + `</t:Name><t:Message><t:MimeContent CharacterSet="UTF-8">` + base64.StdEncoding.EncodeToString(data) + `</t:MimeContent></t:Message></t:ItemAttachment>`
}

func ewsAttachmentFetch(reference, shape string) string {
	return `<m:GetAttachment>` + shape + `<m:AttachmentIds><t:AttachmentId Id="` + xmlEscape(reference) + `"/></m:AttachmentIds></m:GetAttachment>`
}

func TestEWSCalendarItemAttachmentLifecycle(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms, publicURL: "https://mail.example.com"}
	raw := []byte("From: Sender <sender@example.com>\r\nTo: guest@example.com\r\nSubject: Plan & budget\r\nMessage-ID: <attached@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=parts\r\n\r\n--parts\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<b>Agenda & notes</b>\r\n--parts\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=plan.bin\r\nContent-Transfer-Encoding: base64\r\n\r\nAAEC/w==\r\n--parts--\r\n")
	oldKey := ewsCalendarChangeKey(f.original, nil)
	request := ewsAttachmentCreate(f.original.ID, oldKey, ewsMessageUpload("Meeting mail", raw)+ewsFileUpload("ordinary.eml", "message/rfc822", raw))
	created := calendarSOAP(t, h, f.ctx, f.owner, request)
	if created.find("ItemAttachment") == nil || created.find("FileAttachment") == nil || created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	files, err := f.st.ListCalendarEventAttachments(f.ctx, f.original.ID)
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	var reference, shared string
	for _, file := range files {
		ref := "calendar:" + f.original.ID + ":" + file.ID
		if file.EWSItemType == "Message" {
			reference = ref
			shared = "calendar:" + f.received.ID + ":" + file.ID
		}
		fileBytes, err := f.h.calendarAttachment(f.ctx, f.owner, ref)
		if err != nil || !bytes.Equal(fileBytes.Data, raw) {
			t.Fatal("MIME bytes changed", err)
		}
	}
	if reference == "" {
		t.Fatal("missing embedded message metadata")
	}
	metadata := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemIds><t:ItemId Id="`+f.original.ID+`"/></m:ItemIds></m:GetItem>`)
	if metadata.find("ItemAttachment") == nil || metadata.find("ItemAttachment").find("Message") != nil {
		t.Fatal(metadata.render())
	}
	get := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentFetch(reference, ""))
	if get.find("Message") == nil || get.find("Subject").Text != "Plan & budget" || get.find("MimeContent") != nil || get.find("Message").child("Body").Text != "<b>Agenda & notes</b>" {
		t.Fatal(get.render())
	}
	child := get.find("Message").find("AttachmentId")
	if child == nil {
		t.Fatal("missing nested file reference")
	}
	childID := nodeAttr(child, "Id")
	nested := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentFetch(childID, ""))
	if nested.find("Content").Text != "AAEC/w==" {
		t.Fatal(nested.render())
	}
	shape := `<m:AttachmentShape><t:IncludeMimeContent>true</t:IncludeMimeContent><t:BodyType>Text</t:BodyType></m:AttachmentShape>`
	get = calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentFetch(shared, shape))
	got, err := base64.StdEncoding.DecodeString(get.find("MimeContent").Text)
	if err != nil || !bytes.Equal(got, raw) || nodeAttr(get.find("Message").child("Body"), "BodyType") != "Text" {
		t.Fatal(get.render(), err)
	}
	// Knowing the organizer's IDs doesn't grant the recipient read access.
	denied := calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentFetch(reference, ""))
	if denied.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
		t.Fatal(denied.render())
	}
	denied = calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentFetch(childID, ""))
	if denied.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
		t.Fatal(denied.render())
	}
	retry := calendarSOAP(t, h, f.ctx, f.owner, request)
	if retry.find("ResponseCode").Text != "ErrorIrresolvableConflict" || calendarBlobCount(t, f.h.ms.Root) != 2 {
		t.Fatal(retry.render())
	}
	deleted := calendarSOAP(t, h, f.ctx, f.owner, `<m:DeleteAttachment><m:AttachmentIds><t:AttachmentId Id="`+reference+`"/></m:AttachmentIds></m:DeleteAttachment>`)
	if deleted.find("ResponseCode").Text != "NoError" {
		t.Fatal(deleted.render())
	}
	for _, ref := range []string{reference, childID, shared} {
		user := f.owner
		if ref == shared {
			user = f.guest
		}
		get := calendarSOAP(t, h, f.ctx, user, ewsAttachmentFetch(ref, ""))
		if get.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
			t.Fatal("deleted attachment still accessible", get.render())
		}
	}
	if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
		t.Fatal("attachment operation mailed invitations")
	}
}

func TestEWSStructuredMessageAttachment(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	message := `<t:ItemAttachment><t:Name>Письмо</t:Name><t:Message><t:Subject>План &amp; бюджет</t:Subject><t:Body BodyType="HTML">&lt;b&gt;Встреча&lt;/b&gt;</t:Body><t:DateTimeSent>2026-11-19T09:00:00+03:00</t:DateTimeSent><t:From><t:Mailbox><t:Name>Отправитель</t:Name><t:EmailAddress>sender@example.com</t:EmailAddress></t:Mailbox></t:From><t:ToRecipients><t:Mailbox><t:EmailAddress>recipient@example.com</t:EmailAddress></t:Mailbox></t:ToRecipients></t:Message></t:ItemAttachment>`
	created := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", message))
	if created.find("ResponseCode").Text != "NoError" {
		t.Fatal(created.render())
	}
	ref := nodeAttr(created.find("AttachmentId"), "Id")
	get := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentFetch(ref, ""))
	if get.find("Subject").Text != "План & бюджет" || get.find("Message").child("Body").Text != "<b>Встреча</b>" || get.find("DateTimeSent").Text != "2026-11-19T06:00:00Z" || get.find("From").find("Name").Text != "Отправитель" {
		t.Fatal(get.render())
	}
	file, err := h.attachment(f.ctx, f.owner, ref)
	if err != nil || file.Name != "Письмо.eml" || file.ContentType != "message/rfc822" {
		t.Fatal(file, err)
	}
}

func TestEWSInvalidItemAttachmentsAreAtomic(t *testing.T) {
	valid := ewsMessageUpload("mail", []byte("Subject: Plan\r\n\r\nBody"))
	for name, invalid := range map[string]string{
		"empty":            `<t:ItemAttachment><t:Name>mail</t:Name><t:Message/></t:ItemAttachment>`,
		"unknown-type":     strings.ReplaceAll(valid, "t:Message", "t:Task"),
		"bad-base64":       strings.Replace(valid, "U3ViamVjdDogUGxhbg0KDQpCb2R5", "!", 1),
		"duplicate-mime":   strings.Replace(valid, "</t:Message>", "<t:MimeContent>AA==</t:MimeContent></t:Message>", 1),
		"unsafe-name":      strings.Replace(valid, ">mail<", ">../mail<", 1),
		"unknown-field":    strings.Replace(valid, "</t:Message>", "<t:ItemId Id=\"other\"/></t:Message>", 1),
		"header-injection": `<t:ItemAttachment><t:Name>mail</t:Name><t:Message><t:Subject>Hi&#13;&#10;Bcc: other@example.com</t:Subject></t:Message></t:ItemAttachment>`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			get := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", valid+invalid))
			if get.find("ResponseCode").Text == "NoError" {
				t.Fatal(get.render())
			}
			files, err := f.st.ListCalendarEventAttachments(f.ctx, f.original.ID)
			if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal("partly saved rejected batch", files, err)
			}
		})
	}
}

func TestEWSItemAttachmentStorageFailureRollsBack(t *testing.T) {
	f := newMeetingFixture(t)
	h := &ewsHandler{store: f.st, ms: f.h.ms}
	before, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.st.(*storage.Store).DB().Exec(`CREATE TRIGGER fail_embedded_mail BEFORE INSERT ON calendar_attachments WHEN NEW.ews_item_type='Message' BEGIN SELECT RAISE(ABORT,'injected'); END`)
	if err != nil {
		t.Fatal(err)
	}
	uploads := ewsFileUpload("first.txt", "text/plain", []byte("first")) + ewsMessageUpload("mail", []byte("Subject: Test\r\n\r\nBody"))
	response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", uploads))
	if response.find("ResponseCode").Text == "NoError" {
		t.Fatal(response.render())
	}
	after, err := f.st.GetCalendarObjectByID(f.ctx, f.original.ID)
	if err != nil || after.Data != before.Data || after.ETag != before.ETag {
		t.Fatal("partial calendar update", err)
	}
	files, err := f.st.ListCalendarEventAttachments(f.ctx, f.original.ID)
	if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
		t.Fatal("staged files survived rollback", files, err)
	}
}
