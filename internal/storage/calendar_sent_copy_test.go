// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"context"
	"errors"
	"github.com/tayga/tms/internal/storage/migrations"
	"strings"
	"testing"
)

func TestCalendarSentCopyMigrationKeepsPendingDelivery(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	if _, err := f.s.SaveCalendarClientObject(ctx, f.user.ID, f.event(t, "legacy-pending", 19, 0)); err != nil {
		t.Fatal(err)
	}
	filename := "057_calendar_sent_copy." + f.s.DialectName() + ".sql"
	raw, err := migrations.FS.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	down := strings.SplitN(string(raw), "-- +goose Down", 2)[1]
	if err = execScript(ctx, f.s.db, down); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.db.ExecContext(ctx, f.s.rebind(`DELETE FROM schema_migrations WHERE version=?`), strings.TrimSuffix(filename, ".sql")); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	job, err := reopened.ClaimCalendarDelivery(ctx)
	if err != nil || job.TargetMailbox != "INBOX" || job.Recipient != "external@elsewhere.example" {
		t.Fatal("pending delivery lost/retargeted", job, err)
	}
}

func TestCalendarSentCopyCommitRejectsWrongMailboxAndLateWorker(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	if _, err := f.s.SaveCalendarClientObject(WithCalendarMailOptions(ctx, true, false), f.user.ID, f.event(t, "sent-fencing", 19, 0)); err != nil {
		t.Fatal(err)
	}
	var job *CalendarDelivery
	for {
		next, err := f.s.ClaimCalendarDelivery(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if next.TargetMailbox == "Sent" {
			job = next
			break
		}
	}
	inbox, err := f.s.EnsureMailbox(ctx, f.user.ID, "INBOX", "mail/owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.CommitCalendarDelivery(ctx, job, &Message{MailboxID: inbox.ID, FilePath: "unused"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("Sent was delivered to INBOX", err)
	}
	sent, err := f.s.EnsureMailbox(ctx, f.user.ID, "Sent", "mail/owner/.Sent")
	if err != nil {
		t.Fatal(err)
	}
	forged := *job
	forged.TargetMailbox = "INBOX"
	if err = f.s.CommitCalendarDelivery(ctx, &forged, &Message{MailboxID: inbox.ID, FilePath: "unused"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("forged target accepted", err)
	}
	if err = f.s.CommitCalendarDelivery(ctx, job, &Message{MailboxID: sent.ID, FilePath: "sent-file", Size: 1}); err != nil {
		t.Fatal(err)
	}
	if err = f.s.CommitCalendarDelivery(ctx, job, &Message{MailboxID: sent.ID, FilePath: "duplicate"}); !errors.Is(err, ErrCalendarConflict) {
		t.Fatal("receipt replay accepted", err)
	}
	if err = f.s.CompleteCalendarDelivery(ctx, job); err != nil {
		t.Fatal(err)
	}
	messages, err := f.s.ListMessages(ctx, sent.ID)
	if err != nil || len(messages) != 1 || messages[0].Flags != `\Seen` {
		t.Fatal(messages, err)
	}
}
