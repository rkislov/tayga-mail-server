// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/tayga/tms/internal/mailsearch"
	"github.com/tayga/tms/internal/storage"
)

const ewsTypesNS = "http://schemas.microsoft.com/exchange/services/2006/types"

func parseEWSItemUpload(n *protocolNode) (ewsCalendarUpload, string, string) {
	if n.child("Message") != nil {
		return parseEWSMessageAttachment(n)
	}
	fail := func(message string) (ewsCalendarUpload, string, string) {
		return ewsCalendarUpload{}, "ErrorInvalidRequest", message
	}
	var item *protocolNode
	seen := map[string]bool{}
	for _, f := range n.Children {
		if seen[f.Name.Local] {
			return fail("Duplicate attachment property")
		}
		seen[f.Name.Local] = true
		switch f.Name.Local {
		case "Name", "ContentType", "IsInline":
			if len(f.Children) != 0 {
				return fail("Invalid attachment metadata")
			}
		case "CalendarItem", "Contact", "Task":
			if item != nil {
				return fail("One attached item required")
			}
			item = f
		default:
			return fail("Unsupported attachment property")
		}
	}
	if item == nil {
		return ewsCalendarUpload{}, "ErrorInvalidAttachmentId", "Attached item required"
	}
	name := strings.TrimSpace(n.value("Name"))
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\r\n\x00") {
		return fail("Invalid attachment name")
	}
	if v := n.value("IsInline"); v != "" && v != "false" && v != "0" {
		return fail("Inline attached items are unsupported")
	}
	if err := validateAttachedProperties(item); err != nil {
		return fail(err.Error())
	}
	data, ct, ext, err := exportAttachedProperties(item)
	if err != nil {
		return fail(err.Error())
	}
	if v := n.value("ContentType"); v != "" && v != ct {
		return fail("Attachment content type does not match item")
	}
	if !strings.HasSuffix(strings.ToLower(name), ext) {
		name += ext
	}
	if len(name) > 255 {
		return fail("Attachment name too long")
	}
	properties := item.render()
	if len(data)+len(properties) > maxEWSCalendarAttachmentBytes {
		return ewsCalendarUpload{}, "ErrorAttachmentSizeLimitExceeded", "Attached item exceeds 32 MiB"
	}
	return ewsCalendarUpload{file: &storage.CalendarAttachment{Filename: name, ContentType: ct, Size: int64(len(data)), EWSItemXML: properties}, data: data}, "", ""
}

func validateAttachedProperties(n *protocolNode) error {
	invalid := fmt.Errorf("invalid or unsupported attached %s property", n.Name.Local)
	allowed := "ItemClass Subject Body Sensitivity Categories "
	switch n.Name.Local {
	case "CalendarItem":
		allowed += "UID Start End IsAllDayEvent Location LegacyFreeBusyStatus RequiredAttendees OptionalAttendees Resources Organizer Recurrence StartTimeZone EndTimeZone"
	case "Contact":
		allowed += "DisplayName GivenName MiddleName Surname Nickname CompanyName Department JobTitle FileAs EmailAddresses PhoneNumbers"
	case "Task":
		allowed += "StartDate DueDate CompleteDate Status PercentComplete IsComplete Companies"
	default:
		return invalid
	}
	if len(n.Children) == 0 || len(n.Attr) != 0 {
		return invalid
	}
	seen := map[string]bool{}
	for _, f := range n.Children {
		key := f.Name.Local
		if seen[key] || !strings.Contains(" "+allowed+" ", " "+key+" ") {
			return invalid
		}
		seen[key] = true
		if f.Name.Space != "" && f.Name.Space != ewsTypesNS {
			return invalid
		}
		f.Name.Space = ewsTypesNS
		switch key {
		case "Recurrence":
			if err := validateAttachedRecurrence(f); err != nil {
				return err
			}
		case "StartTimeZone", "EndTimeZone":
			if len(f.Children) != 0 || len(f.Attr) != 1 || f.Attr[0].Name.Space != "" {
				return invalid
			}
			if _, err := calendarZone(nodeAttr(f, "Id")); err != nil {
				return err
			}
		case "Body":
			if len(f.Attr) > 0 && f.Attr[0].Name.Space != "" {
				return invalid
			}
			if len(f.Children) != 0 || len(f.Attr) != 1 || (nodeAttr(f, "BodyType") != "Text" && nodeAttr(f, "BodyType") != "HTML") {
				return invalid
			}
		case "Categories", "Companies":
			if len(f.Attr) != 0 {
				return invalid
			}
			for _, v := range f.Children {
				if v.Name.Local != "String" || len(v.Children) != 0 || len(v.Attr) != 0 {
					return invalid
				}
				v.Name.Space = ewsTypesNS
			}
		case "EmailAddresses", "PhoneNumbers":
			if len(f.Attr) != 0 {
				return invalid
			}
			keys := map[string]bool{}
			for _, v := range f.Children {
				k := nodeAttr(v, "Key")
				valid := false
				if key == "EmailAddresses" {
					valid = k == "EmailAddress1" || k == "EmailAddress2" || k == "EmailAddress3"
				} else {
					valid = strings.Contains(" BusinessPhone BusinessPhone2 HomePhone HomePhone2 MobilePhone BusinessFax HomeFax Pager OtherTelephone ", " "+k+" ")
				}
				if !valid || keys[k] || v.Name.Local != "Entry" || len(v.Attr) != 1 || len(v.Children) != 0 || strings.ContainsAny(v.Text, "\r\n\x00") {
					return invalid
				}
				keys[k] = true
				v.Name.Space = ewsTypesNS
				if key == "EmailAddresses" {
					if _, err := calendarEmail(v.Text); err != nil {
						return invalid
					}
				}
			}
		case "RequiredAttendees", "OptionalAttendees", "Resources", "Organizer":
			if len(f.Attr) != 0 {
				return invalid
			}
			if key == "Organizer" {
				if len(f.Children) != 1 || f.Children[0].Name.Local != "Mailbox" {
					return invalid
				}
				if err := validateAttachedMailbox(f.Children[0]); err != nil {
					return err
				}
			} else {
				for _, a := range f.Children {
					if a.Name.Local != "Attendee" || len(a.Attr) != 0 || len(a.Children) != 1 || a.Children[0].Name.Local != "Mailbox" {
						return invalid
					}
					a.Name.Space = ewsTypesNS
					if err := validateAttachedMailbox(a.Children[0]); err != nil {
						return err
					}
				}
			}
		default:
			if len(f.Children) != 0 || len(f.Attr) != 0 || strings.ContainsRune(f.Text, 0) {
				return invalid
			}
			switch key {
			case "ItemClass":
				if f.Text != map[string]string{"CalendarItem": "IPM.Appointment", "Contact": "IPM.Contact", "Task": "IPM.Task"}[n.Name.Local] {
					return invalid
				}
			case "Start", "End", "StartDate", "DueDate", "CompleteDate":
				if _, err := time.Parse(time.RFC3339, f.Text); err != nil {
					return invalid
				}
			case "IsAllDayEvent", "IsComplete":
				if f.Text != "true" && f.Text != "false" && f.Text != "1" && f.Text != "0" {
					return invalid
				}
			case "PercentComplete":
				v, err := strconv.ParseFloat(f.Text, 64)
				if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
					return invalid
				}
			case "Status":
				if !strings.Contains(" NotStarted InProgress Completed WaitingOnOthers Deferred ", " "+f.Text+" ") || f.Text == "" {
					return invalid
				}
			case "Sensitivity":
				if !strings.Contains(" Normal Personal Private Confidential ", " "+f.Text+" ") || f.Text == "" {
					return invalid
				}
			case "LegacyFreeBusyStatus":
				if !strings.Contains(" Free Tentative Busy OOF NoData ", " "+f.Text+" ") || f.Text == "" {
					return invalid
				}
			}
		}
	}
	if n.Name.Local == "CalendarItem" && n.child("UID") == nil {
		n.Children = append(n.Children, &protocolNode{Name: xml.Name{Space: ewsTypesNS, Local: "UID"}, Text: storage.NewID()})
	}
	if n.Name.Local == "Task" {
		if n.child("StartDate") != nil && n.child("DueDate") != nil {
			start, _ := time.Parse(time.RFC3339, n.value("StartDate"))
			due, _ := time.Parse(time.RFC3339, n.value("DueDate"))
			if due.Before(start) {
				return fmt.Errorf("task due date precedes start")
			}
		}
		completed := n.value("Status") == "Completed" || n.value("IsComplete") == "true" || n.value("IsComplete") == "1" || n.child("CompleteDate") != nil
		if completed {
			if (n.child("Status") != nil && n.value("Status") != "Completed") || (n.child("IsComplete") != nil && (n.value("IsComplete") == "false" || n.value("IsComplete") == "0")) {
				return fmt.Errorf("conflicting task completion state")
			}
		}
	}
	if n.child("ItemClass") == nil {
		n.Children = append(n.Children, &protocolNode{Name: xml.Name{Space: ewsTypesNS, Local: "ItemClass"}, Text: map[string]string{"CalendarItem": "IPM.Appointment", "Contact": "IPM.Contact", "Task": "IPM.Task"}[n.Name.Local]})
	}
	order := "ItemClass Subject Sensitivity Body Categories "
	switch n.Name.Local {
	case "CalendarItem":
		order += "UID Start End IsAllDayEvent LegacyFreeBusyStatus Location Organizer RequiredAttendees OptionalAttendees Resources Recurrence StartTimeZone EndTimeZone"
	case "Contact":
		order += "FileAs DisplayName GivenName MiddleName Nickname CompanyName EmailAddresses PhoneNumbers Department JobTitle Surname"
	case "Task":
		order += "CompleteDate DueDate IsComplete PercentComplete StartDate Status Companies"
	}
	positions := map[string]int{}
	for i, k := range strings.Fields(order) {
		positions[k] = i
	}
	sort.SliceStable(n.Children, func(i, j int) bool { return positions[n.Children[i].Name.Local] < positions[n.Children[j].Name.Local] })
	n.Name.Space = ewsTypesNS
	if n.Name.Local == "CalendarItem" {
		start, e1 := time.Parse(time.RFC3339, n.value("Start"))
		end, e2 := time.Parse(time.RFC3339, n.value("End"))
		if e1 != nil || e2 != nil || !end.After(start) {
			return fmt.Errorf("calendar Start and End required in increasing order")
		}
	}
	return nil
}
func validateAttachedMailbox(n *protocolNode) error {
	bad := fmt.Errorf("invalid attached item mailbox")
	if len(n.Attr) != 0 {
		return bad
	}
	seen := map[string]bool{}
	for _, v := range n.Children {
		key := v.Name.Local
		if seen[key] || (key != "Name" && key != "EmailAddress" && key != "RoutingType") || len(v.Attr) != 0 || len(v.Children) != 0 || strings.ContainsAny(v.Text, "\r\n\x00") {
			return bad
		}
		seen[key] = true
		v.Name.Space = ewsTypesNS
	}
	if _, err := calendarEmail(n.value("EmailAddress")); err != nil {
		return bad
	}
	if r := n.value("RoutingType"); r != "" && r != "SMTP" {
		return bad
	}
	n.Name.Space = ewsTypesNS
	return nil
}

func exportAttachedProperties(n *protocolNode) ([]byte, string, string, error) {
	body := n.value("Body")
	if nodeAttr(n.child("Body"), "BodyType") == "HTML" {
		body = mailsearch.ExtractHTMLText(body)
	}
	if n.Name.Local == "Contact" {
		card := vcard.Card{}
		card.SetValue(vcard.FieldVersion, "3.0")
		display := n.value("DisplayName")
		if display == "" {
			display = strings.TrimSpace(n.value("GivenName") + " " + n.value("Surname"))
		}
		if display == "" {
			display = n.value("Subject")
		}
		if display == "" {
			display = n.value("CompanyName")
		}
		card.SetValue(vcard.FieldFormattedName, display)
		card.SetName(&vcard.Name{FamilyName: n.value("Surname"), GivenName: n.value("GivenName"), AdditionalName: n.value("MiddleName")})
		card.SetValue(vcard.FieldOrganization, n.value("CompanyName"))
		card.SetValue(vcard.FieldTitle, n.value("JobTitle"))
		card.SetValue(vcard.FieldNote, body)
		card.SetValue(vcard.FieldNickname, n.value("Nickname"))
		if categories := n.child("Categories"); categories != nil {
			var values []string
			for _, v := range categories.Children {
				values = append(values, v.Text)
			}
			card.SetCategories(values)
		}
		if dep := n.value("Department"); dep != "" {
			card.SetValue(vcard.FieldOrganization, n.value("CompanyName")+";"+dep)
		}
		if emails := n.child("EmailAddresses"); emails != nil {
			for _, e := range emails.Children {
				card.AddValue(vcard.FieldEmail, e.Text)
			}
		}
		if phones := n.child("PhoneNumbers"); phones != nil {
			for _, p := range phones.Children {
				card.AddValue(vcard.FieldTelephone, p.Text)
			}
		}
		var b strings.Builder
		err := vcard.NewEncoder(&b).Encode(card)
		return []byte(b.String()), "text/vcard", ".vcf", err
	}
	cal := ical.NewCalendar()
	cal.Props.SetText("VERSION", "2.0")
	cal.Props.SetText("PRODID", "-//Tayga//Attached item//EN")
	kind := "VEVENT"
	if n.Name.Local == "Task" {
		kind = "VTODO"
	}
	item := ical.NewComponent(kind)
	cal.Children = append(cal.Children, item)
	uid := n.value("UID")
	if uid == "" {
		uid = storage.NewID()
	}
	item.Props.SetText("UID", uid)
	item.Props.SetDateTime("DTSTAMP", time.Unix(0, 0).UTC())
	item.Props.SetText("SUMMARY", n.value("Subject"))
	item.Props.SetText("DESCRIPTION", body)
	if sensitivity := n.value("Sensitivity"); sensitivity != "" {
		value := "PUBLIC"
		if sensitivity != "Normal" {
			value = "PRIVATE"
		}
		if sensitivity == "Confidential" {
			value = "CONFIDENTIAL"
		}
		item.Props.SetText("CLASS", value)
	}
	if categories := n.child("Categories"); categories != nil {
		for _, c := range categories.Children {
			p := ical.NewProp("CATEGORIES")
			p.SetText(c.Text)
			item.Props.Add(p)
		}
	}
	for field, prop := range map[string]string{"Start": "DTSTART", "End": "DTEND", "StartDate": "DTSTART", "DueDate": "DUE", "CompleteDate": "COMPLETED"} {
		if value := n.value(field); value != "" {
			date, _ := time.Parse(time.RFC3339, value)
			if kind == "VEVENT" && (n.value("IsAllDayEvent") == "true" || n.value("IsAllDayEvent") == "1") {
				item.Props.SetDate(prop, date)
			} else {
				item.Props.SetDateTime(prop, date)
			}
		}
	}
	if kind == "VEVENT" {
		item.Props.SetText("LOCATION", n.value("Location"))
		for _, key := range []string{"RequiredAttendees", "OptionalAttendees", "Resources"} {
			if group := n.child(key); group != nil {
				for _, a := range group.Children {
					m := a.child("Mailbox")
					p := ical.NewProp("ATTENDEE")
					p.Value = "mailto:" + m.value("EmailAddress")
					p.Params.Set("CN", m.value("Name"))
					if key == "OptionalAttendees" {
						p.Params.Set("ROLE", "OPT-PARTICIPANT")
					}
					if key == "Resources" {
						p.Params.Set("CUTYPE", "RESOURCE")
					}
					item.Props.Add(p)
				}
			}
		}
		if org := n.child("Organizer"); org != nil {
			p := ical.NewProp("ORGANIZER")
			p.Value = "mailto:" + org.child("Mailbox").value("EmailAddress")
			item.Props.Set(p)
		}
	} else {
		status := map[string]string{"NotStarted": "NEEDS-ACTION", "InProgress": "IN-PROCESS", "Completed": "COMPLETED", "WaitingOnOthers": "IN-PROCESS", "Deferred": "CANCELLED"}[n.value("Status")]
		if n.value("IsComplete") == "true" || n.value("IsComplete") == "1" || n.child("CompleteDate") != nil {
			status = "COMPLETED"
		}
		if status != "" {
			item.Props.SetText("STATUS", status)
		}
		if value := n.value("PercentComplete"); value != "" {
			v, _ := strconv.ParseFloat(value, 64)
			p := ical.NewProp("PERCENT-COMPLETE")
			p.Value = strconv.Itoa(int(v))
			item.Props.Set(p)
		}
	}
	var b strings.Builder
	err := ical.NewEncoder(&b).Encode(cal)
	if err != nil {
		return nil, "", "", err
	}
	data := b.String()
	if n.Name.Local == "CalendarItem" {
		var edits []ewsCalendarEdit
		for _, key := range []string{"Recurrence", "StartTimeZone", "EndTimeZone"} {
			if f := n.child(key); f != nil {
				edits = append(edits, ewsCalendarEdit{kind: "SetItemField", name: key, node: f})
			}
		}
		zone, e := ewsCalendarZone(edits, "")
		if e != nil {
			return nil, "", "", e
		}
		data, err = finishEWSRecurrence(data, zone, edits)
	}
	return []byte(data), "text/calendar", ".ics", err
}

func ewsAttachedItemXML(a messageAttachment, reference, request string) (string, error) {
	if a.EWSItemXML == "" {
		return ewsAttachedMessageXML(a, reference, request)
	}
	doc, err := parseProtocolXML([]byte(a.EWSItemXML))
	if err != nil {
		return "", err
	}
	n := doc.Children[0]
	if err = validateAttachedProperties(n); err != nil {
		return "", err
	}
	if extractTag(request, "BodyType") == "Text" {
		if body := n.child("Body"); body != nil && nodeAttr(body, "BodyType") == "HTML" {
			body.Text = mailsearch.ExtractHTMLText(body.Text)
			body.Attr = []xml.Attr{{Name: xml.Name{Local: "BodyType"}, Value: "Text"}}
		}
	}
	return n.render(), nil
}

func validateAttachedRecurrence(n *protocolNode) error {
	allowed := "Recurrence DailyRecurrence WeeklyRecurrence AbsoluteMonthlyRecurrence RelativeMonthlyRecurrence AbsoluteYearlyRecurrence RelativeYearlyRecurrence NoEndRecurrence EndDateRecurrence NumberedRecurrence Interval DaysOfWeek DayOfMonth Month DayOfWeekIndex FirstDayOfWeek StartDate EndDate NumberOfOccurrences"
	if len(n.Attr) != 0 || !strings.Contains(" "+allowed+" ", " "+n.Name.Local+" ") {
		return fmt.Errorf("invalid attached recurrence")
	}
	seen := map[string]bool{}
	for _, c := range n.Children {
		if seen[c.Name.Local] {
			return fmt.Errorf("duplicate recurrence property")
		}
		seen[c.Name.Local] = true
		if err := validateAttachedRecurrence(c); err != nil {
			return err
		}
	}
	n.Name.Space = ewsTypesNS
	return nil
}
