package flowsync

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestEWSMailSentDateAndLegacySyncRepair(t *testing.T) {
	for _, test := range []struct{ name, date, want string }{
		{"timezone", "Mon, 19 Oct 2026 12:34:56 +0300", "2026-10-19T09:34:56Z"},
		{"missing", "", "2026-10-26T10:00:00Z"},
		{"invalid", "not a date", "2026-10-26T10:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, st, u, eas, _, _ := syncFixture(t)
			h := &ewsHandler{store: st, ms: eas.ms}
			inbox, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
			if err != nil {
				t.Fatal(err)
			}
			header := ""
			if test.date != "" {
				header = "Date: " + test.date + "\r\n"
			}
			path, size, err := h.ms.Deliver(u.Email, "INBOX", []byte("From: sender@example.com\r\nSubject: Original date\r\n"+header+"\r\nBody"))
			if err != nil {
				t.Fatal(err)
			}
			received := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
			msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size, InternalDate: received})
			if err != nil {
				t.Fatal(err)
			}
			listing, err := h.findMailItems(ctx, u, inbox.ID)
			if err != nil {
				t.Fatal(err)
			}
			item, err := h.getMailItem(ctx, u, msg, "<GetItem/>")
			if err != nil {
				t.Fatal(err)
			}
			for _, response := range []string{listing, item} {
				if !strings.Contains(response, "<t:DateTimeSent>"+test.want+"</t:DateTimeSent>") || !strings.Contains(response, "<t:DateTimeReceived>2026-10-26T10:00:00Z</t:DateTimeReceived>") {
					t.Fatalf("sent and received dates lost: %s", response)
				}
			}
			doc, err := parseProtocolXML([]byte(listing))
			if err != nil {
				t.Fatal(err)
			}
			original := doc.find("Items").Children[0]
			legacyNode := *original
			legacyNode.Children = nil
			for _, field := range original.Children {
				if field.Name.Local != "DateTimeSent" {
					legacyNode.Children = append(legacyNode.Children, field)
				}
			}
			legacy := legacyNode.render()

			client, key := "ews:legacy-date-test", "initial"
			err = saveCollectionState(ctx, st, u.ID, client, inbox.ID, collectionState{Key: key, Items: map[string]string{msg.ID: digest(legacy)}})
			if err != nil {
				t.Fatal(err)
			}
			body := `<SyncFolderItems><SyncFolderId><FolderId Id="` + inbox.ID + `"/></SyncFolderId><SyncState>` + client + `/` + key + `</SyncState></SyncFolderItems>`
			result, err := h.syncEWSItems(ctx, u, body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result, "<t:Update>") || strings.Contains(result, "<t:Create>") || !strings.Contains(result, msg.ID) {
				t.Fatalf("legacy metadata not updated in place: %s", result)
			}
			next := extractTag(result, "SyncState")
			result, err = h.syncEWSItems(ctx, u, strings.ReplaceAll(body, client+"/"+key, next))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(result, "<t:Update>") {
				t.Fatal("unchanged date repeatedly emitted")
			}
		})
	}
}
