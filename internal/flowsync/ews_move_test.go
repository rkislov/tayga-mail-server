package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestEWSMoveItemOwnershipAndBatch(t *testing.T) {
	ctx, s, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: s, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	inbox, _ := s.GetMailbox(ctx, u.ID, "INBOX")
	trash, _ := s.GetMailbox(ctx, u.ID, "Trash")
	m, err := s.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: "move-test", InternalDate: time.Now(), Subject: "Move", Flags: `\Seen \Flagged`})
	if err != nil {
		t.Fatal(err)
	}
	run := func(user *storage.User, dest, ids string) string {
		t.Helper()
		out, err := h.batchItems(ctx, user, "MoveItem", `<MoveItem><ToFolderId>`+dest+`</ToFolderId><ItemIds>`+ids+`</ItemIds></MoveItem>`, h.moveItem)
		if err != nil {
			t.Fatal(err)
		}
		requireXML(t, out)
		return out
	}
	item := `<ItemId Id="` + m.ID + `"/>`
	foreign := *u
	foreign.ID = "foreign"
	if out := run(&foreign, `<FolderId Id="`+trash.ID+`"/>`, item); !strings.Contains(out, "ErrorAccessDenied") {
		t.Fatal(out)
	}
	if out := run(u, `<FolderId Id="foreign"/>`, item); !strings.Contains(out, "ErrorAccessDenied") {
		t.Fatal(out)
	}
	out := run(u, `<DistinguishedFolderId Id="deleteditems"/>`, item+`<ItemId Id="missing"/>`)
	if !strings.Contains(out, "NoError") || !strings.Contains(out, "ErrorItemNotFound") {
		t.Fatal(out)
	}
	saved, _ := s.GetMessageByID(ctx, m.ID)
	if saved.MailboxID != trash.ID || saved.Flags != m.Flags || saved.FilePath != m.FilePath {
		t.Fatal(saved)
	}
	if out := run(u, `<FolderId Id="`+trash.ID+`"/>`, item); !strings.Contains(out, "NoError") {
		t.Fatal(out)
	}
}
