package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"testing"
)

func TestEWSHierarchyIgnoresMessageCountsAndStillDiscoversNewFolder(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	sync := func(key string) (string, *protocolNode) {
		t.Helper()
		body := `<SyncFolderHierarchy>`
		if key != "" {
			body += `<SyncState>` + key + `</SyncState>`
		}
		out, err := h.syncEWSHierarchy(ctx, u, body+`</SyncFolderHierarchy>`)
		if err != nil {
			t.Fatal(err)
		}
		d, err := parseProtocolXML([]byte(out))
		if err != nil {
			t.Fatal(err)
		}
		return extractTag(out, "SyncState"), d.find("Changes")
	}
	key, _ := sync("")
	inbox, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: "fixture", Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	key, changes := sync(key)
	if len(changes.Children) != 0 {
		t.Fatalf("mail arrival changed hierarchy: %s", changes.render())
	}
	if err = st.UpdateMessageFlags(ctx, m.ID, `\Seen`); err != nil {
		t.Fatal(err)
	}
	key, changes = sync(key)
	if len(changes.Children) != 0 {
		t.Fatalf("read flag changed hierarchy: %s", changes.render())
	}
	siem, err := st.EnsureMailbox(ctx, u.ID, "siem", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	_, changes = sync(key)
	if len(changes.Children) != 1 || changes.Children[0].Name.Local != "Create" || extractAttr(changes.render(), "FolderId", "Id") != siem.ID {
		t.Fatalf("new siem folder was not discovered alone: %s", changes.render())
	}
}
