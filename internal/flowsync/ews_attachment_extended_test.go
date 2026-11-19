// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"encoding/base64"
	"github.com/emersion/go-ical"
	"strings"
	"testing"
)

func extendedAttachmentProperty(selector, values string) string {
	return `<t:ExtendedProperty><t:ExtendedFieldURI ` + selector + `/>` + values + `</t:ExtendedProperty>`
}
func TestEWSAttachedExtendedPropertiesRoundTrip(t *testing.T) {
	properties := extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="Integer"`, `<t:Value>-42</t:Value>`) + extendedAttachmentProperty(`DistinguishedPropertySetId="PublicStrings" PropertyName="Project" PropertyType="StringArray"`, `<t:Values><t:Value>Тайга &amp; Outlook</t:Value><t:Value>&lt;safe&gt;</t:Value></t:Values>`) + extendedAttachmentProperty(`PropertySetId="00062008-0000-0000-c000-000000000046" PropertyId="34072" PropertyType="Binary"`, `<t:Value>AAEC/w==</t:Value>`)
	for _, kind := range []string{"CalendarItem", "Contact", "Task", "Message"} {
		t.Run(kind, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			fields := `<t:Subject>Объект</t:Subject>`
			var original []byte
			switch kind {
			case "CalendarItem":
				fields += `<t:Start>2026-11-20T10:00:00Z</t:Start><t:End>2026-11-20T11:00:00Z</t:End>`
			case "Message":
				original = []byte("Subject: Unchanged MIME\r\n\r\nBody")
				fields = `<t:MimeContent>` + base64.StdEncoding.EncodeToString(original) + `</t:MimeContent>`
			}
			item := `<t:` + kind + `>` + fields + properties + `</t:` + kind + `>`
			response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", attachedItemUpload(item)))
			if response.find("ResponseCode").Text != "NoError" {
				t.Fatal(response.render())
			}
			ref := nodeAttr(response.find("AttachmentId"), "Id")
			get := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentFetch(ref, `<m:AttachmentShape><t:IncludeMimeContent>true</t:IncludeMimeContent></m:AttachmentShape>`))
			object := get.find(kind)
			if object == nil {
				t.Fatal(get.render())
			}
			var found []*protocolNode
			for _, c := range object.Children {
				if c.Name.Local == "ExtendedProperty" {
					found = append(found, c)
				}
			}
			if len(found) != 3 || found[0].value("Value") != "-42" || found[1].child("Values").Children[0].Text != "Тайга & Outlook" || found[2].value("Value") != "AAEC/w==" {
				t.Fatal(get.render())
			}
			if kind == "Message" {
				data, err := base64.StdEncoding.DecodeString(object.value("MimeContent"))
				if err != nil || !bytes.Equal(data, original) {
					t.Fatal("extended metadata modified original MIME", err)
				}
			}
			files, err := f.st.ListCalendarEventAttachments(f.ctx, f.received.ID)
			if err != nil || len(files) != 1 {
				t.Fatal(files, err)
			}
			shared := "calendar:" + f.received.ID + ":" + files[0].ID
			get = calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentFetch(shared, ""))
			if get.find("ExtendedProperty") == nil {
				t.Fatal(get.render())
			}
			deleted := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentDelete(ref))
			if deleted.find("ResponseCode").Text != "NoError" {
				t.Fatal(deleted.render())
			}
			get = calendarSOAP(t, h, f.ctx, f.guest, ewsAttachmentFetch(shared, ""))
			if get.find("ExtendedProperty") != nil || get.find("ResponseCode").Text != "ErrorInvalidAttachmentId" {
				t.Fatal(get.render())
			}
		})
	}
}
func TestEWSAttachedExtendedValueTypes(t *testing.T) {
	for kind, value := range map[string]string{"ApplicationTime": "1.25", "Binary": "AA==", "Boolean": "true", "CLSID": "00062008-0000-0000-c000-000000000046", "Currency": "-9223372036854775808", "Double": "1e10", "Float": "1.25", "Integer": "-2147483648", "Long": "18446744073709551615", "Short": "-32768", "String": "&lt;text&gt;", "SystemTime": "2026-11-19T10:00:00+03:00"} {
		t.Run(kind, func(t *testing.T) {
			for _, array := range []bool{false, true} {
				if array && kind == "Boolean" {
					continue
				}
				typ := kind
				values := `<t:Value>` + value + `</t:Value>`
				if array {
					typ += "Array"
					values = `<t:Values>` + values + values + `</t:Values>`
				}
				doc, err := parseProtocolXML([]byte(`<t:ExtendedProperty xmlns:t="` + ewsTypesNS + `"><t:ExtendedFieldURI PropertyTag="0x1234" PropertyType="` + typ + `"/>` + values + `</t:ExtendedProperty>`))
				if err != nil {
					t.Fatal(err)
				}
				if _, err = validateAttachedExtended(doc.Children[0]); err != nil {
					t.Fatal(typ, err)
				}
			}
		})
	}
}
func TestEWSAttachedBadExtendedPropertiesRejectBatch(t *testing.T) {
	valid := extendedAttachmentProperty(`PropertyTag="4660" PropertyType="Integer"`, `<t:Value>1</t:Value>`)
	for _, bad := range []string{
		extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="Integer"`, `<t:Value>2147483648</t:Value>`),
		extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="Double"`, `<t:Value>NaN</t:Value>`),
		extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="Binary"`, `<t:Value>not base64</t:Value>`),
		extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="StringArray"`, `<t:Value>wrong shape</t:Value>`),
		extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="Object"`, `<t:Value>opaque</t:Value>`),
		extendedAttachmentProperty(`PropertyTag="0x1234" PropertyName="ambiguous" PropertyType="String"`, `<t:Value>x</t:Value>`),
		extendedAttachmentProperty(`PropertySetId="invalid" PropertyId="1" PropertyType="String"`, `<t:Value>x</t:Value>`),
		valid + extendedAttachmentProperty(`PropertyTag="0x1234" PropertyType="Integer"`, `<t:Value>2</t:Value>`),
	} {
		t.Run(bad, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			request := ewsFileUpload("first.txt", "text/plain", []byte("first")) + attachedItemUpload(`<t:Task><t:Subject>Task</t:Subject>`+bad+`</t:Task>`)
			response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", request))
			if response.find("ResponseCode").Text == "NoError" {
				t.Fatal(response.render())
			}
			files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
			if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal("rejected batch partly committed", err)
			}
		})
	}
}
func TestEWSAttachedTaskRecurrenceRoundTrip(t *testing.T) {
	patterns := []string{
		`<t:DailyRecurrence><t:Interval>2</t:Interval></t:DailyRecurrence>`,
		`<t:WeeklyRecurrence><t:Interval>1</t:Interval><t:DaysOfWeek>Monday Friday</t:DaysOfWeek></t:WeeklyRecurrence>`,
		`<t:AbsoluteMonthlyRecurrence><t:Interval>1</t:Interval><t:DayOfMonth>15</t:DayOfMonth></t:AbsoluteMonthlyRecurrence>`,
		`<t:RelativeMonthlyRecurrence><t:Interval>1</t:Interval><t:DaysOfWeek>Tuesday</t:DaysOfWeek><t:DayOfWeekIndex>Second</t:DayOfWeekIndex></t:RelativeMonthlyRecurrence>`,
		`<t:AbsoluteYearlyRecurrence><t:DayOfMonth>15</t:DayOfMonth><t:Month>November</t:Month></t:AbsoluteYearlyRecurrence>`,
		`<t:RelativeYearlyRecurrence><t:DaysOfWeek>Tuesday</t:DaysOfWeek><t:DayOfWeekIndex>Second</t:DayOfWeekIndex><t:Month>November</t:Month></t:RelativeYearlyRecurrence>`,
	}
	for _, unit := range []string{"Daily", "Weekly", "Monthly", "Yearly"} {
		patterns = append(patterns, `<t:`+unit+`Regeneration><t:Interval>2</t:Interval></t:`+unit+`Regeneration>`)
	}
	for _, pattern := range patterns {
		for _, span := range []string{`<t:NoEndRecurrence><t:StartDate>2026-11-15</t:StartDate></t:NoEndRecurrence>`, `<t:NumberedRecurrence><t:StartDate>2026-11-15</t:StartDate><t:NumberOfOccurrences>4</t:NumberOfOccurrences></t:NumberedRecurrence>`, `<t:EndDateRecurrence><t:StartDate>2026-11-15</t:StartDate><t:EndDate>2027-11-15</t:EndDate></t:EndDateRecurrence>`} {
			t.Run(pattern+span, func(t *testing.T) {
				f := newMeetingFixture(t)
				h := &ewsHandler{store: f.st, ms: f.h.ms}
				item := `<t:Task><t:Subject>Recurring</t:Subject><t:StartDate>2026-11-15T09:00:00Z</t:StartDate><t:DueDate>2026-11-16T09:00:00Z</t:DueDate><t:Recurrence>` + pattern + span + `</t:Recurrence></t:Task>`
				response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", attachedItemUpload(item)))
				if response.find("ResponseCode").Text != "NoError" {
					t.Fatal(response.render())
				}
				ref := nodeAttr(response.find("AttachmentId"), "Id")
				get := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentFetch(ref, ""))
				task := get.find("Task")
				if task == nil || task.value("IsRecurring") != "true" || task.child("Recurrence") == nil {
					t.Fatal(get.render())
				}
				blob, err := h.attachment(f.ctx, f.owner, ref)
				if err != nil {
					t.Fatal(err)
				}
				cal, err := ical.NewDecoder(strings.NewReader(string(blob.Data))).Decode()
				if err != nil {
					t.Fatal(err)
				}
				todo := cal.Children[0]
				regen := strings.Contains(pattern, "Regeneration")
				if regen {
					if todo.Props.Get("RRULE") != nil || todo.Props.Get("X-TAYGA-EWS-TASK-REGENERATION") == nil {
						t.Fatal(string(blob.Data))
					}
				} else {
					if todo.Props.Get("RRULE") == nil {
						t.Fatal(string(blob.Data))
					}
				}
				if jobs := drainCalendarJobs(t, f.ctx, f.st); len(jobs) != 0 {
					t.Fatal("detached recurrence scheduled mail")
				}
			})
		}
	}
}

func TestEWSAttachedTaskBadRecurrenceIsAtomic(t *testing.T) {
	for _, rec := range []string{
		`<t:DailyRecurrence><t:Interval>0</t:Interval></t:DailyRecurrence><t:NoEndRecurrence><t:StartDate>2026-11-15</t:StartDate></t:NoEndRecurrence>`,
		`<t:DailyRecurrence><t:Interval>1</t:Interval></t:DailyRecurrence><t:NumberedRecurrence><t:StartDate>2026-11-15</t:StartDate><t:NumberOfOccurrences>0</t:NumberOfOccurrences></t:NumberedRecurrence>`,
		`<t:WeeklyRegeneration><t:Interval>2</t:Interval><t:Interval>3</t:Interval></t:WeeklyRegeneration><t:NoEndRecurrence><t:StartDate>2026-11-15</t:StartDate></t:NoEndRecurrence>`,
		`<t:MonthlyRegeneration><t:Interval>1</t:Interval></t:MonthlyRegeneration><t:EndDateRecurrence><t:StartDate>2026-11-15</t:StartDate><t:EndDate>2026-11-01</t:EndDate></t:EndDateRecurrence>`,
		`<t:DailyRecurrence><t:Interval>1</t:Interval></t:DailyRecurrence><t:WeeklyRegeneration><t:Interval>1</t:Interval></t:WeeklyRegeneration>`,
	} {
		t.Run(rec, func(t *testing.T) {
			f := newMeetingFixture(t)
			h := &ewsHandler{store: f.st, ms: f.h.ms}
			item := `<t:Task><t:Subject>Invalid series</t:Subject><t:Recurrence>` + rec + `</t:Recurrence></t:Task>`
			response := calendarSOAP(t, h, f.ctx, f.owner, ewsAttachmentCreate(f.original.ID, "", ewsFileUpload("first.txt", "text/plain", []byte("first"))+attachedItemUpload(item)))
			if response.find("ResponseCode").Text == "NoError" {
				t.Fatal(response.render())
			}
			files, err := f.st.ListCalendarAttachments(f.ctx, f.original.ID)
			if err != nil || len(files) != 0 || calendarBlobCount(t, f.h.ms.Root) != 0 {
				t.Fatal("invalid recurrence left files", files, err)
			}
		})
	}
}
