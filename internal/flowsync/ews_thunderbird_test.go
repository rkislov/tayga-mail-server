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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		write := func(v any) { w.Header().Set("Content-Type", "application/json"); json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/auth/login", "/api/v1/auth/token":
			write(map[string]any{"tokens": map[string]any{"access_token": "fixture-token", "refresh_token": "fixture-refresh", "expires_in": 3600}})
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
			write(map[string]any{"calendarId": calendar.ID, "bookId": book, "events": evs, "contacts": cs, "mailFlags": flags,
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
