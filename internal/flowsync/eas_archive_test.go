// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestArchiveWBXMLSyncAndFetch(t *testing.T) {
	for _, version := range []string{"14.1", "16.0", "16.1"} {
		t.Run(version, func(t *testing.T) {
			ctx, st, user, handler, _, _ := syncFixture(t)
			archive, err := st.EnsureMailbox(ctx, user.ID, "Archive", "")
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("From: bob@example.com\r\nTo: alice@example.com\r\nSubject: Archived\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>Архивное письмо</p>")
			path, size, err := handler.ms.Deliver(user.Email, "Archive", raw)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(path, ".gz") {
				t.Fatal("archive fixture is not compressed")
			}
			msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: archive.ID, FilePath: path, Size: size, InternalDate: time.Now(), Subject: "Archived", FromAddr: "bob@example.com", Archived: true})
			if err != nil {
				t.Fatal(err)
			}
			call := func(key, extra string) *protocolNode {
				t.Helper()
				wire, err := encodeWire(`<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + archive.ID + `</CollectionId>` + extra + `</Collection></Collections></Sync>`)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest("POST", "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId=archive-phone&DeviceType=iPhone", bytes.NewReader(wire)).WithContext(withUser(ctx, user))
				req.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
				req.Header.Set("MS-ASProtocolVersion", version)
				res := httptest.NewRecorder()
				handler.ServeHTTP(res, req)
				if res.Code != 200 {
					t.Fatalf("HTTP %d: %s", res.Code, res.Body.String())
				}
				xml, err := decodeWire(res.Body.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				doc, err := parseProtocolXML(xml)
				if err != nil {
					t.Fatal(err)
				}
				collection := doc.find("Collection")
				if collection == nil || collection.value("Status") != "1" {
					t.Fatalf("unsuccessful Sync: %s", xml)
				}
				return collection
			}
			initial := call("0", `<Options><BodyPreference xmlns="AirSyncBase:"><Type>2</Type></BodyPreference></Options>`)
			page := call(initial.value("SyncKey"), "")
			if page.find("Add") == nil || page.find("Add").value("ServerId") != msg.ID || page.find("Body").value("Data") != "<p>Архивное письмо</p>" {
				t.Fatal(page.render())
			}
			if replay := call(initial.value("SyncKey"), ""); replay.render() != page.render() {
				t.Fatal("retry changed the archive page")
			}
			fetch := call(page.value("SyncKey"), `<GetChanges>0</GetChanges><Commands><Fetch><ServerId>`+msg.ID+`</ServerId></Fetch></Commands>`)
			if fetch.find("Fetch") == nil || fetch.find("Fetch").value("Status") != "1" || fetch.find("Body").value("Data") != "<p>Архивное письмо</p>" {
				t.Fatal(fetch.render())
			}
			if idle := call(page.value("SyncKey"), ""); idle.child("Commands") != nil {
				t.Fatal("unchanged archive message repeated", idle.render())
			}
		})
	}
}
