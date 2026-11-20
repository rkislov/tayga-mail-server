package flowsync

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestEWSMailFolderLifecycle(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	run := func(body string) string {
		t.Helper()
		req := httptest.NewRequest("POST", "/EWS/Exchange.asmx", strings.NewReader(soapEnvelope(body)))
		req = req.WithContext(context.WithValue(ctx, userKey, u))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		out := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(out, "<m:ResponseCode>NoError</m:ResponseCode>") || strings.Contains(out, `ResponseClass="Error"`) {
			t.Fatalf("%s: %s", body, out)
		}
		if _, err := parseProtocolXML([]byte(out)); err != nil {
			t.Fatal(err)
		}
		return out
	}
	create := func(name string) string {
		return run(`<m:CreateFolder><m:ParentFolderId><t:DistinguishedFolderId Id="msgfolderroot"/></m:ParentFolderId><m:Folders><t:Folder><t:DisplayName>` + name + `</t:DisplayName></t:Folder></m:Folders></m:CreateFolder>`)
	}
	id := extractAttr(create("siem"), "FolderId", "Id")
	if id == "" {
		t.Fatal("no folder id")
	}
	path, _, err := eas.ms.Deliver(u.Email, "siem", []byte("Subject: SIEM\r\n\r\nmessage"))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: id, FilePath: path, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	run(fmt.Sprintf(`<m:UpdateFolder><m:FolderChanges><t:FolderChange><t:FolderId Id="%s"/><t:Updates><t:SetFolderField><t:FieldURI FieldURI="folder:DisplayName"/><t:Folder><t:DisplayName>siem-renamed</t:DisplayName></t:Folder></t:SetFolderField></t:Updates></t:FolderChange></m:FolderChanges></m:UpdateFolder>`, id))
	mb, err := st.GetMailbox(ctx, u.ID, "siem-renamed")
	if err != nil || mb.ID != id {
		t.Fatalf("rename lost identity: %v", err)
	}
	saved, err := st.GetMessageByID(ctx, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = eas.ms.Read(saved.FilePath); err != nil {
		t.Fatalf("rename lost message: %v", err)
	}
	run(fmt.Sprintf(`<m:EmptyFolder DeleteType="HardDelete" DeleteSubFolders="false"><m:FolderIds><t:FolderId Id="%s"/></m:FolderIds></m:EmptyFolder>`, id))
	messages, err := st.ListMessages(ctx, id)
	if err != nil || len(messages) != 0 {
		t.Fatalf("folder not empty: %v", err)
	}
	if _, err = eas.ms.Read(saved.FilePath); err == nil {
		t.Fatal("message file survived hard delete")
	}
	run(fmt.Sprintf(`<m:DeleteFolder DeleteType="HardDelete"><m:FolderIds><t:FolderId Id="%s"/></m:FolderIds></m:DeleteFolder>`, id))
	if _, err = st.GetMailbox(ctx, u.ID, "siem-renamed"); err != storage.ErrNotFound {
		t.Fatalf("folder survived: %v", err)
	}
	newID := extractAttr(create("siem"), "FolderId", "Id")
	if newID == id || newID == "" {
		t.Fatal("recreated folder has old id")
	}
}

func TestEWSFolderWriteRejectsInvalidAndForeignRequests(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	mb, err := st.EnsureMailbox(ctx, u.ID, "siem", "")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ op, body, code string }{
		{"UpdateFolder", fmt.Sprintf(`<UpdateFolder><FolderChanges><FolderChange><FolderId Id="%s"/><Updates><SetFolderField><FieldURI FieldURI="folder:DisplayName"/><Folder><DisplayName>../escape</DisplayName></Folder></SetFolderField></Updates></FolderChange></FolderChanges></UpdateFolder>`, mb.ID), "ErrorInvalidPropertySet"},
		{"DeleteFolder", `<DeleteFolder DeleteType="HardDelete"><FolderIds><DistinguishedFolderId Id="inbox"/></FolderIds></DeleteFolder>`, "ErrorDeleteDistinguishedFolder"},
		{"DeleteFolder", `<DeleteFolder DeleteType="HardDelete"><FolderIds><FolderId Id="foreign-id"/></FolderIds></DeleteFolder>`, "ErrorFolderNotFound"},
		{"CreateFolder", `<CreateFolder><ParentFolderId><DistinguishedFolderId Id="msgfolderroot"><Mailbox><EmailAddress>other@example.com</EmailAddress></Mailbox></DistinguishedFolderId></ParentFolderId><Folders><Folder><DisplayName>siem</DisplayName></Folder></Folders></CreateFolder>`, "ErrorAccessDenied"},
		{"CreateFolder", `<CreateFolder><ParentFolderId><DistinguishedFolderId Id="msgfolderroot"/></ParentFolderId><Folders><Folder><DisplayName>siem</DisplayName></Folder></Folders></CreateFolder>`, "ErrorFolderExists"},
	}
	for _, tt := range cases {
		t.Run(tt.op+tt.code, func(t *testing.T) {
			out, err := h.writeEWSFolders(ctx, u, tt.op, tt.body)
			if err != nil || !strings.Contains(out, "<m:ResponseCode>"+tt.code+"</m:ResponseCode>") {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}

func TestEWSFolderTreeAndMoveToTrash(t *testing.T) {
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
	parent := extractAttr(call("CreateFolder", `<CreateFolder><ParentFolderId><DistinguishedFolderId Id="msgfolderroot"/></ParentFolderId><Folders><Folder><DisplayName>siem</DisplayName></Folder></Folders></CreateFolder>`), "FolderId", "Id")
	child := extractAttr(call("CreateFolder", fmt.Sprintf(`<CreateFolder><ParentFolderId><FolderId Id="%s"/></ParentFolderId><Folders><Folder><DisplayName>child</DisplayName></Folder></Folders></CreateFolder>`, parent)), "FolderId", "Id")
	path, _, err := eas.ms.Deliver(u.Email, "siem/child", []byte("Subject: keep\r\n\r\ndata"))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: child, FilePath: path, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	call("DeleteFolder", fmt.Sprintf(`<DeleteFolder DeleteType="MoveToDeletedItems"><FolderIds><FolderId Id="%s"/></FolderIds></DeleteFolder>`, parent))
	mb, err := st.GetMailbox(ctx, u.ID, "Trash/siem/child")
	if err != nil || mb.ID != child {
		t.Fatalf("child not moved: %v", err)
	}
	saved, err := st.GetMessageByID(ctx, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = eas.ms.Read(saved.FilePath); err != nil {
		t.Fatalf("lost moved child contents: %v", err)
	}
	call("EmptyFolder", fmt.Sprintf(`<EmptyFolder DeleteType="MoveToDeletedItems" DeleteSubFolders="true"><FolderIds><FolderId Id="%s"/></FolderIds></EmptyFolder>`, parent))
	if _, err = st.GetMailbox(ctx, u.ID, "Trash/siem/child"); err != storage.ErrNotFound {
		t.Fatalf("child survived: %v", err)
	}
	trash, err := st.GetMailbox(ctx, u.ID, "Trash")
	if err != nil {
		t.Fatal(err)
	}
	saved, err = st.GetMessageByID(ctx, msg.ID)
	if err != nil || saved.MailboxID != trash.ID {
		t.Fatalf("message not moved to Trash: %v", err)
	}
	if _, err = eas.ms.Read(saved.FilePath); err != nil {
		t.Fatalf("lost message after removing child: %v", err)
	}
}
