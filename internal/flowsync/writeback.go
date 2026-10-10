// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

type clientOp struct {
	Kind     string // add | change | delete
	ClientID string
	ServerID string
	Fields   map[string]string
}

type writebackResult struct {
	ClientID string
	ServerID string
	Status   int
	Kind     string // add | change | delete
}

// parseSyncClientOps extracts Add/Change/Delete from Sync request XML (or WBXML-decoded text fields).
func parseSyncClientOps(body []byte) []clientOp {
	if len(body) == 0 {
		return nil
	}
	src := string(body)
	if body[0] == wbxmlVersion {
		return parseWBXMLClientOps(body)
	}
	return parseXMLClientOps(src)
}

func parseXMLClientOps(src string) []clientOp {
	var ops []clientOp
	doc, err := parseProtocolXML([]byte(src))
	if err != nil {
		return nil
	}
	commands := doc.find("Commands")
	if commands == nil {
		return nil
	}
	for _, n := range commands.Children {
		kind := strings.ToLower(n.Name.Local)
		if kind != "add" && kind != "change" && kind != "delete" {
			continue
		}
		ops = append(ops, clientOp{Kind: kind, ClientID: n.value("ClientId"), ServerID: n.value("ServerId"), Fields: extractAppDataFields(n.render())})
	}
	return ops
}

func extractAppDataFields(block string) map[string]string {
	fields := map[string]string{}
	keys := []string{
		"Subject", "Location", "StartTime", "EndTime", "UID", "AllDayEvent",
		"BusyStatus", "OrganizerEmail", "OrganizerName", "Sensitivity", "Timezone",
		"FileAs", "FirstName", "LastName", "MiddleName", "NickName",
		"Email1Address", "Email2Address", "Email3Address",
		"MobilePhoneNumber", "HomePhoneNumber", "BusinessPhoneNumber",
		"CompanyName", "JobTitle", "Department", "OfficeLocation", "WebPage",
		"Read", "Importance", "To", "Cc", "ReplyTo", "Body", "BodyType",
		"LastModifiedDate", "Categories",
	}
	for _, k := range keys {
		if v := extractTag(block, k); v != "" {
			fields[k] = v
		}
	}
	return fields
}

func (h *easHandler) applyWritebacks(ctx context.Context, u *storage.User, kind collectionKind, collectionID string, ops []clientOp) []writebackResult {
	var out []writebackResult
	for _, op := range ops {
		res := writebackResult{ClientID: op.ClientID, ServerID: op.ServerID, Kind: op.Kind, Status: 1}
		var err error
		switch kind {
		case kindCalendar:
			res.ServerID, err = h.wbCalendar(ctx, collectionID, op)
		case kindContacts:
			res.ServerID, err = h.wbContacts(ctx, collectionID, op)
		case kindNotes:
			res.ServerID, err = h.wbNotes(ctx, u, collectionID, op)
		default:
			res.ServerID, err = h.wbMail(ctx, u, collectionID, op)
		}
		if err != nil {
			res.Status = 6 // error
			if res.ServerID == "" {
				res.ServerID = op.ServerID
			}
		}
		out = append(out, res)
	}
	return out
}

func (h *easHandler) wbCalendar(ctx context.Context, calendarID string, op clientOp) (string, error) {
	switch op.Kind {
	case "add":
		uid := op.Fields["UID"]
		if uid == "" {
			uid = storage.NewID()
		}
		href := uid + ".ics"
		start, end := parseASTime(op.Fields["StartTime"]), parseASTime(op.Fields["EndTime"])
		data := buildVEVENT(uid, op.Fields["Subject"], op.Fields["Location"], start, end, op.Fields["AllDayEvent"] == "1")
		o, err := h.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
			CalendarID: calendarID,
			UID:        uid,
			HrefName:   href,
			Component:  "VEVENT",
			DTStart:    start,
			DTEnd:      end,
			Data:       data,
		})
		if err != nil {
			return "", err
		}
		return o.ID, nil
	case "change":
		existing, err := h.store.GetCalendarObjectByID(ctx, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
		if existing.CalendarID != calendarID {
			return op.ServerID, storage.ErrUnauthorized
		}
		uid := existing.UID
		if v := op.Fields["UID"]; v != "" {
			uid = v
		}
		start, end := existing.DTStart, existing.DTEnd
		if t := parseASTime(op.Fields["StartTime"]); t != nil {
			start = t
		}
		if t := parseASTime(op.Fields["EndTime"]); t != nil {
			end = t
		}
		subject := fieldOr(op.Fields["Subject"], parseICalEvent(existing.Data).Summary)
		location := fieldOr(op.Fields["Location"], parseICalEvent(existing.Data).Location)
		allDay := existing.DTStart != nil && op.Fields["AllDayEvent"] == "1"
		if op.Fields["AllDayEvent"] == "" {
			allDay = parseICalEvent(existing.Data).AllDay
		}
		data := buildVEVENT(uid, subject, location, start, end, allDay)
		changed := map[string]bool{}
		for field, property := range map[string]string{"Subject": "SUMMARY", "Location": "LOCATION", "StartTime": "DTSTART", "EndTime": "DTEND", "UID": "UID"} {
			if _, ok := op.Fields[field]; ok {
				changed[property] = true
			}
		}
		if _, ok := op.Fields["AllDayEvent"]; ok {
			changed["DTSTART"], changed["DTEND"] = true, true
		}
		data = mergeDAVProperties(existing.Data, data, "VEVENT", changed)
		_, err = h.store.UpsertCalendarObject(ctx, &storage.CalendarObject{
			ID:         existing.ID,
			CalendarID: calendarID,
			UID:        uid,
			HrefName:   existing.HrefName,
			Component:  "VEVENT",
			DTStart:    start,
			DTEnd:      end,
			Data:       data,
		})
		return existing.ID, err
	case "delete":
		existing, err := h.store.GetCalendarObjectByID(ctx, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
		if existing.CalendarID != calendarID {
			return op.ServerID, storage.ErrUnauthorized
		}
		return op.ServerID, h.store.DeleteCalendarObject(ctx, calendarID, existing.HrefName)
	default:
		return op.ServerID, fmt.Errorf("unknown op")
	}
}

func (h *easHandler) wbContacts(ctx context.Context, addressBookID string, op clientOp) (string, error) {
	switch op.Kind {
	case "add":
		uid := storage.NewID()
		href := uid + ".vcf"
		data := buildVCARD(uid, op.Fields)
		o, err := h.store.UpsertAddressObject(ctx, &storage.AddressObject{
			AddressBookID: addressBookID,
			UID:           uid,
			HrefName:      href,
			Data:          data,
		})
		if err != nil {
			return "", err
		}
		return o.ID, nil
	case "change":
		existing, err := h.store.GetAddressObjectByID(ctx, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
		if existing.AddressBookID != addressBookID {
			return op.ServerID, storage.ErrUnauthorized
		}
		merged := parseVCard(existing.Data)
		f := op.Fields
		if v := f["FileAs"]; v != "" {
			merged.FN = v
		}
		if v := f["FirstName"]; v != "" {
			merged.FirstName = v
		}
		if v := f["LastName"]; v != "" {
			merged.LastName = v
		}
		if v := f["Email1Address"]; v != "" {
			merged.Email = v
		}
		if v := f["Email2Address"]; v != "" {
			merged.Email2 = v
		}
		if v := f["MobilePhoneNumber"]; v != "" {
			merged.Tel = v
		}
		if v := f["HomePhoneNumber"]; v != "" {
			merged.HomeTel = v
		}
		if v := f["BusinessPhoneNumber"]; v != "" {
			merged.WorkTel = v
		}
		if v := f["CompanyName"]; v != "" {
			merged.Org = v
		}
		if v := f["JobTitle"]; v != "" {
			merged.Title = v
		}
		data := buildVCARD(existing.UID, map[string]string{
			"FileAs": merged.FN, "FirstName": merged.FirstName, "LastName": merged.LastName,
			"Email1Address": merged.Email, "Email2Address": merged.Email2,
			"MobilePhoneNumber": merged.Tel, "HomePhoneNumber": merged.HomeTel,
			"BusinessPhoneNumber": merged.WorkTel, "CompanyName": merged.Org, "JobTitle": merged.Title,
		})
		_, err = h.store.UpsertAddressObject(ctx, &storage.AddressObject{
			ID:            existing.ID,
			AddressBookID: addressBookID,
			UID:           existing.UID,
			HrefName:      existing.HrefName,
			Data:          data,
		})
		return existing.ID, err
	case "delete":
		existing, err := h.store.GetAddressObjectByID(ctx, op.ServerID)
		if err != nil {
			return op.ServerID, err
		}
		if existing.AddressBookID != addressBookID {
			return op.ServerID, storage.ErrUnauthorized
		}
		return op.ServerID, h.store.DeleteAddressObject(ctx, addressBookID, existing.HrefName)
	default:
		return op.ServerID, fmt.Errorf("unknown op")
	}
}

func (h *easHandler) wbMail(ctx context.Context, u *storage.User, mailboxID string, op clientOp) (string, error) {
	msg, err := h.store.GetMessageByID(ctx, op.ServerID)
	if err != nil {
		return op.ServerID, err
	}
	if msg.MailboxID != mailboxID {
		return op.ServerID, storage.ErrUnauthorized
	}
	// ownership of mailbox
	if _, err := h.mailboxByID(ctx, u.ID, mailboxID); err != nil {
		return op.ServerID, err
	}
	switch op.Kind {
	case "change":
		flags := storage.ParseFlags(msg.Flags)
		if v, ok := op.Fields["Read"]; ok {
			if v == "1" {
				flags = addFlag(flags, `\Seen`)
			} else {
				flags = removeFlag(flags, `\Seen`)
			}
		}
		return op.ServerID, h.store.UpdateMessageFlags(ctx, msg.ID, storage.NormalizeFlags(flags))
	case "delete":
		moves, _ := ctx.Value(deleteMovesKey{}).(bool)
		if moves {
			mb, err := h.mailboxByID(ctx, u.ID, mailboxID)
			if err != nil {
				return op.ServerID, err
			}
			if !strings.EqualFold(mb.Name, "Trash") {
				if err := (&ewsHandler{store: h.store, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
					return op.ServerID, err
				}
				trash, err := h.store.GetMailbox(ctx, u.ID, "Trash")
				if err != nil {
					return op.ServerID, err
				}
				_, err = h.store.MoveMessage(ctx, msg.ID, trash.ID)
				return op.ServerID, err
			}
		}
		return op.ServerID, h.store.DeleteMessage(ctx, msg.ID)
	case "add":
		// Creating raw MIME via ActiveSync is out of scope for this milestone.
		return "", fmt.Errorf("mail add not supported")
	default:
		return op.ServerID, fmt.Errorf("unknown op")
	}
}

func addFlag(flags []string, want string) []string {
	if storage.HasFlag(strings.Join(flags, " "), want) {
		return flags
	}
	return append(flags, want)
}

func removeFlag(flags []string, want string) []string {
	want = strings.ToLower(want)
	var out []string
	for _, f := range flags {
		if strings.ToLower(f) != want {
			out = append(out, f)
		}
	}
	return out
}

func fieldOr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func parseASTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	layouts := []string{
		"20060102T150405Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}

func buildVEVENT(uid, subject, location string, start, end *time.Time, allDay bool) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//FlowSync//EN\r\nBEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	if subject != "" {
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(subject))
	}
	if location != "" {
		fmt.Fprintf(&b, "LOCATION:%s\r\n", icsEscape(location))
	}
	if start != nil {
		if allDay {
			fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", start.Format("20060102"))
		} else {
			fmt.Fprintf(&b, "DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z"))
		}
	}
	if end != nil {
		if allDay {
			fmt.Fprintf(&b, "DTEND;VALUE=DATE:%s\r\n", end.Format("20060102"))
		} else {
			fmt.Fprintf(&b, "DTEND:%s\r\n", end.UTC().Format("20060102T150405Z"))
		}
	}
	b.WriteString("END:VEVENT\r\nEND:VCALENDAR\r\n")
	return b.String()
}

func buildVCARD(uid string, f map[string]string) string {
	fn := f["FileAs"]
	if fn == "" {
		fn = strings.TrimSpace(f["FirstName"] + " " + f["LastName"])
	}
	var b strings.Builder
	b.WriteString("BEGIN:VCARD\r\nVERSION:3.0\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	if fn != "" {
		fmt.Fprintf(&b, "FN:%s\r\n", icsEscape(fn))
	}
	fmt.Fprintf(&b, "N:%s;%s;%s;;\r\n", icsEscape(f["LastName"]), icsEscape(f["FirstName"]), icsEscape(f["MiddleName"]))
	if f["NickName"] != "" {
		fmt.Fprintf(&b, "NICKNAME:%s\r\n", icsEscape(f["NickName"]))
	}
	if f["Email1Address"] != "" {
		fmt.Fprintf(&b, "EMAIL;TYPE=INTERNET:%s\r\n", icsEscape(f["Email1Address"]))
	}
	if f["Email2Address"] != "" {
		fmt.Fprintf(&b, "EMAIL;TYPE=INTERNET:%s\r\n", icsEscape(f["Email2Address"]))
	}
	if f["Email3Address"] != "" {
		fmt.Fprintf(&b, "EMAIL;TYPE=INTERNET:%s\r\n", icsEscape(f["Email3Address"]))
	}
	if f["MobilePhoneNumber"] != "" {
		fmt.Fprintf(&b, "TEL;TYPE=CELL:%s\r\n", icsEscape(f["MobilePhoneNumber"]))
	}
	if f["HomePhoneNumber"] != "" {
		fmt.Fprintf(&b, "TEL;TYPE=HOME:%s\r\n", icsEscape(f["HomePhoneNumber"]))
	}
	if f["BusinessPhoneNumber"] != "" {
		fmt.Fprintf(&b, "TEL;TYPE=WORK:%s\r\n", icsEscape(f["BusinessPhoneNumber"]))
	}
	if f["CompanyName"] != "" {
		fmt.Fprintf(&b, "ORG:%s\r\n", icsEscape(f["CompanyName"]))
	}
	if f["JobTitle"] != "" {
		fmt.Fprintf(&b, "TITLE:%s\r\n", icsEscape(f["JobTitle"]))
	}
	if f["Department"] != "" {
		fmt.Fprintf(&b, "X-DEPARTMENT:%s\r\n", icsEscape(f["Department"]))
	}
	if f["OfficeLocation"] != "" {
		fmt.Fprintf(&b, "X-OFFICE:%s\r\n", icsEscape(f["OfficeLocation"]))
	}
	if f["WebPage"] != "" {
		fmt.Fprintf(&b, "URL:%s\r\n", icsEscape(f["WebPage"]))
	}
	b.WriteString("END:VCARD\r\n")
	return b.String()
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `;`, `\;`)
	s = strings.ReplaceAll(s, `,`, `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func renderWritebackResponses(results []writebackResult) string {
	if len(results) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<Responses>`)
	for _, r := range results {
		tag := "Add"
		switch r.Kind {
		case "change":
			tag = "Change"
		case "delete":
			tag = "Delete"
		}
		fmt.Fprintf(&b, `<%s>`, tag)
		if r.ClientID != "" {
			fmt.Fprintf(&b, `<ClientId>%s</ClientId>`, xmlEscape(r.ClientID))
		}
		if r.ServerID != "" {
			fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(r.ServerID))
		}
		fmt.Fprintf(&b, `<Status>%d</Status>`, r.Status)
		fmt.Fprintf(&b, `</%s>`, tag)
	}
	b.WriteString(`</Responses>`)
	return b.String()
}
