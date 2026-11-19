// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"strings"
	"testing"
)

func attachedItemUpload(item string) string {
	return `<t:ItemAttachment><t:Name>Объект</t:Name>` + item + `</t:ItemAttachment>`
}
func TestEWSStructuredItemAttachmentsLifecycle(t *testing.T) {
	for _, tc := range []struct{ kind, item, prop, want, ext string }{
		{"CalendarItem", `<t:CalendarItem><t:Subject>Совещание</t:Subject><t:Body BodyType="HTML">&lt;b&gt;Повестка&lt;/b&gt;</t:Body><t:Start>2026-11-20T00:00:00Z</t:Start><t:End>2026-11-21T00:00:00Z</t:End><t:IsAllDayEvent>true</t:IsAllDayEvent><t:Recurrence><t:DailyRecurrence><t:Interval>2</t:Interval></t:DailyRecurrence><t:NumberedRecurrence><t:StartDate>2026-11-20</t:StartDate><t:NumberOfOccurrences>3</t:NumberOfOccurrences></t:NumberedRecurrence></t:Recurrence></t:CalendarItem>`, "Subject", "Совещание", ".ics"},
		{"Contact", `<t:Contact><t:DisplayName>Иван Петров</t:DisplayName><t:GivenName>Иван</t:GivenName><t:Surname>Петров</t:Surname><t:EmailAddresses><t:Entry Key="EmailAddress1">ivan@example.com</t:Entry><t:Entry Key="EmailAddress2">work@example.com</t:Entry></t:EmailAddresses><t:PhoneNumbers><t:Entry Key="MobilePhone">+79990000000</t:Entry></t:PhoneNumbers></t:Contact>`, "DisplayName", "Иван Петров", ".vcf"},
		{"Task", `<t:Task><t:Subject>Подготовить отчёт</t:Subject><t:Body BodyType="Text">Сводка</t:Body><t:StartDate>2026-11-19T09:00:00Z</t:StartDate><t:DueDate>2026-11-20T09:00:00Z</t:DueDate><t:Status>Completed</t:Status><t:PercentComplete>100</t:PercentComplete><t:IsComplete>true</t:IsComplete></t:Task>`, "Subject", "Подготовить отчёт", ".ics"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms, publicURL: "https://mail.example.com"}
			key := ewsCalendarChangeKey(f.original, nil)
			request := ewsAttachmentCreate(f.original.ID, key, attachedItemUpload(tc.item))
			created := calendarSOAP(t, h, f.ctx, f.owner, request)
			if created.find("ResponseCode").Text != "NoError" || created.find("ItemAttachment") == nil {
				t.Fatal(created.render())
			}
			reference := nodeAttr(created.find("AttachmentId"), "Id")
			files, err := f.st.ListCalendarEventAttachments(f.ctx, f.received.ID)
			if err != nil || len(files) != 1 || files[0].EWSItemXML == "" {
				t.Fatal(files, err)
			}
			file := files[0]
			shared := "calendar:" + f.received.ID + ":" + file.ID
			metadata := calendarSOAP(t, h, f.ctx, f.owner, `<m:GetItem><m:ItemIds><t:ItemId Id="`+f.original.ID+`"/></m:ItemIds></m:GetItem>`)
			if metadata.find("ItemAttachment") == nil {
				t.Fatal(metadata.render())
			}
			for _, ref := range []string{reference, shared} {
				user := f.owner
				if ref == shared {
					user = f.guest
				}
				got := calendarSOAP(t, h, f.ctx, user, ewsAttachmentFetch(ref, `<m:AttachmentShape><t:BodyType>Text</t:BodyType></m:AttachmentShape>`))
				item := got.find(tc.kind)
				if item == nil || item.value(tc.prop) != tc.want || item.child("ItemId") != nil {
					t.Fatal(got.render())
				}
			}
			download, err := f.h.calendarAttachment(f.ctx, f.guest, shared)
			if err != nil || !strings.HasSuffix(download.Name, tc.ext) {
				t.Fatal(download, err)
			}
			if tc.kind == "Contact" {
				card, err := vcard.NewDecoder(strings.NewReader(string(download.Data))).Decode()
				if err != nil || len(card[vcard.FieldEmail]) != 2 || card.Value(vcard.FieldFormattedName) != tc.want {
					t.Fatal(card, err)
				}
			} else {
				cal, err := ical.NewDecoder(strings.NewReader(string(download.Data))).Decode()
				if err != nil || len(cal.Children) != 1 {
					t.Fatal(cal, err)
				}
				c := cal.Children[0]
				if tc.kind == "CalendarItem" {
					if c.Props.Get("DTSTART").Params.Get("VALUE") != "DATE" || !strings.Contains(c.Props.Get("RRULE").Value, "COUNT=3") {
						t.Fatal(string(download.Data))
					}
				} else if c.Props.Get("STATUS").Value != "COMPLETED" {
					t.Fatal(string(download.Data))
				}
			}
			retry := calendarSOAP(t, h, f.ctx, f.owner, request)
			if retry.find("ResponseCode").Text != "ErrorIrresolvableConflict" {
				t.Fatal(retry.render())
			}
			denied := calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentFetch(reference, ""))
			if denied.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
				t.Fatal(denied.render())
			}
			deleted := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentDelete(reference))
			if deleted.find("ResponseCode").Text != "NoError" {
				t.Fatal(deleted.render())
			}
			if _, err := f.h.calendarAttachment(f.ctx, f.guest, shared); err == nil {
				t.Fatal("recipient access survived deletion")
			}
			if calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal("deleted blob survives")
			}
			if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
				t.Fatal("attached item caused scheduling mail")
			}
		})
	}
}
func TestEWSStructuredItemRejectionIsAtomic(t *testing.T) {
	for _, item := range []string{
		`<t:CalendarItem><t:Start>2026-11-20T10:00:00Z</t:Start><t:End>2026-11-19T10:00:00Z</t:End></t:CalendarItem>`,
		`<t:Task><t:Subject>one</t:Subject><t:Subject>two</t:Subject></t:Task>`,
		`<t:Task><t:PercentComplete>NaN</t:PercentComplete></t:Task>`,
		`<t:Task><t:PercentComplete>101</t:PercentComplete></t:Task>`,
		`<t:Contact><t:EmailAddresses><t:Entry Key="EmailAddress1">bad</t:Entry></t:EmailAddresses></t:Contact>`,
		`<t:Contact><t:ItemId Id="other"/></t:Contact>`,
		`<t:Task><t:Attachments/></t:Task>`,
	} {
		t.Run(item, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", ewsFileUpload("first.txt", "text/plain", []byte("first"))+attachedItemUpload(item)))
			if response.find("ResponseCode").Text == "NoError" {
				t.Fatal(response.render())
			}
			files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
			if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal(files, err)
			}
		})
	}
}
