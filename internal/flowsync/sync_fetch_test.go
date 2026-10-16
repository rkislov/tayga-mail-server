// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tayga/tms/internal/storage"
)

func TestAppleMailSyncFetchFullBodyAndReplay(t *testing.T) {
	ctx, st, u, h, dev, _ := syncFixture(t)
	if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	mb, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("Полный текст письма. ", 3000)
	raw := []byte("From: bob@example.com\r\nSubject: Fetch test\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + text)
	rel, size, err := h.ms.Deliver(u.Email, "INBOX", raw)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := st.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, FilePath: rel, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	request := func(key, extra string) string {
		return `<Sync xmlns="AirSync:"><Collections><Collection><SyncKey>` + key + `</SyncKey><CollectionId>` + mb.ID + `</CollectionId>` + extra + `</Collection></Collections></Sync>`
	}
	call := func(body string) (*protocolNode, string) {
		t.Helper()
		wire, err := encodeWire(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/Microsoft-Server-ActiveSync?Cmd=Sync&DeviceId="+dev.DeviceID, bytes.NewReader(wire))
		r.Header.Set("Content-Type", "application/vnd.ms-sync.wbxml")
		r.Header.Set("MS-ASProtocolVersion", "16.1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r.WithContext(withUser(ctx, u)))
		if w.Code != 200 {
			t.Fatalf("HTTP %d", w.Code)
		}
		decoded, err := decodeWire(w.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parseProtocolXML(decoded)
		if err != nil {
			t.Fatal(err)
		}
		return doc, string(decoded)
	}
	initial, _ := call(request("0", `<Options><BodyPreference xmlns="AirSyncBase:"><Type>1</Type><TruncationSize>500</TruncationSize></BodyPreference></Options>`))
	page, _ := call(request(initial.find("Collection").value("SyncKey"), ""))
	if page.find("Body").value("Truncated") != "1" {
		t.Fatal("fixture must start truncated")
	}
	key := page.find("Collection").value("SyncKey")
	extra := `<GetChanges>0</GetChanges><Options><BodyPreference xmlns="AirSyncBase:"><Type>1</Type></BodyPreference></Options><Commands><Fetch><ServerId>` + msg.ID + `</ServerId></Fetch></Commands>`
	fetchRequest := request(key, extra)
	fetched, first := call(fetchRequest)
	response := fetched.find("Responses")
	if response == nil || response.child("Fetch") == nil {
		t.Fatal("missing Responses/Fetch")
	}
	fetch := response.child("Fetch")
	if fetch.value("Status") != "1" || fetch.value("ServerId") != msg.ID || fetch.find("Body").value("Data") != text || fetch.find("Body").value("Truncated") != "0" {
		t.Fatal("Fetch did not return full body")
	}
	if fetched.find("Commands") != nil {
		t.Fatal("GetChanges=0 returned unrelated changes")
	}
	_, retry := call(fetchRequest)
	if first != retry {
		t.Fatal("retry changed the response or key")
	}
	// Reproduce a pre-fix response cached on production, then retry the same key.
	stored, err := loadCollectionState(ctx, st, u.ID, dev.ID, mb.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored.Response = `<Collection xmlns="AirSync:"><SyncKey>` + stored.Key + `</SyncKey><CollectionId>` + mb.ID + `</CollectionId><Status>1</Status></Collection>`
	if err := saveCollectionState(ctx, st, u.ID, dev.ID, mb.ID, stored); err != nil {
		t.Fatal(err)
	}
	repaired, _ := call(fetchRequest)
	if repaired.find("Fetch") == nil || repaired.find("Body").value("Data") != text || repaired.find("Collection").value("SyncKey") != stored.Key {
		t.Fatal("upgrade replay did not repair the old empty Fetch response")
	}
	// Fetch must not expose items from a different mailbox or another account.
	other, err := st.CreateUser(ctx, &storage.User{TenantID: u.TenantID, DomainID: u.DomainID, LocalPart: "other", Email: "other@example.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&ewsHandler{store: st, ms: h.ms}).ensureStandardMailFolders(ctx, other); err != nil {
		t.Fatal(err)
	}
	otherBox, err := st.GetMailbox(ctx, other.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := st.InsertMessage(ctx, &storage.Message{MailboxID: otherBox.ID, FilePath: rel, Size: size, InternalDate: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{storage.NewID(), foreign.ID} {
		denied, _ := call(request(stored.Key, `<GetChanges>0</GetChanges><Commands><Fetch><ServerId>`+id+`</ServerId></Fetch></Commands>`))
		fetch := denied.find("Fetch")
		if fetch == nil || fetch.value("Status") != "8" || fetch.find("ApplicationData") != nil {
			t.Fatal("missing/foreign item leaked data or returned success")
		}
		stored.Key = denied.find("Collection").value("SyncKey")
	}
	bad, _ := call(request("invalid-key", extra))
	if bad.find("Collection").value("Status") != "3" || bad.find("ApplicationData") != nil {
		t.Fatal("invalid key allowed Fetch")
	}
}

func TestBodyPreferenceWithoutTruncationReturnsEntireBody(t *testing.T) {
	text := strings.Repeat("x", (1<<20)+10)
	got, _, _, truncated := preferredBody(messageContent{Text: text}, syncPreferences{Bodies: []bodyPreference{{Type: "1"}}})
	if truncated || got != text {
		t.Fatal("explicit preference without TruncationSize must not truncate")
	}
}
