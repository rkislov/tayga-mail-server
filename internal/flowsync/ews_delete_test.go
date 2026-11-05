package flowsync

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestEWSOutlookLargeDeleteBatch(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	mb, err := st.EnsureMailbox(ctx, u.ID, "siem", "")
	if err != nil {
		t.Fatal(err)
	}
	var ids strings.Builder
	for i := 0; i < 6253; i++ {
		msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: fmt.Sprintf("missing-%d.eml", i), Subject: "SIEM", InternalDate: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		if err = st.UpsertMessageSearch(ctx, msg.ID, "SIEM", "", "", "delete this"); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&ids, `<t:ItemId Id="%s"/>`, msg.ID)
	}
	other, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "other", Email: "other@example.com", AuthSource: "local", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := st.EnsureMailbox(ctx, other.ID, "siem", "")
	if err != nil {
		t.Fatal(err)
	}
	foreignMsg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: foreign.ID, FilePath: "foreign.eml", InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&ids, `<t:ItemId Id="%s"/><t:ItemId Id="missing"/>`, foreignMsg.ID)
	started := time.Now()
	out, err := h.deleteEWSItems(ctx, u, `<m:DeleteItem xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages" xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types" DeleteType="HardDelete"><m:ItemIds>`+ids.String()+`</m:ItemIds></m:DeleteItem>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "<m:ResponseCode>NoError</m:ResponseCode>") != 6253 || !strings.Contains(out, "ErrorAccessDenied") || !strings.Contains(out, "ErrorItemNotFound") {
		t.Fatalf("unexpected batch results: %s", out[:min(len(out), 500)])
	}
	if _, err = st.GetMessageByID(ctx, foreignMsg.ID); err != nil {
		t.Fatalf("foreign message changed: %v", err)
	}
	remaining, err := st.ListMessages(ctx, mb.ID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("siem not empty: %v %d", err, len(remaining))
	}
	sqlStore := st.(*storage.Store)
	var count int
	if err = sqlStore.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM message_fts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("search index not cleared: %d %v", count, err)
	}
	t.Logf("6253 messages and per-item errors processed in %s", time.Since(started))
}

func TestEWSMessageDisposalMovesAndRollback(t *testing.T) {
	for _, mode := range []string{"MoveToDeletedItems", "SoftDelete"} {
		t.Run(mode, func(t *testing.T) {
			ctx, st, u, eas, _, _ := syncFixture(t)
			h := &ewsHandler{store: st, ms: eas.ms}
			mb, err := st.EnsureMailbox(ctx, u.ID, "siem", "")
			if err != nil {
				t.Fatal(err)
			}
			path, _, err := eas.ms.Deliver(u.Email, "siem", []byte("Subject: retain\r\n\r\nbody"))
			if err != nil {
				t.Fatal(err)
			}
			msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Flags: `\Seen \Flagged`, InternalDate: time.Now()})
			if err != nil {
				t.Fatal(err)
			}
			out, err := h.deleteEWSItems(ctx, u, fmt.Sprintf(`<DeleteItem DeleteType="%s"><ItemIds><ItemId Id="%s"/></ItemIds></DeleteItem>`, mode, msg.ID))
			if err != nil || !strings.Contains(out, "NoError") {
				t.Fatalf("%v %s", err, out)
			}
			saved, err := st.GetMessageByID(ctx, msg.ID)
			if err != nil {
				t.Fatal(err)
			}
			name := "Trash"
			if mode == "SoftDelete" {
				name = ewsRecoveryFolder
			}
			dst, err := st.GetMailbox(ctx, u.ID, name)
			if err != nil {
				t.Fatal(err)
			}
			if saved.MailboxID != dst.ID || saved.Flags != msg.Flags || saved.FilePath != path {
				t.Fatal("move lost message metadata")
			}
			if _, err = eas.ms.Read(saved.FilePath); err != nil {
				t.Fatal("move lost body", err)
			}
		})
	}
	t.Run("rollback", func(t *testing.T) {
		ctx, st, u, _, _, _ := syncFixture(t)
		mb, err := st.EnsureMailbox(ctx, u.ID, "siem", "")
		if err != nil {
			t.Fatal(err)
		}
		msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, InternalDate: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		if err = st.UpsertMessageSearch(ctx, msg.ID, "keep", "", "", "body"); err != nil {
			t.Fatal(err)
		}
		db := st.(*storage.Store).DB()
		if _, err = db.ExecContext(ctx, `CREATE TRIGGER reject_disposal BEFORE DELETE ON messages BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
			t.Fatal(err)
		}
		if _, err = st.DisposeMessages(ctx, u.ID, []string{msg.ID}, ""); err == nil {
			t.Fatal("failure not propagated")
		}
		if _, err = st.GetMessageByID(ctx, msg.ID); err != nil {
			t.Fatal("rollback lost message")
		}
		var count int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_fts`).Scan(&count); err != nil || count != 1 {
			t.Fatal("rollback lost search index", count, err)
		}
	})
}

func TestEWSOutlookMoveDeleteRecreateFolder(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	call := func(op, body string) string {
		t.Helper()
		out, err := h.writeEWSFolders(ctx, u, op, body)
		if err != nil || strings.Contains(out, `ResponseClass="Error"`) || !strings.Contains(out, "NoError") {
			t.Fatalf("%s: %v %s", op, err, out)
		}
		return out
	}
	create := func() string {
		return extractAttr(call("CreateFolder", `<CreateFolder><ParentFolderId><DistinguishedFolderId Id="msgfolderroot"/></ParentFolderId><Folders><Folder><DisplayName>siem</DisplayName></Folder></Folders></CreateFolder>`), "FolderId", "Id")
	}
	id := create()
	call("MoveFolder", fmt.Sprintf(`<MoveFolder><ToFolderId><DistinguishedFolderId Id="deleteditems"/></ToFolderId><FolderIds><FolderId Id="%s"/></FolderIds></MoveFolder>`, id))
	trash, err := st.GetMailbox(ctx, u.ID, "Trash/siem")
	if err != nil || trash.ID != id {
		t.Fatal("folder not moved", err)
	}
	newID := create()
	if id == newID {
		t.Fatal("recreate reused old identity")
	}
	call("DeleteFolder", fmt.Sprintf(`<DeleteFolder DeleteType="SoftDelete"><FolderIds><FolderId Id="%s"/></FolderIds></DeleteFolder>`, newID))
	if _, err = st.GetMailbox(ctx, u.ID, "siem"); err != storage.ErrNotFound {
		t.Fatal("soft delete retained source name", err)
	}
	recovered, err := st.GetMailbox(ctx, u.ID, ewsRecoveryFolder+"/siem-"+newID)
	if err != nil || recovered.ID != newID {
		t.Fatal("soft delete not recoverable", err)
	}
	create()
}
