package flowsync

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

// Opt in: launches the installed Thunderbird using a disposable profile.
func TestTaygaEWSThunderbirdNative(t *testing.T) {
	if os.Getenv("TAYGA_THUNDERBIRD_E2E") != "1" {
		t.Skip("set TAYGA_THUNDERBIRD_E2E=1 to run Thunderbird")
	}
	ctx, st, u, eas, _, book := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	var sentMu sync.Mutex
	sentCount := 0
	sentAttachment := false
	var lostCreate atomic.Bool
	sentDateElement := regexp.MustCompile(`<(?:[A-Za-z0-9_]+:)?DateTimeSent\b[^>]*>[^<]*</(?:[A-Za-z0-9_]+:)?DateTimeSent>`)
	var legacySentDate atomic.Bool
	legacySentDate.Store(true)
	h.sender = &mailSubmission{store: st, ms: eas.ms, submit: func(_ context.Context, _ *storage.User, _ []string, raw []byte) error {
		sentMu.Lock()
		defer sentMu.Unlock()
		sentCount++
		content, e := parseMessageContent(raw)
		if e == nil {
			for _, a := range content.Attachments {
				if a.Name == "test.txt" && string(a.Data) == "attachment contents" {
					sentAttachment = true
				}
			}
		}
		return nil
	}}
	calendar, _ := st.GetCalendarByName(ctx, u.ID, "default")
	guest, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "bob", Email: "bob@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureDAVDefaults(ctx, guest.ID); err != nil {
		t.Fatal(err)
	}
	organizerCalendar, _ := st.GetCalendarByName(ctx, guest.ID, "default")
	incomingRaw := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Tayga//Incoming invitation test//EN\r\nBEGIN:VEVENT\r\nUID:incoming-meeting\r\nDTSTART:20261110T090000Z\r\nDTEND:20261110T100000Z\r\nSUMMARY:Встреча от Bob <b>2026</b>\r\nLOCATION:Комната А\r\nRRULE:FREQ=DAILY;COUNT=2\r\nORGANIZER:mailto:bob@example.com\r\nATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:alice@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	incoming, err := st.SaveCalendarClientObject(ctx, guest.ID, &storage.CalendarObject{CalendarID: organizerCalendar.ID, UID: "incoming-meeting", HrefName: "incoming-meeting.ics", Component: "VEVENT", Data: incomingRaw, CreateOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	event := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Tayga//Test//EN\nBEGIN:VEVENT\nUID:server-event\nDTSTART:20261026T090000Z\nDTEND:20261026T100000Z\nSUMMARY:Server event\nRRULE:FREQ=DAILY;COUNT=4\nEND:VEVENT\nBEGIN:VEVENT\nUID:server-event\nRECURRENCE-ID:20261027T090000Z\nDTSTART:20261027T110000Z\nDTEND:20261027T120000Z\nSUMMARY:Exception\nEND:VEVENT\nEND:VCALENDAR\n"
	saved, err := st.UpsertCalendarObject(ctx, &storage.CalendarObject{CalendarID: calendar.ID, UID: "server-event", HrefName: "server-event.ics", Component: "VEVENT", Data: event})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertAddressObject(ctx, &storage.AddressObject{AddressBookID: book, UID: "server-person", HrefName: "server-person.vcf", Data: "BEGIN:VCARD\nVERSION:4.0\nUID:server-person\nFN:Server person\nEMAIL:first@example.com\nEMAIL:second@example.com\nNOTE:Keep me\nEND:VCARD\n"})
	if err != nil {
		t.Fatal(err)
	}
	if err = h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Projects", "Projects/2026", "Projects/2026/October"} {
		if _, err := st.EnsureMailbox(ctx, u.ID, name, ""); err != nil {
			t.Fatal(err)
		}
	}
	inbox, _ := st.GetMailbox(ctx, u.ID, "INBOX")
	raw := []byte("From: sender@example.com\r\nTo: alice@example.com\r\nSubject: Tayga EWS native smoke\r\nMessage-ID: <ews-native-test@example.com>\r\nDate: Mon, 19 Oct 2026 12:34:56 +0300\r\n\r\nHello from EWS\r\n")
	path, size, err := eas.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size, Subject: "Tayga EWS native smoke", FromAddr: "sender@example.com", ToAddr: u.Email, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	var incomingReplyCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		write := func(v any) { w.Header().Set("Content-Type", "application/json"); json.NewEncoder(w).Encode(v) }
		if strings.HasPrefix(r.URL.Path, "/api/v1/calendar/") {
			if r.Header.Get("Authorization") != "Bearer fixture-token" {
				http.Error(w, "unauthorized", 401)
				return
			}
			if r.URL.Path == "/api/v1/calendar/invites" {
				invites, err := st.ListCalendarInvitesForUser(ctx, u.ID, false)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				items := []map[string]any{}
				for _, inv := range invites {
					item := map[string]any{"id": inv.ID, "calendar_object_id": inv.CalendarObjectID, "event_uid": inv.EventUID, "summary": inv.Summary, "partstat": inv.PartStat, "can_reply": false}
					if obj, err := st.GetCalendarObjectByID(ctx, inv.CalendarObjectID); err == nil {
						ev, _ := calutil.ParseICS(obj.Data)
						item["etag"], item["cancelled"], item["can_reply"] = obj.ETag, ev.Status == "CANCELLED", ev.Status != "CANCELLED"
						item["recurring"], item["all_day"], item["location"] = strings.Contains(obj.Data, "RRULE:"), ev.AllDay, ev.Location
						item["organizer"] = map[string]string{"email": ev.Organizer.Email, "name": ev.Organizer.Name}
						if ev.Start != nil {
							item["start"] = ev.Start.UTC().Format(time.RFC3339)
						}
						if ev.End != nil {
							item["end"] = ev.End.UTC().Format(time.RFC3339)
						}
					}
					items = append(items, item)
				}
				write(map[string]any{"invites": items})
				return
			}
			if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/reply") {
				id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/calendar/invites/"), "/reply")
				inv, err := st.GetCalendarInvite(ctx, id)
				if err != nil || inv.AttendeeUserID != u.ID {
					http.Error(w, "not found", 404)
					return
				}
				var request struct {
					Action string `json:"action"`
					ETag   string `json:"etag"`
				}
				if json.NewDecoder(r.Body).Decode(&request) != nil {
					http.Error(w, "bad request", 400)
					return
				}
				status := map[string]string{"accept": "ACCEPTED", "tentative": "TENTATIVE", "decline": "DECLINED"}[request.Action]
				if status == "" {
					http.Error(w, "bad action", 400)
					return
				}
				inv, err = st.ReplyCalendarInvite(ctx, u.ID, id, status, nil, nil, request.ETag)
				if err != nil {
					http.Error(w, "invitation changed", 409)
					return
				}
				incomingReplyCount.Add(1)
				write(map[string]string{"id": inv.ID, "partstat": inv.PartStat})
				return
			}
		}
		switch r.URL.Path {
		case "/api/v1/auth/login", "/api/v1/auth/token":
			write(map[string]any{"tokens": map[string]any{"access_token": "fixture-token", "refresh_token": "fixture-refresh", "expires_in": 3600}})
			return
		case "/test/incoming/move", "/test/incoming/cancel":
			obj, err := st.GetCalendarObjectByID(ctx, incoming.ID)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if r.URL.Path == "/test/incoming/cancel" {
				err = st.DeleteCalendarMeeting(ctx, guest.ID, obj)
			} else {
				ev, _ := calutil.ParseICS(obj.Data)
				ev.Summary = "Updated invitation from Bob"
				ev.Start, ev.End = parseASTime("2026-11-10T11:00:00Z"), parseASTime("2026-11-10T12:00:00Z")
				obj.Data, err = calutil.BuildSimpleVEVENT(ev.Event)
				obj.ExpectedETag = obj.ETag
				if err == nil {
					_, err = st.SaveCalendarClientObject(ctx, guest.ID, obj)
				}
			}
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			write(map[string]bool{"ok": true})
			return
		case "/test/meeting/reply":
			invites, err := st.ListCalendarInvitesForUser(ctx, guest.ID, false)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			for _, inv := range invites {
				if inv.EventUID != "native-meeting" {
					continue
				}
				copy, err := st.GetCalendarObjectByID(ctx, inv.CalendarObjectID)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				xml := `<MeetingResponse xmlns="MeetingResponse:"><Request><UserResponse>1</UserResponse><CollectionId>` + copy.CalendarID + `</CollectionId><RequestId>` + copy.ID + `</RequestId><SendResponse/></Request></MeetingResponse>`
				wire, err := encodeWire(xml)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=MeetingResponse&DeviceId=native-guest&DeviceType=iPhone", bytes.NewReader(wire)).WithContext(withUser(ctx, guest))
				req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
				req.Header.Set("MS-ASProtocolVersion", "16.1")
				recorded := httptest.NewRecorder()
				eas.ServeHTTP(recorded, req)
				decoded, err := decodeWire(recorded.Body.Bytes())
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				response, err := parseProtocolXML(decoded)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				write(map[string]string{"status": response.find("Status").Text})
				return
			}
			http.Error(w, "meeting not found", 404)
			return
		case "/test/meeting/move":
			obj, err := st.GetCalendarObjectByUID(ctx, calendar.ID, "native-meeting")
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			body := meetingEWSUpdate(obj.ID, ewsCalendarChangeKey(obj, nil), "SendOnlyToAll", `<t:Subject>Moved from another client</t:Subject><t:Start>2026-11-07T11:00:00Z</t:Start><t:End>2026-11-07T12:00:00Z</t:End>`)
			response, err := h.updateItem(ctx, u, body)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			doc, err := parseProtocolXML([]byte(response))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			write(map[string]string{"code": doc.find("ResponseCode").Text})
			return
		case "/test/state":
			events, _ := st.ListCalendarObjects(ctx, calendar.ID)
			cards, _ := st.ListAddressObjects(ctx, book)
			evs := []map[string]string{}
			cs := []map[string]string{}
			for _, e := range events {
				evs = append(evs, map[string]string{"uid": e.UID, "data": e.Data})
			}
			for _, c := range cards {
				cs = append(cs, map[string]string{"uid": c.UID, "data": c.Data})
			}
			mail, _ := st.ListMessages(ctx, inbox.ID)
			flags := ""
			if len(mail) > 0 {
				flags = mail[0].Flags
			}
			incomingPartstat := ""
			if original, err := st.GetCalendarObjectByID(ctx, incoming.ID); err == nil {
				parsed, _ := calutil.ParseICS(original.Data)
				for _, attendee := range parsed.Attendees {
					if attendee.Email == u.Email {
						incomingPartstat = attendee.PartStat
					}
				}
			}
			meeting := map[string]string{}
			invites, _ := st.ListCalendarInvitesForUser(ctx, guest.ID, false)
			for _, inv := range invites {
				if inv.EventUID == "native-meeting" {
					meeting["partstat"] = inv.PartStat
					if copy, err := st.GetCalendarObjectByID(ctx, inv.CalendarObjectID); err == nil {
						meeting["copy"] = copy.Data
						if parsed, err := calutil.ParseICS(copy.Data); err == nil {
							meeting["status"] = parsed.Status
						}
					}
				}
			}
			write(map[string]any{"incoming_partstat": incomingPartstat, "incoming_replies": incomingReplyCount.Load(), "meeting": meeting, "calendarId": calendar.ID, "bookId": book, "events": evs, "contacts": cs, "mailFlags": flags,
				"sentCopies": func() int {
					mb, e := st.GetMailbox(ctx, u.ID, "Sent")
					if e != nil {
						return 0
					}
					msgs, _ := st.ListMessages(ctx, mb.ID)
					return len(msgs)
				}(),
				"sentAttachment": func() bool { sentMu.Lock(); defer sentMu.Unlock(); return sentAttachment }(),
				"sentCount":      func() int { sentMu.Lock(); defer sentMu.Unlock(); return sentCount }()})
			return
		case "/test/restore-dates":
			legacySentDate.Store(false)
			msgs, _ := st.ListMessages(ctx, inbox.ID)
			if len(msgs) > 0 {
				if err := st.UpdateMessageFlags(ctx, msgs[0].ID, `\Flagged`); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
			write(map[string]bool{"ok": true})
			return
		case "/test/mutate":
			if r.URL.Query().Get("contact") == "delete" {
				cards, _ := st.ListAddressObjects(ctx, book)
				for _, card := range cards {
					if card.UID == "server-person" {
						st.DeleteAddressObject(ctx, book, card.HrefName)
					}
				}
				write(map[string]bool{"ok": true})
				return
			}
			current, _ := st.GetCalendarObjectByID(ctx, saved.ID)
			current.Data = strings.Replace(current.Data, "SUMMARY:", "SUMMARY:Web concurrent ", 1)
			_, e := st.UpsertCalendarObject(ctx, current)
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			write(map[string]bool{"ok": true})
			return
		case "/EWS/Exchange.asmx":
			if r.Header.Get("Authorization") != "Bearer fixture-token" {
				w.Header().Set("WWW-Authenticate", `Basic realm="FlowSync", Bearer`)
				http.Error(w, "unauthorized", 401)
				return
			}
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			if strings.Contains(string(body), "TaygaCreateId") && strings.Contains(string(body), "UID:native-event") && !lostCreate.Swap(true) {
				recorded := httptest.NewRecorder()
				h.ServeHTTP(recorded, r.WithContext(withUser(r.Context(), u)))
				if !strings.Contains(recorded.Body.String(), "NoError") {
					w.Write(recorded.Body.Bytes())
					return
				}
				http.Error(w, "fixture response lost after commit", 503)
				return
			}
			if legacySentDate.Load() {
				recorded := httptest.NewRecorder()
				h.ServeHTTP(recorded, r.WithContext(withUser(r.Context(), u)))
				for key, values := range recorded.Header() {
					w.Header()[key] = values
				}
				w.WriteHeader(recorded.Code)
				io.WriteString(w, sentDateElement.ReplaceAllString(recorded.Body.String(), ""))
				return
			}
			h.ServeHTTP(w, r.WithContext(withUser(r.Context(), u)))
			return
		default:
			write(map[string]any{"enabled": false, "scripts": []any{}})
		}
	}))
	defer server.Close()
	runctx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runctx, "node", "../../extensions/thunderbird-tayga/tests/thunderbird-smoke.mjs")
	cmd.Env = append(os.Environ(), "TAYGA_EWS_FIXTURE="+server.URL)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
