package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestEWSMailSyncIdOnlyAndReplay(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	mb, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("Subject: Correct shape\r\nMessage-ID: <shape@example.com>\r\n\r\nBody")
	path, size, err := eas.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: path, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	body := `<SyncFolderItems><ItemShape><BaseShape>IdOnly</BaseShape></ItemShape><SyncFolderId><FolderId Id="` + mb.ID + `"/></SyncFolderId></SyncFolderItems>`
	initial, err := h.syncEWSItems(ctx, u, body)
	if err != nil {
		t.Fatal(err)
	}
	check := func(response, operation string) {
		t.Helper()
		doc, err := parseProtocolXML([]byte(response))
		if err != nil {
			t.Fatal(err)
		}
		message := doc.find("Message")
		if doc.find(operation) == nil || message == nil || len(message.Children) != 1 || message.Children[0].Name.Local != "ItemId" {
			t.Fatal("IdOnly leaked partial metadata", response)
		}
	}
	check(initial, "Create")
	token := extractTag(initial, "SyncState")
	nextBody := strings.Replace(body, "</SyncFolderItems>", "<SyncState>"+token+"</SyncState></SyncFolderItems>", 1)
	if err := st.UpdateMessageFlags(ctx, m.ID, `\Seen`); err != nil {
		t.Fatal(err)
	}
	update, err := h.syncEWSItems(ctx, u, nextBody)
	if err != nil {
		t.Fatal(err)
	}
	check(update, "Update")
	replay, err := h.syncEWSItems(ctx, u, nextBody)
	if err != nil {
		t.Fatal(err)
	}
	if replay != update {
		t.Fatal("current response replay not byte-identical")
	}
	listing, err := h.findMailItems(ctx, u, mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	listDoc, err := parseProtocolXML([]byte(listing))
	if err != nil {
		t.Fatal(err)
	}
	item := listDoc.find("Message").render()
	start := strings.Index(initial, "<t:Create>")
	end := strings.Index(initial, "</t:Create>") + len("</t:Create>")
	legacy := initial[:start] + "<t:Create>" + item + "</t:Create>" + initial[end:]
	legacyBody := strings.Replace(nextBody, token, "ews:legacy-mail-shape/old", 1)
	err = saveCollectionState(ctx, st, u.ID, "ews:legacy-mail-shape", mb.ID, collectionState{Key: "new", PreviousKey: "old", RequestHash: digest(legacyBody), Response: legacy})
	if err != nil {
		t.Fatal(err)
	}
	repaired, err := h.syncEWSItems(ctx, u, legacyBody)
	if err != nil {
		t.Fatal(err)
	}
	check(repaired, "Create")
	// Explicit fields are still returned; other fields remain absent.
	shape, err := parseEWSMailShape(`<GetItem><ItemShape><BaseShape>IdOnly</BaseShape><AdditionalProperties><FieldURI FieldURI="item:Subject"/></AdditionalProperties></ItemShape></GetItem>`)
	if err != nil {
		t.Fatal(err)
	}
	shape.projectMessage(listDoc.find("Message"))
	if listDoc.find("Subject") == nil || listDoc.find("DateTimeSent") != nil {
		t.Fatal("additional property selection broken")
	}
}
