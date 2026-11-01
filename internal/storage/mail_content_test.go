package storage

import (
	"context"
	"testing"
)

func TestMessageContentMoveAtomic(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	inbox, err := f.s.EnsureMailbox(ctx, f.user.ID, "INBOX", "inbox")
	if err != nil {
		t.Fatal(err)
	}
	junk, err := f.s.EnsureMailbox(ctx, f.user.ID, "Junk", "junk")
	if err != nil {
		t.Fatal(err)
	}
	f.s.SetSpamLearningPolicy(func() (bool, string) { return true, "Junk" })
	m, err := f.s.InsertMessage(ctx, &Message{MailboxID: inbox.ID, FilePath: "original", Size: 1234, Subject: "unchanged", Flags: `\Flagged`})
	if err != nil {
		t.Fatal(err)
	}
	// The destination failure leaves the original path and mailbox intact.
	if _, err = f.s.MoveMessageContent(ctx, m, "missing-mailbox", "staged", 456, true); err == nil {
		t.Fatal("invalid destination accepted")
	}
	current, _ := f.s.GetMessageByID(ctx, m.ID)
	if current.FilePath != m.FilePath || current.MailboxID != m.MailboxID {
		t.Fatal("partial failed move")
	}
	current, err = f.s.MoveMessageContent(ctx, m, junk.ID, "compressed", 456, true)
	if err != nil {
		t.Fatal(err)
	}
	if current.FilePath != "compressed" || current.MailboxID != junk.ID || current.Size != 456 || !current.Archived || current.Subject != m.Subject || current.Flags != m.Flags {
		t.Fatalf("content move lost metadata: %+v", current)
	}
	jobs, err := f.s.DueSpamLearning(ctx)
	if err != nil || len(jobs) != 1 || jobs[0].Class != "spam" {
		t.Fatal("learning not queued atomically", err, jobs)
	}
	if _, err = f.s.MoveMessageContent(ctx, m, inbox.ID, "stale", 0, false); err != ErrMessageChanged {
		t.Fatal("stale content overwrote new path", err)
	}
	if _, err = f.s.MoveMessageContent(ctx, current, inbox.ID, "expanded", 1234, false); err != nil {
		t.Fatal(err)
	}
	jobs, err = f.s.DueSpamLearning(ctx)
	if err != nil || len(jobs) != 1 || jobs[0].Class != "ham" {
		t.Fatal("reverse learning not queued", err, jobs)
	}
}
