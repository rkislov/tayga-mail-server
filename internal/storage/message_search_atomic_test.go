package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestSearchIndexConcurrentUpdatesAreUnique(t *testing.T) {
	f := newBookingFixture(t)
	ctx := t.Context()
	box, err := f.s.EnsureMailbox(ctx, f.user.ID, "INBOX", "mail/search-fixture")
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.s.InsertMessage(ctx, &Message{MailboxID: box.ID, FilePath: "search-fixture.eml", Subject: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.s.UpsertMessageSearch(ctx, m.ID, "needle", "", "", fmt.Sprint("document ", i))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	table := "message_fts"
	if f.s.dialect == DialectPostgres {
		table = "message_search"
	}
	var count int
	if err = f.s.DB().QueryRowContext(ctx, f.s.rebind(`SELECT count(*) FROM `+table+` WHERE message_id=?`), m.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate/missing search document", count, err)
	}
}
