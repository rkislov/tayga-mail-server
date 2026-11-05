// SPDX-License-Identifier: Apache-2.0
package httpapi

import (
	"bytes"
	"context"
	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestCalendarSentCopyRestartFailureAndReceipt(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	st := f.s.store
	owner, _ := st.GetUserByID(ctx, f.users[0])
	reader, _ := st.GetUserByID(ctx, f.users[1])
	cal, _ := st.EnsureCalendar(ctx, owner.ID, "default", "Calendar")
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	data, err := calutil.BuildRequestICS(calutil.Event{UID: "sent-copy-worker", Summary: "Durable Sent", Start: &start, End: &end, Organizer: calutil.Attendee{Email: owner.Email}, Attendees: []calutil.Attendee{{Email: reader.Email}, {Email: "outside@remote.example"}}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := st.SaveCalendarClientObject(storage.WithCalendarMailOptions(ctx, true, false), owner.ID, &storage.CalendarObject{CalendarID: cal.ID, UID: "sent-copy-worker", HrefName: "sent-worker.ics", Component: "VEVENT", Data: data, DTStart: &start, DTEnd: &end, CreateOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	var sentID, path, name string
	var seq int
	db := st.(*storage.Store).DB()
	if err = db.QueryRow(`SELECT id FROM calendar_deliveries WHERE target_mailbox='Sent'`).Scan(&sentID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	f.s.store = st
	db = st.(*storage.Store).DB()
	// Fail only Sent insertion, after regular deliveries have been accepted.
	if _, err = db.Exec(`CREATE TRIGGER fail_sent_message BEFORE INSERT ON messages WHEN (SELECT name FROM mailboxes WHERE id=NEW.mailbox_id)='Sent' BEGIN SELECT RAISE(ABORT,'injected Sent failure'); END`); err != nil {
		t.Fatal(err)
	}
	hub := notify.NewHub()
	pushed := []string{}
	hub.SetSink(func(userID string, _ notify.Event) error { pushed = append(pushed, userID); return nil })
	f.s.SetNotify(hub)
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	if err = db.QueryRow(`SELECT state FROM calendar_deliveries WHERE id=?`, sentID).Scan(&state); err != nil || state != "pending" {
		t.Fatal(state, err)
	}
	sent, err := st.GetMailbox(ctx, owner.ID, "Sent")
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := st.ListMessages(ctx, sent.ID)
	if len(messages) != 0 {
		t.Fatal("failed copy committed")
	}
	inbox, _ := st.GetMailbox(ctx, reader.ID, "INBOX")
	delivered, _ := st.ListMessages(ctx, inbox.ID)
	if len(delivered) != 1 {
		t.Fatal("recipient not delivered", len(delivered))
	}
	if _, err = db.Exec(`DROP TRIGGER fail_sent_message`); err != nil {
		t.Fatal(err)
	}
	// The message commits, but completion fails: a restart must only retry the receipt.
	if _, err = db.Exec(`CREATE TRIGGER fail_sent_completion BEFORE UPDATE ON calendar_deliveries WHEN NEW.target_mailbox='Sent' AND NEW.state='done' BEGIN SELECT RAISE(ABORT,'injected completion failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE calendar_deliveries SET next_attempt=0 WHERE id=?`, sentID); err != nil {
		t.Fatal(err)
	}
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	messages, _ = st.ListMessages(ctx, sent.ID)
	if len(messages) != 1 || messages[0].ID != sentID || !strings.Contains(messages[0].Flags, `\Seen`) {
		t.Fatal("invalid Sent copy", messages)
	}
	raw, err := f.s.ms.Read(messages[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	to, err := mail.ParseAddressList(message.Header.Get("To"))
	if err != nil || len(to) != 2 || message.Header.Get("Message-ID") != "<"+sentID+"@tayga>" {
		t.Fatal(message.Header, err)
	}
	if len(pushed) != 1 || pushed[0] != reader.ID {
		t.Fatal("Sent emitted incoming calendar notification", pushed)
	}
	if err = db.QueryRow(`SELECT state FROM calendar_deliveries WHERE id=?`, sentID).Scan(&state); err != nil || state != "notifying" {
		t.Fatal(state, err)
	}
	if err = st.DeleteMessage(ctx, sentID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_sent_completion`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE calendar_deliveries SET next_attempt=0 WHERE id=?`, sentID); err != nil {
		t.Fatal(err)
	}
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	messages, _ = st.ListMessages(ctx, sent.ID)
	if len(messages) != 0 {
		t.Fatal("deleted Sent copy resurrected")
	}
	var queued int
	if err = db.QueryRow(`SELECT COUNT(*) FROM outbound_queue`).Scan(&queued); err != nil || queued != 1 {
		t.Fatal("duplicate external delivery", queued, err)
	}
	delivered, _ = st.ListMessages(ctx, inbox.ID)
	if len(delivered) != 1 {
		t.Fatal("duplicate local delivery")
	}
	saved, err := st.GetCalendarObjectByID(ctx, source.ID)
	if err != nil || saved.ETag != source.ETag {
		t.Fatal("delivery changed calendar", err)
	}
	if err = db.QueryRow(`SELECT state FROM calendar_deliveries WHERE id=?`, sentID).Scan(&state); err != nil || state != "done" {
		t.Fatal(state, err)
	}
}
