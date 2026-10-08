package flowsync

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tayga/tms/internal/storage"
)

const (
	folderTypeCalendar = 8
	folderTypeContacts = 9
	// folderTypeNotes is ActiveSync folder type for Notes (MS-ASCMD).
)

type collectionKind int

const (
	kindMail collectionKind = iota
	kindCalendar
	kindContacts
	kindNotes
)

type calendarAdd struct {
	ServerID  string
	Subject   string
	Location  string
	StartTime string
	EndTime   string
	UID       string
	AllDay    bool
}

type contactAdd struct {
	ServerID  string
	FileAs    string
	FirstName string
	LastName  string
	Email1    string
	Mobile    string
}

func (h *easHandler) resolveCollection(ctx context.Context, userID, collectionID string) (collectionKind, error) {
	if _, err := h.mailboxByID(ctx, userID, collectionID); err == nil {
		return kindMail, nil
	}
	if _, err := h.store.GetCalendarByID(ctx, userID, collectionID); err == nil {
		return kindCalendar, nil
	}
	if _, err := h.store.GetAddressBookByID(ctx, userID, collectionID); err == nil {
		return kindContacts, nil
	}
	if _, err := h.store.GetNoteFolderByID(ctx, userID, collectionID); err == nil {
		return kindNotes, nil
	}
	return 0, storage.ErrNotFound
}

func (h *easHandler) syncCollection(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, reqBody []byte, wbxml bool) (string, []byte, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	_ = h.store.EnsureNoteDefaults(ctx, u.ID)

	collectionID := extractCollectionID(reqBody)
	classHint := strings.ToLower(extractClass(reqBody))

	if collectionID == "" {
		switch {
		case classHint == "calendar":
			cal, err := h.store.GetCalendarByName(ctx, u.ID, "default")
			if err != nil {
				return "", nil, err
			}
			collectionID = cal.ID
		case classHint == "contacts":
			ab, err := h.store.GetAddressBookByName(ctx, u.ID, "default")
			if err != nil {
				return "", nil, err
			}
			collectionID = ab.ID
		case classHint == "notes":
			nf, err := h.store.EnsureNoteFolder(ctx, u.ID, "notes", "Notes")
			if err != nil {
				return "", nil, err
			}
			collectionID = nf.ID
		default:
			mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
			if err != nil {
				return "", nil, err
			}
			collectionID = mb.ID
		}
	}

	kind, err := h.resolveCollection(ctx, u.ID, collectionID)
	if err != nil {
		return "", nil, err
	}

	wbResults := h.applyWritebacks(ctx, u, kind, collectionID, parseSyncClientOps(reqBody))

	key, err := h.store.GetFlowSyncSyncKey(ctx, dev.ID, collectionID)
	if err != nil {
		return "", nil, err
	}
	next := nextSyncKey(key)
	if err := h.store.SetFlowSyncSyncKey(ctx, dev.ID, collectionID, next); err != nil {
		return "", nil, err
	}

	switch kind {
	case kindCalendar:
		return h.syncCalendar(ctx, collectionID, next, wbxml, wbResults)
	case kindContacts:
		return h.syncContacts(ctx, collectionID, next, wbxml, wbResults)
	case kindNotes:
		return h.syncNotes(ctx, u, collectionID, next, wbxml, wbResults)
	default:
		return h.syncMailCollection(ctx, u, collectionID, next, wbxml, wbResults)
	}
}

func (h *easHandler) syncMailCollection(ctx context.Context, u *storage.User, collectionID, next string, wbxml bool, wb []writebackResult) (string, []byte, error) {
	mb, err := h.mailboxByID(ctx, u.ID, collectionID)
	if err != nil {
		return "", nil, err
	}
	msgs, err := h.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return "", nil, err
	}
	window := 25
	if len(msgs) > window {
		msgs = msgs[len(msgs)-window:]
	}
	adds := make([]syncAdd, 0, len(msgs))
	for _, m := range msgs {
		hdr := readMsgHeaders(h.ms, m.FilePath)
		adds = append(adds, syncAdd{
			ServerID: m.ID,
			Subject:  hdr.Subject,
			From:     hdr.From,
			Date:     m.InternalDate.UTC().Format("2006-01-02T15:04:05.000Z"),
			Read:     storage.HasFlag(m.Flags, `\Seen`),
		})
	}
	if wbxml {
		return "", encodeSyncWBXML(next, collectionID, "Email", adds), nil
	}
	return renderMailSyncXML(next, collectionID, adds, wb), nil, nil
}

func (h *easHandler) syncCalendar(ctx context.Context, collectionID, next string, wbxml bool, wb []writebackResult) (string, []byte, error) {
	objs, err := h.store.ListCalendarObjects(ctx, collectionID)
	if err != nil {
		return "", nil, err
	}
	adds := make([]calendarAdd, 0, len(objs))
	for _, o := range objs {
		ev := parseICalEvent(o.Data)
		start, end := formatASTime(o.DTStart), formatASTime(o.DTEnd)
		if start == "" {
			start = ev.Start
		}
		if end == "" {
			end = ev.End
		}
		subject := ev.Summary
		if subject == "" {
			subject = o.UID
		}
		adds = append(adds, calendarAdd{
			ServerID:  o.ID, // calendar object UUID
			Subject:   subject,
			Location:  ev.Location,
			StartTime: start,
			EndTime:   end,
			UID:       o.UID,
			AllDay:    ev.AllDay,
		})
	}
	if wbxml {
		return "", encodeCalendarSyncWBXML(next, collectionID, adds), nil
	}
	return renderCalendarSyncXML(next, collectionID, adds, wb), nil, nil
}

func (h *easHandler) syncContacts(ctx context.Context, collectionID, next string, wbxml bool, wb []writebackResult) (string, []byte, error) {
	objs, err := h.store.ListAddressObjects(ctx, collectionID)
	if err != nil {
		return "", nil, err
	}
	adds := make([]contactAdd, 0, len(objs))
	for _, o := range objs {
		c := parseVCard(o.Data)
		fileAs := c.FN
		if fileAs == "" {
			fileAs = strings.TrimSpace(c.FirstName + " " + c.LastName)
		}
		if fileAs == "" {
			fileAs = o.UID
		}
		adds = append(adds, contactAdd{
			ServerID:  o.ID, // address object UUID
			FileAs:    fileAs,
			FirstName: c.FirstName,
			LastName:  c.LastName,
			Email1:    c.Email,
			Mobile:    c.Tel,
		})
	}
	if wbxml {
		return "", encodeContactsSyncWBXML(next, collectionID, adds), nil
	}
	return renderContactsSyncXML(next, collectionID, adds, wb), nil, nil
}

func renderMailSyncXML(syncKey, collectionID string, adds []syncAdd, wb []writebackResult) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Sync xmlns="AirSync:"><Collections><Collection>`)
	fmt.Fprintf(&b, `<Class>Email</Class><SyncKey>%s</SyncKey>`, xmlEscape(syncKey))
	fmt.Fprintf(&b, `<CollectionId>%s</CollectionId>`, xmlEscape(collectionID))
	b.WriteString(`<Status>1</Status>`)
	b.WriteString(renderWritebackResponses(wb))
	b.WriteString(`<Commands>`)
	for _, a := range adds {
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(a.ServerID))
		b.WriteString(`<ApplicationData>`)
		fmt.Fprintf(&b, `<Email:Subject xmlns:Email="Email:">%s</Email:Subject>`, xmlEscape(a.Subject))
		if a.From != "" {
			fmt.Fprintf(&b, `<Email:From xmlns:Email="Email:">%s</Email:From>`, xmlEscape(a.From))
		}
		fmt.Fprintf(&b, `<Email:DateReceived xmlns:Email="Email:">%s</Email:DateReceived>`, xmlEscape(a.Date))
		fmt.Fprintf(&b, `<Email:Read xmlns:Email="Email:">%d</Email:Read>`, bool01(a.Read))
		b.WriteString(`</ApplicationData></Add>`)
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String()
}

func renderCalendarSyncXML(syncKey, collectionID string, adds []calendarAdd, wb []writebackResult) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Sync xmlns="AirSync:"><Collections><Collection>`)
	fmt.Fprintf(&b, `<Class>Calendar</Class><SyncKey>%s</SyncKey>`, xmlEscape(syncKey))
	fmt.Fprintf(&b, `<CollectionId>%s</CollectionId>`, xmlEscape(collectionID))
	b.WriteString(`<Status>1</Status>`)
	b.WriteString(renderWritebackResponses(wb))
	b.WriteString(`<Commands>`)
	for _, a := range adds {
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(a.ServerID))
		b.WriteString(`<ApplicationData>`)
		fmt.Fprintf(&b, `<Calendar:Subject xmlns:Calendar="Calendar:">%s</Calendar:Subject>`, xmlEscape(a.Subject))
		if a.Location != "" {
			fmt.Fprintf(&b, `<Calendar:Location xmlns:Calendar="Calendar:">%s</Calendar:Location>`, xmlEscape(a.Location))
		}
		if a.StartTime != "" {
			fmt.Fprintf(&b, `<Calendar:StartTime xmlns:Calendar="Calendar:">%s</Calendar:StartTime>`, xmlEscape(a.StartTime))
		}
		if a.EndTime != "" {
			fmt.Fprintf(&b, `<Calendar:EndTime xmlns:Calendar="Calendar:">%s</Calendar:EndTime>`, xmlEscape(a.EndTime))
		}
		fmt.Fprintf(&b, `<Calendar:UID xmlns:Calendar="Calendar:">%s</Calendar:UID>`, xmlEscape(a.UID))
		fmt.Fprintf(&b, `<Calendar:AllDayEvent xmlns:Calendar="Calendar:">%d</Calendar:AllDayEvent>`, bool01(a.AllDay))
		b.WriteString(`</ApplicationData></Add>`)
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String()
}

func renderContactsSyncXML(syncKey, collectionID string, adds []contactAdd, wb []writebackResult) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Sync xmlns="AirSync:"><Collections><Collection>`)
	fmt.Fprintf(&b, `<Class>Contacts</Class><SyncKey>%s</SyncKey>`, xmlEscape(syncKey))
	fmt.Fprintf(&b, `<CollectionId>%s</CollectionId>`, xmlEscape(collectionID))
	b.WriteString(`<Status>1</Status>`)
	b.WriteString(renderWritebackResponses(wb))
	b.WriteString(`<Commands>`)
	for _, a := range adds {
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(a.ServerID))
		b.WriteString(`<ApplicationData>`)
		fmt.Fprintf(&b, `<Contacts:FileAs xmlns:Contacts="Contacts:">%s</Contacts:FileAs>`, xmlEscape(a.FileAs))
		if a.FirstName != "" {
			fmt.Fprintf(&b, `<Contacts:FirstName xmlns:Contacts="Contacts:">%s</Contacts:FirstName>`, xmlEscape(a.FirstName))
		}
		if a.LastName != "" {
			fmt.Fprintf(&b, `<Contacts:LastName xmlns:Contacts="Contacts:">%s</Contacts:LastName>`, xmlEscape(a.LastName))
		}
		if a.Email1 != "" {
			fmt.Fprintf(&b, `<Contacts:Email1Address xmlns:Contacts="Contacts:">%s</Contacts:Email1Address>`, xmlEscape(a.Email1))
		}
		if a.Mobile != "" {
			fmt.Fprintf(&b, `<Contacts:MobilePhoneNumber xmlns:Contacts="Contacts:">%s</Contacts:MobilePhoneNumber>`, xmlEscape(a.Mobile))
		}
		b.WriteString(`</ApplicationData></Add>`)
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String()
}

type icalEvent struct {
	Summary  string
	Location string
	Start    string
	End      string
	AllDay   bool
}

func parseICalEvent(data string) icalEvent {
	var ev icalEvent
	for _, line := range unfoldICS(data) {
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "SUMMARY"):
			ev.Summary = icsValue(line)
		case strings.HasPrefix(upper, "LOCATION"):
			ev.Location = icsValue(line)
		case strings.HasPrefix(upper, "DTSTART"):
			ev.Start, ev.AllDay = normalizeICSDate(icsValue(line), strings.Contains(upper, "VALUE=DATE"))
		case strings.HasPrefix(upper, "DTEND"):
			ev.End, _ = normalizeICSDate(icsValue(line), strings.Contains(upper, "VALUE=DATE"))
		}
	}
	return ev
}

type vcardFields struct {
	FN, FirstName, LastName, Email, Email2, Tel, HomeTel, WorkTel, Org, Title string
}

func parseVCard(data string) vcardFields {
	var c vcardFields
	for _, line := range unfoldICS(data) {
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "FN"):
			c.FN = icsValue(line)
		case strings.HasPrefix(upper, "N:") || strings.HasPrefix(upper, "N;"):
			parts := strings.Split(icsValue(line), ";")
			if len(parts) > 0 {
				c.LastName = parts[0]
			}
			if len(parts) > 1 {
				c.FirstName = parts[1]
			}
		case strings.HasPrefix(upper, "ORG"):
			c.Org = icsValue(line)
		case strings.HasPrefix(upper, "TITLE"):
			c.Title = icsValue(line)
		case strings.HasPrefix(upper, "EMAIL"):
			v := icsValue(line)
			if c.Email == "" {
				c.Email = v
			} else if c.Email2 == "" {
				c.Email2 = v
			}
		case strings.HasPrefix(upper, "TEL"):
			v := icsValue(line)
			switch {
			case strings.Contains(upper, "TYPE=HOME"):
				c.HomeTel = v
			case strings.Contains(upper, "TYPE=WORK"):
				c.WorkTel = v
			case strings.Contains(upper, "TYPE=CELL"), c.Tel == "":
				c.Tel = v
			}
		}
	}
	return c
}

func unfoldICS(data string) []string {
	raw := strings.ReplaceAll(data, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	var out []string
	for _, line := range lines {
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(out) > 0 {
			out[len(out)-1] += strings.TrimLeft(line, " \t")
			continue
		}
		out = append(out, line)
	}
	return out
}

func icsValue(line string) string {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(line[i+1:])
}

func normalizeICSDate(v string, dateOnly bool) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", dateOnly
	}
	if dateOnly || (len(v) == 8 && !strings.Contains(v, "T")) {
		if t, err := time.Parse("20060102", v); err == nil {
			return t.Format("2006-01-02T00:00:00.000Z"), true
		}
	}
	clean := strings.TrimSuffix(v, "Z")
	if t, err := time.Parse("20060102T150405", clean); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05.000Z"), false
	}
	return v, dateOnly
}

func formatASTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func extractClass(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if body[0] == wbxmlVersion {
		return extractWBXMLTagString(body, "Class")
	}
	return extractTag(string(body), "Class")
}
