package storage

import (
	"context"
	"errors"
	"testing"
)

func TestNoteRevisionHistoryAtomicityAndPersistence(t *testing.T) {
	f := newBookingFixture(t)
	ctx := context.Background()
	folder, err := f.s.EnsureNoteFolder(ctx, f.user.ID, "notes", "Notes")
	if err != nil {
		t.Fatal(err)
	}
	n, err := f.s.CreateNoteItem(ctx, &NoteItem{FolderID: folder.ID, UserID: f.user.ID, Title: "Original", DocumentJSON: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`, BodyText: "Original", BodyHTML: "<p>Original</p>"})
	if err != nil {
		t.Fatal(err)
	}
	old := *n
	n.Title = "Updated"
	if err := f.s.UpdateNoteItem(ctx, n); err != nil {
		t.Fatal(err)
	}
	stale := old
	stale.Title = "Stale"
	if err := f.s.UpdateNoteItem(ctx, &stale); !errors.Is(err, ErrNoteConflict) {
		t.Fatal(err)
	}
	if err := f.s.DeleteNoteItemIfMatch(ctx, n.ID, old.ETag); !errors.Is(err, ErrNoteConflict) {
		t.Fatal("stale delete", err)
	}
	history, err := f.s.ListNoteVersions(ctx, n.ID)
	if err != nil || len(history) != 1 || history[0].Note.Title != "Original" || history[0].Note.ETag != old.ETag {
		t.Fatal(history, err)
	}
	// Reopening a connection must preserve both versions and allow migrations to replay.
	reopened, err := Open(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	history, err = reopened.ListNoteVersions(ctx, n.ID)
	if err != nil || len(history) != 1 {
		t.Fatal(history, err)
	}
	if f.s.dialect != DialectSQLite {
		return
	}
	if _, err := f.s.db.Exec(`CREATE TRIGGER reject_note_history BEFORE INSERT ON note_versions BEGIN SELECT RAISE(ABORT,'history failure'); END`); err != nil {
		t.Fatal(err)
	}
	key := n.ETag
	n.Title = "Must roll back"
	if err := f.s.UpdateNoteItem(ctx, n); err == nil {
		t.Fatal("history failure ignored")
	}
	got, err := f.s.GetNoteItemByID(ctx, n.ID)
	if err != nil || got.Title != "Updated" || got.ETag != key || n.ETag != key {
		t.Fatal(got, err)
	}
	history, err = f.s.ListNoteVersions(ctx, n.ID)
	if err != nil || len(history) != 1 {
		t.Fatal(history, err)
	}
}
