package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tayga/tms/internal/config"
	"github.com/tayga/tms/internal/storage"
)

func TestNoteCollaborativeACL(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	store, err := storage.Open(ctx, config.StorageConfig{
		Driver: "sqlite",
		SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "t.db")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ten, err := store.CreateTenant(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	dom, err := store.CreateDomain(ctx, ten.ID, "ex.com")
	if err != nil {
		t.Fatal(err)
	}
	alice, err := store.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID, LocalPart: "alice", Email: "alice@ex.com",
		DisplayName: "Alice", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID, LocalPart: "bob", Email: "bob@ex.com",
		DisplayName: "Bob", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	carol, err := store.CreateUser(ctx, &storage.User{
		TenantID: ten.ID, DomainID: dom.ID, LocalPart: "carol", Email: "carol@ex.com",
		DisplayName: "Carol", AuthSource: "local", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	folder, err := store.EnsureNoteFolder(ctx, alice.ID, "notes", "Notes")
	if err != nil {
		t.Fatal(err)
	}
	note, err := store.CreateNoteItem(ctx, &storage.NoteItem{
		FolderID: folder.ID, UserID: alice.ID, Title: "Shared note",
		DocumentJSON: `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`,
		BodyHTML:     "<p>hello</p>", BodyText: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SetNoteACL(ctx, note.ID, bob.ID, "write"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetNoteACL(ctx, note.ID, carol.ID, "read"); err != nil {
		t.Fatal(err)
	}

	if r, err := store.NoteRightsForUser(ctx, note.ID, bob.ID); err != nil || r != "write" {
		t.Fatalf("bob rights=%q err=%v", r, err)
	}
	if r, err := store.NoteRightsForUser(ctx, note.ID, carol.ID); err != nil || r != "read" {
		t.Fatalf("carol rights=%q err=%v", r, err)
	}

	shared, err := store.ListSharedNoteItems(ctx, bob.ID)
	if err != nil || len(shared) != 1 || shared[0].ID != note.ID {
		t.Fatalf("bob shared=%v err=%v", shared, err)
	}

	note.Title = "Updated by Bob"
	note.BodyText = "from bob"
	if err := store.UpdateNoteItem(ctx, note); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetNoteItemByID(ctx, note.ID)
	if err != nil || got.Title != "Updated by Bob" {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	if err := store.SetNoteFolderACL(ctx, folder.ID, bob.ID, "write"); err != nil {
		t.Fatal(err)
	}
	folders, err := store.ListNoteFoldersForUser(ctx, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range folders {
		if f.ID == folder.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected shared folder in bob list: %+v", folders)
	}

	inFolder, err := store.ListNoteItemsInFolder(ctx, folder.ID, false)
	if err != nil || len(inFolder) != 1 {
		t.Fatalf("inFolder=%v err=%v", inFolder, err)
	}
}
