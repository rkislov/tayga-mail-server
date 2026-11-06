// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestEWSMailThreadHeadersAndSyncRepair(t *testing.T) {
	for _, tc := range []struct{ name, headers, references, parent string }{
		{"root", "", "", ""},
		{"reply", "References: <root@example.com>\r\nIn-Reply-To: <root@example.com>\r\n", "<root@example.com>", "<root@example.com>"},
		{"folded-chain", "References: <root@example.com>\r\n\t<parent@example.com>\r\nIn-Reply-To: <parent@example.com>\r\n", "<root@example.com> <parent@example.com>", "<parent@example.com>"},
		{"reply-without-references", "In-Reply-To: <root@example.com>\r\n", "", "<root@example.com>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, st, u, eas, _, _ := syncFixture(t)
			h := &ewsHandler{store: st, ms: eas.ms}
			inbox, err := st.EnsureMailbox(ctx, u.ID, "INBOX", "")
			if err != nil {
				t.Fatal(err)
			}
			path, size, err := h.ms.Deliver(u.Email, "INBOX", []byte("From: sender@example.com\r\nSubject: Re: Changed subject\r\nMessage-ID: <this&message@example.com>\r\n"+tc.headers+"\r\nBody"))
			if err != nil {
				t.Fatal(err)
			}
			msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: inbox.ID, FilePath: path, Size: size, InternalDate: time.Now()})
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
				requireXML(t, response)
				if extractTag(response, "InternetMessageId") != "<this&message@example.com>" {
					t.Fatal("RFC Message-ID lost", response)
				}
				if response == listing {
					continue
				}
				if got := strings.Join(strings.Fields(extractTag(response, "References")), " "); got != tc.references {
					t.Fatalf("references = %q, want %q", got, tc.references)
				}
				if tc.parent != "" && !strings.Contains(response, `HeaderName="In-Reply-To">`+xmlEscape(tc.parent)+`</t:InternetMessageHeader>`) {
					t.Fatal("reply parent lost", response)
				}
				if tc.parent == "" && strings.Contains(response, "InternetMessageHeader ") {
					t.Fatal("invented reply parent")
				}
			}
			doc, err := parseProtocolXML([]byte(listing))
			if err != nil {
				t.Fatal(err)
			}
			node := *doc.find("Items").Children[0]
			children := node.Children
			node.Children = nil
			for _, field := range children {
				switch field.Name.Local {
				case "InternetMessageId", "References", "InternetMessageHeaders":
				default:
					node.Children = append(node.Children, field)
				}
			}
			client, key := "ews:legacy-thread-test", "initial"
			if err := saveCollectionState(ctx, st, u.ID, client, inbox.ID, collectionState{Key: key, Items: map[string]string{msg.ID: digest(node.render())}}); err != nil {
				t.Fatal(err)
			}
			body := `<SyncFolderItems><SyncFolderId><FolderId Id="` + inbox.ID + `"/></SyncFolderId><SyncState>` + client + `/` + key + `</SyncState></SyncFolderItems>`
			result, err := h.syncEWSItems(ctx, u, body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result, "<t:Update>") || strings.Contains(result, "<t:Create>") {
				t.Fatal("legacy headers not updated in place", result)
			}
			next := extractTag(result, "SyncState")
			result, err = h.syncEWSItems(ctx, u, strings.ReplaceAll(body, client+"/"+key, next))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(result, "<t:Update>") {
				t.Fatal("unchanged thread emitted repeatedly", result)
			}
		})
	}
}
