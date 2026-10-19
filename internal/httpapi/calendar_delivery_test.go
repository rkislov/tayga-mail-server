package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/notify"
	"github.com/tayga/tms/internal/storage"
)

func queueCalendarFixture(t *testing.T, recipient string) (*fileFixture, string) {
	t.Helper()
	f := newFileFixture(t)
	cal, err := f.s.store.EnsureCalendar(context.Background(), f.users[0], "default", "Calendar")
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/calendar/calendars/"+cal.ID+"/events", fmt.Sprintf(`{"uid":"durable","summary":"Meeting","start":"2026-10-19T10:00:00Z","end":"2026-10-19T11:00:00Z","attendees":[{"email":%q}]}`, recipient), 200)
	objects, err := f.s.store.ListCalendarObjects(context.Background(), cal.ID)
	if err != nil || len(objects) != 1 {
		t.Fatal(err, len(objects))
	}
	return f, objects[0].ID
}

func TestCalendarDeliveryRestartAndNotificationRetry(t *testing.T) {
	f, _ := queueCalendarFixture(t, "reader@example.com")
	ctx := context.Background()
	// Close and reopen SQLite after the HTTP transaction, before any worker.
	var seq int
	var name, path string
	if err := f.s.store.(*storage.Store).DB().QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.s.store.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := storage.Open(ctx, config.StorageConfig{Driver: "sqlite", SQLite: config.SQLiteConfig{Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	f.s.store = st
	hub := notify.NewHub()
	f.s.SetNotify(hub)
	hub.SetSink(func(string, notify.Event) error { return errors.New("push unavailable") })
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, f.users[1], "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := st.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatal(err, len(msgs))
	}
	id := msgs[0].ID
	var state, lastError string
	if err = st.(*storage.Store).DB().QueryRow(`SELECT state,last_error FROM calendar_deliveries WHERE id=?`, id).Scan(&state, &lastError); err != nil || state != "notifying" || lastError != "push unavailable" {
		t.Fatal(err, state, lastError)
	}
	if _, err = st.ClaimCalendarDelivery(ctx); err != storage.ErrNotFound {
		t.Fatal("failed notification retried without backoff", err)
	}
	raw, err := f.s.ms.Read(msgs[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Message-ID: <"+id+"@tayga>") || msgs[0].MessageID != "<"+id+"@tayga>" {
		t.Fatal("message ID drift")
	}
	// Deleting mail must not resurrect it when push is retried after restart.
	if err = st.DeleteMessage(ctx, id); err != nil {
		t.Fatal(err)
	}
	var pushed notify.Event
	hub.SetSink(func(_ string, e notify.Event) error { pushed = e; return nil })
	if _, err = st.(*storage.Store).DB().Exec(`UPDATE calendar_deliveries SET next_attempt=0`); err != nil {
		t.Fatal(err)
	}
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	msgs, err = st.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 0 {
		t.Fatal("mail resurrected", err)
	}
	if pushed.ID != id {
		t.Fatal("notification id changed")
	}
	var data []byte
	if err = st.(*storage.Store).DB().QueryRow(`SELECT state,data FROM calendar_deliveries WHERE id=?`, id).Scan(&state, &data); err != nil || state != "done" || len(data) != 0 {
		t.Fatal(err, state, len(data))
	}
}

func TestCalendarLocalDeliveryReceiptRollsBackWithMessage(t *testing.T) {
	f, _ := queueCalendarFixture(t, "reader@example.com")
	ctx := context.Background()
	db := f.s.store.(*storage.Store).DB()
	if _, err := db.Exec(`CREATE TRIGGER fail_local_mail BEFORE INSERT ON messages BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRow(`SELECT state FROM calendar_deliveries`).Scan(&state); err != nil || state != "pending" {
		t.Fatal("receipt committed without message", state, err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_local_mail`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE calendar_deliveries SET next_attempt=0`); err != nil {
		t.Fatal(err)
	}
	if err := f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mb, err := f.s.store.GetMailbox(ctx, f.users[1], "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := f.s.store.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 1 {
		t.Fatal(err, len(msgs))
	}
}

func TestCalendarDeliveryAtomicExternalHandoffAndLease(t *testing.T) {
	f, _ := queueCalendarFixture(t, "external@remote.example")
	ctx := context.Background()
	st := f.s.store
	db := st.(*storage.Store).DB()
	var wg sync.WaitGroup
	claimed := make(chan *storage.CalendarDelivery, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job, err := st.ClaimCalendarDelivery(ctx)
			if err == nil {
				claimed <- job
			} else if err != storage.ErrNotFound {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	close(claimed)
	var job *storage.CalendarDelivery
	count := 0
	for j := range claimed {
		job = j
		count++
	}
	if count != 1 {
		t.Fatalf("claimed %d times", count)
	}
	if _, err := db.Exec(`UPDATE calendar_deliveries SET lease_until=0`); err != nil {
		t.Fatal(err)
	}
	fresh, err := st.ClaimCalendarDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CommitCalendarDelivery(ctx, job, nil); !errors.Is(err, storage.ErrCalendarConflict) {
		t.Fatal("expired worker committed", err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_enqueue BEFORE INSERT ON outbound_queue BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err = st.CommitCalendarDelivery(ctx, fresh, nil); err == nil {
		t.Fatal("expected injected failure")
	}
	var state string
	if err = db.QueryRow(`SELECT state FROM calendar_deliveries WHERE id=?`, job.ID).Scan(&state); err != nil || state != "pending" {
		t.Fatal("lost job", err, state)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_enqueue`); err != nil {
		t.Fatal(err)
	}
	if err = st.CommitCalendarDelivery(ctx, fresh, nil); err != nil {
		t.Fatal(err)
	}
	out, err := st.GetOutbound(ctx, job.ID)
	if err != nil || string(out.Data) != string(job.Data) || out.MessageID != "<"+job.ID+"@tayga>" {
		t.Fatal("bad handoff", err)
	}
	// Simulate SMTP consumption and worker death before Complete.
	if err = st.DeleteOutbound(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE calendar_deliveries SET lease_until=0`); err != nil {
		t.Fatal(err)
	}
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = st.GetOutbound(ctx, job.ID); err != storage.ErrNotFound {
		t.Fatal("duplicate SMTP handoff", err)
	}
}

func TestCalendarDeliveryFailureRollsBackEventAndCancellation(t *testing.T) {
	f, id := queueCalendarFixture(t, "reader@example.com")
	ctx := context.Background()
	st := f.s.store
	db := st.(*storage.Store).DB()
	o, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_calendar_queue BEFORE INSERT ON calendar_deliveries BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "POST", "/api/v1/calendar/calendars/"+o.CalendarID+"/events", fmt.Sprintf(`{"id":%q,"summary":"Changed","start":"2026-10-19T12:00:00Z","end":"2026-10-19T13:00:00Z","attendees":[{"email":"reader@example.com"}]}`, id), 500)
	got, err := st.GetCalendarObjectByID(ctx, id)
	if err != nil || got.Data != o.Data {
		t.Fatal("event committed without mail", err)
	}
	f.call(0, "DELETE", "/api/v1/calendar/events/"+id, "", 500)
	if _, err = st.GetCalendarObjectByID(ctx, id); err != nil {
		t.Fatal("deletion committed without mail", err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM calendar_deliveries`).Scan(&count); err != nil || count != 1 {
		t.Fatal(err, count)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_calendar_queue`); err != nil {
		t.Fatal(err)
	}
	f.call(0, "DELETE", "/api/v1/calendar/events/"+id, "", 409, "If-Match", "stale")
	f.call(0, "DELETE", "/api/v1/calendar/events/"+id, "", 200, "If-Match", o.ETag)
	if err = f.s.drainCalendarDeliveries(ctx); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, f.users[1], "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := st.ListMessages(ctx, mb.ID)
	if err != nil || len(msgs) != 2 {
		t.Fatal(err, len(msgs))
	}
}
