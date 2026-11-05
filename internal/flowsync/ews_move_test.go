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

func TestEWSMoveAllLargeFolder(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	box, err := st.EnsureMailbox(ctx, u.ID, "siem", "")
	if err != nil {
		t.Fatal(err)
	}
	trash, _ := st.GetMailbox(ctx, u.ID, "Trash")
	var ids strings.Builder
	const count = 1808
	for i := 0; i < count; i++ {
		m, e := st.InsertMessage(ctx, &storage.Message{MailboxID: box.ID, FilePath: "retained-source", Flags: `\Seen \Flagged`, Size: 12})
		if e != nil {
			t.Fatal(e)
		}
		ids.WriteString(`<ItemId Id="` + m.ID + `"/>`)
	}
	body := `<MoveItem><ToFolderId><DistinguishedFolderId Id="deleteditems"/></ToFolderId><ItemIds>` + ids.String() + `</ItemIds></MoveItem>`
	out, err := h.batchItems(ctx, u, "MoveItem", body, h.moveItem)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseProtocolXML([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	responses := doc.find("ResponseMessages")
	if responses == nil || len(responses.Children) != count {
		t.Fatalf("missing per-item results: %s", out[:min(len(out), 400)])
	}
	for _, response := range responses.Children {
		if code := response.find("ResponseCode"); code == nil || code.Text != "NoError" {
			t.Fatalf("large move failed: %s", response.render())
		}
	}
	remaining, err := st.ListMessages(ctx, box.ID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("source still contains %d: %v", len(remaining), err)
	}
	moved, err := st.ListMessages(ctx, trash.ID)
	if err != nil || len(moved) != count {
		t.Fatalf("trash contains %d: %v", len(moved), err)
	}
	for _, m := range moved {
		if m.FilePath != "retained-source" || m.Flags != `\Seen \Flagged` {
			t.Fatal("move lost file or flags")
		}
	}
}
